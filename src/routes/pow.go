/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
)

const (
	proofOfWorkVerifiedSessionKey  = "pow_verified"
	proofOfWorkChallengeSessionKey = "pow_challenge"
	proofOfWorkExpiresSessionKey   = "pow_expires_at"
	proofOfWorkNextSessionKey      = "pow_next"

	// DefaultProofOfWorkDifficulty is the default number of leading zero bits.
	DefaultProofOfWorkDifficulty = 20

	proofOfWorkVerifyMaxBodyBytes int64 = 1024

	proofOfWorkMinDifficulty = 8
	proofOfWorkMaxDifficulty = 28
)

var proofOfWorkChallengeTTL = 3 * time.Minute

// ProofOfWorkConfig controls challenge hardness and expiry.
type ProofOfWorkConfig struct {
	Difficulty int
	TTL        time.Duration
}

// RequireProofOfWork enforces a one-time proof of work in each session.
func RequireProofOfWork(config ProofOfWorkConfig) flamego.Handler {
	normalized := normalizeProofOfWorkConfig(config)

	return func(c flamego.Context, s session.Session, t template.Template, data template.Data) {
		if hasProofOfWorkAccess(s) || isProofOfWorkExemptPath(c.Request().Request) {
			c.Next()
			return
		}

		next := nextPathForChallenge(c.Request().Request)
		if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodHead {
			logAccessDenied(c, s, "proof_of_work_required", http.StatusOK, c.Request().URL.Path, "next", next)
			renderProofOfWorkChallenge(c, s, t, data, next, normalized)

			return
		}

		redirectURL := "/pow?next=" + url.QueryEscape(next)
		logAccessDenied(c, s, "proof_of_work_required", http.StatusSeeOther, "/pow", "next", next)
		c.Redirect(redirectURL, http.StatusSeeOther)
	}
}

// PowForm renders the proof-of-work challenge page.
func PowForm(config ProofOfWorkConfig) flamego.Handler {
	normalized := normalizeProofOfWorkConfig(config)

	return func(c flamego.Context, s session.Session, t template.Template, data template.Data) {
		next := sanitizeNextPath(c.Query("next"))
		if raw := strings.TrimSpace(c.Query("next")); raw == "" {
			next = "/"
		}

		if hasProofOfWorkAccess(s) {
			c.Redirect(next, http.StatusSeeOther)
			return
		}

		renderProofOfWorkChallenge(c, s, t, data, next, normalized)
	}
}

func renderProofOfWorkChallenge(c flamego.Context, s session.Session, t template.Template, data template.Data, next string, config ProofOfWorkConfig) {
	challenge, err := generateProofOfWorkChallenge()
	if err != nil {
		logger.Error("Failed to generate proof-of-work challenge", "error", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)

		return
	}

	expiresAt := time.Now().Add(config.TTL)

	s.Set(proofOfWorkChallengeSessionKey, challenge)
	s.Set(proofOfWorkExpiresSessionKey, expiresAt.Unix())
	s.Set(proofOfWorkNextSessionKey, next)

	data["HideNav"] = true
	data["PoWChallenge"] = challenge
	data["PoWDifficulty"] = config.Difficulty
	data["PoWExpiresAt"] = expiresAt.Unix()

	t.HTML(http.StatusOK, "pow")
}

// PowVerify checks a browser-computed proof and unlocks the session.
func PowVerify(config ProofOfWorkConfig) flamego.Handler {
	normalized := normalizeProofOfWorkConfig(config)

	return func(c flamego.Context, s session.Session) {
		challenge, ok := s.Get(proofOfWorkChallengeSessionKey).(string)
		if !ok || challenge == "" {
			writeJSONError(c, http.StatusBadRequest, "challenge missing")
			return
		}

		defer clearProofOfWorkChallenge(s)

		expiresAt, ok := sessionInt64Value(s.Get(proofOfWorkExpiresSessionKey))
		if !ok || time.Now().Unix() > expiresAt {
			writeJSONError(c, http.StatusBadRequest, "challenge expired")

			return
		}

		var req struct {
			Nonce uint64 `json:"nonce"`
		}

		limitedBody := http.MaxBytesReader(c.ResponseWriter(), c.Request().Body().ReadCloser(), proofOfWorkVerifyMaxBodyBytes)

		defer func() {
			_ = limitedBody.Close()
		}()

		decoder := json.NewDecoder(limitedBody)

		if err := decoder.Decode(&req); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeJSONError(c, http.StatusRequestEntityTooLarge, "request payload too large")
				return
			}

			writeJSONError(c, http.StatusBadRequest, "invalid request payload")

			return
		}

		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeJSONError(c, http.StatusRequestEntityTooLarge, "request payload too large")
				return
			}

			writeJSONError(c, http.StatusBadRequest, "invalid request payload")

			return
		}

		if !verifyProofOfWork(challenge, req.Nonce, normalized.Difficulty) {
			writeJSONError(c, http.StatusUnauthorized, "invalid proof")
			return
		}

		next, _ := s.Get(proofOfWorkNextSessionKey).(string)

		next = sanitizeNextPath(next)
		if next == "" {
			next = "/"
		}

		s.Set(proofOfWorkVerifiedSessionKey, true)

		writeJSON(c, map[string]string{"redirect": next})
	}
}

func hasProofOfWorkAccess(s session.Session) bool {
	allowed, ok := s.Get(proofOfWorkVerifiedSessionKey).(bool)
	return ok && allowed
}

func isProofOfWorkExemptPath(request *http.Request) bool {
	path := request.URL.Path
	if path == "/pow" || path == "/pow/verify" || path == "/connectivity" {
		return true
	}

	return path == "/ext" || strings.HasPrefix(path, "/ext/")
}

func nextPathForChallenge(request *http.Request) string {
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		return sanitizeNextPath(request.URL.RequestURI())
	}

	return sanitizeNextPath(request.Header.Get("Referer"))
}

func clearProofOfWorkChallenge(s session.Session) {
	s.Delete(proofOfWorkChallengeSessionKey)
	s.Delete(proofOfWorkExpiresSessionKey)
	s.Delete(proofOfWorkNextSessionKey)
}

func normalizeProofOfWorkConfig(config ProofOfWorkConfig) ProofOfWorkConfig {
	normalized := config
	if normalized.Difficulty == 0 {
		normalized.Difficulty = DefaultProofOfWorkDifficulty
	}

	if normalized.Difficulty < proofOfWorkMinDifficulty {
		normalized.Difficulty = proofOfWorkMinDifficulty
	}

	if normalized.Difficulty > proofOfWorkMaxDifficulty {
		normalized.Difficulty = proofOfWorkMaxDifficulty
	}

	if normalized.TTL <= 0 {
		normalized.TTL = proofOfWorkChallengeTTL
	}

	return normalized
}

func generateProofOfWorkChallenge() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random challenge bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func verifyProofOfWork(challenge string, nonce uint64, difficulty int) bool {
	if challenge == "" {
		return false
	}

	payload := challenge + ":" + strconv.FormatUint(nonce, 10)
	hash := sha256.Sum256([]byte(payload))

	return hasLeadingZeroBits(hash[:], difficulty)
}

func hasLeadingZeroBits(hash []byte, bits int) bool {
	if bits <= 0 {
		return true
	}

	fullBytes := bits / 8
	for i := range fullBytes {
		if hash[i] != 0 {
			return false
		}
	}

	remaining := bits % 8
	if remaining == 0 {
		return true
	}

	mask := byte(0xFF << (8 - remaining))

	return hash[fullBytes]&mask == 0
}

func sessionInt64Value(raw interface{}) (int64, bool) {
	switch value := raw.(type) {
	case int64:
		return value, true
	case int:
		return int64(value), true
	case float64:
		return int64(value), true
	default:
		return 0, false
	}
}
