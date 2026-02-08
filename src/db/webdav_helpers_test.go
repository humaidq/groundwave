// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import "testing"

func TestFilesPathHelpers(t *testing.T) {
	config := &WebDAVConfig{FilesPath: "https://example.com/files"}
	base := filesBasePath(config)
	if base != "/files/" {
		t.Fatalf("expected base path /files/, got %q", base)
	}

	if got := filesReadDirTarget(base, ""); got != base {
		t.Fatalf("expected base target for empty path, got %q", got)
	}
	if got := filesReadDirTarget(base, "."); got != base {
		t.Fatalf("expected base target for dot path, got %q", got)
	}
	if got := filesReadDirTarget(base, "/"); got != base {
		t.Fatalf("expected base target for slash path, got %q", got)
	}
	if got := filesReadDirTarget(base, "docs"); got != "/files/docs/" {
		t.Fatalf("expected nested target, got %q", got)
	}

	if got := normalizeFilesPathForCompare("/files/docs/", base); got != "docs" {
		t.Fatalf("expected docs, got %q", got)
	}
	if got := normalizeFilesPathForCompare(".", ""); got != "" {
		t.Fatalf("expected empty path for dot, got %q", got)
	}
	if got := normalizeFilesPathForCompare("/files/docs", "/files"); got != "docs" {
		t.Fatalf("expected docs for alt base, got %q", got)
	}

	if !isListingSelf("/files/docs/", "docs", base) {
		t.Fatalf("expected listing self for directory")
	}
	if isListingSelf("/files/other", "docs", base) {
		t.Fatalf("did not expect listing self for different directory")
	}
}
