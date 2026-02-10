// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/google/uuid"
)

func TestZettelkastenComments(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	if err := CreateZettelComment(ctx, "", "note"); err == nil {
		t.Fatalf("expected error for invalid zettel id")
	}

	zettelID := uuid.New().String()
	if err := CreateZettelComment(ctx, zettelID, "First"); err != nil {
		t.Fatalf("CreateZettelComment failed: %v", err)
	}

	comments, err := GetCommentsForZettel(ctx, zettelID)
	if err != nil {
		t.Fatalf("GetCommentsForZettel failed: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	count, err := GetZettelCommentCount(ctx)
	if err != nil {
		t.Fatalf("GetZettelCommentCount failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 comment, got %d", count)
	}

	all, err := GetAllZettelComments(ctx)
	if err != nil {
		t.Fatalf("GetAllZettelComments failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 enriched comment, got %d", len(all))
	}
	if !all[0].OrphanedNote {
		t.Fatalf("expected orphaned note without WebDAV config")
	}

	if err := DeleteZettelComment(ctx, comments[0].ID); err != nil {
		t.Fatalf("DeleteZettelComment failed: %v", err)
	}

	if err := CreateZettelComment(ctx, zettelID, "Second"); err != nil {
		t.Fatalf("CreateZettelComment failed: %v", err)
	}
	if err := CreateZettelComment(ctx, zettelID, "Third"); err != nil {
		t.Fatalf("CreateZettelComment failed: %v", err)
	}

	if err := DeleteAllZettelComments(ctx, zettelID); err != nil {
		t.Fatalf("DeleteAllZettelComments failed: %v", err)
	}

	comments, err = GetCommentsForZettel(ctx, zettelID)
	if err != nil {
		t.Fatalf("GetCommentsForZettel failed: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}
}
