package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

func newWebAuthnHandlerPathTestApp(s session.Session) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.Next()
	})

	f.Post("/webauthn/setup/start", func(c flamego.Context, sess session.Session) {
		SetupStart(c, sess, nil)
	})
	f.Post("/webauthn/setup/finish", func(c flamego.Context, sess session.Session) {
		SetupFinish(c, sess, nil, nil)
	})
	f.Post("/webauthn/login/finish", func(c flamego.Context, sess session.Session) {
		PasskeyLoginFinish(c, sess, nil, nil)
	})

	f.Group("", func() {
		f.Post("/break-glass/start", func(c flamego.Context, sess session.Session) {
			BreakGlassStart(c, sess, nil)
		})
		f.Post("/break-glass/finish", func(c flamego.Context, sess session.Session) {
			BreakGlassFinish(c, sess, nil)
		})
		f.Post("/health/break-glass/start", func(c flamego.Context, sess session.Session) {
			BreakGlassStart(c, sess, nil)
		})
		f.Post("/health/break-glass/finish", func(c flamego.Context, sess session.Session) {
			BreakGlassFinish(c, sess, nil)
		})
	}, RequireAuth, RequireSensitiveAccessForHealth)

	return f
}

func TestWebAuthnSetupStartAuthorization(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		seed func(*testSession)
	}{
		{
			name: "authenticated setup start denied",
			seed: func(s *testSession) {
				s.Set("authenticated", true)
			},
		},
		{
			name: "missing setup authorization flags denied",
			seed: func(*testSession) {},
		},
		{
			name: "tampered setup authorization flags denied",
			seed: func(s *testSession) {
				s.Set(webauthnBootstrapAllowedKey, "true")
				s.Set(webauthnInviteAllowedKey, 1)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSession()
			tc.seed(s)

			f := newWebAuthnHandlerPathTestApp(s)
			rec := performWebAuthnPOST(f, "/webauthn/setup/start")

			assertJSONErrorResponse(t, rec, http.StatusForbidden, "setup not permitted")
		})
	}
}

func TestWebAuthnSetupFinishAuthorizationAndTamper(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		seed          func(*testSession)
		wantStatus    int
		wantError     string
		clearStateKey bool
	}{
		{
			name:       "missing setup authorization denied",
			seed:       func(*testSession) {},
			wantStatus: http.StatusForbidden,
			wantError:  "setup not permitted",
		},
		{
			name: "tampered setup state flag rejected",
			seed: func(s *testSession) {
				s.Set(webauthnBootstrapAllowedKey, true)
				s.Set(webauthnSetupIsAdminKey, "true")
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "setup state missing",
		},
		{
			name: "tampered setup ceremony payload rejected",
			seed: func(s *testSession) {
				s.Set(webauthnBootstrapAllowedKey, true)
				s.Set(webauthnSetupIsAdminKey, true)
				s.Set(webauthnSetupSessionKey, "tampered")
			},
			wantStatus:    http.StatusBadRequest,
			wantError:     "setup session missing",
			clearStateKey: true,
		},
		{
			name: "missing setup user metadata rejected",
			seed: func(s *testSession) {
				s.Set(webauthnBootstrapAllowedKey, true)
				s.Set(webauthnSetupIsAdminKey, true)
				s.Set(webauthnSetupSessionKey, webauthn.SessionData{
					Challenge: "challenge",
					Expires:   time.Now().Add(time.Minute),
				})
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "setup user missing",
		},
		{
			name: "invalid setup user id rejected",
			seed: func(s *testSession) {
				s.Set(webauthnBootstrapAllowedKey, true)
				s.Set(webauthnSetupIsAdminKey, true)
				s.Set(webauthnSetupSessionKey, webauthn.SessionData{
					Challenge: "challenge",
					Expires:   time.Now().Add(time.Minute),
				})
				s.Set(webauthnSetupUserIDKey, "not-a-uuid")
				s.Set(webauthnSetupDisplayNameKey, "Admin")
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid setup user",
		},
		{
			name: "missing display name rejected",
			seed: func(s *testSession) {
				s.Set(webauthnBootstrapAllowedKey, true)
				s.Set(webauthnSetupIsAdminKey, true)
				s.Set(webauthnSetupSessionKey, webauthn.SessionData{
					Challenge: "challenge",
					Expires:   time.Now().Add(time.Minute),
				})
				s.Set(webauthnSetupUserIDKey, uuid.NewString())
			},
			wantStatus: http.StatusBadRequest,
			wantError:  "display name missing",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSession()
			tc.seed(s)

			f := newWebAuthnHandlerPathTestApp(s)
			rec := performWebAuthnPOST(f, "/webauthn/setup/finish")

			assertJSONErrorResponse(t, rec, tc.wantStatus, tc.wantError)
			if tc.clearStateKey && s.Get(webauthnSetupSessionKey) != nil {
				t.Fatal("expected tampered setup session data to be removed")
			}
		})
	}
}

func TestWebAuthnLoginFinishSessionStateValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		seed              func(*testSession)
		wantError         string
		expectStatePruned bool
	}{
		{
			name:      "missing login session rejected",
			seed:      func(*testSession) {},
			wantError: "login session missing",
		},
		{
			name: "tampered login session payload rejected",
			seed: func(s *testSession) {
				s.Set(webauthnLoginSessionKey, 123)
			},
			wantError:         "login session missing",
			expectStatePruned: true,
		},
		{
			name: "expired login session rejected",
			seed: func(s *testSession) {
				s.Set(webauthnLoginSessionKey, webauthn.SessionData{
					Challenge: "expired",
					Expires:   time.Now().Add(-time.Minute),
				})
			},
			wantError:         "login session missing",
			expectStatePruned: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSession()
			tc.seed(s)

			f := newWebAuthnHandlerPathTestApp(s)
			rec := performWebAuthnPOST(f, "/webauthn/login/finish")

			assertJSONErrorResponse(t, rec, http.StatusBadRequest, tc.wantError)
			if tc.expectStatePruned && s.Get(webauthnLoginSessionKey) != nil {
				t.Fatal("expected invalid login session data to be removed")
			}
		})
	}
}

func TestBreakGlassRoutesRequireAuthentication(t *testing.T) {
	t.Parallel()

	paths := []string{
		"/break-glass/start",
		"/break-glass/finish",
		"/health/break-glass/start",
		"/health/break-glass/finish",
	}

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			s := newTestSession()
			f := newWebAuthnHandlerPathTestApp(s)

			rec := performWebAuthnPOST(f, path)
			assertRedirectResponse(t, rec, http.StatusFound, "/login")
		})
	}
}

func TestBreakGlassStartRequiresSessionUserID(t *testing.T) {
	t.Parallel()

	paths := []string{
		"/break-glass/start",
		"/health/break-glass/start",
	}

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			s := newTestSession()
			s.Set("authenticated", true)

			f := newWebAuthnHandlerPathTestApp(s)
			rec := performWebAuthnPOST(f, path)

			assertJSONErrorResponse(t, rec, http.StatusUnauthorized, "not authenticated")
		})
	}
}

func TestBreakGlassFinishSessionStateTamper(t *testing.T) {
	t.Parallel()

	paths := []string{
		"/break-glass/finish",
		"/health/break-glass/finish",
	}

	for _, path := range paths {
		path := path
		t.Run(path, func(t *testing.T) {
			s := newTestSession()
			s.Set("authenticated", true)
			s.Set("user_id", uuid.NewString())
			s.Set(webauthnBreakGlassSessionKey, "tampered")

			f := newWebAuthnHandlerPathTestApp(s)
			rec := performWebAuthnPOST(f, path)

			assertJSONErrorResponse(t, rec, http.StatusBadRequest, "verification session missing")
			if s.Get(webauthnBreakGlassSessionKey) != nil {
				t.Fatal("expected tampered break-glass session data to be removed")
			}
		})
	}
}

func performWebAuthnPOST(f *flamego.Flame, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)
	return rec
}

func assertJSONErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantError string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("expected status %d, got %d", wantStatus, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var payload map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode JSON error body: %v", err)
	}
	if got := payload["error"]; got != wantError {
		t.Fatalf("expected error %q, got %q", wantError, got)
	}
}

func assertRedirectResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantLocation string) {
	t.Helper()

	if rec.Code != wantStatus {
		t.Fatalf("expected status %d, got %d", wantStatus, rec.Code)
	}
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Fatalf("expected redirect to %q, got %q", wantLocation, got)
	}
}
