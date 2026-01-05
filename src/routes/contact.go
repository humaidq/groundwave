/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/flamego/flamego"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

// NewContactForm renders the add contact form
func NewContactForm(c flamego.Context, t template.Template, data template.Data) {
	data["IsNewContact"] = true
	t.HTML(http.StatusOK, "contact_new")
}

// CreateContact handles the contact creation form submission
func CreateContact(c flamego.Context, t template.Template, data template.Data) {
	// Parse form data
	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		data["Error"] = "Failed to parse form data"
		t.HTML(http.StatusBadRequest, "contact_new")
		return
	}

	form := c.Request().Form

	// Helper to get optional string
	getOptionalString := func(key string) *string {
		val := strings.TrimSpace(form.Get(key))
		if val == "" {
			return nil
		}
		return &val
	}

	// Check if this is a CardDAV import
	cardDAVUUID := getOptionalString("carddav_uuid")
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
		nameFamily = getOptionalString("name_family")
		organization = getOptionalString("organization")
		title = getOptionalString("title")
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
		Email:        getOptionalString("email"),
		Phone:        getOptionalString("phone"),
		CallSign:     getOptionalString("call_sign"),
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

	// Redirect to contact view page on success
	log.Printf("Successfully created contact: %s", contactID)
	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// ViewContact displays a contact's details
func ViewContact(c flamego.Context, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		data["Error"] = "Contact ID is required"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		log.Printf("Error fetching contact %s: %v", contactID, err)
		data["Error"] = "Contact not found"
		t.HTML(http.StatusNotFound, "error")
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

	data["Contact"] = contact
	data["ContactName"] = contact.NameDisplay
	data["TierLower"] = strings.ToLower(string(contact.Tier))
	data["CardDAVContact"] = contact.CardDAVContact

	// If contact has a call sign, fetch QSOs
	if contact.CallSign != nil && *contact.CallSign != "" {
		qsos, err := db.GetQSOsByCallSign(c.Request().Context(), *contact.CallSign)
		if err != nil {
			log.Printf("Error fetching QSOs for call sign %s: %v", *contact.CallSign, err)
		} else {
			data["QSOs"] = qsos
		}
	}

	t.HTML(http.StatusOK, "contact_view")
}

// EditContactForm displays the edit contact form
func EditContactForm(c flamego.Context, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		data["Error"] = "Contact ID is required"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	contact, err := db.GetContact(c.Request().Context(), contactID)
	if err != nil {
		log.Printf("Error fetching contact %s: %v", contactID, err)
		data["Error"] = "Contact not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	data["Contact"] = contact
	data["ContactName"] = contact.NameDisplay
	t.HTML(http.StatusOK, "contact_edit")
}

// UpdateContact handles the contact update form submission
func UpdateContact(c flamego.Context, t template.Template, data template.Data) {
	contactID := c.Param("id")
	if contactID == "" {
		data["Error"] = "Contact ID is required"
		t.HTML(http.StatusBadRequest, "error")
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

	// Helper to get optional string
	getOptionalString := func(key string) *string {
		val := strings.TrimSpace(form.Get(key))
		if val == "" {
			return nil
		}
		return &val
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
		NameFamily:   getOptionalString("name_family"),
		Organization: getOptionalString("organization"),
		Title:        getOptionalString("title"),
		CallSign:     getOptionalString("call_sign"),
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

	// Redirect to contact view page on success
	log.Printf("Successfully updated contact: %s", contactID)
	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// AddEmail handles adding a new email to a contact
func AddEmail(c flamego.Context) {
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
	email := strings.TrimSpace(form.Get("email"))
	if email == "" {
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	emailType := db.EmailType(form.Get("email_type"))
	if emailType == "" {
		emailType = db.EmailPersonal
	}

	isPrimary := form.Get("is_primary") == "on"

	input := db.AddEmailInput{
		ContactID: contactID,
		Email:     email,
		EmailType: emailType,
		IsPrimary: isPrimary,
	}

	err := db.AddEmail(c.Request().Context(), input)
	if err != nil {
		log.Printf("Error adding email: %v", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// AddPhone handles adding a new phone to a contact
func AddPhone(c flamego.Context) {
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
	phone := strings.TrimSpace(form.Get("phone"))
	if phone == "" {
		c.Redirect("/contact/"+contactID, http.StatusSeeOther)
		return
	}

	phoneType := db.PhoneType(form.Get("phone_type"))
	if phoneType == "" {
		phoneType = db.PhoneCell
	}

	isPrimary := form.Get("is_primary") == "on"

	input := db.AddPhoneInput{
		ContactID: contactID,
		Phone:     phone,
		PhoneType: phoneType,
		IsPrimary: isPrimary,
	}

	err := db.AddPhone(c.Request().Context(), input)
	if err != nil {
		log.Printf("Error adding phone: %v", err)
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

	getOptionalString := func(key string) *string {
		val := strings.TrimSpace(form.Get(key))
		if val == "" {
			return nil
		}
		return &val
	}

	input := db.AddURLInput{
		ContactID: contactID,
		URL:       url,
		URLType:   urlType,
		Label:     getOptionalString("label"),
		Username:  getOptionalString("username"),
	}

	err := db.AddURL(c.Request().Context(), input)
	if err != nil {
		log.Printf("Error adding URL: %v", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// DeleteEmail handles deleting an email
func DeleteEmail(c flamego.Context) {
	contactID := c.Param("id")
	emailID := c.Param("email_id")

	if contactID == "" || emailID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeleteEmail(c.Request().Context(), emailID)
	if err != nil {
		log.Printf("Error deleting email: %v", err)
	}

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
}

// DeletePhone handles deleting a phone
func DeletePhone(c flamego.Context) {
	contactID := c.Param("id")
	phoneID := c.Param("phone_id")

	if contactID == "" || phoneID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeletePhone(c.Request().Context(), phoneID)
	if err != nil {
		log.Printf("Error deleting phone: %v", err)
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
func DeleteContact(c flamego.Context) {
	contactID := c.Param("id")

	if contactID == "" {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	err := db.DeleteContact(c.Request().Context(), contactID)
	if err != nil {
		log.Printf("Error deleting contact: %v", err)
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

	// Get optional string helper
	getOptionalString := func(key string) *string {
		val := strings.TrimSpace(form.Get(key))
		if val == "" {
			return nil
		}
		return &val
	}

	logType := db.LogType(form.Get("log_type"))
	if logType == "" {
		logType = db.LogGeneral
	}

	input := db.AddLogInput{
		ContactID: contactID,
		LogType:   logType,
		LoggedAt:  getOptionalString("logged_at"),
		Subject:   getOptionalString("subject"),
		Content:   getOptionalString("content"),
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

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
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

	c.Redirect("/contact/"+contactID, http.StatusSeeOther)
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

	// Get optional string helper
	getOptionalString := func(key string) *string {
		val := strings.TrimSpace(form.Get(key))
		if val == "" {
			return nil
		}
		return &val
	}

	input := db.AddNoteInput{
		ContactID: contactID,
		Content:   content,
		NotedAt:   getOptionalString("noted_at"),
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
	contacts, err := db.ListServiceContacts(c.Request().Context())
	if err != nil {
		log.Printf("Error fetching service contacts: %v", err)
		data["Error"] = "Failed to load service contacts"
	} else {
		data["ServiceContacts"] = contacts
	}

	data["IsServiceContacts"] = true
	t.HTML(http.StatusOK, "service_contacts")
}
