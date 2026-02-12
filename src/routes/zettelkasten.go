/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"context"
	"crypto/rand"
	"encoding/gob"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
	"github.com/humaidq/groundwave/utils"
)

// ZKHistoryItem represents a visited zettelkasten note in navigation history
type ZKHistoryItem struct {
	ID        string
	Title     string
	IsCurrent bool
}

// Backlink represents a note that links to another note.
type Backlink struct {
	ID    string
	Title string
	URL   string
}

type backlinkVisibility int

const (
	backlinkVisibilityAll backlinkVisibility = iota
	backlinkVisibilityPublicOnly
	backlinkVisibilityHomeOrPublic
)

const zkHistoryKey = "zk_history"
const zkPublicHistoryKey = "zk_public_history"
const zkHistoryMaxItems = 4

var updateZettelCommentDBFn = db.UpdateZettelComment

func init() {
	// Register ZKHistoryItem slice for session serialization
	gob.Register([]ZKHistoryItem{})
}

func getZKHistoryWithKey(s session.Session, key string) []ZKHistoryItem {
	val := s.Get(key)
	if val == nil {
		return []ZKHistoryItem{}
	}

	history, ok := val.([]ZKHistoryItem)
	if !ok {
		return []ZKHistoryItem{}
	}

	return history
}

// updateZKHistory adds a new item to the history (FIFO, max 4 items)
func updateZKHistory(s session.Session, noteID, noteTitle string) []ZKHistoryItem {
	return updateZKHistoryWithKey(s, zkHistoryKey, noteID, noteTitle)
}

func updateZKHistoryWithKey(s session.Session, key string, noteID, noteTitle string) []ZKHistoryItem {
	history := getZKHistoryWithKey(s, key)

	filtered := make([]ZKHistoryItem, 0, len(history))
	for _, item := range history {
		if item.ID != noteID {
			filtered = append(filtered, item)
		}
	}

	history = filtered

	history = append(history, ZKHistoryItem{
		ID:    noteID,
		Title: noteTitle,
	})

	if len(history) > zkHistoryMaxItems {
		history = history[len(history)-zkHistoryMaxItems:]
	}

	s.Set(key, history)

	return history
}

// prepareZKHistoryForTemplate marks the current item and returns the history
func prepareZKHistoryForTemplate(history []ZKHistoryItem, currentID string) []ZKHistoryItem {
	result := make([]ZKHistoryItem, len(history))
	for i, item := range history {
		result[i] = ZKHistoryItem{
			ID:        item.ID,
			Title:     item.Title,
			IsCurrent: item.ID == currentID,
		}
	}

	return result
}

func buildBacklinks(ctx context.Context, backlinkIDs []string, basePath string, visibility backlinkVisibility) []Backlink {
	backlinks := make([]Backlink, 0, len(backlinkIDs))

	trimmedBase := strings.TrimRight(basePath, "/")
	if trimmedBase == "" {
		trimmedBase = "/zk"
	}

	for _, backlinkID := range backlinkIDs {
		if strings.HasPrefix(backlinkID, db.DailyBacklinkPrefix) {
			if visibility != backlinkVisibilityAll {
				continue
			}

			dateString := strings.TrimPrefix(backlinkID, db.DailyBacklinkPrefix)

			title := dateString
			if entry, ok := db.GetJournalEntryByDate(dateString); ok && entry.Title != "" {
				title = entry.Title
			}

			backlinks = append(backlinks, Backlink{
				ID:    backlinkID,
				Title: title,
				URL:   "/journal/" + dateString,
			})

			continue
		}

		backlinkNote, err := db.GetNoteByIDWithBasePath(ctx, backlinkID, trimmedBase)
		if err != nil {
			logger.Error("Error fetching backlink note", "note_id", backlinkID, "error", err)
			continue
		}

		switch visibility {
		case backlinkVisibilityAll:
		case backlinkVisibilityPublicOnly:
			if !backlinkNote.IsPublic {
				continue
			}
		case backlinkVisibilityHomeOrPublic:
			if !backlinkNote.IsPublic && !backlinkNote.IsHome {
				continue
			}
		}

		backlinks = append(backlinks, Backlink{
			ID:    backlinkID,
			Title: backlinkNote.Title,
			URL:   trimmedBase + "/" + backlinkID,
		})
	}

	return backlinks
}

// ZettelkastenIndex renders the zettelkasten index page
func ZettelkastenIndex(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch the index note
	note, err := db.GetIndexNote(ctx)
	if err != nil {
		logger.Error("Error fetching zettelkasten index", "error", err)
		SetErrorFlash(s, "Failed to load zettelkasten. Please check your WEBDAV_ZK_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables.")
		c.Redirect("/", http.StatusSeeOther)

		return
	}

	// Fetch comments for this zettel (if it has an ID)
	var comments []db.ZettelComment
	if note.ID != "" {
		comments, err = db.GetCommentsForZettel(ctx, note.ID)
		if err != nil {
			logger.Error("Error fetching comments for zettel", "zettel_id", note.ID, "error", err)
			// Don't fail the page, just log the error
			comments = []db.ZettelComment{}
		}
	}

	// Fetch backlinks for this zettel
	var backlinkIDs []string
	if note.ID != "" {
		backlinkIDs = db.GetBacklinksFromCache(note.ID)
	}

	backlinks := buildBacklinks(ctx, backlinkIDs, "/zk", backlinkVisibilityAll)

	// Get last cache build time
	lastCacheUpdate := db.GetLastCacheBuildTime()

	// Update navigation history
	var history []ZKHistoryItem
	if note.ID != "" {
		history = updateZKHistory(s, note.ID, note.Title)
		history = prepareZKHistoryForTemplate(history, note.ID)
	}

	// Set template data
	data["Note"] = note
	data["Comments"] = comments
	data["NoteID"] = note.ID // Pass note ID explicitly
	data["Backlinks"] = backlinks
	data["LastCacheUpdate"] = lastCacheUpdate
	data["IsZettelkasten"] = true

	data["ZKHistory"] = history
	if note.ID != "" {
		data["PublishPath"] = "/note/" + note.ID
	}

	t.HTML(http.StatusOK, "zettelkasten")
}

// ViewZKNote renders a specific note by ID
func ViewZKNote(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	noteID := c.Param("id")
	if noteID == "" {
		SetErrorFlash(s, "Note ID is required")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	// Strip "id:" prefix if present (org-roam links include the protocol)
	noteID = strings.TrimPrefix(noteID, "id:")

	ctx := c.Request().Context()

	// Fetch the note
	note, err := db.GetNoteByID(ctx, noteID)
	if err != nil {
		logger.Error("Error fetching note", "note_id", noteID, "error", err)
		SetErrorFlash(s, "Note not found: "+noteID)
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	// Fetch comments for this zettel
	comments, err := db.GetCommentsForZettel(ctx, noteID)
	if err != nil {
		logger.Error("Error fetching comments for zettel", "zettel_id", noteID, "error", err)
		// Don't fail the page, just log the error
		comments = []db.ZettelComment{}
	}

	// Fetch backlinks for this zettel
	backlinkIDs := db.GetBacklinksFromCache(noteID)

	backlinks := buildBacklinks(ctx, backlinkIDs, "/zk", backlinkVisibilityAll)

	// Get last cache build time
	lastCacheUpdate := db.GetLastCacheBuildTime()

	// Update navigation history
	history := updateZKHistory(s, noteID, note.Title)
	history = prepareZKHistoryForTemplate(history, noteID)

	// Set template data
	data["Note"] = note
	data["Comments"] = comments
	data["NoteID"] = noteID // Pass note ID explicitly from URL
	data["Backlinks"] = backlinks
	data["LastCacheUpdate"] = lastCacheUpdate
	data["IsZettelkasten"] = true
	data["ZKHistory"] = history
	data["PublishPath"] = "/note/" + noteID

	t.HTML(http.StatusOK, "zettelkasten")
}

func renderPrivateNote(t template.Template, data template.Data) {
	data["IsZettelkasten"] = true
	data["HideNav"] = true

	t.HTML(http.StatusNotFound, "note_private")
}

// ViewPublicNote renders a published note without authentication
func ViewPublicNote(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	noteID := c.Param("id")
	if noteID == "" {
		renderPrivateNote(t, data)
		return
	}

	noteID = strings.TrimPrefix(noteID, "id:")
	ctx := c.Request().Context()

	lastCacheUpdate := db.GetLastCacheBuildTime()
	if !lastCacheUpdate.IsZero() && !db.IsPublicNoteFromCache(noteID) {
		renderPrivateNote(t, data)
		return
	}

	note, err := db.GetNoteByIDWithBasePath(ctx, noteID, "/note")
	if err != nil || !note.IsPublic {
		if err != nil {
			logger.Error("Error fetching public note", "note_id", noteID, "error", err)
		}

		renderPrivateNote(t, data)

		return
	}

	backlinkIDs := db.GetBacklinksFromCache(noteID)
	backlinks := buildBacklinks(ctx, backlinkIDs, "/note", backlinkVisibilityPublicOnly)

	history := updateZKHistoryWithKey(s, zkPublicHistoryKey, noteID, note.Title)
	history = prepareZKHistoryForTemplate(history, noteID)

	data["Note"] = note
	data["NoteID"] = noteID
	data["Backlinks"] = backlinks
	data["IsZettelkasten"] = true
	data["ZKHistory"] = history
	data["HideNav"] = true

	t.HTML(http.StatusOK, "note_public")
}

// ZettelkastenList renders the list of all zettelkasten notes.
func ZettelkastenList(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	notes, err := db.ListZKNotes(ctx)
	if err != nil {
		logger.Error("Error listing zettelkasten notes", "error", err)

		data["Error"] = "Failed to load zettelkasten notes"
	} else {
		data["Notes"] = notes
		data["NoteCount"] = len(notes)
	}

	data["IsZettelkasten"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Zettelkasten", URL: "/zk", IsCurrent: false},
		{Name: "All Pages", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "zettelkasten_list")
}

// ZettelkastenRandom redirects to a random zettelkasten note.
func ZettelkastenRandom(c flamego.Context, s session.Session) {
	ctx := c.Request().Context()

	notes, err := db.ListZKNotes(ctx)
	if err != nil {
		logger.Error("Error listing zettelkasten notes", "error", err)
		SetErrorFlash(s, "Failed to load zettelkasten notes")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	if len(notes) == 0 {
		SetInfoFlash(s, "No zettelkasten pages available")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	indexMax := big.NewInt(int64(len(notes)))

	index, err := rand.Int(rand.Reader, indexMax)
	if err != nil {
		logger.Error("Error picking random zettelkasten note", "error", err)
		SetErrorFlash(s, "Failed to pick a random page")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	note := notes[index.Int64()]
	c.Redirect("/zk/"+note.ID, http.StatusSeeOther)
}

type zkChatRequest struct {
	NoteIDs []string `json:"note_ids"`
	Message string   `json:"message"`
}

type zkChatLinksRequest struct {
	NoteID string `json:"note_id"`
}

type zkChatBacklinksRequest struct {
	NoteID string `json:"note_id"`
}

// ZettelkastenChat renders the zettelkasten chat page.
func ZettelkastenChat(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	notes, err := db.ListZKNotes(ctx)
	if err != nil {
		logger.Error("Error listing zettelkasten notes", "error", err)
		SetErrorFlash(s, "Failed to load zettelkasten notes")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	data["Notes"] = notes
	data["IsZettelkasten"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Zettelkasten", URL: "/zk", IsCurrent: false},
		{Name: "Chat", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "zettelkasten_chat")
}

// ZettelkastenChatLinks returns linked note IDs for a note.
func ZettelkastenChatLinks(c flamego.Context) {
	w := c.ResponseWriter()

	var reqBody zkChatLinksRequest
	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	noteID := strings.TrimSpace(strings.TrimPrefix(reqBody.NoteID, "id:"))
	if noteID == "" {
		http.Error(w, "Note ID is required", http.StatusBadRequest)
		return
	}

	if err := utils.ValidateUUID(noteID); err != nil {
		http.Error(w, "Invalid note ID", http.StatusBadRequest)
		return
	}

	links := db.GetZKNoteLinks(noteID)

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string][]string{"links": links}); err != nil {
		logger.Error("Error encoding chat links response", "error", err)
	}
}

// ZettelkastenChatBacklinks returns backlinks for a note.
func ZettelkastenChatBacklinks(c flamego.Context) {
	w := c.ResponseWriter()

	var reqBody zkChatBacklinksRequest
	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	noteID := strings.TrimSpace(strings.TrimPrefix(reqBody.NoteID, "id:"))
	if noteID == "" {
		http.Error(w, "Note ID is required", http.StatusBadRequest)
		return
	}

	if err := utils.ValidateUUID(noteID); err != nil {
		http.Error(w, "Invalid note ID", http.StatusBadRequest)
		return
	}

	backlinks := db.GetBacklinksFromCache(noteID)

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string][]string{"backlinks": backlinks}); err != nil {
		logger.Error("Error encoding chat backlinks response", "error", err)
	}
}

// ZettelkastenChatStream streams AI chat responses using SSE.
func ZettelkastenChatStream(c flamego.Context) {
	ctx := c.Request().Context()
	w := c.ResponseWriter()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sendEvent := func(event, data string) {
		if event != "" {
			if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
				logger.Error("Error writing SSE event", "error", err)
				return
			}
		}

		escapedData := strings.ReplaceAll(data, "\n", "\ndata: ")
		if _, err := w.Write([]byte("data: " + escapedData + "\n\n")); err != nil {
			logger.Error("Error writing SSE data", "error", err)
			return
		}

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	sendError := func(message string) {
		sendEvent("error", message)
	}

	var reqBody zkChatRequest
	if err := json.NewDecoder(c.Request().Body().ReadCloser()).Decode(&reqBody); err != nil {
		sendError("Invalid request")
		return
	}

	message := strings.TrimSpace(reqBody.Message)
	if message == "" {
		sendError("Message is required")
		return
	}

	if len(reqBody.NoteIDs) == 0 {
		sendError("Select at least one note")
		return
	}

	notes := make([]db.ZKChatNote, 0, len(reqBody.NoteIDs))
	for _, noteID := range reqBody.NoteIDs {
		noteID = strings.TrimSpace(strings.TrimPrefix(noteID, "id:"))
		if noteID == "" {
			continue
		}

		note, err := db.GetZKNoteForChat(ctx, noteID)
		if err != nil {
			logger.Error("Error fetching zettelkasten note", "note_id", noteID, "error", err)
			sendError("Note not found: " + noteID)

			return
		}

		notes = append(notes, *note)
	}

	if len(notes) == 0 {
		sendError("No valid notes selected")
		return
	}

	err := db.StreamZKChat(ctx, notes, message, func(chunk string) error {
		sendEvent("chunk", chunk)
		return nil
	})
	if err != nil {
		logger.Error("Error generating zettelkasten chat response", "error", err)
		sendError("Failed to generate response: " + err.Error())

		return
	}

	sendEvent("done", "")
}

// AddZettelComment handles posting a comment to a zettel
func AddZettelComment(c flamego.Context, s session.Session, _ template.Template, _ template.Data) {
	zettelID := c.Param("id")
	if zettelID == "" {
		SetErrorFlash(s, "Zettel ID is required")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	// Strip "id:" prefix if present
	zettelID = strings.TrimPrefix(zettelID, "id:")

	// Parse form
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)

		return
	}

	content := strings.TrimSpace(c.Request().Form.Get("content"))
	if content == "" {
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
		return
	}

	ctx := c.Request().Context()

	// Create comment
	if err := db.CreateZettelComment(ctx, zettelID, content); err != nil {
		logger.Error("Error creating zettel comment", "error", err)
		SetErrorFlash(s, "Failed to add comment")
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)

		return
	}

	// Redirect back to the zettel with success message
	SetSuccessFlash(s, "Comment added successfully")
	c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
}

// UpdateZettelComment handles editing an existing comment
func UpdateZettelComment(c flamego.Context, s session.Session, _ template.Template, _ template.Data) {
	zettelID := c.Param("id")
	commentIDStr := c.Param("comment_id")

	if zettelID == "" || commentIDStr == "" {
		SetErrorFlash(s, "Invalid request")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	zettelID = strings.TrimPrefix(zettelID, "id:")
	redirectPath := "/zk/" + zettelID

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to update comment")
		c.Redirect(redirectPath, http.StatusSeeOther)

		return
	}

	if c.Request().Form.Get("redirect_to") == "inbox" {
		redirectPath = "/zettel-inbox"
	}

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		logger.Warn("Invalid comment ID", "error", err)
		SetErrorFlash(s, "Invalid comment ID")
		c.Redirect(redirectPath, http.StatusSeeOther)

		return
	}

	content := strings.TrimSpace(c.Request().Form.Get("content"))
	if content == "" {
		SetErrorFlash(s, "Comment content is required")
		c.Redirect(redirectPath, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()
	if err := updateZettelCommentDBFn(ctx, zettelID, commentID, content); err != nil {
		logger.Error("Error updating zettel comment", "error", err)
		SetErrorFlash(s, "Failed to update comment")
		c.Redirect(redirectPath, http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Comment updated successfully")
	c.Redirect(redirectPath, http.StatusSeeOther)
}

// DeleteZettelComment handles deleting a comment
func DeleteZettelComment(c flamego.Context, s session.Session, _ template.Template, _ template.Data) {
	zettelID := c.Param("id")
	commentIDStr := c.Param("comment_id")

	if zettelID == "" || commentIDStr == "" {
		SetErrorFlash(s, "Invalid request")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	// Parse comment UUID
	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		logger.Warn("Invalid comment ID", "error", err)
		SetErrorFlash(s, "Invalid comment ID")
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	// Delete comment
	if err := db.DeleteZettelComment(ctx, commentID); err != nil {
		logger.Error("Error deleting zettel comment", "error", err)
		SetErrorFlash(s, "Failed to delete comment")
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)

		return
	}

	// Redirect back to the zettel with success message
	SetSuccessFlash(s, "Comment deleted successfully")
	c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
}

// DeleteAllZettelComments handles deleting all comments for a zettel
func DeleteAllZettelComments(c flamego.Context, s session.Session, _ template.Template, _ template.Data) {
	zettelID := c.Param("id")
	if zettelID == "" {
		SetErrorFlash(s, "Zettel ID is required")
		c.Redirect("/zk", http.StatusSeeOther)

		return
	}

	// Strip "id:" prefix if present
	zettelID = strings.TrimPrefix(zettelID, "id:")

	ctx := c.Request().Context()

	if err := db.DeleteAllZettelComments(ctx, zettelID); err != nil {
		logger.Error("Error deleting zettel comments", "zettel_id", zettelID, "error", err)
		SetErrorFlash(s, "Failed to delete comments")
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "All comments deleted")
	c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
}

// RebuildCache manually triggers a full cache rebuild.
func RebuildCache(c flamego.Context, s session.Session) {
	// Trigger cache rebuild asynchronously to avoid blocking the HTTP request
	go func() {
		if err := db.RebuildZettelkastenCaches(c.Request().Context()); err != nil {
			logger.Error("Manual cache rebuild failed", "error", err)
		} else {
			logger.Info("Manual cache rebuild completed successfully")
		}
	}()

	// Immediately redirect back to the zettelkasten index with info message
	// Cache will be rebuilt in the background
	SetInfoFlash(s, "Cache rebuild started in background")
	c.Redirect("/zk", http.StatusSeeOther)
}

// ZettelCommentsInbox renders the unified inbox of all zettel comments
func ZettelCommentsInbox(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch all comments with zettel metadata
	comments, err := db.GetAllZettelComments(ctx)
	if err != nil {
		logger.Error("Error fetching zettel comments", "error", err)
		SetErrorFlash(s, "Failed to load comments inbox")
		c.Redirect("/zk", http.StatusSeeOther)

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
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Zettelkasten", URL: "/zk", IsCurrent: false},
		{Name: "Comments Inbox", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "zettel_inbox")
}
