/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"
	"strings"

	"github.com/flamego/flamego"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

// ZettelkastenIndex renders the zettelkasten index page
func ZettelkastenIndex(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch the index note
	note, err := db.GetIndexNote(ctx)
	if err != nil {
		log.Printf("Error fetching zettelkasten index: %v", err)
		data["Error"] = "Failed to load zettelkasten. Please check your WEBDAV_ZK_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables."
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	// Fetch comments for this zettel (if it has an ID)
	var comments []db.ZettelComment
	if note.ID != "" {
		comments, err = db.GetCommentsForZettel(ctx, note.ID)
		if err != nil {
			log.Printf("Error fetching comments for zettel %s: %v", note.ID, err)
			// Don't fail the page, just log the error
			comments = []db.ZettelComment{}
		}
	}

	// Set template data
	data["Note"] = note
	data["Comments"] = comments
	data["NoteID"] = note.ID // Pass note ID explicitly
	data["IsZettelkasten"] = true

	t.HTML(http.StatusOK, "zettelkasten")
}

// ViewZKNote renders a specific note by ID
func ViewZKNote(c flamego.Context, t template.Template, data template.Data) {
	noteID := c.Param("id")
	if noteID == "" {
		data["Error"] = "Note ID is required"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	// Strip "id:" prefix if present (org-roam links include the protocol)
	noteID = strings.TrimPrefix(noteID, "id:")

	ctx := c.Request().Context()

	// Fetch the note
	note, err := db.GetNoteByID(ctx, noteID)
	if err != nil {
		log.Printf("Error fetching note %s: %v", noteID, err)
		data["Error"] = "Note not found: " + noteID
		data["BackLink"] = "/zk"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	// Fetch comments for this zettel
	comments, err := db.GetCommentsForZettel(ctx, noteID)
	if err != nil {
		log.Printf("Error fetching comments for zettel %s: %v", noteID, err)
		// Don't fail the page, just log the error
		comments = []db.ZettelComment{}
	}

	// Fetch backlinks for this zettel
	backlinkIDs := db.GetBacklinksFromCache(noteID)

	// Enrich backlinks with note titles
	type Backlink struct {
		ID    string
		Title string
	}
	backlinks := make([]Backlink, 0, len(backlinkIDs))
	for _, backlinkID := range backlinkIDs {
		backlinkNote, err := db.GetNoteByID(ctx, backlinkID)
		if err != nil {
			log.Printf("Error fetching backlink note %s: %v", backlinkID, err)
			// Skip this backlink if we can't fetch it
			continue
		}
		backlinks = append(backlinks, Backlink{
			ID:    backlinkID,
			Title: backlinkNote.Title,
		})
	}

	// Get last cache build time
	lastCacheUpdate := db.GetLastCacheBuildTime()

	// Set template data
	data["Note"] = note
	data["Comments"] = comments
	data["NoteID"] = noteID // Pass note ID explicitly from URL
	data["Backlinks"] = backlinks
	data["LastCacheUpdate"] = lastCacheUpdate
	data["IsZettelkasten"] = true

	t.HTML(http.StatusOK, "zettelkasten")
}

// AddZettelComment handles posting a comment to a zettel
func AddZettelComment(c flamego.Context, t template.Template, data template.Data) {
	zettelID := c.Param("id")
	if zettelID == "" {
		data["Error"] = "Zettel ID is required"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	// Strip "id:" prefix if present
	zettelID = strings.TrimPrefix(zettelID, "id:")

	// Parse form
	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/zk/" + zettelID)
		return
	}

	content := strings.TrimSpace(c.Request().Form.Get("content"))
	if content == "" {
		c.Redirect("/zk/" + zettelID)
		return
	}

	ctx := c.Request().Context()

	// Create comment
	if err := db.CreateZettelComment(ctx, zettelID, content); err != nil {
		log.Printf("Error creating zettel comment: %v", err)
		data["Error"] = "Failed to add comment"
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	// Redirect back to the zettel
	c.Redirect("/zk/" + zettelID)
}

// DeleteZettelComment handles deleting a comment
func DeleteZettelComment(c flamego.Context, t template.Template, data template.Data) {
	zettelID := c.Param("id")
	commentIDStr := c.Param("comment_id")

	if zettelID == "" || commentIDStr == "" {
		data["Error"] = "Invalid request"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	// Parse comment UUID
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		log.Printf("Invalid comment ID: %v", err)
		data["Error"] = "Invalid comment ID"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	ctx := c.Request().Context()

	// Delete comment
	if err := db.DeleteZettelComment(ctx, commentID); err != nil {
		log.Printf("Error deleting zettel comment: %v", err)
		data["Error"] = "Failed to delete comment"
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	// Redirect back to the zettel
	c.Redirect("/zk/" + zettelID)
}

// RefreshBacklinks manually triggers a backlink cache rebuild
func RefreshBacklinks(c flamego.Context) {
	ctx := c.Request().Context()

	// Trigger cache rebuild
	if err := db.BuildBacklinkCache(ctx); err != nil {
		log.Printf("Error refreshing backlink cache: %v", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		return
	}

	// Redirect back to the zettelkasten index
	c.Redirect("/zk")
}

// ZettelCommentsInbox renders the unified inbox of all zettel comments
func ZettelCommentsInbox(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch all comments with zettel metadata
	comments, err := db.GetAllZettelComments(ctx)
	if err != nil {
		log.Printf("Error fetching zettel comments: %v", err)
		data["Error"] = "Failed to load comments inbox"
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	// Group comments by zettel_id
	type ZettelGroup struct {
		ZettelID       string
		ZettelTitle    string
		ZettelFilename string
		OrphanedNote   bool
		Comments       []db.ZettelComment
	}

	groupMap := make(map[string]*ZettelGroup)
	var groups []ZettelGroup

	for _, enriched := range comments {
		if _, exists := groupMap[enriched.ZettelID]; !exists {
			group := ZettelGroup{
				ZettelID:       enriched.ZettelID,
				ZettelTitle:    enriched.ZettelTitle,
				ZettelFilename: enriched.ZettelFilename,
				OrphanedNote:   enriched.OrphanedNote,
				Comments:       []db.ZettelComment{},
			}
			groupMap[enriched.ZettelID] = &group
			groups = append(groups, group)
		}
		groupMap[enriched.ZettelID].Comments = append(
			groupMap[enriched.ZettelID].Comments,
			enriched.ZettelComment,
		)
	}

	// Update groups slice with populated data
	for i := range groups {
		if g, exists := groupMap[groups[i].ZettelID]; exists {
			groups[i] = *g
		}
	}

	// Set template data
	data["Groups"] = groups
	data["CommentCount"] = len(comments)
	data["IsZettelkasten"] = true

	t.HTML(http.StatusOK, "zettel_inbox")
}
