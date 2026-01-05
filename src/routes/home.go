/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

// Welcome renders the welcome/dashboard page
func Welcome(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Get total contacts count
	totalContacts, err := db.GetContactsCount(ctx)
	if err != nil {
		log.Printf("Error fetching contacts count: %v", err)
		totalContacts = 0
	}
	data["TotalContacts"] = totalContacts

	// Get overdue contacts count
	overdueContacts, err := db.GetOverdueContacts(ctx)
	if err != nil {
		log.Printf("Error fetching overdue contacts: %v", err)
		data["OverdueCount"] = 0
	} else {
		data["OverdueCount"] = len(overdueContacts)
	}

	// Get QSO count
	qsoCount, err := db.GetQSOCount(ctx)
	if err != nil {
		log.Printf("Error fetching QSO count: %v", err)
		qsoCount = 0
	}
	data["QSOCount"] = qsoCount

	// Get recent contacts (last 5 modified)
	recentContacts, err := db.GetRecentContacts(ctx, 5)
	if err != nil {
		log.Printf("Error fetching recent contacts: %v", err)
	} else {
		data["RecentContacts"] = recentContacts
	}

	data["IsWelcome"] = true
	t.HTML(http.StatusOK, "welcome")
}

// Home renders the contacts list
func Home(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Get tag filter from URL query
	tagIDs := c.QueryStrings("tag")

	var contacts []db.ContactListItem
	var err error

	if len(tagIDs) == 0 {
		// No filter - list all contacts
		contacts, err = db.ListContacts(ctx)
	} else {
		// Filter by tags
		contacts, err = db.GetContactsByTags(ctx, tagIDs)
	}

	if err != nil {
		log.Printf("Error fetching contacts: %v", err)
		data["Error"] = "Failed to load contacts"
	} else {
		data["Contacts"] = contacts
	}

	// Fetch all tags for the filter UI
	allTags, err := db.ListAllTags(ctx)
	if err != nil {
		log.Printf("Error fetching tags: %v", err)
	} else {
		data["AllTags"] = allTags
		data["SelectedTags"] = tagIDs
	}

	// Get overdue contacts count for the button
	overdueContacts, err := db.GetOverdueContacts(ctx)
	if err != nil {
		log.Printf("Error fetching overdue contacts: %v", err)
		data["OverdueCount"] = 0
	} else {
		data["OverdueCount"] = len(overdueContacts)
	}

	t.HTML(http.StatusOK, "home")
}

// Overdue renders the overdue contacts list
func Overdue(c flamego.Context, t template.Template, data template.Data) {
	// Fetch overdue contacts from database
	contacts, err := db.GetOverdueContacts(c.Request().Context())
	if err != nil {
		log.Printf("Error fetching overdue contacts: %v", err)
		data["Error"] = "Failed to load overdue contacts"
	} else {
		data["OverdueContacts"] = contacts
	}

	data["IsOverdue"] = true
	t.HTML(http.StatusOK, "overdue")
}
