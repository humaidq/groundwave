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
	"time"

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
	f.Get("/qrz", func(c flamego.Context) {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})

	return f
}

func fixedASNResolver(asn uint32, country string, known bool) ClientASNResolver {
	return func(*http.Request) (uint32, string, bool) {
		if !known {
			return 0, "", false
		}

		return asn, country, true
	}
}

func solveProofOfWork(t *testing.T, f *flamego.Flame, s session.Session, difficulty int) {
	t.Helper()

	formReq := httptest.NewRequest(http.MethodGet, "/pow?next=%2Fprotected", nil)
	formRec := httptest.NewRecorder()
	f.ServeHTTP(formRec, formReq)

	if formRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, formRec.Code)
	}

	challenge, _ := s.Get(proofOfWorkChallengeSessionKey).(string)
	if challenge == "" {
		t.Fatal("expected proof-of-work challenge in session")
	}

	nonce := uint64(0)
	for !verifyProofOfWork(challenge, nonce, difficulty) {
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
}

func TestRequireProofOfWorkRendersChallengeAtRequestedPath(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

	req := httptest.NewRequest(http.MethodGet, "/protected?x=1", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
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

func TestRequireProofOfWorkSkipsExtensionEndpointsForAllowedASN(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{
		Difficulty:       8,
		LowRiskASNs:      map[uint32]struct{}{64512: {}},
		ResolveClientASN: fixedASNResolver(64512, "US", true),
	})

	req := httptest.NewRequest(http.MethodGet, "/ext/validate", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRequireProofOfWorkSkipsExtensionEndpointsForLocalClientIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ip   string
	}{
		{name: "RFC1918 10/8", ip: "10.10.1.2"},
		{name: "RFC1918 172.16/12", ip: "172.16.2.3"},
		{name: "RFC1918 192.168/16", ip: "192.168.4.5"},
		{name: "IPv4 loopback", ip: "127.0.0.1"},
		{name: "IPv6 loopback", ip: "::1"},
		{name: "IPv6 unique local", ip: "fc00::1"},
		{name: "IPv6 link-local", ip: "fe80::1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newTestSession()
			f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

			req := httptest.NewRequest(http.MethodGet, "/ext/validate", nil)
			req.Header.Set("X-Forwarded-For", tc.ip)

			rec := httptest.NewRecorder()

			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
			}
		})
	}
}

func TestRequireProofOfWorkBlocksExtensionEndpointsForDisallowedASN(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{
		Difficulty:       8,
		LowRiskASNs:      map[uint32]struct{}{64512: {}},
		ResolveClientASN: fixedASNResolver(64513, "US", true),
	})

	req := httptest.NewRequest(http.MethodGet, "/ext/validate", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRequireProofOfWorkBlocksExtensionEndpointsForUnknownPublicIP(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

	req := httptest.NewRequest(http.MethodGet, "/ext/validate", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")

	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRequireProofOfWorkBlocksExtensionEndpointsForHighRiskCountry(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{
		Difficulty:        8,
		LowRiskASNs:       map[uint32]struct{}{64512: {}},
		HighRiskCountries: map[string]struct{}{"CN": {}},
		ResolveClientASN:  fixedASNResolver(64512, "CN", true),
	})

	req := httptest.NewRequest(http.MethodGet, "/ext/validate", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
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

func TestRequireProofOfWorkSkipsQRZPath(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{Difficulty: 8})

	req := httptest.NewRequest(http.MethodGet, "/qrz", nil)
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

	if formRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, formRec.Code)
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

	if _, ok := sessionInt64Value(s.Get(proofOfWorkVerifiedAtSessionKey)); !ok {
		t.Fatal("expected proof-of-work verification timestamp to be set")
	}
}

func TestRequireProofOfWorkExpiresSolvedChallengeForNonLowRisk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config ProofOfWorkConfig
	}{
		{
			name: "unknown ASN expires after 15 minutes",
			config: ProofOfWorkConfig{
				MediumDifficulty: 8,
			},
		},
		{
			name: "high risk ASN expires after 15 minutes",
			config: ProofOfWorkConfig{
				EasyDifficulty:   8,
				MediumDifficulty: 8,
				HardDifficulty:   8,
				HighRiskASNs:     map[uint32]struct{}{64513: {}},
				ResolveClientASN: fixedASNResolver(64513, "US", true),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newTestSession()
			f := newProofOfWorkTestApp(s, tc.config)

			solveProofOfWork(t, f, s, 8)
			s.Set(proofOfWorkVerifiedAtSessionKey, time.Now().Add(-16*time.Minute).Unix())

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			rec := httptest.NewRecorder()
			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
			}

			if got := s.Get(proofOfWorkVerifiedSessionKey); got != nil {
				t.Fatalf("expected proof-of-work access to be cleared, got %#v", got)
			}

			if got := s.Get(proofOfWorkVerifiedAtSessionKey); got != nil {
				t.Fatalf("expected proof-of-work verification time to be cleared, got %#v", got)
			}
		})
	}
}

func TestRequireProofOfWorkKeepsSolvedChallengeForLowRiskASN(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{
		EasyDifficulty:   8,
		MediumDifficulty: 8,
		HardDifficulty:   8,
		LowRiskASNs:      map[uint32]struct{}{64512: {}},
		ResolveClientASN: fixedASNResolver(64512, "US", true),
	})

	solveProofOfWork(t, f, s, 8)
	s.Set(proofOfWorkVerifiedAtSessionKey, time.Now().Add(-(14*24*time.Hour)+time.Minute).Unix())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
}

func TestRequireProofOfWorkKeepsSolvedChallengeForAuthenticatedSession(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	f := newProofOfWorkTestApp(s, ProofOfWorkConfig{
		EasyDifficulty:   8,
		MediumDifficulty: 8,
		HardDifficulty:   8,
		HighRiskASNs:     map[uint32]struct{}{64513: {}},
		ResolveClientASN: fixedASNResolver(64513, "US", true),
	})

	solveProofOfWork(t, f, s, 8)
	s.Set(proofOfWorkVerifiedAtSessionKey, time.Now().Add(-2*time.Hour).Unix())
	s.Set("authenticated", true)
	s.Set(authenticatedExpiresAtSessionKey, time.Now().Add(time.Hour).Unix())

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
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

	if formRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, formRec.Code)
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

	if formRec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, formRec.Code)
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

func TestRequireProofOfWorkUsesRiskSpecificDifficulty(t *testing.T) {
	t.Parallel()

	const (
		easyDifficulty   = 9
		mediumDifficulty = 13
		hardDifficulty   = 17
	)

	tests := []struct {
		name        string
		resolverASN uint32
		country     string
		resolverOK  bool
		config      ProofOfWorkConfig
		want        int
	}{
		{
			name:        "low risk ASN uses easy difficulty",
			resolverASN: 64496,
			country:     "US",
			resolverOK:  true,
			config: ProofOfWorkConfig{
				EasyDifficulty:   easyDifficulty,
				MediumDifficulty: mediumDifficulty,
				HardDifficulty:   hardDifficulty,
				LowRiskASNs:      map[uint32]struct{}{64496: {}},
			},
			want: easyDifficulty,
		},
		{
			name:        "high risk ASN uses hard difficulty",
			resolverASN: 64497,
			country:     "US",
			resolverOK:  true,
			config: ProofOfWorkConfig{
				EasyDifficulty:   easyDifficulty,
				MediumDifficulty: mediumDifficulty,
				HardDifficulty:   hardDifficulty,
				HighRiskASNs:     map[uint32]struct{}{64497: {}},
			},
			want: hardDifficulty,
		},
		{
			name:        "high risk country uses hard difficulty",
			resolverASN: 64498,
			country:     "CN",
			resolverOK:  true,
			config: ProofOfWorkConfig{
				EasyDifficulty:    easyDifficulty,
				MediumDifficulty:  mediumDifficulty,
				HardDifficulty:    hardDifficulty,
				HighRiskCountries: map[string]struct{}{"CN": {}},
			},
			want: hardDifficulty,
		},
		{
			name:       "unknown ASN uses medium difficulty",
			resolverOK: false,
			config: ProofOfWorkConfig{
				EasyDifficulty:   easyDifficulty,
				MediumDifficulty: mediumDifficulty,
				HardDifficulty:   hardDifficulty,
			},
			want: mediumDifficulty,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newTestSession()

			config := tc.config
			config.ResolveClientASN = fixedASNResolver(tc.resolverASN, tc.country, tc.resolverOK)

			f := newProofOfWorkTestApp(s, config)

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			rec := httptest.NewRecorder()

			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
			}

			got, ok := sessionIntValue(s.Get(proofOfWorkDifficultySessionKey))
			if !ok {
				t.Fatal("expected proof-of-work difficulty in session")
			}

			if got != tc.want {
				t.Fatalf("unexpected difficulty: got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestChallengeTTLForDifficulty(t *testing.T) {
	t.Parallel()

	baseTTL := time.Minute

	tests := []struct {
		name       string
		difficulty int
		want       time.Duration
	}{
		{name: "difficulty at threshold keeps base ttl", difficulty: proofOfWorkDifficultyTTLRelaxThreshold, want: baseTTL},
		{name: "difficulty above threshold doubles ttl", difficulty: proofOfWorkDifficultyTTLRelaxThreshold + 1, want: 2 * baseTTL},
		{name: "difficulty two above threshold quadruples ttl", difficulty: proofOfWorkDifficultyTTLRelaxThreshold + 2, want: 4 * baseTTL},
		{name: "difficulty six above threshold scales exponentially", difficulty: proofOfWorkDifficultyTTLRelaxThreshold + 6, want: 64 * baseTTL},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := challengeTTLForDifficulty(baseTTL, tc.difficulty)
			if got != tc.want {
				t.Fatalf("unexpected ttl: got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestRequireProofOfWorkExtendsTTLForHighDifficulty(t *testing.T) {
	t.Parallel()

	config := ProofOfWorkConfig{Difficulty: proofOfWorkDifficultyTTLRelaxThreshold + 1, TTL: time.Minute}
	s := newTestSession()
	f := newProofOfWorkTestApp(s, config)

	before := time.Now().Unix()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rec.Code)
	}

	expiresAt, ok := sessionInt64Value(s.Get(proofOfWorkExpiresSessionKey))
	if !ok {
		t.Fatal("expected challenge expiry in session")
	}

	after := time.Now().Unix()
	expectedTTLSeconds := int64((2 * time.Minute).Seconds())
	lowerBound := before + expectedTTLSeconds - 1
	upperBound := after + expectedTTLSeconds + 1

	if expiresAt < lowerBound || expiresAt > upperBound {
		t.Fatalf("unexpected challenge expiry: got %d, expected between %d and %d", expiresAt, lowerBound, upperBound)
	}
}
