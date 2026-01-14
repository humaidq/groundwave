/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"encoding/gob"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

// ZKHistoryItem represents a visited zettelkasten note in navigation history
type ZKHistoryItem struct {
	ID        string
	Title     string
	IsCurrent bool
}

const zkHistoryKey = "zk_history"
const zkHistoryMaxItems = 4

func init() {
	// Register ZKHistoryItem slice for session serialization
	gob.Register([]ZKHistoryItem{})
}

// getZKHistory retrieves the navigation history from session
func getZKHistory(s session.Session) []ZKHistoryItem {
	val := s.Get(zkHistoryKey)
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
	history := getZKHistory(s)

	// Remove any existing entry for this note (avoid duplicates)
	filtered := make([]ZKHistoryItem, 0, len(history))
	for _, item := range history {
		if item.ID != noteID {
			filtered = append(filtered, item)
		}
	}
	history = filtered

	// Add new item at the end
	history = append(history, ZKHistoryItem{
		ID:    noteID,
		Title: noteTitle,
	})

	// Keep only the last N items (FIFO)
	if len(history) > zkHistoryMaxItems {
		history = history[len(history)-zkHistoryMaxItems:]
	}

	// Save to session
	s.Set(zkHistoryKey, history)

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

// ZettelkastenIndex renders the zettelkasten index page
func ZettelkastenIndex(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch the index note
	note, err := db.GetIndexNote(ctx)
	if err != nil {
		log.Printf("Error fetching zettelkasten index: %v", err)
		SetErrorFlash(s, "Failed to load zettelkasten. Please check your WEBDAV_ZK_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables.")
		c.Redirect("/", http.StatusSeeOther)
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

	// Fetch backlinks for this zettel
	var backlinkIDs []string
	if note.ID != "" {
		backlinkIDs = db.GetBacklinksFromCache(note.ID)
	}

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
			continue
		}
		backlinks = append(backlinks, Backlink{
			ID:    backlinkID,
			Title: backlinkNote.Title,
		})
	}

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
		log.Printf("Error fetching note %s: %v", noteID, err)
		SetErrorFlash(s, "Note not found: "+noteID)
		c.Redirect("/zk", http.StatusSeeOther)
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

	t.HTML(http.StatusOK, "zettelkasten")
}

type zkChatRequest struct {
	NoteIDs []string `json:"note_ids"`
	Message string   `json:"message"`
}

// ZettelkastenChat renders the zettelkasten chat page.
func ZettelkastenChat(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	notes, err := db.ListZKNotes(ctx)
	if err != nil {
		log.Printf("Error listing zettelkasten notes: %v", err)
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
			w.Write([]byte("event: " + event + "\n"))
		}
		escapedData := strings.ReplaceAll(data, "\n", "\ndata: ")
		w.Write([]byte("data: " + escapedData + "\n\n"))
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
			log.Printf("Error fetching zettelkasten note %s: %v", noteID, err)
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
		log.Printf("Error generating zettelkasten chat response: %v", err)
		sendError("Failed to generate response: " + err.Error())
		return
	}

	sendEvent("done", "")
}

// AddZettelComment handles posting a comment to a zettel
func AddZettelComment(c flamego.Context, s session.Session, t template.Template, data template.Data) {
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
		log.Printf("Error parsing form: %v", err)
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
		log.Printf("Error creating zettel comment: %v", err)
		SetErrorFlash(s, "Failed to add comment")
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
		return
	}

	// Redirect back to the zettel with success message
	SetSuccessFlash(s, "Comment added successfully")
	c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
}

// DeleteZettelComment handles deleting a comment
func DeleteZettelComment(c flamego.Context, s session.Session, t template.Template, data template.Data) {
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
		log.Printf("Invalid comment ID: %v", err)
		SetErrorFlash(s, "Invalid comment ID")
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
		return
	}

	ctx := c.Request().Context()

	// Delete comment
	if err := db.DeleteZettelComment(ctx, commentID); err != nil {
		log.Printf("Error deleting zettel comment: %v", err)
		SetErrorFlash(s, "Failed to delete comment")
		c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
		return
	}

	// Redirect back to the zettel with success message
	SetSuccessFlash(s, "Comment deleted successfully")
	c.Redirect("/zk/"+zettelID, http.StatusSeeOther)
}

// RefreshBacklinks manually triggers a backlink cache rebuild
func RefreshBacklinks(c flamego.Context, s session.Session) {
	// Trigger cache rebuild asynchronously to avoid blocking the HTTP request
	go func() {
		if err := db.BuildBacklinkCache(c.Request().Context()); err != nil {
			log.Printf("[ERROR] Manual backlink cache refresh failed: %v", err)
		} else {
			log.Println("[INFO] Manual backlink cache refresh completed successfully")
		}
	}()

	// Immediately redirect back to the zettelkasten index with info message
	// Cache will be rebuilt in the background
	SetInfoFlash(s, "Backlink cache refresh started in background")
	c.Redirect("/zk", http.StatusSeeOther)
}

// ZettelCommentsInbox renders the unified inbox of all zettel comments
func ZettelCommentsInbox(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch all comments with zettel metadata
	comments, err := db.GetAllZettelComments(ctx)
	if err != nil {
		log.Printf("Error fetching zettel comments: %v", err)
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
