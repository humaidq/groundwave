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

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

type inventoryDownloadTemplateStub struct{}

func (s *inventoryDownloadTemplateStub) HTML(int, string) {}

func newInventoryDownloadTestApp(s session.Session, tmpl template.Template, data template.Data) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.MapTo(tmpl, (*template.Template)(nil))
		c.Map(data)
		c.Next()
	})

	f.Get("/inventory/{id}/file/{filename}", func(c flamego.Context, sess session.Session, t template.Template, d template.Data) {
		DownloadInventoryFile(c, sess, t, d)
	})

	return f
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestDownloadInventoryFilePassesThroughRangeResponse(t *testing.T) {
	originalOpenStreamFn := inventoryOpenFileStreamFn

	t.Cleanup(func() {
		inventoryOpenFileStreamFn = originalOpenStreamFn
	})

	inventoryOpenFileStreamFn = func(_ context.Context, _ string, _ string, rangeHeader string, ifRangeHeader string) (db.WebDAVFileStream, error) {
		if rangeHeader != "bytes=0-2" {
			t.Fatalf("expected range header %q, got %q", "bytes=0-2", rangeHeader)
		}

		if ifRangeHeader != "\"inv-etag\"" {
			t.Fatalf("expected if-range header %q, got %q", "\"inv-etag\"", ifRangeHeader)
		}

		return db.WebDAVFileStream{
			Reader:        io.NopCloser(bytes.NewReader([]byte("abc"))),
			ContentType:   "video/mp4",
			StatusCode:    http.StatusPartialContent,
			AcceptRanges:  "bytes",
			ContentRange:  "bytes 0-2/10",
			ContentLength: "3",
			ETag:          "\"inv-etag\"",
		}, nil
	}

	s := newTestSession()
	tpl := &inventoryDownloadTemplateStub{}
	data := template.Data{}
	f := newInventoryDownloadTestApp(s, tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/inventory/GW-00001/file/clip.mp4", nil)
	req.Header.Set("Range", "bytes=0-2")
	req.Header.Set("If-Range", "\"inv-etag\"")

	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", http.StatusPartialContent, rec.Code)
	}

	if got := rec.Header().Get("Content-Range"); got != "bytes 0-2/10" {
		t.Fatalf("unexpected content-range header: %q", got)
	}

	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("unexpected accept-ranges header: %q", got)
	}

	if got := rec.Header().Get("ETag"); got != "\"inv-etag\"" {
		t.Fatalf("unexpected etag header: %q", got)
	}

	if got := rec.Body.String(); got != "abc" {
		t.Fatalf("unexpected response body: %q", got)
	}
}
