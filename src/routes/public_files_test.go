// SPDX-FileCopyrightText: 2026 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"context"
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

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesViewRendersTextPreview(t *testing.T) {
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
func TestPublicFilesRawDownloadHeaders(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
	})

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		return db.WebDAVEntry{Name: "document.txt", IsDir: false}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		return []byte("abc"), "text/plain", nil
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
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestPublicFilesPreviewServesInlinePDF(t *testing.T) {
	originalStatFn := publicFilesStatFn
	originalFetchFn := publicFilesFetchFn

	t.Cleanup(func() {
		publicFilesStatFn = originalStatFn
		publicFilesFetchFn = originalFetchFn
	})

	publicFilesStatFn = func(context.Context, string) (db.WebDAVEntry, error) {
		return db.WebDAVEntry{Name: "manual.pdf", IsDir: false}, nil
	}
	publicFilesFetchFn = func(context.Context, string) ([]byte, string, error) {
		return []byte("%PDF-1.4"), "application/pdf", nil
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
