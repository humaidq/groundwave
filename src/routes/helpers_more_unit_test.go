package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/flamego/flamego"

	"github.com/humaidq/groundwave/db"
)

func kvPairsToMap(values []interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for i := 0; i+1 < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			continue
		}
		result[key] = values[i+1]
	}
	return result
}

func TestExtensionTokenStoreAddHas(t *testing.T) {
	t.Parallel()

	store := newExtensionTokenStore()
	if store.Has("token") {
		t.Fatal("expected empty token store to miss token")
	}

	store.Add("token")
	if !store.Has("token") {
		t.Fatal("expected token to be present after Add")
	}
}

func TestGenerateExtensionToken(t *testing.T) {
	t.Parallel()

	tokenA, err := generateExtensionToken()
	if err != nil {
		t.Fatalf("generateExtensionToken returned error: %v", err)
	}
	tokenB, err := generateExtensionToken()
	if err != nil {
		t.Fatalf("generateExtensionToken returned error: %v", err)
	}

	if tokenA == tokenB {
		t.Fatal("expected generated tokens to differ")
	}
	if len(tokenA) != 43 {
		t.Fatalf("expected token length 43, got %d", len(tokenA))
	}
	if strings.Contains(tokenA, "=") {
		t.Fatalf("expected raw base64url token without padding, got %q", tokenA)
	}

	re := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	if !re.MatchString(tokenA) || !re.MatchString(tokenB) {
		t.Fatalf("expected URL-safe tokens, got %q and %q", tokenA, tokenB)
	}
}

func TestHasValidExtensionToken(t *testing.T) {
	originalStore := extTokens
	extTokens = newExtensionTokenStore()
	defer func() {
		extTokens = originalStore
	}()

	f := flamego.New()
	f.Get("/", func(c flamego.Context) {
		if hasValidExtensionToken(c) {
			c.ResponseWriter().WriteHeader(http.StatusNoContent)
			return
		}
		c.ResponseWriter().WriteHeader(http.StatusUnauthorized)
	})

	missingReq := httptest.NewRequest(http.MethodGet, "/", nil)
	missingRec := httptest.NewRecorder()
	f.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for missing token, got %d", missingRec.Code)
	}

	const token = "token-123"
	extTokens.Add(token)
	validReq := httptest.NewRequest(http.MethodGet, "/", nil)
	validReq.Header.Set(extensionTokenHeader, "  "+token+"  ")
	validRec := httptest.NewRecorder()
	f.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusNoContent {
		t.Fatalf("expected success for valid token, got %d", validRec.Code)
	}
}

func TestWriteExtensionAuthError(t *testing.T) {
	t.Parallel()

	f := flamego.New()
	f.Get("/", func(c flamego.Context) {
		writeExtensionAuthError(c, http.StatusUnauthorized)
	})

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content type: %q", ct)
	}

	var payload map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if valid, ok := payload["valid"]; !ok || valid {
		t.Fatalf("expected valid=false payload, got %#v", payload)
	}
}

func TestAddExtensionCORSHeaders(t *testing.T) {
	t.Parallel()

	f := flamego.New()
	f.Get("/", func(c flamego.Context) {
		addExtensionCORSHeaders(c)
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

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

func TestRequestLoggingHelpers(t *testing.T) {
	t.Parallel()

	f := flamego.New()
	s := newTestSession()
	s.Set("authenticated", true)
	s.Set("user_id", "user-123")

	var fields []interface{}
	var ipFromContext string
	f.Get("/resource", func(c flamego.Context) {
		fields = baseRequestFields(c, s)
		ipFromContext = clientIP(c)
		c.ResponseWriter().WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/resource?view=full", nil)
	req.Header.Set("X-Forwarded-For", " 203.0.113.10, 198.51.100.20")
	req.Header.Set("User-Agent", "groundwave-test")
	req.RemoteAddr = "198.51.100.1:9876"
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if ipFromContext != "203.0.113.10" {
		t.Fatalf("unexpected client IP from context: %q", ipFromContext)
	}

	mapped := kvPairsToMap(fields)
	if got := mapped["method"]; got != http.MethodGet {
		t.Fatalf("unexpected request method field: %#v", got)
	}
	if got := mapped["path"]; got != "/resource" {
		t.Fatalf("unexpected request path field: %#v", got)
	}
	if got := mapped["ip"]; got != "203.0.113.10" {
		t.Fatalf("unexpected request ip field: %#v", got)
	}
	if got := mapped["user_agent"]; got != "groundwave-test" {
		t.Fatalf("unexpected request user_agent field: %#v", got)
	}
	if got, ok := mapped["authenticated"].(bool); !ok || !got {
		t.Fatalf("unexpected authenticated field: %#v", mapped["authenticated"])
	}
	if got := mapped["user_id"]; got != "user-123" {
		t.Fatalf("unexpected user_id field: %#v", got)
	}

	// Ensure clientIP falls back to remote address when X-Forwarded-For is absent.
	fallbackReq := httptest.NewRequest(http.MethodGet, "/resource", nil)
	fallbackReq.RemoteAddr = "198.51.100.55:2222"
	fallbackRec := httptest.NewRecorder()
	f.ServeHTTP(fallbackRec, fallbackReq)
	if ipFromContext != "198.51.100.55" {
		t.Fatalf("unexpected fallback client IP: %q", ipFromContext)
	}
}

func TestParseChatPlatformAllSupportedValues(t *testing.T) {
	t.Parallel()

	tests := map[string]db.ChatPlatform{
		"email":    db.ChatPlatformEmail,
		"whatsapp": db.ChatPlatformWhatsApp,
		"signal":   db.ChatPlatformSignal,
		"wechat":   db.ChatPlatformWeChat,
		"teams":    db.ChatPlatformTeams,
		"slack":    db.ChatPlatformSlack,
		"other":    db.ChatPlatformOther,
	}

	for input, want := range tests {
		if got := parseChatPlatform(input); got != want {
			t.Fatalf("parseChatPlatform(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGetSessionUserID(t *testing.T) {
	t.Parallel()

	s := newTestSession()
	if _, ok := getSessionUserID(s); ok {
		t.Fatal("expected missing user_id to return false")
	}

	s.Set("user_id", 123)
	if _, ok := getSessionUserID(s); ok {
		t.Fatal("expected non-string user_id to return false")
	}

	s.Set("user_id", "")
	if _, ok := getSessionUserID(s); ok {
		t.Fatal("expected empty user_id to return false")
	}

	s.Set("user_id", "abc-123")
	if userID, ok := getSessionUserID(s); !ok || userID != "abc-123" {
		t.Fatalf("unexpected session user id result: %q ok=%v", userID, ok)
	}
}

func TestSplitEnvList(t *testing.T) {
	t.Parallel()

	if got := splitEnvList(" one, , two,,three "); !slices.Equal(got, []string{"one", "two", "three"}) {
		t.Fatalf("unexpected splitEnvList values: %#v", got)
	}

	if got := splitEnvList(""); len(got) != 0 {
		t.Fatalf("expected no values for empty input, got %#v", got)
	}
}

func TestWriteJSONHelpers(t *testing.T) {
	t.Parallel()

	f := flamego.New()
	f.Get("/ok", func(c flamego.Context) {
		writeJSON(c, map[string]string{"status": "ok"})
	})
	f.Get("/bad", func(c flamego.Context) {
		writeJSON(c, map[string]interface{}{"bad": func() {}})
	})
	f.Get("/err", func(c flamego.Context) {
		writeJSONError(c, http.StatusBadRequest, "bad request")
	})

	okRec := httptest.NewRecorder()
	f.ServeHTTP(okRec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected status %d for writeJSON, got %d", http.StatusOK, okRec.Code)
	}
	if ct := okRec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content type for writeJSON: %q", ct)
	}
	var okPayload map[string]string
	if err := json.NewDecoder(okRec.Body).Decode(&okPayload); err != nil {
		t.Fatalf("failed decoding writeJSON payload: %v", err)
	}
	if okPayload["status"] != "ok" {
		t.Fatalf("unexpected writeJSON payload: %#v", okPayload)
	}

	badRec := httptest.NewRecorder()
	f.ServeHTTP(badRec, httptest.NewRequest(http.MethodGet, "/bad", nil))
	if badRec.Code != http.StatusOK {
		t.Fatalf("expected status %d for failed writeJSON encode, got %d", http.StatusOK, badRec.Code)
	}
	if ct := badRec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content type for failed writeJSON encode: %q", ct)
	}

	errRec := httptest.NewRecorder()
	f.ServeHTTP(errRec, httptest.NewRequest(http.MethodGet, "/err", nil))
	if errRec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d for writeJSONError, got %d", http.StatusBadRequest, errRec.Code)
	}
	var errPayload map[string]string
	if err := json.NewDecoder(errRec.Body).Decode(&errPayload); err != nil {
		t.Fatalf("failed decoding writeJSONError payload: %v", err)
	}
	if errPayload["error"] != "bad request" {
		t.Fatalf("unexpected writeJSONError payload: %#v", errPayload)
	}
}

func TestParseOQRSPath(t *testing.T) {
	t.Parallel()

	callsign, timestampStr, timestamp, ok := parseOQRSPath("A%2FB-1700000000")
	if !ok {
		t.Fatal("expected parseOQRSPath success")
	}
	if callsign != "A/B" {
		t.Fatalf("unexpected callsign: %q", callsign)
	}
	if timestampStr != "1700000000" || timestamp != 1700000000 {
		t.Fatalf("unexpected timestamp values: %q %d", timestampStr, timestamp)
	}

	invalid := []string{"", "no-dash", "bad%ZZ-1700000000", "ABCD-not-a-number"}
	for _, value := range invalid {
		if _, _, _, ok := parseOQRSPath(value); ok {
			t.Fatalf("expected parseOQRSPath to fail for %q", value)
		}
	}
}

func TestParseOQRSTime(t *testing.T) {
	t.Parallel()

	parsed, err := parseOQRSTime("2026", "02", "11", "23", "45")
	if err != nil {
		t.Fatalf("expected valid OQRS time parse, got error: %v", err)
	}
	if parsed.Format(time.RFC3339) != "2026-02-11T23:45:00Z" {
		t.Fatalf("unexpected parsed time: %s", parsed.Format(time.RFC3339))
	}

	invalidInputs := [][]string{
		{"1999", "01", "01", "00", "00"}, // year
		{"2026", "13", "01", "00", "00"}, // month
		{"2026", "02", "31", "00", "00"}, // invalid calendar date
		{"2026", "02", "11", "24", "00"}, // hour
		{"2026", "02", "11", "23", "60"}, // minute
		{"abcd", "02", "11", "23", "45"}, // parse error
	}
	for _, input := range invalidInputs {
		if _, err := parseOQRSTime(input[0], input[1], input[2], input[3], input[4]); err == nil {
			t.Fatalf("expected parseOQRSTime to fail for %#v", input)
		}
	}
}

func TestFormatTimeAgo(t *testing.T) {
	t.Parallel()

	if got := formatTimeAgo(time.Now().Add(10 * time.Second)); got != "just now" {
		t.Fatalf("unexpected future short offset format: %q", got)
	}
	if got := formatTimeAgo(time.Now().Add(-2*time.Minute - 5*time.Second)); got != "2m ago" {
		t.Fatalf("unexpected minute format: %q", got)
	}
	if got := formatTimeAgo(time.Now().Add(-3*time.Hour - 5*time.Minute)); got != "3h ago" {
		t.Fatalf("unexpected hour format: %q", got)
	}
	if got := formatTimeAgo(time.Now().Add(-(3*24*time.Hour + 2*time.Hour))); got != "3d ago" {
		t.Fatalf("unexpected day format: %q", got)
	}
	if got := formatTimeAgo(time.Now().Add(-(65*24*time.Hour + time.Hour))); got != "2mo ago" {
		t.Fatalf("unexpected month format: %q", got)
	}
	if got := formatTimeAgo(time.Now().Add(-(800*24*time.Hour + time.Hour))); got != "2y ago" {
		t.Fatalf("unexpected year format: %q", got)
	}
}

func TestQSOTimestampUTC(t *testing.T) {
	t.Parallel()

	if got := qsoTimestampUTC(nil); !got.IsZero() {
		t.Fatalf("expected zero time for nil QSO, got %v", got)
	}

	qso := &db.QSO{
		QSODate: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.FixedZone("GST", 4*60*60)),
		TimeOn:  time.Date(2026, time.January, 1, 18, 30, 45, 0, time.FixedZone("EST", -5*60*60)),
	}

	got := qsoTimestampUTC(qso)
	want := time.Date(2026, time.March, 5, 23, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("unexpected qsoTimestampUTC value: %v, want %v", got, want)
	}
}
