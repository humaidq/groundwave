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

// ListTags renders the tag management page
func ListTags(c flamego.Context, t template.Template, data template.Data) {
	searchQuery := strings.TrimSpace(c.Query("q"))

	var (
		tags []db.TagWithUsage
		err  error
	)

	if searchQuery == "" {
		tags, err = db.ListAllTags(c.Request().Context())
	} else {
		tags, err = db.SearchTags(c.Request().Context(), searchQuery)
	}

	if err != nil {
		logger.Error("Error fetching tags", "error", err)

		data["Error"] = "Failed to load tags"
	} else {
		data["Tags"] = tags
		data["SearchQuery"] = searchQuery
	}

	data["IsContacts"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Tags", URL: "/tags", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "tags")
}

// EditTagForm renders the edit tag form
func EditTagForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	tagID := c.Param("id")
	if tagID == "" {
		SetErrorFlash(s, "Tag ID is required")
		c.Redirect("/tags", http.StatusSeeOther)

		return
	}

	tag, err := db.GetTag(c.Request().Context(), tagID)
	if err != nil {
		logger.Error("Error fetching tag", "tag_id", tagID, "error", err)
		SetErrorFlash(s, "Tag not found")
		c.Redirect("/tags", http.StatusSeeOther)

		return
	}

	data["Tag"] = tag
	data["IsContacts"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Tags", URL: "/tags", IsCurrent: false},
		{Name: tag.Name, URL: "/tags/" + tagID + "/contacts", IsCurrent: false},
		{Name: "Edit", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "tag_edit")
}

// UpdateTag handles tag rename/update
func UpdateTag(c flamego.Context, s session.Session) {
	tagID := c.Param("id")
	if tagID == "" {
		c.Redirect("/tags", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/tags/"+tagID+"/edit", http.StatusSeeOther)

		return
	}

	form := c.Request().Form

	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		c.Redirect("/tags/"+tagID+"/edit", http.StatusSeeOther)
		return
	}

	err := db.RenameTag(c.Request().Context(), tagID, name, getOptionalString(form.Get("description")))
	if err != nil {
		logger.Error("Error updating tag", "error", err)
		SetErrorFlash(s, "Failed to update tag")
	} else {
		SetSuccessFlash(s, "Tag updated successfully")
	}

	c.Redirect("/tags", http.StatusSeeOther)
}

// DeleteTag handles tag deletion
func DeleteTag(c flamego.Context, s session.Session) {
	tagID := c.Param("id")
	if tagID == "" {
		SetErrorFlash(s, "Tag ID is required")
		c.Redirect("/tags", http.StatusSeeOther)

		return
	}

	err := db.DeleteTag(c.Request().Context(), tagID)
	if err != nil {
		logger.Error("Error deleting tag", "tag_id", tagID, "error", err)
		SetErrorFlash(s, "Failed to delete tag")
	} else {
		SetSuccessFlash(s, "Tag deleted successfully")
	}

	c.Redirect("/tags", http.StatusSeeOther)
}

// ViewTagContacts shows all contacts with a specific tag
func ViewTagContacts(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	tagID := c.Param("id")
	if tagID == "" {
		SetErrorFlash(s, "Tag ID is required")
		c.Redirect("/tags", http.StatusSeeOther)

		return
	}

	tag, err := db.GetTag(c.Request().Context(), tagID)
	if err != nil {
		logger.Error("Error fetching tag", "tag_id", tagID, "error", err)
		SetErrorFlash(s, "Tag not found")
		c.Redirect("/tags", http.StatusSeeOther)

		return
	}

	contacts, err := db.GetContactsByTags(c.Request().Context(), []string{tagID})
	if err != nil {
		logger.Error("Error fetching contacts for tag", "tag_id", tagID, "error", err)

		data["Error"] = "Failed to load contacts"
	} else {
		data["Contacts"] = contacts
	}

	data["Tag"] = tag
	data["IsContacts"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Tags", URL: "/tags", IsCurrent: false},
		{Name: tag.Name, URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "tag_contacts")
}
