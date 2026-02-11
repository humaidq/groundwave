/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/flamego/flamego"

	"github.com/humaidq/groundwave/db"
)

const extensionTokenHeader = "X-Groundwave-Token" //nolint:gosec // HTTP header name, not a credential.

type extensionTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]struct{}
}

func newExtensionTokenStore() *extensionTokenStore {
	return &extensionTokenStore{
		tokens: make(map[string]struct{}),
	}
}

func (s *extensionTokenStore) Add(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[token] = struct{}{}
}

func (s *extensionTokenStore) Has(token string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.tokens[token]

	return ok
}

var extTokens = newExtensionTokenStore()

// ExtensionAuth handles the extension authentication flow.
func ExtensionAuth(c flamego.Context) {
	token, err := generateExtensionToken()
	if err != nil {
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)

		if _, writeErr := c.ResponseWriter().Write([]byte("failed to generate token")); writeErr != nil {
			logger.Error("Error writing extension auth response", "error", writeErr)
		}

		return
	}

	extTokens.Add(token)
	redirectURL := "/ext/complete?token=" + url.QueryEscape(token)
	c.Redirect(redirectURL, http.StatusSeeOther)
}

// ExtensionComplete confirms authentication completion.
func ExtensionComplete(c flamego.Context) {
	c.ResponseWriter().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.ResponseWriter().WriteHeader(http.StatusOK)

	if _, err := c.ResponseWriter().Write([]byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Groundwave Connector</title>
  <style>
    body { font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f3f2ef; margin: 0; }
    .card { max-width: 520px; margin: 12vh auto; background: #fff; border-radius: 10px; padding: 28px 32px; box-shadow: 0 2px 8px rgba(0,0,0,0.08); }
    h1 { font-size: 20px; margin: 0 0 12px; }
    p { margin: 0; color: #555; }
  </style>
</head>
<body>
  <div class="card">
    <h1>Groundwave Connector authenticated</h1>
    <p>You can close this tab and return to LinkedIn.</p>
  </div>
  <script>
    const url = new URL(window.location.href);
    if (url.searchParams.has("token")) {
      url.searchParams.delete("token");
      url.searchParams.set("status", "ok");
      window.history.replaceState({}, "", url.toString());
    }
  </script>
</body>
</html>`)); err != nil {
		logger.Error("Error writing extension completion HTML", "error", err)
	}
}

// ExtensionValidate checks whether the provided token is valid.
func ExtensionValidate(c flamego.Context) {
	addExtensionCORSHeaders(c)

	if c.Request().Method == http.MethodOptions {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
		return
	}

	if !hasValidExtensionToken(c) {
		writeExtensionAuthError(c)
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]bool{"valid": true}); err != nil {
		logger.Error("Error encoding extension validation", "error", err)
	}
}

// ExtensionLinkedInLookup checks if LinkedIn URLs exist in contacts.
func ExtensionLinkedInLookup(c flamego.Context) {
	addExtensionCORSHeaders(c)

	if c.Request().Method == http.MethodOptions {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
		return
	}

	if !hasValidExtensionToken(c) {
		writeExtensionAuthError(c)
		return
	}

	var request struct {
		URLs []string `json:"urls"`
	}

	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&request); err != nil {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "invalid request body"}); err != nil {
			logger.Error("Error encoding extension lookup error", "error", err)
		}

		return
	}

	normalizedLookup := make(map[string]struct{})

	for _, rawURL := range request.URLs {
		normalized, ok := db.NormalizeLinkedInURL(rawURL)
		if !ok {
			continue
		}

		normalizedLookup[normalized] = struct{}{}
	}

	storedURLs, err := db.ListLinkedInURLs(c.Request().Context())
	if err != nil {
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to fetch LinkedIn URLs"}); err != nil {
			logger.Error("Error encoding extension lookup error", "error", err)
		}

		return
	}

	storedNormalized := make(map[string]string)

	for _, entry := range storedURLs {
		normalized, ok := db.NormalizeLinkedInURL(entry.URL)
		if !ok {
			continue
		}

		if _, exists := storedNormalized[normalized]; exists {
			continue
		}

		storedNormalized[normalized] = entry.ContactID
	}

	matches := make(map[string]bool)
	contacts := make(map[string]string)

	for normalized := range normalizedLookup {
		contactID, found := storedNormalized[normalized]

		matches[normalized] = found
		if found {
			contacts[normalized] = contactID
		}
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(c.ResponseWriter()).Encode(struct {
		Matches  map[string]bool   `json:"matches"`
		Contacts map[string]string `json:"contacts"`
	}{
		Matches:  matches,
		Contacts: contacts,
	}); err != nil {
		logger.Error("Error encoding extension lookup response", "error", err)
	}
}

type extensionContactSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ExtensionContactsWithoutLinkedIn lists contacts missing LinkedIn URLs.
func ExtensionContactsWithoutLinkedIn(c flamego.Context) {
	addExtensionCORSHeaders(c)

	if c.Request().Method == http.MethodOptions {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
		return
	}

	if !hasValidExtensionToken(c) {
		writeExtensionAuthError(c)
		return
	}

	contacts, err := db.ListContactsWithoutLinkedIn(c.Request().Context())
	if err != nil {
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to fetch contacts"}); err != nil {
			logger.Error("Error encoding extension contacts error", "error", err)
		}

		return
	}

	response := make([]extensionContactSummary, 0, len(contacts))
	for _, contact := range contacts {
		response = append(response, extensionContactSummary{
			ID:   contact.ID,
			Name: contact.NameDisplay,
		})
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(c.ResponseWriter()).Encode(struct {
		Contacts []extensionContactSummary `json:"contacts"`
	}{
		Contacts: response,
	}); err != nil {
		logger.Error("Error encoding extension contacts response", "error", err)
	}
}

// ExtensionLinkedInAssign links a LinkedIn URL to a contact.
func ExtensionLinkedInAssign(c flamego.Context) {
	addExtensionCORSHeaders(c)

	if c.Request().Method == http.MethodOptions {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
		return
	}

	if !hasValidExtensionToken(c) {
		writeExtensionAuthError(c)
		return
	}

	var request struct {
		ContactID string `json:"contactId"`
		URL       string `json:"url"`
	}

	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&request); err != nil {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "invalid request body"}); err != nil {
			logger.Error("Error encoding extension assign error", "error", err)
		}

		return
	}

	if strings.TrimSpace(request.ContactID) == "" {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "missing contact ID"}); err != nil {
			logger.Error("Error encoding extension assign error", "error", err)
		}

		return
	}

	normalized, ok := db.NormalizeLinkedInURL(request.URL)
	if !ok {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)

		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "invalid LinkedIn URL"}); err != nil {
			logger.Error("Error encoding extension assign error", "error", err)
		}

		return
	}

	if err := db.AddURL(c.Request().Context(), db.AddURLInput{
		ContactID: request.ContactID,
		URL:       normalized,
		URLType:   db.URLLinkedIn,
	}); err != nil {
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)

		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to add LinkedIn URL"}); err != nil {
			logger.Error("Error encoding extension assign error", "error", err)
		}

		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{
		"status":        "ok",
		"normalizedUrl": normalized,
	}); err != nil {
		logger.Error("Error encoding extension assign response", "error", err)
	}
}

func hasValidExtensionToken(c flamego.Context) bool {
	token := strings.TrimSpace(c.Request().Header.Get(extensionTokenHeader))
	if token == "" {
		return false
	}

	return extTokens.Has(token)
}

func writeExtensionAuthError(c flamego.Context) {
	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(http.StatusUnauthorized)

	if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]bool{"valid": false}); err != nil {
		logger.Error("Error encoding extension auth error", "error", err)
	}
}

func generateExtensionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func addExtensionCORSHeaders(c flamego.Context) {
	c.ResponseWriter().Header().Set("Access-Control-Allow-Origin", "*")
	c.ResponseWriter().Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.ResponseWriter().Header().Set("Access-Control-Allow-Headers", "Content-Type, "+extensionTokenHeader)
}
