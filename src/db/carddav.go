/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
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
	Email string
	Type  string
}

// CardDAVPhone represents a phone number from CardDAV
type CardDAVPhone struct {
	Phone string
	Type  string
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
		return nil, fmt.Errorf("CardDAV configuration incomplete: CARDDAV_URL, CARDDAV_USERNAME, and CARDDAV_PASSWORD must all be set")
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
	return t.Base.RoundTrip(req)
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
	req, err := http.NewRequestWithContext(ctx, "GET", config.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VCF file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch VCF file: HTTP %d", resp.StatusCode)
	}

	// Parse the VCF file
	decoder := vcard.NewDecoder(resp.Body)
	var contacts []CardDAVContact

	for {
		card, err := decoder.Decode()
		if err == io.EOF {
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

	return nil, fmt.Errorf("contact with UUID %s not found (checked %d contacts)", uuid, len(contacts))
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
	if emailFields, ok := card[vcard.FieldEmail]; ok {
		for _, field := range emailFields {
			emailType := "other"
			if field.Params.HasType("work") {
				emailType = "work"
			} else if field.Params.HasType("home") {
				emailType = "personal"
			}
			contact.Emails = append(contact.Emails, CardDAVEmail{
				Email: field.Value,
				Type:  emailType,
			})
		}
	}

	// Get phones
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
				Phone: field.Value,
				Type:  phoneType,
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
	for _, url := range card.Values(vcard.FieldURL) {
		contact.URLs = append(contact.URLs, url)
	}

	// Get notes
	contact.Notes = card.Value(vcard.FieldNote)

	// Get photo URL
	if photoField := card.Get(vcard.FieldPhoto); photoField != nil {
		contact.PhotoURL = photoField.Value
	}

	return contact
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

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

// SyncContactFromCardDAV updates a contact's details from CardDAV
// It syncs: name_given, name_family, organization, title
func SyncContactFromCardDAV(ctx context.Context, contactID string, cardDAVUUID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
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

	// Update the contact in the database
	query := `
		UPDATE contacts SET
			name_display = $1,
			name_given = $2,
			name_family = $3,
			organization = $4,
			title = $5,
			updated_at = now()
		WHERE id = $6
	`
	_, err = pool.Exec(ctx, query,
		nameDisplay,
		cardDAVContact.GivenName,
		nameFamilyPtr,
		organizationPtr,
		titlePtr,
		contactID,
	)
	if err != nil {
		return fmt.Errorf("failed to update contact from CardDAV: %w", err)
	}

	return nil
}

// SyncAllCardDAVContacts syncs all contacts that are linked to CardDAV
func SyncAllCardDAVContacts(ctx context.Context) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
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
		var contactID string
		var cardDAVUUID string
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
