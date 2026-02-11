// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestNewWebAuthnFromEnvEnforcesCeremonyTimeouts(t *testing.T) {
	t.Setenv("WEBAUTHN_RP_ID", "example.com")
	t.Setenv("WEBAUTHN_RP_ORIGINS", "https://example.com")
	t.Setenv("WEBAUTHN_RP_NAME", "Groundwave")

	web, err := NewWebAuthnFromEnv()
	if err != nil {
		t.Fatalf("NewWebAuthnFromEnv returned error: %v", err)
	}

	if !web.Config.Timeouts.Login.Enforce {
		t.Fatal("expected login timeout enforcement to be enabled")
	}

	if !web.Config.Timeouts.Registration.Enforce {
		t.Fatal("expected registration timeout enforcement to be enabled")
	}

	if web.Config.Timeouts.Login.Timeout <= 0 {
		t.Fatalf("expected login timeout to be set, got %v", web.Config.Timeouts.Login.Timeout)
	}

	if web.Config.Timeouts.Login.TimeoutUVD <= 0 {
		t.Fatalf("expected login UVD timeout to be set, got %v", web.Config.Timeouts.Login.TimeoutUVD)
	}

	if web.Config.Timeouts.Registration.Timeout <= 0 {
		t.Fatalf("expected registration timeout to be set, got %v", web.Config.Timeouts.Registration.Timeout)
	}

	if web.Config.Timeouts.Registration.TimeoutUVD <= 0 {
		t.Fatalf("expected registration UVD timeout to be set, got %v", web.Config.Timeouts.Registration.TimeoutUVD)
	}
}

func TestGetSessionDataAtRequiresUnexpiredSession(t *testing.T) {
	t.Parallel()

	now := mustParseTime(t, "2026-02-11T10:00:00Z")
	s := newTestSession()

	fresh := webauthn.SessionData{
		Challenge: "fresh",
		Expires:   now.Add(time.Minute),
	}
	s.Set(webauthnLoginSessionKey, fresh)

	got, ok := getSessionDataAt(s, webauthnLoginSessionKey, now)
	if !ok {
		t.Fatal("expected fresh session data to be accepted")
	}

	if got.Challenge != "fresh" {
		t.Fatalf("unexpected challenge from session data: %q", got.Challenge)
	}

	if s.Get(webauthnLoginSessionKey) == nil {
		t.Fatal("expected fresh session data to remain in session")
	}

	expired := webauthn.SessionData{
		Challenge: "expired",
		Expires:   now.Add(-time.Second),
	}
	s.Set(webauthnLoginSessionKey, expired)

	if _, ok := getSessionDataAt(s, webauthnLoginSessionKey, now); ok {
		t.Fatal("expected expired session data to be rejected")
	}

	if s.Get(webauthnLoginSessionKey) != nil {
		t.Fatal("expected expired session data to be removed from session")
	}
}

func TestGetSessionDataAtRejectsSessionWithoutExpiry(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	s.Set(webauthnRegisterSessionKey, webauthn.SessionData{Challenge: "missing-expiry"})

	if _, ok := getSessionDataAt(s, webauthnRegisterSessionKey, time.Now()); ok {
		t.Fatal("expected session data without expiry to be rejected")
	}

	if s.Get(webauthnRegisterSessionKey) != nil {
		t.Fatal("expected session data without expiry to be removed from session")
	}
}
