// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import "testing"

func TestTagsLifecycle(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	contactID := mustCreateContact(t, CreateContactInput{
		NameGiven: "Tag",
		Tier:      TierB,
	})

	if err := AddTagToContact(ctx, contactID, "Friends"); err != nil {
		t.Fatalf("AddTagToContact failed: %v", err)
	}
	if err := AddTagToContact(ctx, contactID, "Colleagues"); err != nil {
		t.Fatalf("AddTagToContact failed: %v", err)
	}

	allTags, err := ListAllTags(ctx)
	if err != nil {
		t.Fatalf("ListAllTags failed: %v", err)
	}
	if len(allTags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(allTags))
	}

	searchTags, err := SearchTags(ctx, "fri")
	if err != nil {
		t.Fatalf("SearchTags failed: %v", err)
	}
	if len(searchTags) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(searchTags))
	}

	selectedTag, err := GetTag(ctx, searchTags[0].ID.String())
	if err != nil {
		t.Fatalf("GetTag failed: %v", err)
	}
	if selectedTag.Name != "friends" {
		t.Fatalf("expected normalized name friends, got %q", selectedTag.Name)
	}

	tags, err := GetContactTags(ctx, contactID)
	if err != nil {
		t.Fatalf("GetContactTags failed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 contact tags, got %d", len(tags))
	}

	newDescription := "Close friends"
	if err := RenameTag(ctx, selectedTag.ID.String(), "Friends", &newDescription); err != nil {
		t.Fatalf("RenameTag failed: %v", err)
	}

	contacts, err := GetContactsByTags(ctx, []string{selectedTag.ID.String()})
	if err != nil {
		t.Fatalf("GetContactsByTags failed: %v", err)
	}
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact by tag, got %d", len(contacts))
	}

	if err := RemoveTagFromContact(ctx, contactID, selectedTag.ID.String()); err != nil {
		t.Fatalf("RemoveTagFromContact failed: %v", err)
	}

	if err := DeleteTag(ctx, selectedTag.ID.String()); err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}
}

func TestTagsErrors(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	if _, err := GetContactsByTags(ctx, []string{}); err == nil {
		t.Fatalf("expected error for missing tag IDs")
	}
}
