// SPDX-FileCopyrightText: 2026 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenWebDAVFileStreamsPassThroughRangeHeaders(t *testing.T) {
	resetDatabase(t)

	const (
		rangeHeader   = "bytes=2-5"
		ifRangeHeader = "\"etag-123\""
	)

	requestCounts := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCounts[r.URL.Path]++

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		if got := r.Header.Get("Range"); got != rangeHeader {
			t.Errorf("unexpected range header for %s: %q", r.URL.Path, got)
		}

		if got := r.Header.Get("If-Range"); got != ifRangeHeader {
			t.Errorf("unexpected if-range header for %s: %q", r.URL.Path, got)
		}

		switch r.URL.Path {
		case "/files/clip.mp4":
			w.Header().Set("Content-Type", "video/mp4")
		case "/public/clip.mp4":
			w.Header().Set("Content-Type", "video/mp4")
		case "/inv/GW-00001/manual.pdf":
			w.Header().Set("Content-Type", "application/pdf")
		default:
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", "bytes 2-5/10")
		w.Header().Set("Content-Length", "4")
		w.Header().Set("ETag", "\"etag-123\"")
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		w.WriteHeader(http.StatusPartialContent)

		if _, err := io.WriteString(w, "2345"); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")
	t.Setenv("WEBDAV_FILES_PATH", server.URL+"/files")
	t.Setenv("WEBDAV_PUBLIC_PATH", server.URL+"/public")
	t.Setenv("WEBDAV_INV_PATH", server.URL+"/inv")
	t.Setenv("WEBDAV_ZK_PATH", server.URL+"/zk/index.org")

	assertStream := func(t *testing.T, stream WebDAVFileStream, wantContentType string) {
		t.Helper()

		if stream.StatusCode != http.StatusPartialContent {
			t.Fatalf("expected status %d, got %d", http.StatusPartialContent, stream.StatusCode)
		}

		if stream.ContentType != wantContentType {
			t.Fatalf("expected content type %q, got %q", wantContentType, stream.ContentType)
		}

		if stream.AcceptRanges != "bytes" {
			t.Fatalf("expected accept-ranges bytes, got %q", stream.AcceptRanges)
		}

		if stream.ContentRange != "bytes 2-5/10" {
			t.Fatalf("expected content-range bytes 2-5/10, got %q", stream.ContentRange)
		}

		if stream.ContentLength != "4" {
			t.Fatalf("expected content-length 4, got %q", stream.ContentLength)
		}

		if stream.ETag != "\"etag-123\"" {
			t.Fatalf("expected etag %q, got %q", "\"etag-123\"", stream.ETag)
		}

		if stream.LastModified != "Wed, 21 Oct 2015 07:28:00 GMT" {
			t.Fatalf("expected last-modified header, got %q", stream.LastModified)
		}

		body, err := io.ReadAll(stream.Reader)
		if err != nil {
			t.Fatalf("failed to read stream body: %v", err)
		}

		if err := stream.Reader.Close(); err != nil {
			t.Fatalf("failed to close stream body: %v", err)
		}

		if got := string(body); got != "2345" {
			t.Fatalf("expected body %q, got %q", "2345", got)
		}
	}

	filesStream, err := OpenFilesFileStream(testContext(), "clip.mp4", rangeHeader, ifRangeHeader)
	if err != nil {
		t.Fatalf("OpenFilesFileStream failed: %v", err)
	}

	assertStream(t, filesStream, "video/mp4")

	publicStream, err := OpenPublicFileStream(testContext(), "clip.mp4", rangeHeader, ifRangeHeader)
	if err != nil {
		t.Fatalf("OpenPublicFileStream failed: %v", err)
	}

	assertStream(t, publicStream, "video/mp4")

	inventoryStream, err := OpenInventoryFileStream(testContext(), "GW-00001", "manual.pdf", rangeHeader, ifRangeHeader)
	if err != nil {
		t.Fatalf("OpenInventoryFileStream failed: %v", err)
	}

	assertStream(t, inventoryStream, "application/pdf")

	if requestCounts["/files/clip.mp4"] != 1 {
		t.Fatalf("expected one request to files path, got %d", requestCounts["/files/clip.mp4"])
	}

	if requestCounts["/public/clip.mp4"] != 1 {
		t.Fatalf("expected one request to public path, got %d", requestCounts["/public/clip.mp4"])
	}

	if requestCounts["/inv/GW-00001/manual.pdf"] != 1 {
		t.Fatalf("expected one request to inventory path, got %d", requestCounts["/inv/GW-00001/manual.pdf"])
	}
}
