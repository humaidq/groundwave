// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/flamego/flamego"
)

func newExtensionEndpointAuthTestApp() *flamego.Flame {
	f := flamego.New()
	f.Get("/ext/validate", ExtensionValidate)
	f.Get("/ext/contacts-no-linkedin", ExtensionContactsWithoutLinkedIn)
	f.Post("/ext/linkedin-lookup", ExtensionLinkedInLookup)
	f.Post("/ext/linkedin-assign", ExtensionLinkedInAssign)
	f.Options("/ext/validate", ExtensionValidate)
	f.Options("/ext/contacts-no-linkedin", ExtensionContactsWithoutLinkedIn)
	f.Options("/ext/linkedin-lookup", ExtensionLinkedInLookup)
	f.Options("/ext/linkedin-assign", ExtensionLinkedInAssign)

	return f
}

func isolateExtensionTokenStore(t *testing.T) {
	t.Helper()

	originalStore := extTokens
	extTokens = newExtensionTokenStore()

	t.Cleanup(func() {
		extTokens = originalStore
	})
}

func assertExtensionCORSHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("unexpected Access-Control-Allow-Origin: %q", got)
	}

	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("unexpected Access-Control-Allow-Methods: %q", got)
	}

	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, "+extensionTokenHeader {
		t.Fatalf("unexpected Access-Control-Allow-Headers: %q", got)
	}
}

func assertExtensionAuthError(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()

	var payload map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed decoding auth error response: %v", err)
	}

	if valid, ok := payload["valid"]; !ok || valid {
		t.Fatalf("expected valid=false auth payload, got %#v", payload)
	}
}

func assertExtensionErrorMessage(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()

	var payload map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed decoding error response: %v", err)
	}

	if got := payload["error"]; got != want {
		t.Fatalf("unexpected error message: got %q, want %q", got, want)
	}
}

func TestExtensionEndpointsRejectMissingToken(t *testing.T) {
	isolateExtensionTokenStore(t)

	f := newExtensionEndpointAuthTestApp()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "validate", method: http.MethodGet, path: "/ext/validate"},
		{name: "contacts no linkedin", method: http.MethodGet, path: "/ext/contacts-no-linkedin"},
		{name: "linkedin lookup malformed payload", method: http.MethodPost, path: "/ext/linkedin-lookup", body: `{"urls":`},
		{name: "linkedin assign malformed payload", method: http.MethodPost, path: "/ext/linkedin-assign", body: `{"contactId":`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.method == http.MethodPost {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
			}

			assertExtensionCORSHeaders(t, rec)
			assertExtensionAuthError(t, rec)
		})
	}
}

func TestExtensionPostEndpointsRejectBadPayloadWithValidToken(t *testing.T) {
	isolateExtensionTokenStore(t)

	f := newExtensionEndpointAuthTestApp()

	const token = "issued-token"
	extTokens.Add(token)

	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "linkedin lookup", path: "/ext/linkedin-lookup", body: `{"urls":`},
		{name: "linkedin assign", path: "/ext/linkedin-assign", body: `{"contactId":"abc"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(extensionTokenHeader, token)

			rec := httptest.NewRecorder()
			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}

			assertExtensionCORSHeaders(t, rec)
			assertExtensionErrorMessage(t, rec, "invalid request body")
		})
	}
}

func TestExtensionEndpointsHandleOptionsPreflight(t *testing.T) {
	isolateExtensionTokenStore(t)

	f := newExtensionEndpointAuthTestApp()

	paths := []string{
		"/ext/validate",
		"/ext/contacts-no-linkedin",
		"/ext/linkedin-lookup",
		"/ext/linkedin-assign",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			req.Header.Set("Origin", "https://attacker.example")
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)

			rec := httptest.NewRecorder()
			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
			}

			assertExtensionCORSHeaders(t, rec)

			if body := strings.TrimSpace(rec.Body.String()); body != "" {
				t.Fatalf("expected empty preflight body, got %q", body)
			}
		})
	}
}

func TestExtensionValidateRejectsTokenBypassVectors(t *testing.T) {
	isolateExtensionTokenStore(t)

	f := newExtensionEndpointAuthTestApp()

	const token = "issued-token"
	extTokens.Add(token)

	tests := []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{
			name: "query string token",
			path: "/ext/validate?token=" + url.QueryEscape(token),
		},
		{
			name: "authorization bearer token",
			path: "/ext/validate",
			headers: map[string]string{
				"Authorization": "Bearer " + token,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			rec := httptest.NewRecorder()
			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
			}

			assertExtensionCORSHeaders(t, rec)
			assertExtensionAuthError(t, rec)
		})
	}
}

func TestExtensionLinkedInAssignRejectsMaliciousURLPayload(t *testing.T) {
	isolateExtensionTokenStore(t)

	f := newExtensionEndpointAuthTestApp()

	const token = "issued-token"
	extTokens.Add(token)

	req := httptest.NewRequest(
		http.MethodPost,
		"/ext/linkedin-assign",
		strings.NewReader(`{"contactId":"contact-1","url":"javascript:alert(1)"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(extensionTokenHeader, token)

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	assertExtensionCORSHeaders(t, rec)
	assertExtensionErrorMessage(t, rec, "invalid LinkedIn URL")
}
