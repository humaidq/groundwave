// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
)

func TestWebDAVFilesAndInventory(t *testing.T) {
	resetDatabase(t)

	server := newWebDAVTestServer(t)
	defer server.close()

	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")
	t.Setenv("WEBDAV_INV_PATH", server.server.URL+"/inv")
	t.Setenv("WEBDAV_FILES_PATH", server.server.URL+"/files")
	t.Setenv("WEBDAV_ZK_PATH", server.server.URL+"/zk/index.org")

	if _, err := GetWebDAVConfig(); err != nil {
		t.Fatalf("GetWebDAVConfig failed: %v", err)
	}

	files, err := ListInventoryFiles(testContext(), server.inventoryID)
	if err != nil {
		t.Fatalf("ListInventoryFiles failed: %v", err)
	}

	if len(files) != 1 || files[0].Name != "manual.pdf" {
		t.Fatalf("expected manual.pdf, got %v", files)
	}

	body, contentType, err := FetchInventoryFile(testContext(), server.inventoryID, "manual.pdf")
	if err != nil {
		t.Fatalf("FetchInventoryFile failed: %v", err)
	}

	if len(body) == 0 || contentType == "" {
		t.Fatalf("expected file body and content type")
	}

	entries, err := ListFilesEntries(testContext(), "")
	if err != nil {
		t.Fatalf("ListFilesEntries failed: %v", err)
	}

	if len(entries) == 0 {
		t.Fatalf("expected entries")
	}

	fileBody, fileType, err := FetchFilesFile(testContext(), "readme.txt")
	if err != nil {
		t.Fatalf("FetchFilesFile failed: %v", err)
	}

	if len(fileBody) == 0 || fileType == "" {
		t.Fatalf("expected file body and type")
	}

	restricted, err := IsFilesPathRestricted(testContext(), "private")
	if err != nil {
		t.Fatalf("IsFilesPathRestricted failed: %v", err)
	}

	if !restricted {
		t.Fatalf("expected private path to be restricted")
	}

	adminOnly, err := IsFilesPathAdminOnly(testContext(), "admin")
	if err != nil {
		t.Fatalf("IsFilesPathAdminOnly failed: %v", err)
	}

	if !adminOnly {
		t.Fatalf("expected admin path to be admin-only")
	}

	if extractFilename("/files/readme.txt") != "readme.txt" {
		t.Fatalf("expected filename extraction")
	}

	if !isListingSelf("/files", "", filesBasePath(&WebDAVConfig{FilesPath: server.server.URL + "/files"})) {
		t.Fatalf("expected listing self to be true for base path")
	}

	if normalizeFilesPathForCompare("/files/", filesBasePath(&WebDAVConfig{FilesPath: server.server.URL + "/files"})) == "" {
		t.Fatalf("expected normalized path")
	}

	if inferContentType("test.pdf") == "" {
		t.Fatalf("expected inferred content type")
	}
}
