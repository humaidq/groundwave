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

	"github.com/humaidq/groundwave/db"
)

// ListTags renders the tag management page
func ListTags(c flamego.Context, t template.Template, data template.Data) {
	searchQuery := strings.TrimSpace(c.Query("q"))

	var tags []db.TagWithUsage
	var err error

	if searchQuery == "" {
		tags, err = db.ListAllTags(c.Request().Context())
	} else {
		tags, err = db.SearchTags(c.Request().Context(), searchQuery)
	}

	if err != nil {
		log.Printf("Error fetching tags: %v", err)
		data["Error"] = "Failed to load tags"
	} else {
		data["Tags"] = tags
		data["SearchQuery"] = searchQuery
	}

	t.HTML(http.StatusOK, "tags")
}

// EditTagForm renders the edit tag form
func EditTagForm(c flamego.Context, t template.Template, data template.Data) {
	tagID := c.Param("id")
	if tagID == "" {
		data["Error"] = "Tag ID is required"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	tag, err := db.GetTag(c.Request().Context(), tagID)
	if err != nil {
		log.Printf("Error fetching tag %s: %v", tagID, err)
		data["Error"] = "Tag not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	data["Tag"] = tag
	t.HTML(http.StatusOK, "tag_edit")
}

// UpdateTag handles tag rename/update
func UpdateTag(c flamego.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.Redirect("/tags", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/tags/"+tagID+"/edit", http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		c.Redirect("/tags/"+tagID+"/edit", http.StatusSeeOther)
		return
	}

	getOptionalString := func(key string) *string {
		val := strings.TrimSpace(form.Get(key))
		if val == "" {
			return nil
		}
		return &val
	}

	err := db.RenameTag(c.Request().Context(), tagID, name, getOptionalString("description"))
	if err != nil {
		log.Printf("Error updating tag: %v", err)
	}

	c.Redirect("/tags", http.StatusSeeOther)
}

// ViewTagContacts shows all contacts with a specific tag
func ViewTagContacts(c flamego.Context, t template.Template, data template.Data) {
	tagID := c.Param("id")
	if tagID == "" {
		data["Error"] = "Tag ID is required"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	tag, err := db.GetTag(c.Request().Context(), tagID)
	if err != nil {
		log.Printf("Error fetching tag %s: %v", tagID, err)
		data["Error"] = "Tag not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	contacts, err := db.GetContactsByTags(c.Request().Context(), []string{tagID})
	if err != nil {
		log.Printf("Error fetching contacts for tag %s: %v", tagID, err)
		data["Error"] = "Failed to load contacts"
	} else {
		data["Contacts"] = contacts
	}

	data["Tag"] = tag
	t.HTML(http.StatusOK, "tag_contacts")
}
