// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
)

type powTemplateStub struct {
	rw http.ResponseWriter
}

func (s *powTemplateStub) HTML(status int, _ string) {
	s.rw.WriteHeader(status)
}

func newProofOfWorkTestApp(s session.Session, config ProofOfWorkConfig) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.MapTo(&powTemplateStub{rw: c.ResponseWriter()}, (*template.Template)(nil))
		c.Map(template.Data{})
		c.Next()
	})

	f.Use(RequireProofOfWork(config))

	f.Get("/pow", PowForm(config))
	f.Post("/pow/verify", PowVerify(config))
	f.Get("/protected", func(c flamego.Context) {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})
	f.Get("/ext/validate", func(c flamego.Context) {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})
	f.Get("/connectivity", func(c flamego.Context) {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})

	return f
}

func TestRequireProofOfWorkRendersChallengeAtRequestedPath(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

	req := httptest.NewRequest(http.MethodGet, "/protected?x=1", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("expected no redirect location, got %q", got)
	}

	if got, _ := s.Get(proofOfWorkNextSessionKey).(string); got != "/protected?x=1" {
		t.Fatalf("expected next path to match requested path, got %q", got)
	}
}

func TestRequireProofOfWorkRedirectsUnverifiedPostRequest(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.Header.Set("Referer", "/protected?x=1")

	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "/pow?next=%2Fprotected%3Fx%3D1" {
		t.Fatalf("expected redirect to proof page, got %q", got)
	}
}

func TestRequireProofOfWorkSkipsExtensionEndpoints(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

	req := httptest.NewRequest(http.MethodGet, "/ext/validate", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRequireProofOfWorkSkipsConnectivityProbe(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

	req := httptest.NewRequest(http.MethodGet, "/connectivity", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestProofOfWorkVerifyUnlocksSession(t *testing.T) {
	t.Parallel()

	config := ProofOfWorkConfig{Difficulty: 8}
	s := newTestSession()
	f := newProofOfWorkTestApp(s, config)

	formReq := httptest.NewRequest(http.MethodGet, "/pow?next=%2Flogin", nil)
	formRec := httptest.NewRecorder()
	f.ServeHTTP(formRec, formReq)

	if formRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, formRec.Code)
	}

	challenge, _ := s.Get(proofOfWorkChallengeSessionKey).(string)
	if challenge == "" {
		t.Fatal("expected proof-of-work challenge in session")
	}

	nonce := uint64(0)
	for !verifyProofOfWork(challenge, nonce, config.Difficulty) {
		nonce++
	}

	body, err := json.Marshal(map[string]uint64{"nonce": nonce})
	if err != nil {
		t.Fatalf("marshal nonce payload: %v", err)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/pow/verify", bytes.NewReader(body))
	verifyRec := httptest.NewRecorder()
	f.ServeHTTP(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, verifyRec.Code)
	}

	if allowed, ok := s.Get(proofOfWorkVerifiedSessionKey).(bool); !ok || !allowed {
		t.Fatal("expected proof-of-work session flag to be set")
	}
}

func TestProofOfWorkVerifyConsumesChallengeAfterInvalidProof(t *testing.T) {
	t.Parallel()

	config := ProofOfWorkConfig{Difficulty: 8}
	s := newTestSession()
	f := newProofOfWorkTestApp(s, config)

	formReq := httptest.NewRequest(http.MethodGet, "/pow?next=%2Flogin", nil)
	formRec := httptest.NewRecorder()
	f.ServeHTTP(formRec, formReq)

	if formRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, formRec.Code)
	}

	challenge, _ := s.Get(proofOfWorkChallengeSessionKey).(string)
	if challenge == "" {
		t.Fatal("expected proof-of-work challenge in session")
	}

	invalidNonce := uint64(0)
	for verifyProofOfWork(challenge, invalidNonce, config.Difficulty) {
		invalidNonce++
	}

	body, err := json.Marshal(map[string]uint64{"nonce": invalidNonce})
	if err != nil {
		t.Fatalf("marshal nonce payload: %v", err)
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/pow/verify", bytes.NewReader(body))
	verifyRec := httptest.NewRecorder()
	f.ServeHTTP(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, verifyRec.Code)
	}

	if got := s.Get(proofOfWorkChallengeSessionKey); got != nil {
		t.Fatalf("expected challenge to be cleared after a failed guess, got %#v", got)
	}

	retryReq := httptest.NewRequest(http.MethodPost, "/pow/verify", bytes.NewReader(body))
	retryRec := httptest.NewRecorder()
	f.ServeHTTP(retryRec, retryReq)

	if retryRec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, retryRec.Code)
	}
}

func TestProofOfWorkVerifyRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	config := ProofOfWorkConfig{Difficulty: 8}
	s := newTestSession()
	f := newProofOfWorkTestApp(s, config)

	formReq := httptest.NewRequest(http.MethodGet, "/pow?next=%2Flogin", nil)
	formRec := httptest.NewRecorder()
	f.ServeHTTP(formRec, formReq)

	if formRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, formRec.Code)
	}

	overSizedPayload := `{"nonce":1,"padding":"` + strings.Repeat("a", int(proofOfWorkVerifyMaxBodyBytes)) + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/pow/verify", bytes.NewBufferString(overSizedPayload))
	verifyRec := httptest.NewRecorder()
	f.ServeHTTP(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status %d, got %d", http.StatusRequestEntityTooLarge, verifyRec.Code)
	}

	if got := s.Get(proofOfWorkChallengeSessionKey); got != nil {
		t.Fatalf("expected challenge to be cleared after oversized request, got %#v", got)
	}
}
