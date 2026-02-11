/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CardDAVConfig holds the CardDAV server configuration
type CardDAVConfig struct {
	URL      string
	Username string
	Password string
}

// CardDAVContact represents a contact fetched from CardDAV
type CardDAVContact struct {
	UUID           string
	DisplayName    string
	GivenName      string
	FamilyName     string
	AdditionalName string
	Nickname       string
	Organization   string
	Title          string
	Birthday       *time.Time
	Anniversary    *time.Time
	Gender         string
	Emails         []CardDAVEmail
	Phones         []CardDAVPhone
	Addresses      []CardDAVAddress
	URLs           []string
	Notes          string
	PhotoURL       string
}

// CardDAVEmail represents an email from CardDAV
type CardDAVEmail struct {
	Email     string
	Type      string
	Preferred bool
}

// CardDAVPhone represents a phone number from CardDAV
type CardDAVPhone struct {
	Phone     string
	Type      string
	Preferred bool
}

// CardDAVAddress represents an address from CardDAV
type CardDAVAddress struct {
	Street     string
	Locality   string
	Region     string
	PostalCode string
	Country    string
	Type       string
}

// GetCardDAVConfig loads CardDAV configuration from environment variables
func GetCardDAVConfig() (*CardDAVConfig, error) {
	url := os.Getenv("CARDDAV_URL")
	username := os.Getenv("CARDDAV_USERNAME")
	password := os.Getenv("CARDDAV_PASSWORD")

	if url == "" || username == "" || password == "" {
		return nil, ErrCardDAVConfigIncomplete
	}

	return &CardDAVConfig{
		URL:      url,
		Username: username,
		Password: password,
	}, nil
}

// newCardDAVClient creates a new CardDAV client
func newCardDAVClient(config *CardDAVConfig) (*carddav.Client, error) {
	// Create HTTP client with Basic Auth
	httpClient := &http.Client{
		Timeout: 3 * time.Second, // Fast timeout for local/same-network CardDAV
		Transport: &basicAuthTransport{
			Username: config.Username,
			Password: config.Password,
			Base:     http.DefaultTransport,
		},
	}

	client, err := carddav.NewClient(httpClient, config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to create CardDAV client: %w", err)
	}

	return client, nil
}

// basicAuthTransport adds HTTP Basic Authentication to all requests
type basicAuthTransport struct {
	Username string
	Password string
	Base     http.RoundTripper
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.Username, t.Password)

	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("carddav round trip failed: %w", err)
	}

	return resp, nil
}

// ListCardDAVContacts fetches all contacts from the CardDAV server
func ListCardDAVContacts(ctx context.Context) ([]CardDAVContact, error) {
	config, err := GetCardDAVConfig()
	if err != nil {
		return nil, err
	}

	// Create HTTP client with Basic Auth
	httpClient := &http.Client{
		Timeout: 3 * time.Second, // Fast timeout for local/same-network CardDAV
		Transport: &basicAuthTransport{
			Username: config.Username,
			Password: config.Password,
			Base:     http.DefaultTransport,
		},
	}

	// Fetch the VCF file via HTTP GET
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VCF file: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close CardDAV response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrFetchVCFFileFailed, resp.StatusCode)
	}

	// Parse the VCF file
	decoder := vcard.NewDecoder(resp.Body)

	var contacts []CardDAVContact

	for {
		card, err := decoder.Decode()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to parse VCF: %w", err)
		}

		contact := parseVCard(card)
		contacts = append(contacts, contact)
	}

	return contacts, nil
}

// GetCardDAVContact fetches a specific contact from the CardDAV server by UUID
func GetCardDAVContact(ctx context.Context, uuid string) (*CardDAVContact, error) {
	// Fetch all contacts and find the one with matching UUID
	contacts, err := ListCardDAVContacts(ctx)
	if err != nil {
		return nil, err
	}

	// Try case-insensitive comparison
	uuidLower := strings.ToLower(uuid)
	for _, contact := range contacts {
		if strings.ToLower(contact.UUID) == uuidLower {
			return &contact, nil
		}
	}

	// Debug: print all available UUIDs if not found
	fmt.Printf("CardDAV contact not found. Looking for: %s\n", uuid)
	fmt.Printf("Available UUIDs:\n")

	for _, contact := range contacts {
		fmt.Printf("  - %s (%s)\n", contact.UUID, contact.DisplayName)
	}

	return nil, fmt.Errorf("%w: %s (checked %d contacts)", ErrCardDAVContactByUUIDNotFound, uuid, len(contacts))
}

// parseVCard converts a vCard to our CardDAVContact struct
func parseVCard(card vcard.Card) CardDAVContact {
	contact := CardDAVContact{
		UUID: card.Value(vcard.FieldUID),
	}

	// Get name fields
	if name := card.Name(); name != nil {
		contact.FamilyName = name.FamilyName
		contact.GivenName = name.GivenName
		contact.AdditionalName = name.AdditionalName
	}

	// Get formatted name
	if fns := card.FormattedNames(); len(fns) > 0 {
		contact.DisplayName = fns[0].Value
	}

	// Get nickname
	contact.Nickname = card.Value(vcard.FieldNickname)

	// Get organization
	contact.Organization = card.Value(vcard.FieldOrganization)

	// Get title
	contact.Title = card.Value(vcard.FieldTitle)

	// Get birthday
	if bdayStr := card.Value(vcard.FieldBirthday); bdayStr != "" {
		if t, err := parseDateString(bdayStr); err == nil {
			contact.Birthday = &t
		}
	}

	// Get anniversary
	if annStr := card.Value(vcard.FieldAnniversary); annStr != "" {
		if t, err := parseDateString(annStr); err == nil {
			contact.Anniversary = &t
		}
	}

	// Get gender
	if sex, _ := card.Gender(); sex != "" {
		contact.Gender = string(sex)
	}

	// Get emails
	preferredEmail := card.Preferred(vcard.FieldEmail)
	if emailFields, ok := card[vcard.FieldEmail]; ok {
		for _, field := range emailFields {
			emailType := "other"
			if field.Params.HasType("work") {
				emailType = "work"
			} else if field.Params.HasType("home") {
				emailType = "home"
			}

			contact.Emails = append(contact.Emails, CardDAVEmail{
				Email:     field.Value,
				Type:      emailType,
				Preferred: field == preferredEmail,
			})
		}
	}

	// Get phone numbers
	preferredPhone := card.Preferred(vcard.FieldTelephone)
	if phoneFields, ok := card[vcard.FieldTelephone]; ok {
		for _, field := range phoneFields {
			phoneType := "other"
			if field.Params.HasType("cell") {
				phoneType = "cell"
			} else if field.Params.HasType("work") {
				phoneType = "work"
			} else if field.Params.HasType("home") {
				phoneType = "home"
			} else if field.Params.HasType("fax") {
				phoneType = "fax"
			}

			contact.Phones = append(contact.Phones, CardDAVPhone{
				Phone:     field.Value,
				Type:      phoneType,
				Preferred: field == preferredPhone,
			})
		}
	}

	// Get addresses
	for _, addr := range card.Addresses() {
		addrType := "other"
		if addr.Params.HasType("work") {
			addrType = "work"
		} else if addr.Params.HasType("home") {
			addrType = "home"
		}

		contact.Addresses = append(contact.Addresses, CardDAVAddress{
			Street:     addr.StreetAddress,
			Locality:   addr.Locality,
			Region:     addr.Region,
			PostalCode: addr.PostalCode,
			Country:    addr.Country,
			Type:       addrType,
		})
	}

	// Get URLs
	contact.URLs = append(contact.URLs, card.Values(vcard.FieldURL)...)

	// Get notes
	contact.Notes = card.Value(vcard.FieldNote)

	// Get photo URL
	if photoField := card.Get(vcard.FieldPhoto); photoField != nil {
		contact.PhotoURL = normalizeCardDAVPhoto(photoField)
	}

	return contact
}

func normalizeCardDAVPhoto(photoField *vcard.Field) string {
	if photoField == nil {
		return ""
	}

	value := strings.TrimSpace(photoField.Value)
	if value == "" {
		return ""
	}

	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return value
	}

	mediaType := strings.TrimSpace(photoField.Params.Get(vcard.ParamMediaType))
	if mediaType == "" {
		mediaType = mediaTypeFromPhotoParams(photoField.Params)
	}

	if mediaType == "" {
		mediaType = "image/jpeg"
	}

	value = strings.Join(strings.Fields(value), "")

	return fmt.Sprintf("data:%s;base64,%s", mediaType, value)
}

func mediaTypeFromPhotoParams(params vcard.Params) string {
	for _, paramType := range params.Types() {
		paramType = strings.ToLower(strings.TrimSpace(paramType))
		if paramType == "" {
			continue
		}

		if strings.Contains(paramType, "/") {
			return paramType
		}

		switch paramType {
		case "png":
			return "image/png"
		case "jpg", "jpeg":
			return "image/jpeg"
		case "gif":
			return "image/gif"
		case "bmp":
			return "image/bmp"
		case "webp":
			return "image/webp"
		}
	}

	return ""
}

func preferredEmailValue(emails []CardDAVEmail) string {
	for _, email := range emails {
		if email.Preferred {
			return email.Email
		}
	}

	return ""
}

func preferredPhoneValue(phones []CardDAVPhone) string {
	for _, phone := range phones {
		if phone.Preferred {
			return phone.Phone
		}
	}

	return ""
}

func normalizeCardDAVEmail(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "mailto:") {
		value = value[len("mailto:"):]
	}

	value = strings.TrimSpace(value)

	return strings.ToLower(value)
}

func normalizeCardDAVPhone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "tel:") {
		value = value[len("tel:"):]
	}

	return strings.TrimSpace(value)
}

func normalizePhoneDigits(value string) string {
	if value == "" {
		return ""
	}

	var b strings.Builder

	for _, r := range value {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}

	return b.String()
}

func selectPrimaryEmail(preferredEmail string, insertedEmails []string) string {
	if preferredEmail != "" {
		for _, email := range insertedEmails {
			if strings.EqualFold(email, preferredEmail) {
				return preferredEmail
			}
		}
	}

	if len(insertedEmails) > 0 {
		return insertedEmails[0]
	}

	return ""
}

func selectPrimaryPhone(preferredPhone string, insertedPhones []string) string {
	if preferredPhone != "" {
		for _, phone := range insertedPhones {
			if phone == preferredPhone {
				return preferredPhone
			}
		}
	}

	if len(insertedPhones) > 0 {
		return insertedPhones[0]
	}

	return ""
}

func selectPrimaryPhoneByDigits(preferredPhoneDigits string, insertedPhones []string) string {
	if preferredPhoneDigits != "" {
		for _, phone := range insertedPhones {
			if normalizePhoneDigits(phone) == preferredPhoneDigits {
				return phone
			}
		}
	}

	if len(insertedPhones) > 0 {
		return insertedPhones[0]
	}

	return ""
}

func isCardDAVEmailAvailable(ctx context.Context, contactID, email string) (bool, error) {
	var existingContactID string

	err := pool.QueryRow(ctx, `SELECT contact_id FROM contact_emails WHERE lower(email) = lower($1) LIMIT 1`, email).Scan(&existingContactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}

		return false, fmt.Errorf("failed to query existing contact email: %w", err)
	}

	return existingContactID == contactID, nil
}

// parseDateString parses various date formats from vCard
func parseDateString(s string) (time.Time, error) {
	// Try common formats
	formats := []string{
		"20060102",   // YYYYMMDD
		"2006-01-02", // YYYY-MM-DD
		time.RFC3339, // Full date-time
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("%w: %s", ErrUnableToParseDate, s)
}

// SyncContactFromCardDAV updates a contact's details from CardDAV
// It syncs: name_given, name_family, organization, title, emails, phones
func SyncContactFromCardDAV(ctx context.Context, contactID string, cardDAVUUID string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	// Fetch the CardDAV contact
	cardDAVContact, err := GetCardDAVContact(ctx, cardDAVUUID)
	if err != nil {
		return fmt.Errorf("failed to fetch CardDAV contact: %w", err)
	}

	// Build display name
	nameDisplay := cardDAVContact.GivenName
	if cardDAVContact.FamilyName != "" {
		if nameDisplay != "" {
			nameDisplay = nameDisplay + " " + cardDAVContact.FamilyName
		} else {
			nameDisplay = cardDAVContact.FamilyName
		}
	}

	if nameDisplay == "" {
		nameDisplay = cardDAVContact.DisplayName
	}

	// Use GivenName, falling back to DisplayName if empty (consistent with CreateContact)
	nameGiven := cardDAVContact.GivenName
	if nameGiven == "" {
		nameGiven = cardDAVContact.DisplayName
	}

	// Prepare optional fields
	var nameFamilyPtr *string
	if cardDAVContact.FamilyName != "" {
		nameFamilyPtr = &cardDAVContact.FamilyName
	}

	var organizationPtr *string
	if cardDAVContact.Organization != "" {
		organizationPtr = &cardDAVContact.Organization
	}

	var titlePtr *string
	if cardDAVContact.Title != "" {
		titlePtr = &cardDAVContact.Title
	}

	var photoURLPtr *string
	if cardDAVContact.PhotoURL != "" {
		photoURLPtr = &cardDAVContact.PhotoURL
	}

	// Update the contact in the database
	query := `
		UPDATE contacts SET
			name_display = $1,
			name_given = $2,
			name_family = $3,
			organization = $4,
			title = $5,
			photo_url = $6,
			updated_at = now()
		WHERE id = $7
	`

	_, err = pool.Exec(ctx, query,
		nameDisplay,
		nameGiven,
		nameFamilyPtr,
		organizationPtr,
		titlePtr,
		photoURLPtr,
		contactID,
	)
	if err != nil {
		return fmt.Errorf("failed to update contact from CardDAV: %w", err)
	}

	// Sync emails: delete existing cached emails, then insert new ones
	_, err = pool.Exec(ctx, `DELETE FROM contact_emails WHERE contact_id = $1`, contactID)
	if err != nil {
		return fmt.Errorf("failed to delete existing contact emails: %w", err)
	}

	preferredEmail := normalizeCardDAVEmail(preferredEmailValue(cardDAVContact.Emails))

	insertedEmails := make([]string, 0, len(cardDAVContact.Emails))

	for _, email := range cardDAVContact.Emails {
		normalizedEmail := normalizeCardDAVEmail(email.Email)
		if normalizedEmail == "" {
			continue
		}
		// Map CardDAV email type to database enum
		emailType := "other"

		switch email.Type {
		case "work":
			emailType = "work"
		case "personal":
			emailType = "personal"
		}

		emailAvailable, err := isCardDAVEmailAvailable(ctx, contactID, normalizedEmail)
		if err != nil {
			fmt.Printf("Warning: failed to check CardDAV email availability %s: %v\n", normalizedEmail, err)
			continue
		}

		if !emailAvailable {
			fmt.Printf("Warning: skipped duplicate CardDAV email %s for contact %s\n", normalizedEmail, contactID)
			continue
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO contact_emails (contact_id, email, email_type, is_primary, source)
			VALUES ($1, $2, $3, false, 'carddav')
			ON CONFLICT (contact_id, lower(email)) DO NOTHING
		`, contactID, normalizedEmail, emailType)
		if err != nil {
			// Log but continue - email might fail validation
			fmt.Printf("Warning: failed to sync CardDAV email %s: %v\n", normalizedEmail, err)
			continue
		}

		insertedEmails = append(insertedEmails, normalizedEmail)
	}

	primaryEmail := selectPrimaryEmail(preferredEmail, insertedEmails)
	if primaryEmail != "" {
		_, err = pool.Exec(ctx, `
			UPDATE contact_emails
			SET is_primary = CASE WHEN lower(email) = lower($2) THEN true ELSE false END
			WHERE contact_id = $1
		`, contactID, primaryEmail)
		if err != nil {
			return fmt.Errorf("failed to set primary email: %w", err)
		}
	}

	// Sync phones: delete existing cached phones, then insert new ones
	_, err = pool.Exec(ctx, `DELETE FROM contact_phones WHERE contact_id = $1`, contactID)
	if err != nil {
		return fmt.Errorf("failed to delete existing contact phones: %w", err)
	}

	preferredPhone := normalizeCardDAVPhone(preferredPhoneValue(cardDAVContact.Phones))
	preferredPhoneDigits := normalizePhoneDigits(preferredPhone)

	insertedPhones := make([]string, 0, len(cardDAVContact.Phones))

	seenPhones := make(map[string]bool)

	for _, phone := range cardDAVContact.Phones {
		normalizedPhone := normalizeCardDAVPhone(phone.Phone)
		if normalizedPhone == "" {
			continue
		}

		phoneDigits := normalizePhoneDigits(normalizedPhone)
		if phoneDigits == "" {
			continue
		}
		// Skip duplicate phone numbers (vCards can have the same number with different TYPE params)
		if seenPhones[phoneDigits] {
			continue
		}

		seenPhones[phoneDigits] = true
		// Map CardDAV phone type to database enum
		phoneType := "other"

		switch phone.Type {
		case "cell":
			phoneType = "cell"
		case "work":
			phoneType = "work"
		case "home":
			phoneType = "home"
		case "fax":
			phoneType = "fax"
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO contact_phones (contact_id, phone, phone_type, is_primary, source)
			VALUES ($1, $2, $3, false, 'carddav')
		`, contactID, normalizedPhone, phoneType)
		if err != nil {
			// Log but continue
			fmt.Printf("Warning: failed to sync CardDAV phone %s: %v\n", normalizedPhone, err)
			continue
		}

		insertedPhones = append(insertedPhones, normalizedPhone)
	}

	primaryPhone := selectPrimaryPhoneByDigits(preferredPhoneDigits, insertedPhones)
	if primaryPhone != "" {
		_, err = pool.Exec(ctx, `
			UPDATE contact_phones
			SET is_primary = CASE WHEN phone = $2 THEN true ELSE false END
			WHERE contact_id = $1
		`, contactID, primaryPhone)
		if err != nil {
			return fmt.Errorf("failed to set primary phone: %w", err)
		}
	}

	return nil
}

// SyncAllCardDAVContacts syncs all contacts that are linked to CardDAV
func SyncAllCardDAVContacts(ctx context.Context) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	// Check if CardDAV is configured
	_, err := GetCardDAVConfig()
	if err != nil {
		// CardDAV not configured, skip sync silently
		return nil
	}

	// Get all contacts with CardDAV UUIDs
	query := `SELECT id, carddav_uuid FROM contacts WHERE carddav_uuid IS NOT NULL`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query contacts with CardDAV UUIDs: %w", err)
	}
	defer rows.Close()

	var syncErrors []string

	syncCount := 0

	for rows.Next() {
		var (
			contactID   string
			cardDAVUUID string
		)

		err := rows.Scan(&contactID, &cardDAVUUID)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("failed to scan contact: %v", err))
			continue
		}

		// Sync each contact
		err = SyncContactFromCardDAV(ctx, contactID, cardDAVUUID)
		if err != nil {
			syncErrors = append(syncErrors, fmt.Sprintf("failed to sync contact %s: %v", contactID, err))
			continue
		}

		syncCount++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating contacts: %w", err)
	}

	if len(syncErrors) > 0 {
		fmt.Printf("CardDAV sync completed with %d contacts synced, %d errors:\n", syncCount, len(syncErrors))

		for _, errMsg := range syncErrors {
			fmt.Printf("  - %s\n", errMsg)
		}
	} else if syncCount > 0 {
		fmt.Printf("CardDAV sync completed: %d contacts synced\n", syncCount)
	}

	return nil
}

// CreateCardDAVContact creates a new contact on the CardDAV server
// Returns the UUID of the created contact
func CreateCardDAVContact(ctx context.Context, contact *ContactDetail) (string, error) {
	config, err := GetCardDAVConfig()
	if err != nil {
		return "", err
	}

	client, err := newCardDAVClient(config)
	if err != nil {
		return "", err
	}

	// Generate a new UUID for the vCard
	newUUID := uuid.New().String()

	// Build the vCard
	card := make(vcard.Card)

	// Set UID
	card.SetValue(vcard.FieldUID, newUUID)

	// Set name fields
	nameGiven := ""
	if contact.NameGiven != nil {
		nameGiven = *contact.NameGiven
	}

	nameFamily := ""
	if contact.NameFamily != nil {
		nameFamily = *contact.NameFamily
	}

	// FN (formatted name) is required
	card.SetValue(vcard.FieldFormattedName, contact.NameDisplay)

	// N (structured name) - set using vcard.Name struct
	card.AddName(&vcard.Name{
		FamilyName: nameFamily,
		GivenName:  nameGiven,
	})

	// Add organization if present
	if contact.Organization != nil && *contact.Organization != "" {
		card.SetValue(vcard.FieldOrganization, *contact.Organization)
	}

	// Add title if present
	if contact.Title != nil && *contact.Title != "" {
		card.SetValue(vcard.FieldTitle, *contact.Title)
	}

	// Add emails (only local ones, not already from carddav)
	for _, email := range contact.Emails {
		if email.Source != "carddav" {
			emailType := "home"
			if email.EmailType == EmailWork {
				emailType = "work"
			}

			card.Add(vcard.FieldEmail, &vcard.Field{
				Value:  email.Email,
				Params: vcard.Params{vcard.ParamType: []string{emailType}},
			})
		}
	}

	// Add phones (only local ones, not already from carddav)
	for _, phone := range contact.Phones {
		if phone.Source != "carddav" {
			phoneType := "cell"

			switch phone.PhoneType {
			case PhoneCell:
				phoneType = "cell"
			case PhoneHome:
				phoneType = "home"
			case PhoneWork:
				phoneType = "work"
			case PhoneFax:
				phoneType = "fax"
			case PhonePager:
				phoneType = "pager"
			case PhoneOther:
				phoneType = "other"
			}

			card.Add(vcard.FieldTelephone, &vcard.Field{
				Value:  phone.Phone,
				Params: vcard.Params{vcard.ParamType: []string{phoneType}},
			})
		}
	}

	// Convert to vCard 4.0
	vcard.ToV4(card)

	// Path is relative to the collection URL (just the filename)
	path := newUUID + ".vcf"

	// Create the contact on the server using PUT
	_, err = client.PutAddressObject(ctx, path, card)
	if err != nil {
		return "", fmt.Errorf("failed to create CardDAV contact: %w", err)
	}

	return newUUID, nil
}

// findCardDAVContactPath queries the CardDAV server to find the path of a contact by its UID
func findCardDAVContactPath(ctx context.Context, client *carddav.Client, uuid string) (string, error) {
	// Query the address book for the contact with matching UID
	query := carddav.AddressBookQuery{
		DataRequest: carddav.AddressDataRequest{
			Props: []string{vcard.FieldUID},
		},
	}

	results, err := client.QueryAddressBook(ctx, "", &query)
	if err != nil {
		return "", fmt.Errorf("failed to query address book: %w", err)
	}

	// Find the contact with matching UID
	uuidLower := strings.ToLower(uuid)

	for _, obj := range results {
		if obj.Card != nil {
			cardUID := obj.Card.Value(vcard.FieldUID)
			if strings.ToLower(cardUID) == uuidLower {
				return obj.Path, nil
			}
		}
	}

	return "", fmt.Errorf("%w in CardDAV server: %s", ErrCardDAVContactByUIDNotFound, uuid)
}

// UpdateCardDAVContact updates an existing contact on the CardDAV server
// This fetches the existing vCard first, updates only the fields we manage,
// and preserves all other fields (notes, photo, birthday, addresses, etc.)
func UpdateCardDAVContact(ctx context.Context, contact *ContactDetail) error {
	if contact.CardDAVUUID == nil || *contact.CardDAVUUID == "" {
		return ErrContactNotLinkedToCardDAV
	}

	config, err := GetCardDAVConfig()
	if err != nil {
		return err
	}

	client, err := newCardDAVClient(config)
	if err != nil {
		return err
	}

	existingUUID := *contact.CardDAVUUID

	// Query the server to find the actual path for this contact
	path, err := findCardDAVContactPath(ctx, client, existingUUID)
	if err != nil {
		return fmt.Errorf("failed to find CardDAV contact path: %w", err)
	}

	fmt.Printf("[DEBUG] Found CardDAV contact at path: %s (UUID: %s)\n", path, existingUUID)

	// Fetch the existing vCard to preserve fields we don't manage
	existingObj, err := client.GetAddressObject(ctx, path)
	if err != nil {
		fmt.Printf("[DEBUG] Failed to fetch CardDAV contact: %v\n", err)
		return fmt.Errorf("failed to fetch existing CardDAV contact: %w", err)
	}

	fmt.Printf("[DEBUG] Successfully fetched CardDAV contact\n")

	card := existingObj.Card

	// Update name fields, preserving components we don't manage (prefix, suffix, middle name)
	nameGiven := ""
	if contact.NameGiven != nil {
		nameGiven = *contact.NameGiven
	}

	nameFamily := ""
	if contact.NameFamily != nil {
		nameFamily = *contact.NameFamily
	}

	// Get existing name to preserve additional fields
	existingName := card.Name()

	var additionalName, honorificPrefix, honorificSuffix string
	if existingName != nil {
		additionalName = existingName.AdditionalName
		honorificPrefix = existingName.HonorificPrefix
		honorificSuffix = existingName.HonorificSuffix
	}

	// FN (formatted name) is required - update it
	card.SetValue(vcard.FieldFormattedName, contact.NameDisplay)

	// N (structured name) - remove existing and add new, preserving prefix/suffix/middle
	delete(card, vcard.FieldName)
	card.AddName(&vcard.Name{
		FamilyName:      nameFamily,
		GivenName:       nameGiven,
		AdditionalName:  additionalName,
		HonorificPrefix: honorificPrefix,
		HonorificSuffix: honorificSuffix,
	})

	// Update organization - remove existing and add if present
	delete(card, vcard.FieldOrganization)

	if contact.Organization != nil && *contact.Organization != "" {
		card.SetValue(vcard.FieldOrganization, *contact.Organization)
	}

	// Update title - remove existing and add if present
	delete(card, vcard.FieldTitle)

	if contact.Title != nil && *contact.Title != "" {
		card.SetValue(vcard.FieldTitle, *contact.Title)
	}

	// Update emails - remove existing and add all from local
	delete(card, vcard.FieldEmail)

	for _, email := range contact.Emails {
		emailType := "home"
		if email.EmailType == EmailWork {
			emailType = "work"
		}

		params := vcard.Params{vcard.ParamType: []string{emailType}}
		if email.IsPrimary {
			params.Set(vcard.ParamPreferred, "1")
		}

		card.Add(vcard.FieldEmail, &vcard.Field{
			Value:  email.Email,
			Params: params,
		})
	}

	// Update phones - remove existing and add all from local
	delete(card, vcard.FieldTelephone)

	for _, phone := range contact.Phones {
		phoneType := "cell"

		switch phone.PhoneType {
		case PhoneCell:
			phoneType = "cell"
		case PhoneHome:
			phoneType = "home"
		case PhoneWork:
			phoneType = "work"
		case PhoneFax:
			phoneType = "fax"
		case PhonePager:
			phoneType = "pager"
		case PhoneOther:
			phoneType = "other"
		}

		params := vcard.Params{vcard.ParamType: []string{phoneType}}
		if phone.IsPrimary {
			params.Set(vcard.ParamPreferred, "1")
		}

		card.Add(vcard.FieldTelephone, &vcard.Field{
			Value:  phone.Phone,
			Params: params,
		})
	}

	// Update the contact on the server using PUT
	_, err = client.PutAddressObject(ctx, path, card)
	if err != nil {
		return fmt.Errorf("failed to update CardDAV contact: %w", err)
	}

	fmt.Printf("UpdateCardDAVContact: Successfully updated CardDAV contact %s\n", existingUUID)

	return nil
}
