// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"bytes"
	"errors"
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

func TestWebDAVFilesUploadMoveDelete(t *testing.T) {
	resetDatabase(t)

	server := newWebDAVTestServer(t)
	defer server.close()

	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")
	t.Setenv("WEBDAV_INV_PATH", server.server.URL+"/inv")
	t.Setenv("WEBDAV_FILES_PATH", server.server.URL+"/files")
	t.Setenv("WEBDAV_ZK_PATH", server.server.URL+"/zk/index.org")

	content := []byte("hello")

	if _, err := UploadFilesFile(testContext(), "uploads/hello.txt", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("UploadFilesFile failed: %v", err)
	}

	if _, err := UploadFilesFile(testContext(), "uploads/hello.txt", bytes.NewReader(content), int64(len(content))); !errors.Is(err, ErrWebDAVFilesEntryExists) {
		t.Fatalf("expected upload conflict, got %v", err)
	}

	body, _, err := FetchFilesFile(testContext(), "uploads/hello.txt")
	if err != nil {
		t.Fatalf("FetchFilesFile failed: %v", err)
	}

	if string(body) != "hello" {
		t.Fatalf("unexpected upload contents: %q", string(body))
	}

	if err := MoveFilesEntry(testContext(), "uploads/hello.txt", "uploads/hello-renamed.txt"); err != nil {
		t.Fatalf("MoveFilesEntry rename failed: %v", err)
	}

	if _, _, err := FetchFilesFile(testContext(), "uploads/hello.txt"); err == nil {
		t.Fatalf("expected renamed file to be missing")
	}

	if _, _, err := FetchFilesFile(testContext(), "uploads/hello-renamed.txt"); err != nil {
		t.Fatalf("expected renamed file to exist: %v", err)
	}

	if _, err := UploadFilesFile(testContext(), "uploads/second.txt", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("UploadFilesFile failed: %v", err)
	}

	if err := MoveFilesEntry(testContext(), "uploads/hello-renamed.txt", "uploads/second.txt"); !errors.Is(err, ErrWebDAVFilesEntryExists) {
		t.Fatalf("expected move conflict, got %v", err)
	}

	if err := MoveFilesEntry(testContext(), "uploads/hello-renamed.txt", "archive/hello-renamed.txt"); err != nil {
		t.Fatalf("MoveFilesEntry move failed: %v", err)
	}

	if err := DeleteFilesFile(testContext(), "archive/hello-renamed.txt"); err != nil {
		t.Fatalf("DeleteFilesFile failed: %v", err)
	}

	if err := DeleteFilesFile(testContext(), "archive/hello-renamed.txt"); !errors.Is(err, ErrWebDAVFilesEntryNotFound) {
		t.Fatalf("expected delete missing error, got %v", err)
	}
}

func TestWebDAVFilesDirectoryCreateAndDelete(t *testing.T) {
	resetDatabase(t)

	server := newWebDAVTestServer(t)
	defer server.close()

	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")
	t.Setenv("WEBDAV_INV_PATH", server.server.URL+"/inv")
	t.Setenv("WEBDAV_FILES_PATH", server.server.URL+"/files")
	t.Setenv("WEBDAV_ZK_PATH", server.server.URL+"/zk/index.org")

	if err := CreateFilesDirectory(testContext(), "uploads/new-folder"); err != nil {
		t.Fatalf("CreateFilesDirectory failed: %v", err)
	}

	if err := CreateFilesDirectory(testContext(), "uploads/new-folder"); !errors.Is(err, ErrWebDAVFilesEntryExists) {
		t.Fatalf("expected directory exists error, got %v", err)
	}

	if err := CreateFilesDirectory(testContext(), "missing/new-folder"); !errors.Is(err, ErrWebDAVFilesEntryNotFound) {
		t.Fatalf("expected missing parent error, got %v", err)
	}

	entries, err := ListFilesEntries(testContext(), "uploads")
	if err != nil {
		t.Fatalf("ListFilesEntries failed: %v", err)
	}

	foundDir := false

	for _, entry := range entries {
		if entry.Path == "uploads/new-folder" {
			if !entry.IsDir {
				t.Fatalf("expected uploads/new-folder to be a directory")
			}

			foundDir = true

			break
		}
	}

	if !foundDir {
		t.Fatalf("expected uploads/new-folder to be listed")
	}

	content := []byte("nested")
	if _, err := UploadFilesFile(testContext(), "uploads/new-folder/nested.txt", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("UploadFilesFile failed: %v", err)
	}

	if err := DeleteFilesDirectory(testContext(), "uploads/new-folder"); !errors.Is(err, ErrWebDAVFilesDirectoryNotEmpty) {
		t.Fatalf("expected non-empty directory error, got %v", err)
	}

	if err := DeleteFilesFile(testContext(), "uploads/new-folder/nested.txt"); err != nil {
		t.Fatalf("DeleteFilesFile failed: %v", err)
	}

	if err := DeleteFilesDirectory(testContext(), "uploads/new-folder"); err != nil {
		t.Fatalf("DeleteFilesDirectory failed: %v", err)
	}

	if err := DeleteFilesDirectory(testContext(), "uploads/new-folder"); !errors.Is(err, ErrWebDAVFilesEntryNotFound) {
		t.Fatalf("expected directory not found error, got %v", err)
	}

	if _, err := UploadFilesFile(testContext(), "uploads/plain.txt", bytes.NewReader(content), int64(len(content))); err != nil {
		t.Fatalf("UploadFilesFile failed: %v", err)
	}

	if err := DeleteFilesDirectory(testContext(), "uploads/plain.txt"); !errors.Is(err, ErrWebDAVFilesEntryNotDirectory) {
		t.Fatalf("expected not-directory error, got %v", err)
	}

	if err := DeleteFilesFile(testContext(), "uploads"); !errors.Is(err, ErrWebDAVFilesEntryIsDirectory) {
		t.Fatalf("expected is-directory error, got %v", err)
	}
}

func TestWebDAVFilesUpdateWithOptimisticLocking(t *testing.T) {
	resetDatabase(t)

	server := newWebDAVTestServer(t)
	defer server.close()

	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")
	t.Setenv("WEBDAV_INV_PATH", server.server.URL+"/inv")
	t.Setenv("WEBDAV_FILES_PATH", server.server.URL+"/files")
	t.Setenv("WEBDAV_ZK_PATH", server.server.URL+"/zk/index.org")

	entries, err := ListFilesEntries(testContext(), "")
	if err != nil {
		t.Fatalf("ListFilesEntries failed: %v", err)
	}

	var readmeETag string

	for _, entry := range entries {
		if entry.Path == "readme.txt" {
			readmeETag = entry.ETag
			break
		}
	}

	if readmeETag == "" {
		t.Fatalf("expected readme.txt etag")
	}

	if err := UpdateFilesFile(testContext(), "readme.txt", []byte("updated"), readmeETag); err != nil {
		t.Fatalf("UpdateFilesFile failed: %v", err)
	}

	body, _, err := FetchFilesFile(testContext(), "readme.txt")
	if err != nil {
		t.Fatalf("FetchFilesFile failed: %v", err)
	}

	if string(body) != "updated" {
		t.Fatalf("unexpected updated contents: %q", string(body))
	}

	if err := UpdateFilesFile(testContext(), "readme.txt", []byte("stale write"), readmeETag); !errors.Is(err, ErrWebDAVFilesEntryConflict) {
		t.Fatalf("expected optimistic-lock conflict, got %v", err)
	}

	body, _, err = FetchFilesFile(testContext(), "readme.txt")
	if err != nil {
		t.Fatalf("FetchFilesFile failed: %v", err)
	}

	if string(body) != "updated" {
		t.Fatalf("expected stale write to be rejected, got %q", string(body))
	}

	if err := UpdateFilesFile(testContext(), "readme.txt", []byte("no etag"), " "); !errors.Is(err, ErrWebDAVFilesEntryETagRequired) {
		t.Fatalf("expected missing-etag error, got %v", err)
	}

	if err := UpdateFilesFile(testContext(), "missing.txt", []byte("x"), readmeETag); !errors.Is(err, ErrWebDAVFilesEntryNotFound) {
		t.Fatalf("expected not-found error, got %v", err)
	}

	if err := UpdateFilesFile(testContext(), "uploads", []byte("x"), readmeETag); !errors.Is(err, ErrWebDAVFilesEntryIsDirectory) {
		t.Fatalf("expected is-directory error, got %v", err)
	}
}
