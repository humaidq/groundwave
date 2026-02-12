/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"
	"strings"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

const homeWikiHistoryKey = "home_wiki_history"

func isHomeWikiVisible(note *db.ZKNote) bool {
	return note.IsPublic || note.IsHome
}

// HomeWikiIndex renders the non-admin wiki index page.
func HomeWikiIndex(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	note, err := db.GetHomeIndexNote(ctx)
	if err != nil {
		logger.Error("Error fetching home wiki index", "error", err)
		SetErrorFlash(s, "Failed to load wiki. Please check your WEBDAV_HOME_PATH, WEBDAV_ZK_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables.")
		c.Redirect("/", http.StatusSeeOther)

		return
	}

	if !isHomeWikiVisible(note) {
		logger.Warn("Home wiki index is not visible to non-admin users", "filename", note.Filename)
		SetErrorFlash(s, "Home wiki index must include #+access: home or #+access: public")
		c.Redirect("/", http.StatusSeeOther)

		return
	}

	var backlinkIDs []string
	if note.ID != "" {
		backlinkIDs = db.GetBacklinksFromCache(note.ID)
	}

	backlinks := buildBacklinks(ctx, backlinkIDs, "/home", backlinkVisibilityHomeOrPublic)
	lastCacheUpdate := db.GetLastCacheBuildTime()

	var history []ZKHistoryItem
	if note.ID != "" {
		history = updateZKHistoryWithKey(s, homeWikiHistoryKey, note.ID, note.Title)
		history = prepareZKHistoryForTemplate(history, note.ID)
	}

	data["Note"] = note
	data["NoteID"] = note.ID
	data["Backlinks"] = backlinks
	data["LastCacheUpdate"] = lastCacheUpdate
	data["IsHomeWiki"] = true
	data["ZKHistory"] = history

	t.HTML(http.StatusOK, "home_wiki")
}

// ViewHomeWikiNote renders a specific home wiki note by ID.
func ViewHomeWikiNote(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	noteID := c.Param("id")
	if noteID == "" {
		SetErrorFlash(s, "Note ID is required")
		c.Redirect("/home", http.StatusSeeOther)

		return
	}

	noteID = strings.TrimPrefix(noteID, "id:")
	ctx := c.Request().Context()

	note, err := db.GetNoteByIDWithBasePath(ctx, noteID, "/home")
	if err != nil {
		logger.Error("Error fetching home wiki note", "note_id", noteID, "error", err)
		SetErrorFlash(s, "Page not found: "+noteID)
		c.Redirect("/home", http.StatusSeeOther)

		return
	}

	if !isHomeWikiVisible(note) {
		SetErrorFlash(s, "Page is not available in Home Wiki")
		c.Redirect("/home", http.StatusSeeOther)

		return
	}

	backlinkIDs := db.GetBacklinksFromCache(noteID)
	backlinks := buildBacklinks(ctx, backlinkIDs, "/home", backlinkVisibilityHomeOrPublic)
	lastCacheUpdate := db.GetLastCacheBuildTime()

	history := updateZKHistoryWithKey(s, homeWikiHistoryKey, noteID, note.Title)
	history = prepareZKHistoryForTemplate(history, noteID)

	data["Note"] = note
	data["NoteID"] = noteID
	data["Backlinks"] = backlinks
	data["LastCacheUpdate"] = lastCacheUpdate
	data["IsHomeWiki"] = true
	data["ZKHistory"] = history

	t.HTML(http.StatusOK, "home_wiki")
}
