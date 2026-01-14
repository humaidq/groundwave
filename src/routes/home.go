/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
	"github.com/humaidq/groundwave/whatsapp"
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

	// Get inventory count
	inventoryCount, err := db.GetInventoryCount(ctx)
	if err != nil {
		log.Printf("Error fetching inventory count: %v", err)
		inventoryCount = 0
	}
	data["InventoryCount"] = inventoryCount

	// Get notes count (from zettelkasten cache)
	orgFiles, err := db.ListOrgFiles(ctx)
	if err != nil {
		log.Printf("Error fetching org files: %v", err)
		data["NotesCount"] = 0
	} else {
		data["NotesCount"] = len(orgFiles)
	}

	// Get recent QSOs (last 5)
	recentQSOs, err := db.ListRecentQSOs(ctx, 5)
	if err != nil {
		log.Printf("Error fetching recent QSOs: %v", err)
	} else {
		data["RecentQSOs"] = recentQSOs
	}

	// Get WhatsApp status
	waClient := whatsapp.GetClient()
	if waClient != nil {
		data["WhatsAppStatus"] = string(waClient.GetStatus())
	} else {
		data["WhatsAppStatus"] = "unavailable"
	}

	data["IsWelcome"] = true
	t.HTML(http.StatusOK, "welcome")
}

// Home renders the contacts list
func Home(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Check private mode from session
	privateMode := false
	if pm := s.Get("private_mode"); pm != nil {
		privateMode = pm.(bool)
	}
	data["PrivateMode"] = privateMode

	// Get tag filter from URL query
	tagIDs := c.QueryStrings("tag")

	// Get data filters from URL query
	filterStrs := c.QueryStrings("filter")
	var filters []db.ContactFilter
	var activeFilters []string
	for _, f := range filterStrs {
		filter := db.ContactFilter(f)
		if db.ValidContactFilters[filter] {
			filters = append(filters, filter)
			activeFilters = append(activeFilters, f)
		}
	}

	var contacts []db.ContactListItem
	var err error

	// Use ListContactsWithFilters to handle private mode sorting
	opts := db.ContactListOptions{
		Filters:        filters,
		TagIDs:         tagIDs,
		IsService:      false,
		AlphabeticSort: privateMode, // Sort alphabetically in private mode
	}
	contacts, err = db.ListContactsWithFilters(ctx, opts)

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

	// Pass active filters to template
	data["ActiveFilters"] = activeFilters

	// Get overdue contacts count for the button
	overdueContacts, err := db.GetOverdueContacts(ctx)
	if err != nil {
		log.Printf("Error fetching overdue contacts: %v", err)
		data["OverdueCount"] = 0
	} else {
		data["OverdueCount"] = len(overdueContacts)
	}

	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Contacts", URL: "/contacts", IsCurrent: true},
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
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Contacts", URL: "/contacts", IsCurrent: false},
		{Name: "Overdue", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "overdue")
}
