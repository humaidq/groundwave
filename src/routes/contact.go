/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"encoding/json"
	"fmt"
	"log"
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
		log.Printf("Error fetching tags: %v", err)
	} else {
		data["AllTags"] = allTags
	}

	t.HTML(http.StatusOK, "contact_new")
}

// CreateContact handles the contact creation form submission
func CreateContact(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	// Parse form data
	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
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
			log.Printf("Error fetching CardDAV contact: %v", err)
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
		log.Printf("Error creating contact: %v", err)
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
			log.Printf("Error adding tag to contact: %v", err)
			// Don't fail the whole operation, just log the error
		}
	}

	// Redirect to contact view page on success
	log.Printf("Successfully created contact: %s", contactID)
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
		log.Printf("Error fetching contact %s: %v", contactID, err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	// If contact is linked to CardDAV, sync details before displaying
	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		err := db.SyncContactFromCardDAV(c.Request().Context(), contactID, *contact.CardDAVUUID)
		if err != nil {
			log.Printf("Error syncing contact %s from CardDAV: %v", contactID, err)
			// Continue anyway, just log the error
		} else {
			// Reload contact after sync
			contact, err = db.GetContact(c.Request().Context(), contactID)
			if err != nil {
				log.Printf("Error reloading contact %s after sync: %v", contactID, err)
			}
		}
	}

	// Check private mode from session
	privateMode := false
	if pm := s.Get("private_mode"); pm != nil {
		privateMode = pm.(bool)
	}
	data["PrivateMode"] = privateMode

	data["Contact"] = contact
	data["ContactName"] = contact.NameDisplay
	data["IsContacts"] = true
	data["TierLower"] = strings.ToLower(string(contact.Tier))
	data["CardDAVContact"] = contact.CardDAVContact
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
			log.Printf("Error fetching QSOs for call sign %s: %v", *contact.CallSign, err)
		} else {
			data["QSOs"] = qsos
		}
	}

	// Fetch all tags for autocomplete
	allTags, err := db.ListAllTags(c.Request().Context())
	if err != nil {
		log.Printf("Error fetching tags: %v", err)
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
		log.Printf("Error fetching contact %s: %v", contactID, err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contacts", http.StatusSeeOther)
		return
	}

	if contact.IsService {
		SetErrorFlash(s, "Chats are not available for service contacts")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	privateMode := false
	if pm := s.Get("private_mode"); pm != nil {
		privateMode = pm.(bool)
	}
	if privateMode {
		SetErrorFlash(s, "Private mode is enabled")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	chats, err := db.GetContactChats(c.Request().Context(), contactID)
	if err != nil {
		log.Printf("Error fetching chats for contact %s: %v", contactID, err)
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
		log.Printf("Error checking service contact %s: %v", contactID, err)
		c.Redirect("/contact/"+contactID+"/chats", http.StatusSeeOther)
		return
	}
	if isService {
		SetErrorFlash(s, "Chats are not available for service contacts")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if pm := s.Get("private_mode"); pm != nil {
		if pm.(bool) {
			SetErrorFlash(s, "Private mode is enabled")
			c.Redirect("/contact/"+contactID, http.StatusSeeOther)
			return
		}
	}

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
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
		log.Printf("Error adding chat entry: %v", err)
	}

	c.Redirect("/contact/"+contactID+"/chats", http.StatusSeeOther)
}

// GenerateContactChatSummary summarizes recent chat history for a contact.
func GenerateContactChatSummary(c flamego.Context, s session.Session) {
	c.ResponseWriter().Header().Set("Content-Type", "application/json")

	contactID := c.Param("id")
	if contactID == "" {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "contact ID is required"})
		return
	}

	ctx := c.Request().Context()

	contact, err := db.GetContact(ctx, contactID)
	if err != nil {
		log.Printf("Error fetching contact %s: %v", contactID, err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "contact not found"})
		return
	}

	if contact.IsService {
		c.ResponseWriter().WriteHeader(http.StatusBadRequest)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "service contacts have no chat summary"})
		return
	}

	if pm := s.Get("private_mode"); pm != nil {
		if pm.(bool) {
			c.ResponseWriter().WriteHeader(http.StatusBadRequest)
			json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "private mode is enabled"})
			return
		}
	}

	since := time.Now().Add(-48 * time.Hour)
	chats, err := db.GetContactChatsSince(ctx, contactID, since)
	if err != nil {
		log.Printf("Error fetching chats for contact %s: %v", contactID, err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to load chat history"})
		return
	}

	if len(chats) == 0 {
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]interface{}{"summary": "No recent chats", "empty": true})
		return
	}

	var summaryBuilder strings.Builder
	if err := db.StreamContactChatSummary(ctx, &contact.Contact, chats, func(chunk string) error {
		summaryBuilder.WriteString(chunk)
		return nil
	}); err != nil {
		log.Printf("Error generating chat summary: %v", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{"error": "failed to generate summary"})
		return
	}

	summary := strings.TrimSpace(summaryBuilder.String())
	isEmpty := false
	if summary == "" {
		summary = "No recent chats"
		isEmpty = true
	}

	json.NewEncoder(c.ResponseWriter()).Encode(map[string]interface{}{"summary": summary, "empty": isEmpty})
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
		log.Printf("Error fetching contact %s: %v", contactID, err)
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
		log.Printf("Error parsing form: %v", err)
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
			log.Printf("Error toggling service status: %v", err)
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
		log.Printf("Error updating contact: %v", err)
		data["Error"] = "Failed to update contact: " + err.Error()
		c.Redirect("/contact/"+contactID+"/edit", http.StatusSeeOther)
		return
	}

	// If contact is linked to CardDAV, push the update
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			log.Printf("Error pushing contact update to CardDAV: %v", err)
			// Don't fail the whole operation, just log the error
		}
	}

	// Redirect to contact view page on success
	log.Printf("Successfully updated contact: %s", contactID)
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
		log.Printf("Error parsing form: %v", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	email := strings.TrimSpace(form.Get("email"))
	if email == "" {
		SetErrorFlash(s, "Email address is required")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	emailType := db.EmailType(form.Get("email_type"))
	if emailType == "" {
		emailType = db.EmailPersonal
	}

	isPrimary := form.Get("is_primary") == "on"

	// Check if contact is linked to CardDAV first
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		log.Printf("Error getting contact: %v", err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		// For CardDAV-linked contacts, add to contact in memory and push to CardDAV
		// The sync will bring it back with source='carddav'
		newEmail := db.ContactEmail{
			Email:     email,
			EmailType: emailType,
			IsPrimary: isPrimary,
			Source:    "carddav",
		}
		contact.Emails = append(contact.Emails, newEmail)

		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			log.Printf("Error pushing new email to CardDAV: %v", err)
			SetErrorFlash(s, "Failed to sync email to CardDAV")
		} else {
			// Sync the contact back to get the email with proper ID
			if err := db.SyncContactFromCardDAV(c.Request().Context(), contact.ID.String(), *contact.CardDAVUUID); err != nil {
				log.Printf("Error syncing contact after adding email: %v", err)
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
			log.Printf("Error adding email: %v", err)
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
		log.Printf("Error parsing form: %v", err)
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

	isPrimary := form.Get("is_primary") == "on"

	// Check if contact is linked to CardDAV first
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		log.Printf("Error getting contact: %v", err)
		SetErrorFlash(s, "Contact not found")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		// For CardDAV-linked contacts, add to contact in memory and push to CardDAV
		// The sync will bring it back with source='carddav'
		newPhone := db.ContactPhone{
			Phone:     phone,
			PhoneType: phoneType,
			IsPrimary: isPrimary,
			Source:    "carddav",
		}
		contact.Phones = append(contact.Phones, newPhone)

		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			log.Printf("Error pushing new phone to CardDAV: %v", err)
			SetErrorFlash(s, "Failed to sync phone to CardDAV")
		} else {
			// Sync the contact back to get the phone with proper ID
			if err := db.SyncContactFromCardDAV(c.Request().Context(), contact.ID.String(), *contact.CardDAVUUID); err != nil {
				log.Printf("Error syncing contact after adding phone: %v", err)
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
			log.Printf("Error adding phone: %v", err)
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
		log.Printf("Error parsing form: %v", err)
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
		ContactID: contactID,
		URL:       url,
		URLType:   urlType,
		Label:     getOptionalString(form.Get("label")),
		Username:  getOptionalString(form.Get("username")),
	}

	err := db.AddURL(c.Request().Context(), input)
	if err != nil {
		log.Printf("Error adding URL: %v", err)
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
		log.Printf("Error deleting email: %v", err)
		SetErrorFlash(s, "Failed to delete email")
	}

	// If contact is linked to CardDAV, push the deletion
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			log.Printf("Error pushing email deletion to CardDAV: %v", err)
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
		log.Printf("Error deleting phone: %v", err)
		SetErrorFlash(s, "Failed to delete phone")
	}

	// If contact is linked to CardDAV, push the deletion
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			log.Printf("Error pushing phone deletion to CardDAV: %v", err)
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
		log.Printf("Error parsing form: %v", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	email := strings.TrimSpace(form.Get("email"))
	emailType := db.EmailType(form.Get("email_type"))
	isPrimary := form.Get("is_primary") == "on"

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
		log.Printf("Error updating email: %v", err)
		SetErrorFlash(s, "Failed to update email")
	}

	// If contact is linked to CardDAV, push the update
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			log.Printf("Error pushing email update to CardDAV: %v", err)
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
		log.Printf("Error parsing form: %v", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form
	phone := strings.TrimSpace(form.Get("phone"))
	phoneType := db.PhoneType(form.Get("phone_type"))
	isPrimary := form.Get("is_primary") == "on"

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
		log.Printf("Error updating phone: %v", err)
		SetErrorFlash(s, "Failed to update phone")
	}

	// If contact is linked to CardDAV, push the update
	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err == nil && contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		if err := db.UpdateCardDAVContact(c.Request().Context(), contact); err != nil {
			log.Printf("Error pushing phone update to CardDAV: %v", err)
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
		log.Printf("Error deleting URL: %v", err)
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
		log.Printf("Error deleting contact: %v", err)
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
		log.Printf("Error parsing form: %v", err)
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
		log.Printf("Error adding log: %v", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
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
		log.Printf("Error deleting log: %v", err)
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
		log.Printf("Error parsing form: %v", err)
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
		log.Printf("Error checking if CardDAV UUID is linked: %v", err)
		data["Error"] = "Failed to check CardDAV link status"
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	if isLinked {
		log.Printf("Cannot link CardDAV UUID %s: already linked to another contact", cardDAVUUID)
		data["Error"] = "This CardDAV contact is already linked to another contact"
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	err = db.LinkCardDAV(c.Request().Context(), contactID, cardDAVUUID)
	if err != nil {
		log.Printf("Error linking CardDAV contact: %v", err)
		data["Error"] = "Failed to link CardDAV contact"
	} else {
		log.Printf("Successfully linked contact %s with CardDAV UUID %s", contactID, cardDAVUUID)
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
		log.Printf("Error unlinking CardDAV contact: %v", err)
	} else {
		log.Printf("Successfully unlinked contact %s from CardDAV", contactID)
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
		log.Printf("Error migrating contact %s to CardDAV: %v", contactID, err)
		SetErrorFlash(s, "Failed to migrate to CardDAV: "+err.Error())
	} else {
		log.Printf("Successfully migrated contact %s to CardDAV", contactID)
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
		log.Printf("Error listing CardDAV contacts: %v", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{
			"error": "Failed to fetch CardDAV contacts: " + err.Error(),
		})
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
		log.Printf("Error getting linked CardDAV UUIDs: %v", err)
		c.ResponseWriter().WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(c.ResponseWriter()).Encode(map[string]string{
			"error": "Failed to fetch linked CardDAV UUIDs: " + err.Error(),
		})
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
	json.NewEncoder(c.ResponseWriter()).Encode(contactsWithStatus)
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
		log.Printf("Error parsing form: %v", err)
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
		log.Printf("Error adding note: %v", err)
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
		log.Printf("Error deleting note: %v", err)
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
		log.Printf("Error parsing form: %v", err)
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
		log.Printf("Error adding tag: %v", err)
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
		log.Printf("Error removing tag: %v", err)
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
		log.Printf("Error fetching service contacts: %v", err)
		data["Error"] = "Failed to load service contacts"
	} else {
		data["ServiceContacts"] = contacts
	}

	// Fetch all tags for the filter UI
	allTags, err := db.ListAllTags(ctx)
	if err != nil {
		log.Printf("Error fetching tags: %v", err)
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

// TogglePrivateMode toggles the private mode session flag
func TogglePrivateMode(c flamego.Context, s session.Session) {
	// Get current state, default to false
	privateMode := false
	if pm := s.Get("private_mode"); pm != nil {
		privateMode = pm.(bool)
	}

	// Toggle the state
	s.Set("private_mode", !privateMode)

	// Redirect back to the referring page, default to /contacts
	referer := c.Request().Header.Get("Referer")
	if referer == "" {
		referer = "/contacts"
	}
	c.Redirect(referer, http.StatusSeeOther)
}

// BulkContactLogForm renders the bulk contact log form
func BulkContactLogForm(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch all contacts for the multi-select
	contacts, err := db.ListContacts(ctx)
	if err != nil {
		log.Printf("Error fetching contacts: %v", err)
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
		log.Printf("Error parsing form: %v", err)
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
			log.Printf("Error adding log to contact %s: %v", contactID, err)
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
