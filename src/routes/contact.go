/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

// digitRegex matches digits for phone validation
var digitRegex = regexp.MustCompile(`\d`)

// isValidPhone checks if a phone number has at least 7 digits
func isValidPhone(phone string) bool {
	digits := digitRegex.FindAllString(phone, -1)
	return len(digits) >= 7
}

type ActivityGridWeek struct {
	WeekStart  time.Time
	WeekEnd    time.Time
	WeekNumber int
	Count      int
	Level      int
	Tooltip    string
}

type ActivityGridYear struct {
	Year  int
	Weeks []ActivityGridWeek
}

func isoWeekStart(year int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return jan4.AddDate(0, 0, -(weekday - 1))
}

func activityLevel(count int) int {
	switch {
	case count == 0:
		return 0
	case count <= 2:
		return 1
	case count <= 5:
		return 2
	case count <= 9:
		return 3
	default:
		return 4
	}
}

func activityLabel(count int) string {
	if count == 1 {
		return "activity"
	}
	return "activities"
}

func buildActivityGrid(weekCounts map[string]int, currentYear int, yearCount int) []ActivityGridYear {
	rows := make([]ActivityGridYear, 0, yearCount)
	startYear := currentYear - yearCount + 1
	for year := currentYear; year >= startYear; year-- {
		yearStart := isoWeekStart(year)
		weeks := make([]ActivityGridWeek, 0, 52)
		for i := 0; i < 52; i++ {
			weekStart := yearStart.AddDate(0, 0, 7*i)
			weekKey := weekStart.Format("2006-01-02")
			count := weekCounts[weekKey]
			_, weekNumber := weekStart.ISOWeek()
			weekEnd := weekStart.AddDate(0, 0, 6)
			tooltip := fmt.Sprintf("%d-W%02d (%s – %s): %d %s", year, weekNumber, weekStart.Format("Jan 2, 2006"), weekEnd.Format("Jan 2, 2006"), count, activityLabel(count))
			weeks = append(weeks, ActivityGridWeek{
				WeekStart:  weekStart,
				WeekEnd:    weekEnd,
				WeekNumber: weekNumber,
				Count:      count,
				Level:      activityLevel(count),
				Tooltip:    tooltip,
			})
		}
		rows = append(rows, ActivityGridYear{Year: year, Weeks: weeks})
	}
	return rows
}

// NewContactForm renders the add contact form
func NewContactForm(c flamego.Context, t template.Template, data template.Data) {
	data["IsContacts"] = true
	isService := c.Query("is_service") == "true"
	data["IsService"] = isService
	if isService {
		data["Breadcrumbs"] = []BreadcrumbItem{
			{Name: "Contacts", URL: "/contacts", IsCurrent: false},
			{Name: "Service Contacts", URL: "/service-contacts", IsCurrent: false},
			{Name: "New Service Contact", URL: "", IsCurrent: true},
		}
	} else {
		data["Breadcrumbs"] = []BreadcrumbItem{
			{Name: "Contacts", URL: "/contacts", IsCurrent: false},
			{Name: "New Contact", URL: "", IsCurrent: true},
		}
	}

	// Fetch all tags for autocomplete
	allTags, err := db.ListAllTags(c.Request().Context())
	if err != nil {
		logger.Error("Error fetching tags", "error", err)
	} else {
		data["AllTags"] = allTags
	}

	t.HTML(http.StatusOK, "contact_new")
}

// CreateContact handles the contact creation form submission
func CreateContact(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	// Parse form data
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		data["Error"] = "Failed to parse form data"
		t.HTML(http.StatusBadRequest, "contact_new")
		return
	}

	form := c.Request().Form

	// Check if this is a CardDAV import
	cardDAVUUID := getOptionalString(form.Get("carddav_uuid"))
	var nameGiven string
	var nameFamily *string
	var organization *string
	var title *string

	if cardDAVUUID != nil && *cardDAVUUID != "" {
		// Import from CardDAV
		cardDAVContact, err := db.GetCardDAVContact(c.Request().Context(), *cardDAVUUID)
		if err != nil {
			logger.Error("Error fetching CardDAV contact", "error", err)
			data["Error"] = "Failed to fetch contact from CardDAV: " + err.Error()
			t.HTML(http.StatusInternalServerError, "contact_new")
			return
		}

		// Use CardDAV data
		nameGiven = cardDAVContact.GivenName
		if nameGiven == "" {
			nameGiven = cardDAVContact.DisplayName
		}
		if cardDAVContact.FamilyName != "" {
			nameFamily = &cardDAVContact.FamilyName
		}
		if cardDAVContact.Organization != "" {
			organization = &cardDAVContact.Organization
		}
		if cardDAVContact.Title != "" {
			title = &cardDAVContact.Title
		}

		// Validate we got at least a name
		if nameGiven == "" {
			data["Error"] = "CardDAV contact has no name"
			t.HTML(http.StatusBadRequest, "contact_new")
			return
		}
	} else {
		// Standalone contact - get from form
		nameGiven = strings.TrimSpace(form.Get("name_given"))
		if nameGiven == "" {
			data["Error"] = "First name is required"
			data["FormData"] = form
			t.HTML(http.StatusBadRequest, "contact_new")
			return
		}
		nameFamily = getOptionalString(form.Get("name_family"))
		organization = getOptionalString(form.Get("organization"))
		title = getOptionalString(form.Get("title"))
	}

	// Get tier, default to C
	tierStr := form.Get("tier")
	if tierStr == "" {
		tierStr = "C"
	}
	tier := db.Tier(tierStr)

	// Check if this is a service contact
	isService := form.Get("is_service") == "on"

	// Create input struct
	input := db.CreateContactInput{
		NameGiven:    nameGiven,
		NameFamily:   nameFamily,
		Organization: organization,
		Title:        title,
		Email:        getOptionalString(form.Get("email")),
		Phone:        getOptionalString(form.Get("phone")),
		CallSign:     getOptionalString(form.Get("call_sign")),
		CardDAVUUID:  cardDAVUUID,
		IsService:    isService,
		Tier:         tier,
	}

	// Create contact in database
	contactID, err := db.CreateContact(c.Request().Context(), input)
	if err != nil {
		logger.Error("Error creating contact", "error", err)
		data["Error"] = "Failed to create contact: " + err.Error()
		data["FormData"] = form
		t.HTML(http.StatusInternalServerError, "contact_new")
		return
	}

	// Add tag if provided
	tagName := strings.TrimSpace(form.Get("tag"))
	if tagName != "" {
		err = db.AddTagToContact(c.Request().Context(), contactID, tagName)
		if err != nil {
			logger.Error("Error adding tag to contact", "error", err)
			// Don't fail the whole operation, just log the error
		}
	}

	// Redirect to contact view page on success
	logger.Info("Successfully created contact", "contact_id", contactID)
	SetSuccessFlash(s, "Contact created successfully")
	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// ViewContact displays a contact's details
func ViewContact(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		SetErrorFlash(s, "Contact ID is required")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error fetching contact", "contact_id", contactID, "error", err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	// If contact is linked to CardDAV, sync details before displaying
	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		err := db.SyncContactFromCardDAV(c.Request().Context(), contactID, *contact.CardDAVUUID)
		if err != nil {
			logger.Error("Error syncing contact from CardDAV", "contact_id", contactID, "error", err)
			// Continue anyway, just log the error
		} else {
			// Reload contact after sync
			contact, err = db.GetContact(c.Request().Context(), contactID)
			if err != nil {
				logger.Error("Error reloading contact after sync", "contact_id", contactID, "error", err)
			}
		}
	}

	sensitiveAccess := HasSensitiveAccess(s, time.Now())
	data["SensitiveAccess"] = sensitiveAccess

	data["Contact"] = contact
	data["ContactName"] = contact.NameDisplay
	data["IsContacts"] = true
	data["TierLower"] = strings.ToLower(string(contact.Tier))
	data["CardDAVContact"] = contact.CardDAVContact

	if !contact.IsService && sensitiveAccess {
		currentYear := time.Now().UTC().Year()
		start := isoWeekStart(currentYear - 4)
		end := isoWeekStart(currentYear + 1)
		weekCounts, err := db.ListContactWeeklyActivityCounts(c.Request().Context(), contactID, start, end)
		if err != nil {
			logger.Error("Error fetching activity grid for contact", "contact_id", contactID, "error", err)
			weekCounts = map[string]int{}
		}
		data["ActivityGrid"] = buildActivityGrid(weekCounts, currentYear, 5)
	}

	if contact.IsService {
		data["Breadcrumbs"] = []BreadcrumbItem{
			{Name: "Contacts", URL: "/contacts", IsCurrent: false},
			{Name: "Service Contacts", URL: "/service-contacts", IsCurrent: false},
			{Name: contact.NameDisplay, URL: "", IsCurrent: true},
		}
	} else {
		data["Breadcrumbs"] = []BreadcrumbItem{
			{Name: "Contacts", URL: "/contacts", IsCurrent: false},
			{Name: contact.NameDisplay, URL: "", IsCurrent: true},
		}
	}

	// If contact has a call sign, fetch QSOs
	if contact.CallSign != nil && *contact.CallSign != "" {
		qsos, err := db.GetQSOsByCallSign(c.Request().Context(), *contact.CallSign)
		if err != nil {
			logger.Error("Error fetching QSOs for call sign", "call_sign", *contact.CallSign, "error", err)
		} else {
			data["QSOs"] = qsos
		}
	}

	// Fetch all tags for autocomplete
	allTags, err := db.ListAllTags(c.Request().Context())
	if err != nil {
		logger.Error("Error fetching tags", "error", err)
	} else {
		data["AllTags"] = allTags
	}

	t.HTML(http.StatusOK, "contact_view")
}

// ViewContactChats displays chat history for a contact
func ViewContactChats(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		SetErrorFlash(s, "Contact ID is required")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error fetching contact", "contact_id", contactID, "error", err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	if contact.IsService {
		SetErrorFlash(s, "Chats are not available for service contacts")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	chats, err := db.GetContactChats(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error fetching chats for contact", "contact_id", contactID, "error", err)
	}

	data["Contact"] = contact
	data["ContactName"] = contact.NameDisplay
	data["Chats"] = chats
	data["IsContacts"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Contacts", URL: "/contacts", IsCurrent: false},
		{Name: contact.NameDisplay, URL: "/contact/" + contactID, IsCurrent: false},
		{Name: "Chats", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "contact_chats")
}

// AddContactChat handles adding a manual chat entry
func AddContactChat(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	isService, err := db.IsServiceContact(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error checking service contact", "contact_id", contactID, "error", err)
		c.Redirect("/contact/"+contactID+"/chats", http.StatusSeeOther)
		return
	}
	if isService {
		SetErrorFlash(s, "Chats are not available for service contacts")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/contact/"+contactID+"/chats", http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	message := strings.TrimSpace(form.Get("message"))
	if message == "" {
		SetErrorFlash(s, "Message content is required")
		c.Redirect("/contact/"+contactID+"/chats", http.StatusSeeOther)
		return
	}

	platform := parseChatPlatform(form.Get("platform"))
	sender := parseChatSender(form.Get("sender"))

	var sentAt *string
	if value := strings.TrimSpace(form.Get("sent_at")); value != "" {
		sentAt = &value
	}

	input := db.AddChatInput{
		ContactID: contactID,
		Platform:  platform,
		Sender:    sender,
		Message:   message,
		SentAt:    sentAt,
	}

	if err := db.AddChat(c.Request().Context(), input); err != nil {
		logger.Error("Error adding chat entry", "error", err)
	}

	c.Redirect("/contact/"+contactID+"/chats", http.StatusSeeOther)
}

// GenerateContactChatSummary summarizes recent chat history for a contact.
func GenerateContactChatSummary(c flamego.Context, s session.Session) {
	c.ResponseWriter().Header().Set("Content-Type", "application/json")

	contactID := c.Param("id")
	if contactID == "" {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "contact ID is required"}); err != nil {
			logger.Error("Error encoding chat summary error", "error", err)
		}
		return
	}

	ctx := c.Request().Context()

	contact, err := db.GetContact(ctx, contactID)
	if err != nil {
		logger.Error("Error fetching contact", "contact_id", contactID, "error", err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "contact not found"}); err != nil {
			logger.Error("Error encoding chat summary error", "error", err)
		}
		return
	}

	if contact.IsService {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "service contacts have no chat summary"}); err != nil {
			logger.Error("Error encoding chat summary error", "error", err)
		}
		return
	}

	if !HasSensitiveAccess(s, time.Now()) {
		c.ResponseWriter().WriteHeader(http.StatusForbidden)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "sensitive access is locked"}); err != nil {
			logger.Error("Error encoding chat summary error", "error", err)
		}
		return
	}

	since := time.Now().Add(-48 * time.Hour)
	chats, err := db.GetContactChatsSince(ctx, contactID, since)
	if err != nil {
		logger.Error("Error fetching chats for contact", "contact_id", contactID, "error", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to load chat history"}); err != nil {
			logger.Error("Error encoding chat summary error", "error", err)
		}
		return
	}

	if len(chats) == 0 {
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]interface{}{"summary": "No recent chats", "empty": true}); err != nil {
			logger.Error("Error encoding chat summary response", "error", err)
		}
		return
	}

	var summaryBuilder strings.Builder
	if err := db.StreamContactChatSummary(ctx, &contact.Contact, chats, func(chunk string) error {
		summaryBuilder.WriteString(chunk)
		return nil
	}); err != nil {
		logger.Error("Error generating chat summary", "error", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to generate summary"}); err != nil {
			logger.Error("Error encoding chat summary error", "error", err)
		}
		return
	}

	summary := strings.TrimSpace(summaryBuilder.String())
	isEmpty := false
	if summary == "" {
		summary = "No recent chats"
		isEmpty = true
	}

	if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]interface{}{"summary": summary, "empty": isEmpty}); err != nil {
		logger.Error("Error encoding chat summary response", "error", err)
	}
}

func parseChatPlatform(value string) db.ChatPlatform {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(db.ChatPlatformEmail):
		return db.ChatPlatformEmail
	case string(db.ChatPlatformWhatsApp):
		return db.ChatPlatformWhatsApp
	case string(db.ChatPlatformSignal):
		return db.ChatPlatformSignal
	case string(db.ChatPlatformWeChat):
		return db.ChatPlatformWeChat
	case string(db.ChatPlatformTeams):
		return db.ChatPlatformTeams
	case string(db.ChatPlatformSlack):
		return db.ChatPlatformSlack
	case string(db.ChatPlatformOther):
		return db.ChatPlatformOther
	default:
		return db.ChatPlatformManual
	}
}

func parseChatSender(value string) db.ChatSender {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(db.ChatSenderMe):
		return db.ChatSenderMe
	case string(db.ChatSenderMix):
		return db.ChatSenderMix
	default:
		return db.ChatSenderThem
	}
}

// EditContactForm displays the edit contact form
func EditContactForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		SetErrorFlash(s, "Contact ID is required")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error fetching contact", "contact_id", contactID, "error", err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	data["Contact"] = contact
	data["ContactName"] = contact.NameDisplay
	data["IsContacts"] = true
	if contact.IsService {
		data["Breadcrumbs"] = []BreadcrumbItem{
			{Name: "Contacts", URL: "/contacts", IsCurrent: false},
			{Name: "Service Contacts", URL: "/service-contacts", IsCurrent: false},
			{Name: contact.NameDisplay, URL: "/contact/" + contactID, IsCurrent: false},
			{Name: "Edit", URL: "", IsCurrent: true},
		}
	} else {
		data["Breadcrumbs"] = []BreadcrumbItem{
			{Name: "Contacts", URL: "/contacts", IsCurrent: false},
			{Name: contact.NameDisplay, URL: "/contact/" + contactID, IsCurrent: false},
			{Name: "Edit", URL: "", IsCurrent: true},
		}
	}
	t.HTML(http.StatusOK, "contact_edit")
}

// UpdateContact handles the contact update form submission
func UpdateContact(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		SetErrorFlash(s, "Contact ID is required")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	// Parse form data
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		data["Error"] = "Failed to parse form data"
		c.Redirect("/contact/"+contactID+"/edit", http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	// Get and validate required fields
	nameGiven := strings.TrimSpace(form.Get("name_given"))
	if nameGiven == "" {
		data["Error"] = "First name is required"
		data["FormData"] = form
		c.Redirect("/contact/"+contactID+"/edit", http.StatusSeeOther)
		return
	}

	// Get tier, default to C
	tierStr := form.Get("tier")
	if tierStr == "" {
		tierStr = "C"
	}
	tier := db.Tier(tierStr)

	// Check if service status toggle was requested
	if form.Get("toggle_service") == "true" {
		isService := form.Get("is_service") == "true"
		err := db.ToggleServiceStatus(c.Request().Context(), contactID, isService)
		if err != nil {
			logger.Error("Error toggling service status", "error", err)
		}
	}

	// Create input struct
	input := db.UpdateContactInput{
		ID:           contactID,
		NameGiven:    nameGiven,
		NameFamily:   getOptionalString(form.Get("name_family")),
		Organization: getOptionalString(form.Get("organization")),
		Title:        getOptionalString(form.Get("title")),
		CallSign:     getOptionalString(form.Get("call_sign")),
		Tier:         tier,
	}

	// Update contact in database
	err := db.UpdateContact(c.Request().Context(), input)
	if err != nil {
		logger.Error("Error updating contact", "error", err)
		data["Error"] = "Failed to update contact: " + err.Error()
		c.Redirect("/contact/"+contactID+"/edit", http.StatusSeeOther)
		return
	}

	// If contact is linked to CardDAV, push the update
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			logger.Error("Error pushing contact update to CardDAV", "error", err)
			// Don't fail the whole operation, just log the error
		}
	}

	// Redirect to contact view page on success
	logger.Info("Successfully updated contact", "contact_id", contactID)
	SetSuccessFlash(s, "Contact updated successfully")
	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// AddEmail handles adding a new email to a contact
func AddEmail(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	email := strings.TrimSpace(form.Get("email"))
	isPrimary := isPrimaryChecked(form.Get("is_primary"))
	if email == "" {
		SetErrorFlash(s, "Email address is required")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	emailType := db.EmailType(form.Get("email_type"))
	if emailType == "" {
		emailType = db.EmailPersonal
	}

	// Check if contact is linked to CardDAV first
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error getting contact", "error", err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		// For CardDAV-linked contacts, add to contact in memory and push to CardDAV
		// The sync will bring it back with source='carddav'
		if isPrimary {
			for i := range contact.Emails {
				contact.Emails[i].IsPrimary = false
			}
		}
		newEmail := db.ContactEmail{
			Email:     email,
			EmailType: emailType,
			IsPrimary: isPrimary,
			Source:    "carddav",
		}
		contact.Emails = append(contact.Emails, newEmail)

		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			logger.Error("Error pushing new email to CardDAV", "error", err)
			SetErrorFlash(s, "Failed to sync email to CardDAV")
		} else {
			// Sync the contact back to get the email with proper ID
			if err := db.SyncContactFromCardDAV(c.Request().Context(), contact.ID.String(), *contact.CardDAVUUID); err != nil {
				logger.Error("Error syncing contact after adding email", "error", err)
			}
		}
	} else {
		// For local contacts, add to database directly
		input := db.AddEmailInput{
			ContactID: contactID,
			Email:     email,
			EmailType: emailType,
			IsPrimary: isPrimary,
		}

		if err := db.AddEmail(c.Request().Context(), input); err != nil {
			logger.Error("Error adding email", "error", err)
			SetErrorFlash(s, "Failed to add email")
		}
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// AddPhone handles adding a new phone to a contact
func AddPhone(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	phone := strings.TrimSpace(form.Get("phone"))
	if phone == "" {
		SetErrorFlash(s, "Phone number is required")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if !isValidPhone(phone) {
		SetErrorFlash(s, "Phone number must have at least 7 digits")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	phoneType := db.PhoneType(form.Get("phone_type"))
	if phoneType == "" {
		phoneType = db.PhoneCell
	}

	isPrimary := isPrimaryChecked(form.Get("is_primary"))

	// Check if contact is linked to CardDAV first
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error getting contact", "error", err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		// For CardDAV-linked contacts, add to contact in memory and push to CardDAV
		// The sync will bring it back with source='carddav'
		if isPrimary {
			for i := range contact.Phones {
				contact.Phones[i].IsPrimary = false
			}
		}
		newPhone := db.ContactPhone{
			Phone:     phone,
			PhoneType: phoneType,
			IsPrimary: isPrimary,
			Source:    "carddav",
		}
		contact.Phones = append(contact.Phones, newPhone)

		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			logger.Error("Error pushing new phone to CardDAV", "error", err)
			SetErrorFlash(s, "Failed to sync phone to CardDAV")
		} else {
			// Sync the contact back to get the phone with proper ID
			if err := db.SyncContactFromCardDAV(c.Request().Context(), contact.ID.String(), *contact.CardDAVUUID); err != nil {
				logger.Error("Error syncing contact after adding phone", "error", err)
			}
		}
	} else {
		// For local contacts, add to database directly
		input := db.AddPhoneInput{
			ContactID: contactID,
			Phone:     phone,
			PhoneType: phoneType,
			IsPrimary: isPrimary,
		}

		if err := db.AddPhone(c.Request().Context(), input); err != nil {
			logger.Error("Error adding phone", "error", err)
			SetErrorFlash(s, "Failed to add phone")
		}
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// AddURL handles adding a new URL/social media to a contact
func AddURL(c flamego.Context) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	url := strings.TrimSpace(form.Get("url"))
	if url == "" {
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	urlType := db.URLType(form.Get("url_type"))
	if urlType == "" {
		urlType = db.URLWebsite
	}

	input := db.AddURLInput{
		ContactID:   contactID,
		URL:         url,
		URLType:     urlType,
		Description: getOptionalString(form.Get("description")),
	}

	err := db.AddURL(c.Request().Context(), input)
	if err != nil {
		logger.Error("Error adding URL", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// DeleteEmail handles deleting an email
func DeleteEmail(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	emailID := c.Param("email_id")

	if contactID == "" || emailID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeleteEmail(c.Request().Context(), emailID, contactID)
	if err != nil {
		logger.Error("Error deleting email", "error", err)
		SetErrorFlash(s, "Failed to delete email")
	}

	// If contact is linked to CardDAV, push the deletion
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			logger.Error("Error pushing email deletion to CardDAV", "error", err)
		}
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// DeletePhone handles deleting a phone
func DeletePhone(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	phoneID := c.Param("phone_id")

	if contactID == "" || phoneID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeletePhone(c.Request().Context(), phoneID, contactID)
	if err != nil {
		logger.Error("Error deleting phone", "error", err)
		SetErrorFlash(s, "Failed to delete phone")
	}

	// If contact is linked to CardDAV, push the deletion
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			logger.Error("Error pushing phone deletion to CardDAV", "error", err)
		}
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// UpdateEmail handles updating an email
func UpdateEmail(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	emailID := c.Param("email_id")

	if contactID == "" || emailID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	email := strings.TrimSpace(form.Get("email"))
	emailType := db.EmailType(form.Get("email_type"))
	isPrimary := isPrimaryChecked(form.Get("is_primary"))

	if email == "" {
		SetErrorFlash(s, "Email address is required")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	input := db.UpdateEmailInput{
		ID:        emailID,
		ContactID: contactID,
		Email:     email,
		EmailType: emailType,
		IsPrimary: isPrimary,
	}

	err := db.UpdateEmail(c.Request().Context(), input)
	if err != nil {
		logger.Error("Error updating email", "error", err)
		SetErrorFlash(s, "Failed to update email")
	}

	// If contact is linked to CardDAV, push the update
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if isPrimary {
			for i := range contact.Emails {
				contact.Emails[i].IsPrimary = contact.Emails[i].ID.String() == emailID
			}
		} else {
			for i := range contact.Emails {
				if contact.Emails[i].ID.String() == emailID {
					contact.Emails[i].IsPrimary = false
					break
				}
			}
		}
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			logger.Error("Error pushing email update to CardDAV", "error", err)
		}
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// UpdatePhone handles updating a phone
func UpdatePhone(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	phoneID := c.Param("phone_id")

	if contactID == "" || phoneID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	phone := strings.TrimSpace(form.Get("phone"))
	phoneType := db.PhoneType(form.Get("phone_type"))
	isPrimary := isPrimaryChecked(form.Get("is_primary"))

	if phone == "" {
		SetErrorFlash(s, "Phone number is required")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if !isValidPhone(phone) {
		SetErrorFlash(s, "Phone number must have at least 7 digits")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	input := db.UpdatePhoneInput{
		ID:        phoneID,
		ContactID: contactID,
		Phone:     phone,
		PhoneType: phoneType,
		IsPrimary: isPrimary,
	}

	err := db.UpdatePhone(c.Request().Context(), input)
	if err != nil {
		logger.Error("Error updating phone", "error", err)
		SetErrorFlash(s, "Failed to update phone")
	}

	// If contact is linked to CardDAV, push the update
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if isPrimary {
			for i := range contact.Phones {
				contact.Phones[i].IsPrimary = contact.Phones[i].ID.String() == phoneID
			}
		} else {
			for i := range contact.Phones {
				if contact.Phones[i].ID.String() == phoneID {
					contact.Phones[i].IsPrimary = false
					break
				}
			}
		}
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			logger.Error("Error pushing phone update to CardDAV", "error", err)
		}
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// DeleteURL handles deleting a URL
func DeleteURL(c flamego.Context) {
	contactID := c.Param("id")
	urlID := c.Param("url_id")

	if contactID == "" || urlID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeleteURL(c.Request().Context(), urlID)
	if err != nil {
		logger.Error("Error deleting URL", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// DeleteContact handles deleting an entire contact
func DeleteContact(c flamego.Context, s session.Session) {
	contactID := c.Param("id")

	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeleteContact(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error deleting contact", "error", err)
		SetErrorFlash(s, "Failed to delete contact")
	} else {
		SetSuccessFlash(s, "Contact deleted successfully")
	}

	c.Redirect("/", http.StatusSeeOther)
}

// AddLog handles adding a new contact log
func AddLog(c flamego.Context) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	logType := db.LogType(form.Get("log_type"))
	if logType == "" {
		logType = db.LogGeneral
	}

	input := db.AddLogInput{
		ContactID: contactID,
		LogType:   logType,
		LoggedAt:  getOptionalString(form.Get("logged_at")),
		Subject:   getOptionalString(form.Get("subject")),
		Content:   getOptionalString(form.Get("content")),
	}

	err := db.AddLog(c.Request().Context(), input)
	if err != nil {
		logger.Error("Error adding log", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

func isPrimaryChecked(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "on" || value == "true" || value == "1"
}

// DeleteLog handles deleting a contact log
func DeleteLog(c flamego.Context) {
	contactID := c.Param("id")
	logID := c.Param("log_id")

	if contactID == "" || logID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeleteLog(c.Request().Context(), logID)
	if err != nil {
		logger.Error("Error deleting log", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// LinkCardDAV handles linking a contact with a CardDAV contact
func LinkCardDAV(c flamego.Context, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	cardDAVUUID := strings.TrimSpace(form.Get("carddav_uuid"))
	if cardDAVUUID == "" {
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	// Check if this CardDAV UUID is already linked to another contact
	isLinked, err := db.IsCardDAVUUIDLinked(c.Request().Context(), cardDAVUUID)
	if err != nil {
		logger.Error("Error checking if CardDAV UUID is linked", "error", err)
		data["Error"] = "Failed to check CardDAV link status"
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if isLinked {
		logger.Warn("CardDAV UUID already linked to another contact", "carddav_uuid", cardDAVUUID)
		data["Error"] = "This CardDAV contact is already linked to another contact"
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	err = db.LinkCardDAV(c.Request().Context(), contactID, cardDAVUUID)
	if err != nil {
		logger.Error("Error linking CardDAV contact", "error", err)
		data["Error"] = "Failed to link CardDAV contact"
	} else {
		logger.Info("Successfully linked contact with CardDAV UUID", "contact_id", contactID, "carddav_uuid", cardDAVUUID)
	}

	c.Redirect("/contact/"+contactID+"/edit", http.StatusSeeOther)
}

// UnlinkCardDAV handles unlinking a contact from CardDAV
func UnlinkCardDAV(c flamego.Context) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.UnlinkCardDAV(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error unlinking CardDAV contact", "error", err)
	} else {
		logger.Info("Successfully unlinked contact from CardDAV", "contact_id", contactID)
	}

	c.Redirect("/contact/"+contactID+"/edit", http.StatusSeeOther)
}

// MigrateToCardDAV handles creating a new CardDAV contact from local data
func MigrateToCardDAV(c flamego.Context, s session.Session) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.MigrateContactToCardDAV(c.Request().Context(), contactID)
	if err != nil {
		logger.Error("Error migrating contact to CardDAV", "contact_id", contactID, "error", err)
		SetErrorFlash(s, "Failed to migrate to CardDAV: "+err.Error())
	} else {
		logger.Info("Successfully migrated contact to CardDAV", "contact_id", contactID)
		SetSuccessFlash(s, "Contact successfully migrated to CardDAV")
	}

	c.Redirect("/contact/"+contactID+"/edit", http.StatusSeeOther)
}

// CardDAVContactWithStatus represents a CardDAV contact with linked status
type CardDAVContactWithStatus struct {
	db.CardDAVContact
	IsLinked  bool `json:"IsLinked"`
	IsService bool `json:"IsService"`
}

// shouldHideCardDAVContact returns true if contact should be hidden from the list
func shouldHideCardDAVContact(contact db.CardDAVContact) bool {
	// Hide contacts with "/hide" in notes
	if strings.Contains(contact.Notes, "/hide") {
		return true
	}

	// Hide contacts with only company name (no first or last name)
	if contact.GivenName == "" && contact.FamilyName == "" && contact.Organization != "" {
		return true
	}

	return false
}

// ListCardDAVContacts returns a list of CardDAV contacts as JSON
func ListCardDAVContacts(c flamego.Context) {
	contacts, err := db.ListCardDAVContacts(c.Request().Context())
	if err != nil {
		logger.Error("Error listing CardDAV contacts", "error", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{
			"error": "Failed to fetch CardDAV contacts: " + err.Error(),
		}); err != nil {
			logger.Error("Error encoding CardDAV contacts error", "error", err)
		}
		return
	}

	// Filter out hidden contacts
	var filteredContacts []db.CardDAVContact
	for _, contact := range contacts {
		if !shouldHideCardDAVContact(contact) {
			filteredContacts = append(filteredContacts, contact)
		}
	}

	// Get all linked CardDAV UUIDs with service status
	linkedMap, err := db.GetLinkedCardDAVUUIDsWithServiceStatus(c.Request().Context())
	if err != nil {
		logger.Error("Error getting linked CardDAV UUIDs", "error", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{
			"error": "Failed to fetch linked CardDAV UUIDs: " + err.Error(),
		}); err != nil {
			logger.Error("Error encoding CardDAV links error", "error", err)
		}
		return
	}

	// Add IsLinked and IsService status to each contact
	contactsWithStatus := make([]CardDAVContactWithStatus, 0, len(filteredContacts))
	for _, contact := range filteredContacts {
		isService, isLinked := linkedMap[strings.ToLower(contact.UUID)]
		if !isLinked {
			isService = false // Not linked means not a service contact
		}
		contactsWithStatus = append(contactsWithStatus, CardDAVContactWithStatus{
			CardDAVContact: contact,
			IsLinked:       isLinked,
			IsService:      isService,
		})
	}

	c.ResponseWriter().Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(c.ResponseWriter()).Encode(contactsWithStatus); err != nil {
		logger.Error("Error encoding CardDAV contacts response", "error", err)
	}
}

// CardDAVPicker renders the CardDAV contact picker popup
func CardDAVPicker(c flamego.Context, t template.Template, data template.Data) {
	t.HTML(http.StatusOK, "carddav_picker")
}

// AddNote handles adding a new contact note
func AddNote(c flamego.Context) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	content := strings.TrimSpace(form.Get("content"))
	if content == "" {
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	input := db.AddNoteInput{
		ContactID: contactID,
		Content:   content,
		NotedAt:   getOptionalString(form.Get("noted_at")),
	}

	err := db.AddNote(c.Request().Context(), input)
	if err != nil {
		logger.Error("Error adding note", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// DeleteNote handles deleting a contact note
func DeleteNote(c flamego.Context) {
	contactID := c.Param("id")
	noteID := c.Param("note_id")

	if contactID == "" || noteID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeleteNote(c.Request().Context(), noteID)
	if err != nil {
		logger.Error("Error deleting note", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// AddTag handles adding a tag to a contact
func AddTag(c flamego.Context) {
	contactID := c.Param("id")
	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	tagName := strings.TrimSpace(c.Request().Form.Get("tag_name"))
	if tagName == "" {
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	err := db.AddTagToContact(c.Request().Context(), contactID, tagName)
	if err != nil {
		logger.Error("Error adding tag", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// RemoveTag handles removing a tag from a contact
func RemoveTag(c flamego.Context) {
	contactID := c.Param("id")
	tagID := c.Param("tag_id")

	if contactID == "" || tagID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.RemoveTagFromContact(c.Request().Context(), contactID, tagID)
	if err != nil {
		logger.Error("Error removing tag", "error", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// ListServiceContacts renders the service contacts list page
func ListServiceContacts(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Get tag filter from URL query
	tagIDs := c.QueryStrings("tag")

	var contacts []db.ContactListItem
	var err error

	// Use ListContactsWithFilters if tags are specified
	if len(tagIDs) > 0 {
		opts := db.ContactListOptions{
			TagIDs:    tagIDs,
			IsService: true,
		}
		contacts, err = db.ListContactsWithFilters(ctx, opts)
	} else {
		contacts, err = db.ListServiceContacts(ctx)
	}

	if err != nil {
		logger.Error("Error fetching service contacts", "error", err)
		data["Error"] = "Failed to load service contacts"
	} else {
		data["ServiceContacts"] = contacts
	}

	// Fetch all tags for the filter UI
	allTags, err := db.ListAllTags(ctx)
	if err != nil {
		logger.Error("Error fetching tags", "error", err)
	} else {
		data["AllTags"] = allTags
		data["SelectedTags"] = tagIDs
	}

	data["IsServiceContacts"] = true
	data["IsContacts"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Contacts", URL: "/contacts", IsCurrent: false},
		{Name: "Service Contacts", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "service_contacts")
}

// BulkContactLogForm renders the bulk contact log form
func BulkContactLogForm(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch all contacts for the multi-select
	contacts, err := db.ListContacts(ctx)
	if err != nil {
		logger.Error("Error fetching contacts", "error", err)
		data["Error"] = "Failed to load contacts"
	} else {
		data["Contacts"] = contacts
	}

	data["IsContacts"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Contacts", URL: "/contacts", IsCurrent: false},
		{Name: "Bulk Contact Log", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "bulk_contact_log")
}

// BulkAddLog handles adding a contact log to multiple contacts
func BulkAddLog(c flamego.Context, s session.Session) {
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/bulk-contact-log", http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	// Get contact IDs array
	contactIDs := form["contact_ids[]"]
	if len(contactIDs) == 0 {
		SetErrorFlash(s, "Please select at least one contact")
		c.Redirect("/bulk-contact-log", http.StatusSeeOther)
		return
	}

	// Get form fields
	logType := db.LogType(form.Get("log_type"))
	if logType == "" {
		logType = db.LogGeneral
	}

	loggedAt := getOptionalString(form.Get("logged_at"))
	content := getOptionalString(form.Get("content"))

	// Track successes and failures
	successCount := 0
	var failedContacts []string

	// Loop through contacts and add log to each
	for _, contactID := range contactIDs {
		input := db.AddLogInput{
			ContactID: contactID,
			LogType:   logType,
			LoggedAt:  loggedAt,
			Subject:   nil, // Always nil - no subject field in bulk log
			Content:   content,
		}

		err := db.AddLog(ctx, input)
		if err != nil {
			logger.Error("Error adding log to contact", "contact_id", contactID, "error", err)
			failedContacts = append(failedContacts, contactID)
		} else {
			successCount++
		}
	}

	// Flash message based on results
	if len(failedContacts) == 0 {
		SetSuccessFlash(s, fmt.Sprintf("Successfully added log to %d contact(s)", successCount))
	} else if successCount == 0 {
		SetErrorFlash(s, "Failed to add logs to all contacts")
	} else {
		SetWarningFlash(s, fmt.Sprintf("Added log to %d contact(s), failed for %d contact(s)", successCount, len(failedContacts)))
	}

	c.Redirect("/contacts", http.StatusSeeOther)
}
