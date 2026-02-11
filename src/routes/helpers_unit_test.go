// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flamego/csrf"
	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

type testSession struct {
	id    string
	data  map[interface{}]interface{}
	flash interface{}
}

func newTestSession() *testSession {
	return &testSession{
		id:   "test-session",
		data: make(map[interface{}]interface{}),
	}
}

func (s *testSession) ID() string {
	return s.id
}

func (s *testSession) RegenerateID(http.ResponseWriter, *http.Request) error {
	return nil
}

func (s *testSession) Get(key interface{}) interface{} {
	return s.data[key]
}

func (s *testSession) Set(key, val interface{}) {
	s.data[key] = val
}

func (s *testSession) SetFlash(val interface{}) {
	s.flash = val
}

func (s *testSession) Delete(key interface{}) {
	delete(s.data, key)
}

func (s *testSession) Flush() {
	s.data = make(map[interface{}]interface{})
}

func (s *testSession) Encode() ([]byte, error) {
	return nil, nil
}

func (s *testSession) HasChanged() bool {
	return true
}

type testCSRF struct {
	token string
}

func (c testCSRF) Token() string {
	return c.token
}

func (c testCSRF) ValidToken(string) bool {
	return true
}

func (c testCSRF) Error(http.ResponseWriter) {}

func (c testCSRF) Validate(flamego.Context) {}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	tm, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", value, err)
	}

	return tm
}

func TestSetFlashHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		set     func(session.Session, string)
		wantTyp FlashType
	}{
		{name: "error", set: SetErrorFlash, wantTyp: FlashError},
		{name: "success", set: SetSuccessFlash, wantTyp: FlashSuccess},
		{name: "warning", set: SetWarningFlash, wantTyp: FlashWarning},
		{name: "info", set: SetInfoFlash, wantTyp: FlashInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newTestSession()
			tt.set(s, "hello")

			msg, ok := s.flash.(FlashMessage)
			if !ok {
				t.Fatalf("flash has unexpected type: %T", s.flash)
			}

			if msg.Type != tt.wantTyp || msg.Message != "hello" {
				t.Fatalf("unexpected flash message: %#v", msg)
			}
		})
	}
}

func TestCSRFInjector(t *testing.T) {
	t.Parallel()

	handler, ok := CSRFInjector().(func(csrf.CSRF, template.Data))
	if !ok {
		t.Fatalf("unexpected CSRFInjector handler type")
	}

	data := template.Data{}
	handler(testCSRF{token: "csrf-123"}, data)

	if got, ok := data["csrf_token"].(string); !ok || got != "csrf-123" {
		t.Fatalf("unexpected csrf_token value: %#v", data["csrf_token"])
	}
}

func TestNoCacheHeaders(t *testing.T) {
	t.Parallel()

	f := flamego.New()
	f.Use(NoCacheHeaders())
	f.Get("/", func(c flamego.Context) {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})
	f.Post("/", func(c flamego.Context) {
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getRec := httptest.NewRecorder()
	f.ServeHTTP(getRec, getReq)

	if got := getRec.Header().Get("Cache-Control"); got != "no-store, max-age=0" {
		t.Fatalf("unexpected Cache-Control for GET: %q", got)
	}

	if got := getRec.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("unexpected Pragma for GET: %q", got)
	}

	if got := getRec.Header().Get("Expires"); got != "0" {
		t.Fatalf("unexpected Expires for GET: %q", got)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/", nil)
	postRec := httptest.NewRecorder()
	f.ServeHTTP(postRec, postReq)

	if got := postRec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("expected no Cache-Control for POST, got %q", got)
	}
}

func TestParseUserAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ua   string
		want string
	}{
		{ua: "", want: "Unknown device"},
		{ua: "Mozilla/5.0 (Windows NT 10.0) Edg/120", want: "Windows / Edge"},
		{ua: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0) AppleWebKit Safari", want: "iOS / Safari"},
		{ua: "Mozilla/5.0 (X11; Linux x86_64) Firefox/122.0", want: "Linux / Firefox"},
		{ua: "Mozilla/5.0 (Macintosh) Chrome/120.0.0.0 Safari", want: "macOS / Chrome"},
	}

	for _, tt := range tests {
		if got := parseUserAgent(tt.ua); got != tt.want {
			t.Fatalf("parseUserAgent(%q) = %q, want %q", tt.ua, got, tt.want)
		}
	}
}

func TestGetClientIP(t *testing.T) {
	t.Parallel()

	withXFF := &flamego.Request{Request: httptest.NewRequest(http.MethodGet, "http://example.test", nil)}
	withXFF.Header.Set("X-Forwarded-For", " 203.0.113.4, 198.51.100.2 ")

	withXFF.RemoteAddr = "10.0.0.1:1234"
	if got := getClientIP(withXFF); got != "203.0.113.4" {
		t.Fatalf("expected X-Forwarded-For IP, got %q", got)
	}

	withRealIP := &flamego.Request{Request: httptest.NewRequest(http.MethodGet, "http://example.test", nil)}
	withRealIP.Header.Set("X-Real-IP", "198.51.100.9")

	withRealIP.RemoteAddr = "10.0.0.2:1234"
	if got := getClientIP(withRealIP); got != "198.51.100.9" {
		t.Fatalf("expected X-Real-IP, got %q", got)
	}

	withRemoteAddr := &flamego.Request{Request: httptest.NewRequest(http.MethodGet, "http://example.test", nil)}

	withRemoteAddr.RemoteAddr = "192.0.2.10:8080"
	if got := getClientIP(withRemoteAddr); got != "192.0.2.10" {
		t.Fatalf("expected host from RemoteAddr, got %q", got)
	}

	withRawRemoteAddr := &flamego.Request{Request: httptest.NewRequest(http.MethodGet, "http://example.test", nil)}

	withRawRemoteAddr.RemoteAddr = "not-a-host-port"
	if got := getClientIP(withRawRemoteAddr); got != "not-a-host-port" {
		t.Fatalf("expected raw RemoteAddr fallback, got %q", got)
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d    time.Duration
		out  string
		name string
	}{
		{name: "expired", d: -time.Second, out: "expired"},
		{name: "zero", d: 0, out: "in 0m"},
		{name: "minutes", d: 32 * time.Minute, out: "in 32m"},
		{name: "hours minutes", d: 2*time.Hour + 15*time.Minute, out: "in 2h 15m"},
		{name: "days hours", d: 26 * time.Hour, out: "in 1d 2h"},
		{name: "days minutes", d: 24*time.Hour + 5*time.Minute, out: "in 1d 5m"},
	}

	for _, tt := range tests {
		if got := formatDuration(tt.d); got != tt.out {
			t.Fatalf("%s: formatDuration(%v) = %q, want %q", tt.name, tt.d, got, tt.out)
		}
	}
}

func TestBuildExternalURL(t *testing.T) {
	t.Parallel()

	fromForwarded := &flamego.Request{Request: httptest.NewRequest(http.MethodGet, "http://internal.local", nil)}
	fromForwarded.Header.Set("X-Forwarded-Proto", "https, http")
	fromForwarded.Header.Set("X-Forwarded-Host", "gw.example.com")

	if got := buildExternalURL(fromForwarded, "security"); got != "https://gw.example.com/security" {
		t.Fatalf("unexpected forwarded URL: %q", got)
	}

	fromTLS := &flamego.Request{Request: httptest.NewRequest(http.MethodGet, "https://example.test", nil)}

	fromTLS.TLS = &tls.ConnectionState{}
	if got := buildExternalURL(fromTLS, "/setup"); got != "https://example.test/setup" {
		t.Fatalf("unexpected TLS URL: %q", got)
	}

	withoutHost := &flamego.Request{Request: httptest.NewRequest(http.MethodGet, "http://example.test", nil)}

	withoutHost.Host = ""
	if got := buildExternalURL(withoutHost, "/setup"); got != "/setup" {
		t.Fatalf("expected path fallback, got %q", got)
	}
}

func TestSensitiveAccessHelpers(t *testing.T) {
	t.Parallel()

	now := mustParseTime(t, "2026-02-11T10:00:00Z")
	s := newTestSession()

	if HasSensitiveAccess(s, now) {
		t.Fatal("expected no sensitive access when timestamp is missing")
	}

	s.Set(sensitiveAccessSessionKey, now.Add(-10*time.Minute).Unix())

	if !HasSensitiveAccess(s, now) {
		t.Fatal("expected sensitive access within window")
	}

	s.Set(sensitiveAccessSessionKey, now.Add(-sensitiveAccessWindow-time.Second).Unix())

	if HasSensitiveAccess(s, now) {
		t.Fatal("expected sensitive access to expire")
	}

	s.Set(sensitiveAccessSessionKey, int(now.Unix()))

	if _, ok := getSensitiveAccessTime(s); !ok {
		t.Fatal("expected int timestamp to be supported")
	}

	s.Set(sensitiveAccessSessionKey, float64(now.Unix()))

	if _, ok := getSensitiveAccessTime(s); !ok {
		t.Fatal("expected float64 timestamp to be supported")
	}

	timeCopy := now
	s.Set(sensitiveAccessSessionKey, &timeCopy)

	if got, ok := getSensitiveAccessTime(s); !ok || !got.Equal(now) {
		t.Fatalf("expected *time.Time timestamp, got %v ok=%v", got, ok)
	}

	s.Set(sensitiveAccessSessionKey, &time.Time{})

	if _, ok := getSensitiveAccessTime(s); !ok {
		t.Fatal("expected non-nil *time.Time to be accepted")
	}

	var nilTime *time.Time
	s.Set(sensitiveAccessSessionKey, nilTime)

	if _, ok := getSensitiveAccessTime(s); ok {
		t.Fatal("expected nil *time.Time to be rejected")
	}

	s.Set(sensitiveAccessSessionKey, struct{}{})

	if _, ok := getSensitiveAccessTime(s); ok {
		t.Fatal("expected unknown type to be rejected")
	}

	if got := getSessionDisplayName(s); got != "Admin" {
		t.Fatalf("expected default display name, got %q", got)
	}

	s.Set("user_display_name", "Humaid")

	if got := getSessionDisplayName(s); got != "Humaid" {
		t.Fatalf("expected display name from session, got %q", got)
	}
}

func TestLockSensitiveAccessRedirectsHomeForSensitivePages(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())

	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.Next()
	})
	f.Post("/sensitive-access/lock", func(sess session.Session, c flamego.Context) {
		LockSensitiveAccess(sess, c)
	})

	req := httptest.NewRequest(http.MethodPost, "/sensitive-access/lock", strings.NewReader("requires_sensitive_access=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "/timeline")

	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("expected redirect to '/', got %q", got)
	}

	if s.Get(sensitiveAccessSessionKey) != nil {
		t.Fatal("expected sensitive access session key to be cleared")
	}
}

func TestLockSensitiveAccessRedirectsToReferrerWhenPageNotSensitive(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	s.Set(sensitiveAccessSessionKey, time.Now().Unix())

	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.Next()
	})
	f.Post("/sensitive-access/lock", func(sess session.Session, c flamego.Context) {
		LockSensitiveAccess(sess, c)
	})

	req := httptest.NewRequest(http.MethodPost, "/sensitive-access/lock", strings.NewReader("requires_sensitive_access=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://example.test/contacts?filter=no_phone")

	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "/contacts?filter=no_phone" {
		t.Fatalf("expected redirect to sanitized referrer, got %q", got)
	}

	if s.Get(sensitiveAccessSessionKey) != nil {
		t.Fatal("expected sensitive access session key to be cleared")
	}
}

func TestSanitizeNextPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want string
	}{
		{raw: "", want: "/contacts"},
		{raw: "not/absolute", want: "/contacts"},
		{raw: "//double", want: "/contacts"},
		{raw: "/health?id=1", want: "/health?id=1"},
		{raw: "https://evil.example/health?x=1", want: "/health?x=1"},
		{raw: "https://evil.example", want: "/"},
		{raw: "/safe\npath", want: "/contacts"},
	}

	for _, tt := range tests {
		raw := strings.ReplaceAll(tt.raw, "\\n", "\n")
		if got := sanitizeNextPath(raw); got != tt.want {
			t.Fatalf("sanitizeNextPath(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestContactHelperFunctions(t *testing.T) {
	t.Parallel()

	if !isValidPhone("+1 (650) 555-0123") {
		t.Fatal("expected valid phone")
	}

	if isValidPhone("12-34") {
		t.Fatal("expected invalid short phone")
	}

	if got := isoWeekStart(2024).Format("2006-01-02"); got != "2024-01-01" {
		t.Fatalf("unexpected ISO week start: %s", got)
	}

	for count, want := range map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 5: 2, 6: 3, 9: 3, 10: 4} {
		if got := activityLevel(count); got != want {
			t.Fatalf("activityLevel(%d) = %d, want %d", count, got, want)
		}
	}

	if got := activityLabel(1); got != "activity" {
		t.Fatalf("unexpected activity label for 1: %q", got)
	}

	if got := activityLabel(2); got != "activities" {
		t.Fatalf("unexpected activity label for 2: %q", got)
	}

	weekStart := isoWeekStart(2026)
	weekKey := weekStart.Format("2006-01-02")

	rows := buildActivityGrid(map[string]int{weekKey: 1}, 2026, 1)
	if len(rows) != 1 || len(rows[0].Weeks) != 52 {
		t.Fatalf("unexpected grid dimensions: years=%d weeks=%d", len(rows), len(rows[0].Weeks))
	}

	if rows[0].Weeks[0].Count != 1 || rows[0].Weeks[0].Level != 1 {
		t.Fatalf("unexpected first week: %#v", rows[0].Weeks[0])
	}

	if !strings.Contains(rows[0].Weeks[0].Tooltip, "1 activity") {
		t.Fatalf("unexpected tooltip: %q", rows[0].Weeks[0].Tooltip)
	}

	if got := parseChatPlatform("  slack "); got != db.ChatPlatformSlack {
		t.Fatalf("unexpected chat platform: %q", got)
	}

	if got := parseChatPlatform("unknown"); got != db.ChatPlatformManual {
		t.Fatalf("unexpected fallback platform: %q", got)
	}

	if got := parseChatSender(" me "); got != db.ChatSenderMe {
		t.Fatalf("unexpected chat sender me: %q", got)
	}

	if got := parseChatSender("mix"); got != db.ChatSenderMix {
		t.Fatalf("unexpected chat sender mix: %q", got)
	}

	if got := parseChatSender("other"); got != db.ChatSenderThem {
		t.Fatalf("unexpected fallback sender: %q", got)
	}

	for input, want := range map[string]bool{"on": true, "true": true, "1": true, "yes": false, "": false} {
		if got := isPrimaryChecked(input); got != want {
			t.Fatalf("isPrimaryChecked(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestInventoryHelperFunctions(t *testing.T) {
	t.Parallel()

	if got := getOptionalString("  "); got != nil {
		t.Fatalf("expected nil optional string, got %v", *got)
	}

	if got := getOptionalString(" value "); got == nil || *got != "value" {
		t.Fatalf("unexpected optional string: %#v", got)
	}

	if !isValidFilename("manual.pdf") {
		t.Fatal("expected normal filename to be valid")
	}

	if isValidFilename("../passwd") || isValidFilename("a/b.txt") || isValidFilename(`a\\b.txt`) {
		t.Fatal("expected traversal filename to be invalid")
	}

	for _, status := range []db.InventoryStatus{
		db.InventoryStatusActive,
		db.InventoryStatusStored,
		db.InventoryStatusDamaged,
		db.InventoryStatusMaintenanceRequired,
		db.InventoryStatusGiven,
		db.InventoryStatusDisposed,
		db.InventoryStatusLost,
	} {
		if !isValidInventoryStatus(status) {
			t.Fatalf("expected status %q to be valid", status)
		}
	}

	if isValidInventoryStatus(db.InventoryStatus("nope")) {
		t.Fatal("expected unknown status to be invalid")
	}

	filename := "report\"2026\n.txt\r"

	sanitized := sanitizeFilenameForHeader(filename)
	if strings.Contains(sanitized, "\n") || strings.Contains(sanitized, "\r") {
		t.Fatalf("expected newline characters to be removed: %q", sanitized)
	}

	if !strings.Contains(sanitized, `\"`) {
		t.Fatalf("expected quote to be escaped: %q", sanitized)
	}
}

func TestFilesHelperFunctions(t *testing.T) {
	t.Parallel()

	if got, ok := sanitizeFilesPath("  docs/reference "); !ok || got != "docs/reference" {
		t.Fatalf("expected sanitized path, got %q ok=%v", got, ok)
	}

	if _, ok := sanitizeFilesPath("../etc"); ok {
		t.Fatal("expected parent traversal to be rejected")
	}

	if _, ok := sanitizeFilesPath(".hidden/file"); ok {
		t.Fatal("expected hidden path segment to be rejected")
	}

	if _, ok := sanitizeFilesPath(`docs\\notes`); ok {
		t.Fatal("expected backslash path to be rejected")
	}

	if got := formatFilesPathDisplay(""); got != "/" {
		t.Fatalf("unexpected root display path: %q", got)
	}

	if got := formatFilesPathDisplay("docs"); got != "/docs" {
		t.Fatalf("unexpected display path: %q", got)
	}

	if got := parentFilesPath("docs/reference"); got != "docs" {
		t.Fatalf("unexpected parent path: %q", got)
	}

	if got := parentFilesPath("docs"); got != "" {
		t.Fatalf("unexpected top-level parent path: %q", got)
	}

	if got := filesRedirectPath(""); got != "/files" {
		t.Fatalf("unexpected files root redirect: %q", got)
	}

	if got := filesRedirectPath("docs/a b"); got != "/files?path=docs%2Fa+b" {
		t.Fatalf("unexpected files redirect with query escape: %q", got)
	}

	breadcrumbs := buildFilesBreadcrumbs("docs/reference")
	if len(breadcrumbs) != 3 {
		t.Fatalf("unexpected breadcrumb count: %d", len(breadcrumbs))
	}

	if breadcrumbs[0].Name != "Files" || breadcrumbs[2].IsCurrent != true {
		t.Fatalf("unexpected breadcrumbs: %#v", breadcrumbs)
	}

	for filename, want := range map[string]string{
		"readme.md":   "markdown",
		"photo.JPG":   "image",
		"video.mkv":   "video",
		"audio.mp3":   "audio",
		"doc.pdf":     "pdf",
		"config.toml": "text",
		"archive.bin": "unknown",
		"noext":       "unknown",
	} {
		if got := filesViewerType(filename); got != want {
			t.Fatalf("filesViewerType(%q) = %q, want %q", filename, got, want)
		}
	}

	for value, want := range map[string]bool{"1": true, "TRUE": true, "yes": true, "0": false, "": false} {
		if got := isDownloadRequested(value); got != want {
			t.Fatalf("isDownloadRequested(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestSessionAuthInfo(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	if authenticated, userID := sessionAuthInfo(s); authenticated || userID != "" {
		t.Fatalf("expected unauthenticated session, got authenticated=%v userID=%q", authenticated, userID)
	}

	s.Set("authenticated", true)

	userID := uuid.NewString()
	s.Set("user_id", userID)

	if authenticated, gotUserID := sessionAuthInfo(s); !authenticated || gotUserID != userID {
		t.Fatalf("expected authenticated session with user id, got authenticated=%v userID=%q", authenticated, gotUserID)
	}
}
