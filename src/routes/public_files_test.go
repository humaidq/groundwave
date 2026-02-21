// SPDX-FileCopyrightText: 2026 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

type publicFilesTemplateStub struct {
	called bool
	status int
	name   string
}

func (s *publicFilesTemplateStub) HTML(status int, name string) {
	s.called = true
	s.status = status
	s.name = name
}

func newPublicFilesTestApp(tmpl template.Template, data template.Data) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(tmpl, (*template.Template)(nil))
		c.Map(data)
		c.Next()
	})

	f.Get("/f/raw/{path: **}", func(c flamego.Context) {
		PublicFilesRaw(c)
	})
	f.Get("/f/preview/{path: **}", func(c flamego.Context) {
		PublicFilesPreview(c)
	})
	f.Get("/f/{path: **}", func(c flamego.Context, t template.Template, d template.Data) {
		PublicFilesView(c, t, d)
	})

	return f
}

func TestPublicFilesViewRendersTextPreview(t *testing.T) {
	t.Setenv(publicSiteTitleEnvVar, "Shared Files")

	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
	})

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		return db.WebDAVEntry{
			Name:    "document.txt",
			Size:    int64(len("hello world")),
			ModTime: time.Unix(1735689600, 0),
		}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		return []byte("hello world"), "text/plain", nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/document.txt", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !tpl.called || tpl.name != "files_public_view" {
		t.Fatalf("unexpected template render: %#v", tpl)
	}

	hideNav, _ := data["HideNav"].(bool)
	if !hideNav {
		t.Fatal("expected HideNav to be true")
	}

	if got, _ := data["PageTitle"].(string); got != "Shared Files" {
		t.Fatalf("expected page title %q, got %q", "Shared Files", got)
	}

	if got, _ := data["ViewerType"].(string); got != "text" {
		t.Fatalf("expected viewer type text, got %q", got)
	}

	if got, _ := data["FileText"].(string); got != "hello world" {
		t.Fatalf("expected text preview content, got %q", got)
	}

	if got, _ := data["FileURL"].(string); got != "" {
		t.Fatalf("expected empty file url for text preview, got %q", got)
	}

	if got, _ := data["DownloadURL"].(string); got != "/f/raw/document.txt" {
		t.Fatalf("expected download url %q, got %q", "/f/raw/document.txt", got)
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesViewRendersPDFPreviewURL(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
	})

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		return db.WebDAVEntry{
			Name:    "manual.pdf",
			Size:    int64(4096),
			ModTime: time.Unix(1735689600, 0),
		}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		return nil, "", nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/manual.pdf", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got, _ := data["ViewerType"].(string); got != "pdf" {
		t.Fatalf("expected viewer type pdf, got %q", got)
	}

	if got, _ := data["FileURL"].(string); got != "/f/preview/manual.pdf" {
		t.Fatalf("expected preview file url %q, got %q", "/f/preview/manual.pdf", got)
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesViewReturnsNotFoundForMissingFile(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
	})

	statCalled := false

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		statCalled = true

		return db.WebDAVEntry{}, db.ErrWebDAVFilesEntryNotFound
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		t.Fatal("expected file fetch to be skipped for missing file")

		return nil, "", nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/missing.txt", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", rec.Body.String())
	}

	if tpl.called {
		t.Fatalf("expected template render to be skipped, got %#v", tpl)
	}

	if !statCalled {
		t.Fatal("expected metadata lookup for non-empty path")
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesViewReturnsNotFoundForEmptyPath(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
	})

	statCalled := false

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		statCalled = true

		return db.WebDAVEntry{Name: "ignored.txt", IsDir: false}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		t.Fatal("expected file fetch to be skipped for empty path")

		return nil, "", nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", rec.Body.String())
	}

	if tpl.called {
		t.Fatalf("expected template render to be skipped, got %#v", tpl)
	}

	if statCalled {
		t.Fatal("expected metadata lookup to be skipped for empty path")
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesRawDownloadHeaders(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn
	originalOpenStreamFn := publicFilesOpenStreamFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
		publicFilesOpenStreamFn = originalOpenStreamFn
	})

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		return db.WebDAVEntry{Name: "document.txt", Size: 3, IsDir: false}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		t.Fatal("expected raw download to use streaming open helper")

		return nil, "", nil
	}
	publicFilesOpenStreamFn = func(context.Context, string, string, string) (db.WebDAVFileStream, error) {
		return db.WebDAVFileStream{
			Reader:      io.NopCloser(bytes.NewReader([]byte("abc"))),
			ContentType: "text/plain",
			StatusCode:  http.StatusOK,
		}, nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/raw/document.txt?download=0", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename=\"document.txt\"" {
		t.Fatalf("unexpected content disposition: %q", got)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("unexpected content type: %q", got)
	}

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}

	if got := rec.Body.String(); got != "abc" {
		t.Fatalf("unexpected response body: %q", got)
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesPreviewServesInlinePDF(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn
	originalOpenStreamFn := publicFilesOpenStreamFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
		publicFilesOpenStreamFn = originalOpenStreamFn
	})

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		return db.WebDAVEntry{Name: "manual.pdf", Size: int64(len("%PDF-1.4")), IsDir: false}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		t.Fatal("expected preview to use streaming open helper")

		return nil, "", nil
	}
	publicFilesOpenStreamFn = func(context.Context, string, string, string) (db.WebDAVFileStream, error) {
		return db.WebDAVFileStream{
			Reader:      io.NopCloser(bytes.NewReader([]byte("%PDF-1.4"))),
			ContentType: "application/pdf",
			StatusCode:  http.StatusOK,
		}, nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/preview/manual.pdf", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Disposition"); got != "inline; filename=\"manual.pdf\"" {
		t.Fatalf("unexpected content disposition: %q", got)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("unexpected content type: %q", got)
	}

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff header, got %q", got)
	}

	if got := rec.Body.String(); got != "%PDF-1.4" {
		t.Fatalf("unexpected response body: %q", got)
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesPreviewPassesThroughRangeResponse(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalOpenStreamFn := publicFilesOpenStreamFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesOpenStreamFn = originalOpenStreamFn
	})

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		return db.WebDAVEntry{Name: "clip.mp4", IsDir: false}, nil
	}

	publicFilesOpenStreamFn = func(_ context.Context, _ string, rangeHeader string, ifRangeHeader string) (db.WebDAVFileStream, error) {
		if rangeHeader != "bytes=5-8" {
			t.Fatalf("expected range header %q, got %q", "bytes=5-8", rangeHeader)
		}

		if ifRangeHeader != "\"etag-123\"" {
			t.Fatalf("expected if-range header %q, got %q", "\"etag-123\"", ifRangeHeader)
		}

		return db.WebDAVFileStream{
			Reader:        io.NopCloser(bytes.NewReader([]byte("data"))),
			ContentType:   "video/mp4",
			StatusCode:    http.StatusPartialContent,
			AcceptRanges:  "bytes",
			ContentRange:  "bytes 5-8/100",
			ContentLength: "4",
			ETag:          "\"etag-123\"",
			LastModified:  "Wed, 21 Oct 2015 07:28:00 GMT",
		}, nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/preview/clip.mp4", nil)
	req.Header.Set("Range", "bytes=5-8")
	req.Header.Set("If-Range", "\"etag-123\"")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", http.StatusPartialContent, rec.Code)
	}

	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("unexpected accept-ranges: %q", got)
	}

	if got := rec.Header().Get("Content-Range"); got != "bytes 5-8/100" {
		t.Fatalf("unexpected content-range: %q", got)
	}

	if got := rec.Header().Get("Content-Length"); got != "4" {
		t.Fatalf("unexpected content-length: %q", got)
	}

	if got := rec.Header().Get("ETag"); got != "\"etag-123\"" {
		t.Fatalf("unexpected etag: %q", got)
	}

	if got := rec.Header().Get("Last-Modified"); got != "Wed, 21 Oct 2015 07:28:00 GMT" {
		t.Fatalf("unexpected last-modified: %q", got)
	}

	if got := rec.Body.String(); got != "data" {
		t.Fatalf("unexpected response body: %q", got)
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesRawRejectsDeepEncodedTraversal(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
	})

	statCalled := false

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		statCalled = true

		return db.WebDAVEntry{Name: "secret.txt", IsDir: false}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		return []byte("secret"), "text/plain", nil
	}

	tpl := &publicFilesTemplateStub{}
	data := template.Data{}
	f := newPublicFilesTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/f/raw/%2525252f..%2525252fsecret.txt", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	if statCalled {
		t.Fatal("expected metadata lookup to be skipped for invalid path")
	}
}

func TestSanitizePublicFilesPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
		ok   bool
	}{
		{name: "single file", raw: "document.txt", want: "document.txt", ok: true},
		{name: "nested file", raw: "docs/report.pdf", want: "docs/report.pdf", ok: true},
		{name: "encoded slash", raw: "docs%2Freport.pdf", want: "docs/report.pdf", ok: true},
		{name: "empty", raw: "", want: "", ok: false},
		{name: "dot segment", raw: "./doc.txt", want: "", ok: false},
		{name: "parent traversal segment", raw: "../doc.txt", want: "", ok: false},
		{name: "encoded traversal", raw: "%2e%2e/doc.txt", want: "", ok: false},
		{name: "double encoded absolute path", raw: "%252fetc%252fpasswd", want: "", ok: false},
		{name: "double encoded parent traversal", raw: "%252f..%252fsecret.txt", want: "", ok: false},
		{name: "double encoded embedded traversal", raw: "docs%252f..%252fsecret.txt", want: "", ok: false},
		{name: "double encoded hidden segment", raw: "%252f.secret%252fdoc.txt", want: "", ok: false},
		{name: "double encoded hidden marker", raw: "%252fprivate%252f.gw_admin", want: "", ok: false},
		{name: "double encoded windows separator", raw: "%255cetc%255cpasswd", want: "", ok: false},
		{name: "triple encoded parent traversal", raw: "%25252f..%25252fsecret.txt", want: "", ok: false},
		{name: "quad encoded parent traversal", raw: "%2525252f..%2525252fsecret.txt", want: "", ok: false},
		{name: "literal percent after decoding", raw: "profit%2525report.txt", want: "", ok: false},
		{name: "encoded newline", raw: "docs%0Areport.txt", want: "", ok: false},
		{name: "hidden segment", raw: ".secret/doc.txt", want: "", ok: false},
		{name: "windows separator", raw: `docs\\doc.txt`, want: "", ok: false},
		{name: "double slash", raw: "docs//doc.txt", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := sanitizePublicFilesPath(tt.raw)
			if ok != tt.ok {
				t.Fatalf("sanitizePublicFilesPath(%q) ok = %v, want %v", tt.raw, ok, tt.ok)
			}

			if got != tt.want {
				t.Fatalf("sanitizePublicFilesPath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
