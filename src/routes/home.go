/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"
	"sort"
	"time"

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
	data["IsContacts"] = true

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
	data["IsContacts"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Contacts", URL: "/contacts", IsCurrent: false},
		{Name: "Overdue", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "overdue")
}

// TimelineDay groups journal entries and logs for a single day.
type TimelineDay struct {
	Date       time.Time
	DateString string
	Journal    *db.JournalEntry
	Followups  []db.HealthFollowupSummary
	Logs       []db.ContactLogTimelineEntry
	Notes      []db.ZKTimelineNote
	QSOs       []db.QSOListItem
}

// Timeline renders the unified journal/contact log timeline.
func Timeline(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	logs, err := db.ListContactLogsTimeline(ctx)
	if err != nil {
		log.Printf("Error fetching contact logs: %v", err)
		data["Error"] = "Failed to load timeline"
		logs = []db.ContactLogTimelineEntry{}
	}

	qsos, err := db.ListQSOs(ctx)
	if err != nil {
		log.Printf("Error fetching QSOs: %v", err)
		qsos = []db.QSOListItem{}
	}

	journalEntries := db.GetJournalEntriesFromCache()
	notesByDate := db.GetZKTimelineNotesByDate()

	primaryProfile, err := db.GetPrimaryHealthProfile(ctx)
	if err != nil {
		log.Printf("Error fetching primary health profile: %v", err)
	}

	var followups []db.HealthFollowupSummary
	if primaryProfile != nil {
		data["PrimaryHealthProfileName"] = primaryProfile.Name
		followups, err = db.ListFollowups(ctx, primaryProfile.ID.String())
		if err != nil {
			log.Printf("Error fetching follow-ups for primary health profile: %v", err)
			followups = []db.HealthFollowupSummary{}
		}
	}

	dayMap := make(map[string]*TimelineDay)
	for i := range journalEntries {
		entry := &journalEntries[i]
		dateString := entry.DateString
		dayMap[dateString] = &TimelineDay{
			Date:       entry.Date,
			DateString: dateString,
			Journal:    entry,
			Followups:  []db.HealthFollowupSummary{},
			Logs:       []db.ContactLogTimelineEntry{},
			Notes:      []db.ZKTimelineNote{},
			QSOs:       []db.QSOListItem{},
		}
	}

	for _, followup := range followups {
		entryDate := time.Date(
			followup.FollowupDate.Year(),
			followup.FollowupDate.Month(),
			followup.FollowupDate.Day(),
			0, 0, 0, 0,
			followup.FollowupDate.Location(),
		)
		dateString := entryDate.Format("2006-01-02")
		day, exists := dayMap[dateString]
		if !exists {
			day = &TimelineDay{
				Date:       entryDate,
				DateString: dateString,
				Followups:  []db.HealthFollowupSummary{},
				Logs:       []db.ContactLogTimelineEntry{},
				Notes:      []db.ZKTimelineNote{},
				QSOs:       []db.QSOListItem{},
			}
			dayMap[dateString] = day
		}
		day.Followups = append(day.Followups, followup)
	}

	for date, notes := range notesByDate {
		entryDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			continue
		}
		day, exists := dayMap[date]
		if !exists {
			day = &TimelineDay{
				Date:       entryDate,
				DateString: date,
				Followups:  []db.HealthFollowupSummary{},
				Logs:       []db.ContactLogTimelineEntry{},
				Notes:      []db.ZKTimelineNote{},
				QSOs:       []db.QSOListItem{},
			}
			dayMap[date] = day
		}
		day.Notes = append(day.Notes, notes...)
	}

	for _, logEntry := range logs {
		entryDate := time.Date(
			logEntry.LoggedAt.Year(),
			logEntry.LoggedAt.Month(),
			logEntry.LoggedAt.Day(),
			0, 0, 0, 0,
			logEntry.LoggedAt.Location(),
		)
		dateString := entryDate.Format("2006-01-02")
		day, exists := dayMap[dateString]
		if !exists {
			day = &TimelineDay{
				Date:       entryDate,
				DateString: dateString,
				Followups:  []db.HealthFollowupSummary{},
				Logs:       []db.ContactLogTimelineEntry{},
				Notes:      []db.ZKTimelineNote{},
				QSOs:       []db.QSOListItem{},
			}
			dayMap[dateString] = day
		}
		day.Logs = append(day.Logs, logEntry)
	}

	for _, qso := range qsos {
		entryDate := time.Date(
			qso.QSODate.Year(),
			qso.QSODate.Month(),
			qso.QSODate.Day(),
			0, 0, 0, 0,
			qso.QSODate.Location(),
		)
		dateString := entryDate.Format("2006-01-02")
		day, exists := dayMap[dateString]
		if !exists {
			day = &TimelineDay{
				Date:       entryDate,
				DateString: dateString,
				Followups:  []db.HealthFollowupSummary{},
				Logs:       []db.ContactLogTimelineEntry{},
				Notes:      []db.ZKTimelineNote{},
				QSOs:       []db.QSOListItem{},
			}
			dayMap[dateString] = day
		}
		day.QSOs = append(day.QSOs, qso)
	}

	days := make([]TimelineDay, 0, len(dayMap))
	for _, day := range dayMap {
		days = append(days, *day)
	}

	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.After(days[j].Date)
	})

	data["Days"] = days
	data["IsTimeline"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Timeline", URL: "/timeline", IsCurrent: true},
	}
	data["PageTitle"] = "Timeline"

	t.HTML(http.StatusOK, "timeline")
}

// ViewJournalEntry renders a full daily journal entry.
func ViewJournalEntry(c flamego.Context, t template.Template, data template.Data) {
	date := c.Param("date")
	entry, exists := db.GetJournalEntryByDate(date)
	if !exists {
		data["Error"] = "Journal entry not found"
		data["Breadcrumbs"] = []BreadcrumbItem{
			{Name: "Timeline", URL: "/timeline", IsCurrent: false},
			{Name: date, URL: "", IsCurrent: true},
		}
		data["IsTimeline"] = true
		t.HTML(http.StatusNotFound, "journal_entry")
		return
	}

	data["Entry"] = entry
	data["IsTimeline"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Timeline", URL: "/timeline", IsCurrent: false},
		{Name: entry.DateString, URL: "", IsCurrent: true},
	}
	data["PageTitle"] = "Journal " + entry.DateString

	t.HTML(http.StatusOK, "journal_entry")
}
