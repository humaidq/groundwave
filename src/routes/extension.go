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

const extensionTokenHeader = "X-Groundwave-Token"

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
		c.ResponseWriter().Write([]byte("failed to generate token"))
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
	c.ResponseWriter().Write([]byte(`<!doctype html>
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
</html>`))
}

// ExtensionValidate checks whether the provided token is valid.
func ExtensionValidate(c flamego.Context) {
	addExtensionCORSHeaders(c)
	if c.Request().Method == http.MethodOptions {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
		return
	}

	if !hasValidExtensionToken(c) {
		writeExtensionAuthError(c, http.StatusUnauthorized)
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	json.NewEncoder(c.ResponseWriter()).Encode(map[string]bool{"valid": true})
}

// ExtensionLinkedInLookup checks if LinkedIn URLs exist in contacts.
func ExtensionLinkedInLookup(c flamego.Context) {
	addExtensionCORSHeaders(c)
	if c.Request().Method == http.MethodOptions {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
		return
	}

	if !hasValidExtensionToken(c) {
		writeExtensionAuthError(c, http.StatusUnauthorized)
		return
	}

	var request struct {
		URLs []string `json:"urls"`
	}

	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&request); err != nil {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	normalizedLookup := make(map[string]struct{})
	for _, rawURL := range request.URLs {
		normalized, ok := normalizeLinkedInURL(rawURL)
		if !ok {
			continue
		}
		normalizedLookup[normalized] = struct{}{}
	}

	storedURLs, err := db.ListLinkedInURLs(c.Request().Context())
	if err != nil {
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to fetch LinkedIn URLs"})
		return
	}

	storedNormalized := make(map[string]string)
	for _, entry := range storedURLs {
		normalized, ok := normalizeLinkedInURL(entry.URL)
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
	json.NewEncoder(c.ResponseWriter()).Encode(struct {
		Matches  map[string]bool   `json:"matches"`
		Contacts map[string]string `json:"contacts"`
	}{
		Matches:  matches,
		Contacts: contacts,
	})
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
		writeExtensionAuthError(c, http.StatusUnauthorized)
		return
	}

	contacts, err := db.ListContactsWithoutLinkedIn(c.Request().Context())
	if err != nil {
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to fetch contacts"})
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
	json.NewEncoder(c.ResponseWriter()).Encode(struct {
		Contacts []extensionContactSummary `json:"contacts"`
	}{
		Contacts: response,
	})
}

// ExtensionLinkedInAssign links a LinkedIn URL to a contact.
func ExtensionLinkedInAssign(c flamego.Context) {
	addExtensionCORSHeaders(c)
	if c.Request().Method == http.MethodOptions {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
		return
	}

	if !hasValidExtensionToken(c) {
		writeExtensionAuthError(c, http.StatusUnauthorized)
		return
	}

	var request struct {
		ContactID string `json:"contactId"`
		URL       string `json:"url"`
	}

	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&request); err != nil {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if strings.TrimSpace(request.ContactID) == "" {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "missing contact ID"})
		return
	}

	normalized, ok := normalizeLinkedInURL(request.URL)
	if !ok {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "invalid LinkedIn URL"})
		return
	}

	if err := db.AddURL(c.Request().Context(), db.AddURLInput{
		ContactID: request.ContactID,
		URL:       normalized,
		URLType:   db.URLLinkedIn,
	}); err != nil {
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to add LinkedIn URL"})
		return
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{
		"status":        "ok",
		"normalizedUrl": normalized,
	})
}

func hasValidExtensionToken(c flamego.Context) bool {
	token := strings.TrimSpace(c.Request().Header.Get(extensionTokenHeader))
	if token == "" {
		return false
	}
	return extTokens.Has(token)
}

func writeExtensionAuthError(c flamego.Context, status int) {
	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	c.ResponseWriter().WriteHeader(status)
	json.NewEncoder(c.ResponseWriter()).Encode(map[string]bool{"valid": false})
}

func generateExtensionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func normalizeLinkedInURL(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("https://" + trimmed)
		if err != nil {
			return "", false
		}
	}

	host := strings.ToLower(parsed.Host)
	host = strings.TrimPrefix(host, "www.")
	if host != "linkedin.com" {
		return "", false
	}

	path := strings.TrimRight(parsed.Path, "/")
	path = strings.ToLower(path)
	if !strings.HasPrefix(path, "/in/") {
		return "", false
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[1] == "" {
		return "", false
	}

	return "https://linkedin.com/in/" + parts[1], true
}

func addExtensionCORSHeaders(c flamego.Context) {
	c.ResponseWriter().Header().Set("Access-Control-Allow-Origin", "*")
	c.ResponseWriter().Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.ResponseWriter().Header().Set("Access-Control-Allow-Headers", "Content-Type, "+extensionTokenHeader)
}
