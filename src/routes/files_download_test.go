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

	"github.com/humaidq/groundwave/db"
)

func newFilesDownloadTestApp(s session.Session) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.Next()
	})

	f.Get("/files/file", func(c flamego.Context, sess session.Session) {
		DownloadFilesFile(c, sess)
	})

	return f
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestDownloadFilesFilePassesThroughRangeResponse(t *testing.T) {
	originalAdminOnlyFn := filesIsPathAdminOnlyFn
	originalRestrictedFn := filesIsPathRestrictedFn
	originalOpenStreamFn := filesOpenFileStreamFn

	t.Cleanup(func() {
		filesIsPathAdminOnlyFn = originalAdminOnlyFn
		filesIsPathRestrictedFn = originalRestrictedFn
		filesOpenFileStreamFn = originalOpenStreamFn
	})

	filesIsPathAdminOnlyFn = func(context.Context, string) (bool, error) {
		return false, nil
	}

	filesIsPathRestrictedFn = func(context.Context, string) (bool, error) {
		return false, nil
	}

	filesOpenFileStreamFn = func(_ context.Context, _ string, rangeHeader string, ifRangeHeader string) (db.WebDAVFileStream, error) {
		if rangeHeader != "bytes=1-3" {
			t.Fatalf("expected range header %q, got %q", "bytes=1-3", rangeHeader)
		}

		if ifRangeHeader != "\"etag-xyz\"" {
			t.Fatalf("expected if-range header %q, got %q", "\"etag-xyz\"", ifRangeHeader)
		}

		return db.WebDAVFileStream{
			Reader:        io.NopCloser(bytes.NewReader([]byte("bcd"))),
			ContentType:   "video/mp4",
			StatusCode:    http.StatusPartialContent,
			AcceptRanges:  "bytes",
			ContentRange:  "bytes 1-3/10",
			ContentLength: "3",
			ETag:          "\"etag-xyz\"",
		}, nil
	}

	s := newTestSession()
	f := newFilesDownloadTestApp(s)

	req := httptest.NewRequest(http.MethodGet, "/files/file?path=video.mp4", nil)
	req.Header.Set("Range", "bytes=1-3")
	req.Header.Set("If-Range", "\"etag-xyz\"")

	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("expected status %d, got %d", http.StatusPartialContent, rec.Code)
	}

	if got := rec.Header().Get("Content-Range"); got != "bytes 1-3/10" {
		t.Fatalf("unexpected content-range header: %q", got)
	}

	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("unexpected accept-ranges header: %q", got)
	}

	if got := rec.Header().Get("ETag"); got != "\"etag-xyz\"" {
		t.Fatalf("unexpected etag header: %q", got)
	}

	if got := rec.Body.String(); got != "bcd" {
		t.Fatalf("unexpected response body: %q", got)
	}
}
