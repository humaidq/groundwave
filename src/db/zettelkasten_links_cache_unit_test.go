// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"strings"
	"testing"
	"time"
)

func TestContactLinkMatchers(t *testing.T) {
	idProtocol := "ABCDEF12-3456-7890-ABCD-EF1234567890"
	idRelative := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	idAbsolute := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	idBasePath := "dddddddd-dddd-dddd-dddd-dddddddddddd"

	content := strings.Join([]string{
		"contact:" + idProtocol,
		"/contact/" + idRelative,
		"https://groundwave.example.com/base/contact/" + idAbsolute,
		"/base/contact/" + idBasePath,
	}, " ")

	matchers := buildContactLinkMatchers("groundwave.example.com/base")
	got := extractContactLinksFromContent(content, matchers)

	assertContactIDPresent(t, got, strings.ToLower(idProtocol))
	assertContactIDPresent(t, got, idRelative)
	assertContactIDPresent(t, got, idAbsolute)
	assertContactIDPresent(t, got, idBasePath)
}

func TestBuildJournalPreview(t *testing.T) {
	content := strings.Join([]string{
		"#+TITLE: Daily",
		":PROPERTIES:",
		":ID: 123",
		":END:",
		"* Heading",
		"Paragraph one line.",
		"More text.",
		"",
		"Paragraph two line.",
		"",
		"Paragraph three line.",
	}, "\n")

	preview, hasMore := buildJournalPreview(content, 2, 1000)
	if !hasMore {
		t.Fatalf("expected more content")
	}

	if strings.Contains(preview, "#+TITLE") {
		t.Fatalf("did not expect title in preview")
	}

	if strings.Contains(preview, "Heading") {
		t.Fatalf("did not expect heading in preview")
	}

	if !strings.Contains(preview, "Paragraph one line.") {
		t.Fatalf("expected first paragraph in preview")
	}

	if !strings.Contains(preview, "Paragraph two line.") {
		t.Fatalf("expected second paragraph in preview")
	}

	preview, hasMore = buildJournalPreview("Paragraph one line is long enough.", 1, 10)
	if len(preview) == 0 || len(preview) > 10 {
		t.Fatalf("expected truncated preview length, got %d", len(preview))
	}

	if !hasMore {
		t.Fatalf("expected truncated preview to report more content")
	}

	preview, hasMore = buildJournalPreview("#+TITLE: Only\n:PROPERTIES:\n:ID: 1\n:END:\n", 2, 100)
	if preview != "" || hasMore {
		t.Fatalf("expected empty preview for metadata-only content")
	}
}

func TestAnnotateRestrictedNoteLinks(t *testing.T) {
	resetZettelkastenCaches()
	t.Cleanup(resetZettelkastenCaches)

	publicID := "11111111-1111-1111-1111-111111111111"
	restrictedWithClass := "22222222-2222-2222-2222-222222222222"
	restrictedNoClass := "33333333-3333-3333-3333-333333333333"

	input := strings.Join([]string{
		"<a href=\"/note/" + publicID + "\">Public</a>",
		"<a class=\"existing\" href=\"/note/" + restrictedWithClass + "\">Restricted</a>",
		"<a href=\"/note/" + restrictedNoClass + "\">Restricted2</a>",
	}, " ")

	if got := annotateRestrictedNoteLinks(input); got != input {
		t.Fatalf("expected unchanged output when cache not built")
	}

	backlinkMutex.Lock()

	publicNoteCache = map[string]bool{publicID: true}
	lastCacheBuild = time.Now()

	backlinkMutex.Unlock()

	got := annotateRestrictedNoteLinks(input)
	if strings.Contains(got, "restricted-link\" href=\"/note/"+publicID) {
		t.Fatalf("did not expect public note to be restricted")
	}

	if !strings.Contains(got, "class=\"existing restricted-link\" href=\"/note/"+restrictedWithClass) {
		t.Fatalf("expected restricted class to be added to existing class")
	}

	if !strings.Contains(got, "<a class=\"restricted-link\" href=\"/note/"+restrictedNoClass) {
		t.Fatalf("expected restricted class to be added")
	}
}

func TestGetJournalEntryByDateInvalid(t *testing.T) {
	if _, ok := GetJournalEntryByDate(""); ok {
		t.Fatalf("expected empty date to be invalid")
	}

	if _, ok := GetJournalEntryByDate("2024-99-99"); ok {
		t.Fatalf("expected invalid date format to be rejected")
	}
}

func assertContactIDPresent(t *testing.T, values []string, target string) {
	t.Helper()

	for _, value := range values {
		if value == target {
			return
		}
	}

	t.Fatalf("expected contact id %s in %+v", target, values)
}
