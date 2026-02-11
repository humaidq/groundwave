package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
)

type unauthenticatedSession struct {
	data map[interface{}]interface{}
}

func newUnauthenticatedSession() *unauthenticatedSession {
	return &unauthenticatedSession{data: make(map[interface{}]interface{})}
}

func (s *unauthenticatedSession) ID() string {
	return "unauthenticated-session"
}

func (s *unauthenticatedSession) RegenerateID(http.ResponseWriter, *http.Request) error {
	return nil
}

func (s *unauthenticatedSession) Get(key interface{}) interface{} {
	return s.data[key]
}

func (s *unauthenticatedSession) Set(key, val interface{}) {
	s.data[key] = val
}

func (s *unauthenticatedSession) SetFlash(interface{}) {}

func (s *unauthenticatedSession) Delete(key interface{}) {
	delete(s.data, key)
}

func (s *unauthenticatedSession) Flush() {
	s.data = make(map[interface{}]interface{})
}

func (s *unauthenticatedSession) Encode() ([]byte, error) {
	return nil, nil
}

func (s *unauthenticatedSession) HasChanged() bool {
	return false
}

func newUnauthenticatedAccessTestApp() *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(newUnauthenticatedSession(), (*session.Session)(nil))
		c.Next()
	})

	handler := func(c flamego.Context) {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	}

	// Protected routes (RequireAuth + RequireSensitiveAccessForHealth)
	f.Group("", func() {
		f.Get("/files", handler)
		f.Get("/files/view", handler)
		f.Get("/files/file", handler)

		f.Get("/health/break-glass", handler)
		f.Get("/health", handler)
		f.Get("/health/{id}", handler)
		f.Get("/health/{profile_id}/followup/{id}", handler)

		f.Post("/health/break-glass/start", handler)
		f.Post("/health/break-glass/finish", handler)
		f.Post("/health/{profile_id}/followup/{id}/ai-summary", handler)
	}, RequireAuth, RequireSensitiveAccessForHealth)

	// Admin routes (RequireAuth + RequireAdmin + RequireSensitiveAccessForHealth)
	f.Group("", func() {
		f.Get("/health/new", handler)
		f.Get("/health/{id}/edit", handler)
		f.Get("/health/{profile_id}/followup/new", handler)
		f.Get("/health/{profile_id}/followup/{id}/edit", handler)
		f.Get("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", handler)

		f.Post("/health/new", handler)
		f.Post("/health/{id}/edit", handler)
		f.Post("/health/{id}/delete", handler)
		f.Post("/health/{profile_id}/followup/new", handler)
		f.Post("/health/{profile_id}/followup/{id}/edit", handler)
		f.Post("/health/{profile_id}/followup/{id}/delete", handler)
		f.Post("/health/{profile_id}/followup/{followup_id}/result", handler)
		f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/edit", handler)
		f.Post("/health/{profile_id}/followup/{followup_id}/result/{id}/delete", handler)

		f.Get("/zk", handler)
		f.Get("/zk/random", handler)
		f.Get("/zk/list", handler)
		f.Get("/zk/chat", handler)
		f.Get("/zk/{id}", handler)

		f.Post("/zk/chat/links", handler)
		f.Post("/zk/chat/backlinks", handler)
		f.Post("/zk/chat/stream", handler)
		f.Post("/zk/{id}/comment", handler)
		f.Post("/zk/{id}/comment/{comment_id}/edit", handler)
		f.Post("/zk/{id}/comment/{comment_id}/delete", handler)
		f.Post("/zk/{id}/comments/delete", handler)
	}, RequireAuth, RequireAdmin, RequireSensitiveAccessForHealth)

	return f
}

func TestUnauthenticatedAccessRedirectsToLogin(t *testing.T) {
	t.Parallel()

	f := newUnauthenticatedAccessTestApp()

	testCases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "files root", method: http.MethodGet, path: "/files"},
		{name: "files view", method: http.MethodGet, path: "/files/view?path=docs/readme.md"},
		{name: "files download", method: http.MethodGet, path: "/files/file?path=docs/readme.md"},

		{name: "health root", method: http.MethodGet, path: "/health"},
		{name: "health break glass", method: http.MethodGet, path: "/health/break-glass"},
		{name: "health profile", method: http.MethodGet, path: "/health/profile-1"},
		{name: "health followup", method: http.MethodGet, path: "/health/profile-1/followup/followup-1"},
		{name: "health new", method: http.MethodGet, path: "/health/new"},
		{name: "health edit", method: http.MethodGet, path: "/health/profile-1/edit"},
		{name: "health followup new", method: http.MethodGet, path: "/health/profile-1/followup/new"},
		{name: "health followup edit", method: http.MethodGet, path: "/health/profile-1/followup/followup-1/edit"},
		{name: "health result edit", method: http.MethodGet, path: "/health/profile-1/followup/followup-1/result/result-1/edit"},
		{name: "health break glass start", method: http.MethodPost, path: "/health/break-glass/start"},
		{name: "health break glass finish", method: http.MethodPost, path: "/health/break-glass/finish"},
		{name: "health ai summary", method: http.MethodPost, path: "/health/profile-1/followup/followup-1/ai-summary"},
		{name: "health create", method: http.MethodPost, path: "/health/new"},
		{name: "health update", method: http.MethodPost, path: "/health/profile-1/edit"},
		{name: "health delete", method: http.MethodPost, path: "/health/profile-1/delete"},
		{name: "health followup create", method: http.MethodPost, path: "/health/profile-1/followup/new"},
		{name: "health followup update", method: http.MethodPost, path: "/health/profile-1/followup/followup-1/edit"},
		{name: "health followup delete", method: http.MethodPost, path: "/health/profile-1/followup/followup-1/delete"},
		{name: "health result create", method: http.MethodPost, path: "/health/profile-1/followup/followup-1/result"},
		{name: "health result update", method: http.MethodPost, path: "/health/profile-1/followup/followup-1/result/result-1/edit"},
		{name: "health result delete", method: http.MethodPost, path: "/health/profile-1/followup/followup-1/result/result-1/delete"},

		{name: "zk root", method: http.MethodGet, path: "/zk"},
		{name: "zk random", method: http.MethodGet, path: "/zk/random"},
		{name: "zk list", method: http.MethodGet, path: "/zk/list"},
		{name: "zk chat", method: http.MethodGet, path: "/zk/chat"},
		{name: "zk note", method: http.MethodGet, path: "/zk/note-1"},
		{name: "zk links", method: http.MethodPost, path: "/zk/chat/links"},
		{name: "zk backlinks", method: http.MethodPost, path: "/zk/chat/backlinks"},
		{name: "zk stream", method: http.MethodPost, path: "/zk/chat/stream"},
		{name: "zk comment add", method: http.MethodPost, path: "/zk/note-1/comment"},
		{name: "zk comment edit", method: http.MethodPost, path: "/zk/note-1/comment/comment-1/edit"},
		{name: "zk comment delete", method: http.MethodPost, path: "/zk/note-1/comment/comment-1/delete"},
		{name: "zk comments delete", method: http.MethodPost, path: "/zk/note-1/comments/delete"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			f.ServeHTTP(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("expected status %d, got %d", http.StatusFound, rec.Code)
			}
			if got := rec.Header().Get("Location"); got != "/login" {
				t.Fatalf("expected redirect to /login, got %q", got)
			}
		})
	}
}
