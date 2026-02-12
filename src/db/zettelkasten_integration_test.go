// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import "testing"

func TestZettelkastenWebDAVAndCaches(t *testing.T) {
	resetDatabase(t)

	server := newWebDAVTestServer(t)
	defer server.close()

	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")
	t.Setenv("WEBDAV_ZK_PATH", server.server.URL+"/zk/index.org")
	t.Setenv("WEBDAV_HOME_PATH", server.server.URL+"/zk/home.org")
	t.Setenv("GROUNDWAVE_BASE_URL", "https://groundwave.example.com")

	config, err := GetZKConfig()
	if err != nil {
		t.Fatalf("GetZKConfig failed: %v", err)
	}

	if config.IndexFile != "index.org" {
		t.Fatalf("expected index file to be index.org")
	}

	homeConfig, err := GetHomeConfig()
	if err != nil {
		t.Fatalf("GetHomeConfig failed: %v", err)
	}

	if homeConfig.IndexFile != "home.org" {
		t.Fatalf("expected home index file to be home.org")
	}

	content, err := FetchOrgFile(testContext(), "index.org")
	if err != nil {
		t.Fatalf("FetchOrgFile failed: %v", err)
	}

	if content == "" {
		t.Fatalf("expected index content")
	}

	_, err = FetchDailyOrgFile(testContext(), "2024-01-01.org")
	if err != nil {
		t.Fatalf("FetchDailyOrgFile failed: %v", err)
	}

	files, err := ListOrgFiles(testContext())
	if err != nil {
		t.Fatalf("ListOrgFiles failed: %v", err)
	}

	if len(files) < 2 {
		t.Fatalf("expected org files")
	}

	daily, err := ListDailyOrgFiles(testContext())
	if err != nil {
		t.Fatalf("ListDailyOrgFiles failed: %v", err)
	}

	if len(daily) != 1 {
		t.Fatalf("expected 1 daily file, got %d", len(daily))
	}

	fileByID, err := FindFileByID(testContext(), "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("FindFileByID failed: %v", err)
	}

	if fileByID == "" {
		t.Fatalf("expected file name")
	}

	notes, err := ListZKNotes(testContext())
	if err != nil {
		t.Fatalf("ListZKNotes failed: %v", err)
	}

	if len(notes) == 0 {
		t.Fatalf("expected notes")
	}

	chatNote, err := GetZKNoteForChat(testContext(), "22222222-2222-2222-2222-222222222222")
	if err != nil {
		t.Fatalf("GetZKNoteForChat failed: %v", err)
	}

	if chatNote.Title == "" {
		t.Fatalf("expected chat note title")
	}

	links := ExtractLinksFromContent(content)
	if len(links) == 0 {
		t.Fatalf("expected extracted links")
	}

	if err := BuildBacklinkCache(testContext()); err != nil {
		t.Fatalf("BuildBacklinkCache failed: %v", err)
	}

	if err := BuildJournalCache(testContext()); err != nil {
		t.Fatalf("BuildJournalCache failed: %v", err)
	}

	if err := BuildZKTimelineNotesCache(testContext()); err != nil {
		t.Fatalf("BuildZKTimelineNotesCache failed: %v", err)
	}

	backlinks := GetBacklinksFromCache("33333333-3333-3333-3333-333333333333")
	if len(backlinks) == 0 {
		t.Fatalf("expected backlinks")
	}

	forward := GetForwardLinksFromCache("22222222-2222-2222-2222-222222222222")
	if len(forward) == 0 {
		t.Fatalf("expected forward links")
	}

	if len(GetZKNoteLinks("22222222-2222-2222-2222-222222222222")) == 0 {
		t.Fatalf("expected zk note links")
	}

	contactLinks := GetContactLinksFromCache("44444444-4444-4444-4444-444444444444")
	if len(contactLinks) == 0 {
		t.Fatalf("expected contact links")
	}

	foundContactSource := false
	foundDailySource := false

	for _, sourceID := range contactLinks {
		if sourceID == "22222222-2222-2222-2222-222222222222" {
			foundContactSource = true
		}

		if sourceID == "daily:2024-01-01" {
			foundDailySource = true
		}
	}

	if !foundContactSource {
		t.Fatalf("expected contact link source to include note one")
	}

	if !foundDailySource {
		t.Fatalf("expected contact link source to include daily journal")
	}

	if !IsPublicNoteFromCache("22222222-2222-2222-2222-222222222222") {
		t.Fatalf("expected note to be public")
	}

	entries := GetJournalEntriesFromCache()
	if len(entries) == 0 {
		t.Fatalf("expected journal entries")
	}

	if _, ok := GetJournalEntryByDate("2024-01-01"); !ok {
		t.Fatalf("expected journal entry for date")
	}

	if GetLastJournalCacheBuildTime().IsZero() {
		t.Fatalf("expected last journal build time")
	}

	zkNotes := GetZKTimelineNotesByDate()
	if len(zkNotes) == 0 {
		t.Fatalf("expected zk timeline notes")
	}

	if GetLastZKNoteCacheBuildTime().IsZero() {
		t.Fatalf("expected last zk note build time")
	}

	note, err := GetNoteByIDWithBasePath(testContext(), "22222222-2222-2222-2222-222222222222", "/note")
	if err != nil {
		t.Fatalf("GetNoteByIDWithBasePath failed: %v", err)
	}

	if note.Title == "" {
		t.Fatalf("expected note title")
	}

	indexNote, err := GetIndexNote(testContext())
	if err != nil {
		t.Fatalf("GetIndexNote failed: %v", err)
	}

	if indexNote.Title == "" {
		t.Fatalf("expected index note title")
	}

	homeIndexNote, err := GetHomeIndexNote(testContext())
	if err != nil {
		t.Fatalf("GetHomeIndexNote failed: %v", err)
	}

	if !homeIndexNote.IsHome {
		t.Fatalf("expected home index note to include home access")
	}

	t.Setenv("WEBDAV_HOME_PATH", server.server.URL+"/other/home.org")

	if _, err := GetHomeConfig(); err == nil {
		t.Fatalf("expected GetHomeConfig to fail when parent directories differ")
	}

	if err := RebuildZettelkastenCaches(testContext()); err != nil {
		t.Fatalf("RebuildZettelkastenCaches failed: %v", err)
	}
}
