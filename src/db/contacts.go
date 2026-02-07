/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ContactListItem represents a contact in the list view
type ContactListItem struct {
	ID           string  `db:"id"`
	NameDisplay  string  `db:"name_display"`
	Organization *string `db:"organization"`
	Title        *string `db:"title"`
	Tier         Tier    `db:"tier"`
	TierLower    string  // Lowercase tier for CSS classes
	PrimaryEmail *string `db:"primary_email"`
	PrimaryPhone *string `db:"primary_phone"`
	CallSign     *string `db:"call_sign"`
	PhotoURL     *string `db:"photo_url"`
	IsService    bool    `db:"is_service"`
	Tags         []Tag   // Tags associated with this contact
}

// ContactFilter represents a filter type for contact queries
type ContactFilter string

const (
	FilterNoPhone    ContactFilter = "no_phone"
	FilterNoEmail    ContactFilter = "no_email"
	FilterNoCardDAV  ContactFilter = "no_carddav"
	FilterNoLinkedIn ContactFilter = "no_linkedin"
)

// ValidContactFilters returns all valid filter values
var ValidContactFilters = map[ContactFilter]bool{
	FilterNoPhone:    true,
	FilterNoEmail:    true,
	FilterNoCardDAV:  true,
	FilterNoLinkedIn: true,
}

// ContactListOptions holds filtering options for contact queries
type ContactListOptions struct {
	Filters        []ContactFilter
	TagIDs         []string
	IsService      bool
	AlphabeticSort bool // When true, sort alphabetically instead of by tier
}

// ListContacts returns all contacts with their primary email and phone
func ListContacts(ctx context.Context) ([]ContactListItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT
			c.id,
			c.name_display,
			c.organization,
			c.title,
			c.tier,
			c.call_sign,
			c.photo_url,
			c.is_service,
			(SELECT email FROM contact_emails WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_email,
			(SELECT phone FROM contact_phones WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_phone,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'id', t.id::text,
					'name', t.name,
					'description', t.description,
					'created_at', t.created_at
				) ORDER BY t.name)
				 FROM tags t
				 INNER JOIN contact_tags ct ON t.id = ct.tag_id
				 WHERE ct.contact_id = c.id),
				'[]'::json
			) AS tags
		FROM contacts c
		WHERE c.is_service = false
		ORDER BY c.tier ASC, c.name_display ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query contacts: %w", err)
	}
	defer rows.Close()

	var contacts []ContactListItem
	for rows.Next() {
		var contact ContactListItem
		var tagsJSON []byte
		err := rows.Scan(
			&contact.ID,
			&contact.NameDisplay,
			&contact.Organization,
			&contact.Title,
			&contact.Tier,
			&contact.CallSign,
			&contact.PhotoURL,
			&contact.IsService,
			&contact.PrimaryEmail,
			&contact.PrimaryPhone,
			&tagsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}

		// Unmarshal tags JSON
		if len(tagsJSON) > 0 && string(tagsJSON) != "[]" {
			if err := json.Unmarshal(tagsJSON, &contact.Tags); err != nil {
				logger.Warn("Failed to unmarshal tags for contact", "contact_id", contact.ID, "error", err)
				contact.Tags = []Tag{}
			}
		} else {
			contact.Tags = []Tag{}
		}

		// Set lowercase tier for CSS classes
		contact.TierLower = strings.ToLower(string(contact.Tier))
		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contacts: %w", err)
	}

	return contacts, nil
}

// ListContactsWithFilters returns contacts matching the specified filter options
func ListContactsWithFilters(ctx context.Context, opts ContactListOptions) ([]ContactListItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var whereClauses []string
	var args []interface{}
	argNum := 1

	// Base condition - filter by service status
	whereClauses = append(whereClauses, fmt.Sprintf("c.is_service = $%d", argNum))
	args = append(args, opts.IsService)
	argNum++

	// Apply data filters
	for _, f := range opts.Filters {
		switch f {
		case FilterNoPhone:
			whereClauses = append(whereClauses,
				"NOT EXISTS (SELECT 1 FROM contact_phones WHERE contact_id = c.id)")
		case FilterNoEmail:
			whereClauses = append(whereClauses,
				"NOT EXISTS (SELECT 1 FROM contact_emails WHERE contact_id = c.id)")
		case FilterNoCardDAV:
			whereClauses = append(whereClauses, "c.carddav_uuid IS NULL")
		case FilterNoLinkedIn:
			whereClauses = append(whereClauses,
				"NOT EXISTS (SELECT 1 FROM contact_urls WHERE contact_id = c.id AND url_type = 'linkedin')")
		}
	}

	// Apply tag filter (AND logic - must have ALL specified tags)
	if len(opts.TagIDs) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf(
			`c.id IN (
				SELECT contact_id FROM contact_tags
				WHERE tag_id = ANY($%d::uuid[])
				GROUP BY contact_id
				HAVING COUNT(DISTINCT tag_id) = $%d
			)`, argNum, argNum+1))
		args = append(args, opts.TagIDs, len(opts.TagIDs))
	}

	// Build ORDER BY based on service status and alphabetic sort option
	orderBy := "c.tier ASC, c.name_display ASC"
	if opts.AlphabeticSort {
		orderBy = "c.name_display ASC"
	} else if opts.IsService {
		orderBy = "COALESCE(c.organization, c.name_display) ASC, c.name_display ASC"
	}

	// Build final query
	query := fmt.Sprintf(`
		SELECT
			c.id,
			c.name_display,
			c.organization,
			c.title,
			c.tier,
			c.call_sign,
			c.photo_url,
			c.is_service,
			(SELECT email FROM contact_emails WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_email,
			(SELECT phone FROM contact_phones WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_phone,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'id', t.id::text,
					'name', t.name,
					'description', t.description,
					'created_at', t.created_at
				) ORDER BY t.name)
				 FROM tags t
				 INNER JOIN contact_tags ct ON t.id = ct.tag_id
				 WHERE ct.contact_id = c.id),
				'[]'::json
			) AS tags
		FROM contacts c
		WHERE %s
		ORDER BY %s
	`, strings.Join(whereClauses, " AND "), orderBy)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query contacts with filters: %w", err)
	}
	defer rows.Close()

	var contacts []ContactListItem
	for rows.Next() {
		var contact ContactListItem
		var tagsJSON []byte
		err := rows.Scan(
			&contact.ID,
			&contact.NameDisplay,
			&contact.Organization,
			&contact.Title,
			&contact.Tier,
			&contact.CallSign,
			&contact.PhotoURL,
			&contact.IsService,
			&contact.PrimaryEmail,
			&contact.PrimaryPhone,
			&tagsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}

		// Unmarshal tags JSON
		if len(tagsJSON) > 0 && string(tagsJSON) != "[]" {
			if err := json.Unmarshal(tagsJSON, &contact.Tags); err != nil {
				logger.Warn("Failed to unmarshal tags for contact", "contact_id", contact.ID, "error", err)
				contact.Tags = []Tag{}
			}
		} else {
			contact.Tags = []Tag{}
		}

		// Set lowercase tier for CSS classes
		contact.TierLower = strings.ToLower(string(contact.Tier))
		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contacts: %w", err)
	}

	return contacts, nil
}

// CreateContactInput represents the input for creating a new contact
type CreateContactInput struct {
	NameGiven    string
	NameFamily   *string
	Organization *string
	Title        *string
	Email        *string
	Phone        *string
	CallSign     *string
	CardDAVUUID  *string
	IsService    bool
	Tier         Tier
}

// CreateContact creates a new contact and optionally adds email and phone
func CreateContact(ctx context.Context, input CreateContactInput) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	// Start a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			logger.Warn("Failed to rollback contact creation", "error", err)
		}
	}()

	// Build display name from first and last name
	nameDisplay := input.NameGiven
	if input.NameFamily != nil && *input.NameFamily != "" {
		nameDisplay = input.NameGiven + " " + *input.NameFamily
	}

	// Insert contact
	var contactID string
	query := `
		INSERT INTO contacts (
			name_display, name_given, name_family,
			organization, title, call_sign, is_service, carddav_uuid, tier
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	err = tx.QueryRow(ctx, query,
		nameDisplay,
		input.NameGiven,
		input.NameFamily,
		input.Organization,
		input.Title,
		input.CallSign,
		input.IsService,
		input.CardDAVUUID,
		input.Tier,
	).Scan(&contactID)
	if err != nil {
		return "", fmt.Errorf("failed to insert contact: %w", err)
	}

	// Add email if provided
	if input.Email != nil && *input.Email != "" {
		emailQuery := `
			INSERT INTO contact_emails (contact_id, email, email_type, is_primary)
			VALUES ($1, $2, 'personal', true)
		`
		_, err = tx.Exec(ctx, emailQuery, contactID, *input.Email)
		if err != nil {
			return "", fmt.Errorf("failed to insert email: %w", err)
		}
	}

	// Add phone if provided
	if input.Phone != nil && *input.Phone != "" {
		phoneQuery := `
			INSERT INTO contact_phones (contact_id, phone, phone_type, is_primary)
			VALUES ($1, $2, 'cell', true)
		`
		_, err = tx.Exec(ctx, phoneQuery, contactID, *input.Phone)
		if err != nil {
			return "", fmt.Errorf("failed to insert phone: %w", err)
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	// If contact has a call sign, try to link existing QSOs to this contact
	if input.CallSign != nil && *input.CallSign != "" {
		linkedQuery := `
			UPDATE qsos SET contact_id = $1
			WHERE UPPER(call) = UPPER($2) AND contact_id IS NULL
		`
		result, err := pool.Exec(ctx, linkedQuery, contactID, *input.CallSign)
		if err != nil {
			// Log error but don't fail the creation
			fmt.Printf("Warning: failed to auto-link QSOs to new contact: %v\n", err)
		} else {
			rowsAffected := result.RowsAffected()
			if rowsAffected > 0 {
				fmt.Printf("Auto-linked %d existing QSOs to new contact %s\n", rowsAffected, contactID)
			}
		}
	}

	return contactID, nil
}

// ContactDetail represents a full contact with all related data
type ContactDetail struct {
	Contact
	Emails         []ContactEmail
	Phones         []ContactPhone
	Addresses      []ContactAddress
	URLs           []ContactURL
	Logs           []ContactLog
	Notes          []ContactNote
	Tags           []Tag
	CardDAVContact *CardDAVContact // CardDAV contact data if linked
}

// GetContact retrieves a contact by ID with all related data
func GetContact(ctx context.Context, id string) (*ContactDetail, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Get main contact record
	var contact Contact
	query := `
		SELECT
			id, name_given, name_additional, name_family,
			name_display, nickname, organization, title, role, birthday, anniversary,
			gender, timezone, geo_lat, geo_lon, language, photo_url,
			tier, call_sign, is_service, carddav_uuid, created_at, updated_at,
			last_auto_contact
		FROM contacts
		WHERE id = $1
	`
	err := pool.QueryRow(ctx, query, id).Scan(
		&contact.ID,
		&contact.NameGiven,
		&contact.NameAdditional,
		&contact.NameFamily,
		&contact.NameDisplay,
		&contact.Nickname,
		&contact.Organization,
		&contact.Title,
		&contact.Role,
		&contact.Birthday,
		&contact.Anniversary,
		&contact.Gender,
		&contact.Timezone,
		&contact.GeoLat,
		&contact.GeoLon,
		&contact.Language,
		&contact.PhotoURL,
		&contact.Tier,
		&contact.CallSign,
		&contact.IsService,
		&contact.CardDAVUUID,
		&contact.CreatedAt,
		&contact.UpdatedAt,
		&contact.LastAutoContact,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query contact: %w", err)
	}

	detail := &ContactDetail{
		Contact: contact,
	}

	// Get emails
	emailQuery := `SELECT id, contact_id, email, email_type, is_primary, source, created_at
		FROM contact_emails WHERE contact_id = $1 ORDER BY is_primary DESC, created_at`
	emailRows, err := pool.Query(ctx, emailQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query emails: %w", err)
	}
	defer emailRows.Close()

	for emailRows.Next() {
		var email ContactEmail
		err := emailRows.Scan(&email.ID, &email.ContactID, &email.Email, &email.EmailType, &email.IsPrimary, &email.Source, &email.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}
		detail.Emails = append(detail.Emails, email)
	}

	// Get phones
	phoneQuery := `SELECT id, contact_id, phone, phone_type, is_primary, source, created_at
		FROM contact_phones WHERE contact_id = $1 ORDER BY is_primary DESC, created_at`
	phoneRows, err := pool.Query(ctx, phoneQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query phones: %w", err)
	}
	defer phoneRows.Close()

	for phoneRows.Next() {
		var phone ContactPhone
		err := phoneRows.Scan(&phone.ID, &phone.ContactID, &phone.Phone, &phone.PhoneType, &phone.IsPrimary, &phone.Source, &phone.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan phone: %w", err)
		}
		detail.Phones = append(detail.Phones, phone)
	}

	// Get addresses
	addrQuery := `SELECT id, contact_id, street, locality, region, postal_code, country,
		address_type, is_primary, po_box, extended, created_at
		FROM contact_addresses WHERE contact_id = $1 ORDER BY is_primary DESC, created_at`
	addrRows, err := pool.Query(ctx, addrQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query addresses: %w", err)
	}
	defer addrRows.Close()

	for addrRows.Next() {
		var addr ContactAddress
		err := addrRows.Scan(&addr.ID, &addr.ContactID, &addr.Street, &addr.Locality, &addr.Region,
			&addr.PostalCode, &addr.Country, &addr.AddressType, &addr.IsPrimary, &addr.POBox,
			&addr.Extended, &addr.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan address: %w", err)
		}
		detail.Addresses = append(detail.Addresses, addr)
	}

	// Get URLs
	urlQuery := `SELECT id, contact_id, url, url_type, description, created_at
		FROM contact_urls WHERE contact_id = $1 ORDER BY created_at`
	urlRows, err := pool.Query(ctx, urlQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query urls: %w", err)
	}
	defer urlRows.Close()

	for urlRows.Next() {
		var url ContactURL
		err := urlRows.Scan(&url.ID, &url.ContactID, &url.URL, &url.URLType, &url.Description, &url.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan url: %w", err)
		}
		detail.URLs = append(detail.URLs, url)
	}

	// Get contact logs
	logQuery := `SELECT id, contact_id, log_type, logged_at, subject, content, created_at
		FROM contact_logs WHERE contact_id = $1 ORDER BY logged_at DESC, created_at DESC`
	logRows, err := pool.Query(ctx, logQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer logRows.Close()

	for logRows.Next() {
		var log ContactLog
		err := logRows.Scan(&log.ID, &log.ContactID, &log.LogType, &log.LoggedAt, &log.Subject, &log.Content, &log.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log: %w", err)
		}
		detail.Logs = append(detail.Logs, log)
	}

	// Get contact notes
	noteQuery := `SELECT id, contact_id, content, noted_at, created_at, updated_at
		FROM contact_notes WHERE contact_id = $1 ORDER BY noted_at DESC, created_at DESC`
	noteRows, err := pool.Query(ctx, noteQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query notes: %w", err)
	}
	defer noteRows.Close()

	for noteRows.Next() {
		var note ContactNote
		err := noteRows.Scan(&note.ID, &note.ContactID, &note.Content, &note.NotedAt, &note.CreatedAt, &note.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan note: %w", err)
		}
		detail.Notes = append(detail.Notes, note)
	}

	// Get tags
	detail.Tags, err = GetContactTags(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}

	// Fetch CardDAV contact data if linked
	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		cardDAVContact, err := GetCardDAVContact(ctx, *contact.CardDAVUUID)
		if err != nil {
			// Log error but don't fail the request if CardDAV is unavailable
			// Just leave CardDAVContact as nil
			fmt.Printf("Warning: failed to fetch CardDAV contact: %v\n", err)
		} else {
			detail.CardDAVContact = cardDAVContact
		}
	}

	return detail, nil
}

// UpdateContactInput represents the input for updating a contact
type UpdateContactInput struct {
	ID           string
	NameGiven    string
	NameFamily   *string
	Organization *string
	Title        *string
	CallSign     *string
	Tier         Tier
}

// UpdateContact updates an existing contact
func UpdateContact(ctx context.Context, input UpdateContactInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Build display name from first and last name
	nameDisplay := input.NameGiven
	if input.NameFamily != nil && *input.NameFamily != "" {
		nameDisplay = input.NameGiven + " " + *input.NameFamily
	}

	query := `
		UPDATE contacts SET
			name_display = $1,
			name_given = $2,
			name_family = $3,
			organization = $4,
			title = $5,
			call_sign = $6,
			tier = $7
		WHERE id = $8
	`
	_, err := pool.Exec(ctx, query,
		nameDisplay,
		input.NameGiven,
		input.NameFamily,
		input.Organization,
		input.Title,
		input.CallSign,
		input.Tier,
		input.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update contact: %w", err)
	}

	// If contact has a call sign, try to link existing QSOs to this contact
	if input.CallSign != nil && *input.CallSign != "" {
		linkedQuery := `
			UPDATE qsos SET contact_id = $1
			WHERE UPPER(call) = UPPER($2) AND contact_id IS NULL
		`
		result, err := pool.Exec(ctx, linkedQuery, input.ID, *input.CallSign)
		if err != nil {
			// Log error but don't fail the update
			fmt.Printf("Warning: failed to auto-link QSOs to updated contact: %v\n", err)
		} else {
			rowsAffected := result.RowsAffected()
			if rowsAffected > 0 {
				fmt.Printf("Auto-linked %d existing QSOs to contact %s\n", rowsAffected, input.ID)
			}
		}
	}

	return nil
}

// AddEmailInput represents input for adding an email
type AddEmailInput struct {
	ContactID string
	Email     string
	EmailType EmailType
	IsPrimary bool
}

// AddEmail adds a new email to a contact
func AddEmail(ctx context.Context, input AddEmailInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	if !input.IsPrimary {
		var hasPrimary bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM contact_emails WHERE contact_id = $1 AND is_primary = true)`
		if err := pool.QueryRow(ctx, checkQuery, input.ContactID).Scan(&hasPrimary); err != nil {
			return fmt.Errorf("failed to check primary email: %w", err)
		}
		if !hasPrimary {
			input.IsPrimary = true
		}
	}

	// If setting as primary, clear other primaries first
	if input.IsPrimary {
		clearQuery := `UPDATE contact_emails SET is_primary = false WHERE contact_id = $1 AND is_primary = true`
		if _, err := pool.Exec(ctx, clearQuery, input.ContactID); err != nil {
			return fmt.Errorf("failed to clear primary emails: %w", err)
		}
	}

	query := `
		INSERT INTO contact_emails (contact_id, email, email_type, is_primary)
		VALUES ($1, $2, $3, $4)
	`
	_, err := pool.Exec(ctx, query, input.ContactID, input.Email, input.EmailType, input.IsPrimary)
	if err != nil {
		return fmt.Errorf("failed to add email: %w", err)
	}

	return nil
}

// AddPhoneInput represents input for adding a phone
type AddPhoneInput struct {
	ContactID string
	Phone     string
	PhoneType PhoneType
	IsPrimary bool
}

// AddPhone adds a new phone to a contact
func AddPhone(ctx context.Context, input AddPhoneInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	if !input.IsPrimary {
		var hasPrimary bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM contact_phones WHERE contact_id = $1 AND is_primary = true)`
		if err := pool.QueryRow(ctx, checkQuery, input.ContactID).Scan(&hasPrimary); err != nil {
			return fmt.Errorf("failed to check primary phone: %w", err)
		}
		if !hasPrimary {
			input.IsPrimary = true
		}
	}

	// If setting as primary, clear other primaries first
	if input.IsPrimary {
		clearQuery := `UPDATE contact_phones SET is_primary = false WHERE contact_id = $1 AND is_primary = true`
		if _, err := pool.Exec(ctx, clearQuery, input.ContactID); err != nil {
			return fmt.Errorf("failed to clear primary phones: %w", err)
		}
	}

	query := `
		INSERT INTO contact_phones (contact_id, phone, phone_type, is_primary)
		VALUES ($1, $2, $3, $4)
	`
	_, err := pool.Exec(ctx, query, input.ContactID, input.Phone, input.PhoneType, input.IsPrimary)
	if err != nil {
		return fmt.Errorf("failed to add phone: %w", err)
	}

	return nil
}

// AddURLInput represents input for adding a URL
type AddURLInput struct {
	ContactID   string
	URL         string
	URLType     URLType
	Description *string
}

// NormalizeLinkedInURL normalizes a LinkedIn profile URL to a canonical form.
// It strips www prefix, ensures https scheme, and extracts just the username.
// Returns the normalized URL and true if valid, or empty string and false if invalid.
func NormalizeLinkedInURL(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		// Try adding https:// if no scheme present
		parsed, err = url.Parse("https://" + trimmed)
		if err != nil {
			return "", false
		}
	}

	host := strings.ToLower(parsed.Host)
	// Strip www. prefix (handles both www.linkedin.com and linkedin.com)
	host = strings.TrimPrefix(host, "www.")
	if host != "linkedin.com" {
		return "", false
	}

	path := strings.TrimRight(parsed.Path, "/")
	path = strings.ToLower(path)
	if !strings.HasPrefix(path, "/in/") {
		return "", false
	}

	// Extract username from path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 || parts[1] == "" {
		return "", false
	}

	return "https://linkedin.com/in/" + parts[1], true
}

// AddURL adds a new URL to a contact
func AddURL(ctx context.Context, input AddURLInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Normalize LinkedIn URLs to ensure consistent storage
	urlToStore := input.URL
	if input.URLType == URLLinkedIn {
		if normalized, ok := NormalizeLinkedInURL(input.URL); ok {
			urlToStore = normalized
		}
	}

	query := `
		INSERT INTO contact_urls (contact_id, url, url_type, description)
		VALUES ($1, $2, $3, $4)
	`
	_, err := pool.Exec(ctx, query, input.ContactID, urlToStore, input.URLType, input.Description)
	if err != nil {
		return fmt.Errorf("failed to add URL: %w", err)
	}

	return nil
}

// DeleteEmail removes an email from a contact
func DeleteEmail(ctx context.Context, emailID, contactID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM contact_emails WHERE id = $1 AND contact_id = $2`
	_, err := pool.Exec(ctx, query, emailID, contactID)
	if err != nil {
		return fmt.Errorf("failed to delete email: %w", err)
	}

	return nil
}

// DeletePhone removes a phone from a contact
func DeletePhone(ctx context.Context, phoneID, contactID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM contact_phones WHERE id = $1 AND contact_id = $2`
	_, err := pool.Exec(ctx, query, phoneID, contactID)
	if err != nil {
		return fmt.Errorf("failed to delete phone: %w", err)
	}

	return nil
}

// UpdateEmailInput represents input for updating an email
type UpdateEmailInput struct {
	ID        string
	ContactID string
	Email     string
	EmailType EmailType
	IsPrimary bool
}

// UpdateEmail updates an existing email
func UpdateEmail(ctx context.Context, input UpdateEmailInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	if !input.IsPrimary {
		var hasOtherPrimary bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM contact_emails WHERE contact_id = $1 AND is_primary = true AND id != $2)`
		if err := pool.QueryRow(ctx, checkQuery, input.ContactID, input.ID).Scan(&hasOtherPrimary); err != nil {
			return fmt.Errorf("failed to check other primary emails: %w", err)
		}
		if !hasOtherPrimary {
			input.IsPrimary = true
		}
	}

	// If setting as primary, clear other primaries first
	if input.IsPrimary {
		clearQuery := `UPDATE contact_emails SET is_primary = false WHERE contact_id = $1 AND is_primary = true AND id != $2`
		if _, err := pool.Exec(ctx, clearQuery, input.ContactID, input.ID); err != nil {
			return fmt.Errorf("failed to clear primary emails: %w", err)
		}
	}

	query := `
		UPDATE contact_emails
		SET email = $1, email_type = $2, is_primary = $3
		WHERE id = $4 AND contact_id = $5
	`
	_, err := pool.Exec(ctx, query, input.Email, input.EmailType, input.IsPrimary, input.ID, input.ContactID)
	if err != nil {
		return fmt.Errorf("failed to update email: %w", err)
	}

	return nil
}

// UpdatePhoneInput represents input for updating a phone
type UpdatePhoneInput struct {
	ID        string
	ContactID string
	Phone     string
	PhoneType PhoneType
	IsPrimary bool
}

// UpdatePhone updates an existing phone
func UpdatePhone(ctx context.Context, input UpdatePhoneInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	if !input.IsPrimary {
		var hasOtherPrimary bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM contact_phones WHERE contact_id = $1 AND is_primary = true AND id != $2)`
		if err := pool.QueryRow(ctx, checkQuery, input.ContactID, input.ID).Scan(&hasOtherPrimary); err != nil {
			return fmt.Errorf("failed to check other primary phones: %w", err)
		}
		if !hasOtherPrimary {
			input.IsPrimary = true
		}
	}

	// If setting as primary, clear other primaries first
	if input.IsPrimary {
		clearQuery := `UPDATE contact_phones SET is_primary = false WHERE contact_id = $1 AND is_primary = true AND id != $2`
		if _, err := pool.Exec(ctx, clearQuery, input.ContactID, input.ID); err != nil {
			return fmt.Errorf("failed to clear primary phones: %w", err)
		}
	}

	query := `
		UPDATE contact_phones
		SET phone = $1, phone_type = $2, is_primary = $3
		WHERE id = $4 AND contact_id = $5
	`
	_, err := pool.Exec(ctx, query, input.Phone, input.PhoneType, input.IsPrimary, input.ID, input.ContactID)
	if err != nil {
		return fmt.Errorf("failed to update phone: %w", err)
	}

	return nil
}

// GetContactIDByEmailID returns the contact ID for a given email ID
func GetContactIDByEmailID(ctx context.Context, emailID string) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	var contactID string
	query := `SELECT contact_id FROM contact_emails WHERE id = $1`
	err := pool.QueryRow(ctx, query, emailID).Scan(&contactID)
	if err != nil {
		return "", fmt.Errorf("failed to get contact ID for email: %w", err)
	}

	return contactID, nil
}

// GetContactIDByPhoneID returns the contact ID for a given phone ID
func GetContactIDByPhoneID(ctx context.Context, phoneID string) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	var contactID string
	query := `SELECT contact_id FROM contact_phones WHERE id = $1`
	err := pool.QueryRow(ctx, query, phoneID).Scan(&contactID)
	if err != nil {
		return "", fmt.Errorf("failed to get contact ID for phone: %w", err)
	}

	return contactID, nil
}

// DeleteURL removes a URL from a contact
func DeleteURL(ctx context.Context, urlID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM contact_urls WHERE id = $1`
	_, err := pool.Exec(ctx, query, urlID)
	if err != nil {
		return fmt.Errorf("failed to delete URL: %w", err)
	}

	return nil
}

// DeleteContact removes a contact and all associated data
func DeleteContact(ctx context.Context, contactID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// The foreign key constraints with ON DELETE CASCADE will handle
	// deleting related emails, phones, addresses, URLs, etc.
	query := `DELETE FROM contacts WHERE id = $1`
	_, err := pool.Exec(ctx, query, contactID)
	if err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}

	return nil
}

// AddLogInput represents input for adding a contact log
type AddLogInput struct {
	ContactID string
	LogType   LogType
	LoggedAt  *string // Optional, defaults to now
	Subject   *string
	Content   *string
}

// AddLog adds a new interaction log to a contact
func AddLog(ctx context.Context, input AddLogInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `
		INSERT INTO contact_logs (contact_id, log_type, logged_at, subject, content)
		VALUES ($1, $2, COALESCE($3::timestamptz, now()), $4, $5)
	`
	_, err := pool.Exec(ctx, query, input.ContactID, input.LogType, input.LoggedAt, input.Subject, input.Content)
	if err != nil {
		return fmt.Errorf("failed to add log: %w", err)
	}

	return nil
}

// DeleteLog removes a contact log
func DeleteLog(ctx context.Context, logID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM contact_logs WHERE id = $1`
	_, err := pool.Exec(ctx, query, logID)
	if err != nil {
		return fmt.Errorf("failed to delete log: %w", err)
	}

	return nil
}

// ListContactLogsTimeline returns all contact logs for the timeline feed.
func ListContactLogsTimeline(ctx context.Context) ([]ContactLogTimelineEntry, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT l.id, l.contact_id, c.name_display, l.log_type, l.logged_at, l.subject, l.content, l.created_at
		FROM contact_logs l
		INNER JOIN contacts c ON c.id = l.contact_id
		ORDER BY l.logged_at DESC, l.created_at DESC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query contact logs: %w", err)
	}
	defer rows.Close()

	entries := []ContactLogTimelineEntry{}
	for rows.Next() {
		var entry ContactLogTimelineEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.ContactID,
			&entry.ContactName,
			&entry.LogType,
			&entry.LoggedAt,
			&entry.Subject,
			&entry.Content,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan contact log: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// ListContactWeeklyActivityCounts returns weekly activity totals for a contact.
func ListContactWeeklyActivityCounts(ctx context.Context, contactID string, start time.Time, end time.Time) (map[string]int, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		WITH activity AS (
			SELECT date_trunc('week', logged_at AT TIME ZONE 'UTC') AS week_start, COUNT(*) AS count
			FROM contact_logs
			WHERE contact_id = $1 AND logged_at >= $2 AND logged_at < $3
			GROUP BY week_start
			UNION ALL
			SELECT date_trunc('week', (qso_date + time_on) AT TIME ZONE 'UTC') AS week_start, COUNT(*) AS count
			FROM qsos
			WHERE contact_id = $1
				AND (qso_date + time_on) >= ($2 AT TIME ZONE 'UTC')
				AND (qso_date + time_on) < ($3 AT TIME ZONE 'UTC')
			GROUP BY week_start
			UNION ALL
			SELECT date_trunc('week', sent_at AT TIME ZONE 'UTC') AS week_start, COUNT(*) AS count
			FROM contact_chats
			WHERE contact_id = $1 AND sent_at >= $2 AND sent_at < $3
			GROUP BY week_start
		)
		SELECT week_start, SUM(count) AS total_count
		FROM activity
		GROUP BY week_start
		ORDER BY week_start
	`

	rows, err := pool.Query(ctx, query, contactID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query contact activity: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var weekStart time.Time
		var count int
		if err := rows.Scan(&weekStart, &count); err != nil {
			return nil, fmt.Errorf("failed to scan contact activity: %w", err)
		}
		counts[weekStart.UTC().Format("2006-01-02")] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate contact activity: %w", err)
	}

	return counts, nil
}

// AddChatInput represents input for adding a chat entry

type AddChatInput struct {
	ContactID string
	Platform  ChatPlatform
	Sender    ChatSender
	Message   string
	SentAt    *string // Optional, defaults to now
}

// AddChat adds a new chat entry to a contact
func AddChat(ctx context.Context, input AddChatInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	if strings.TrimSpace(input.Message) == "" {
		return fmt.Errorf("chat message is required")
	}

	isService, err := IsServiceContact(ctx, input.ContactID)
	if err != nil {
		return err
	}
	if isService {
		return nil
	}

	platform := input.Platform
	if platform == "" {
		platform = ChatPlatformManual
	}

	sender := input.Sender
	if sender == "" {
		sender = ChatSenderThem
	}

	query := `
		INSERT INTO contact_chats (contact_id, platform, sender, message, sent_at)
		VALUES ($1, $2, $3, $4, COALESCE($5::timestamptz, now()))
	`
	_, err = pool.Exec(ctx, query, input.ContactID, platform, sender, input.Message, input.SentAt)
	if err != nil {
		return fmt.Errorf("failed to add chat entry: %w", err)
	}

	return nil
}

// GetContactChats returns chat history for a contact
func GetContactChats(ctx context.Context, contactID string) ([]ContactChat, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, contact_id, platform, sender, message, sent_at, created_at
		FROM contact_chats
		WHERE contact_id = $1
		ORDER BY sent_at DESC, created_at DESC
	`
	rows, err := pool.Query(ctx, query, contactID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chats: %w", err)
	}
	defer rows.Close()

	var chats []ContactChat
	for rows.Next() {
		var chat ContactChat
		if err := rows.Scan(&chat.ID, &chat.ContactID, &chat.Platform, &chat.Sender, &chat.Message, &chat.SentAt, &chat.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan chat: %w", err)
		}
		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate chats: %w", err)
	}

	return chats, nil
}

// GetContactChatsSince returns chat history for a contact after a timestamp.
func GetContactChatsSince(ctx context.Context, contactID string, since time.Time) ([]ContactChat, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, contact_id, platform, sender, message, sent_at, created_at
		FROM contact_chats
		WHERE contact_id = $1 AND sent_at >= $2
		ORDER BY sent_at ASC, created_at ASC
	`
	rows, err := pool.Query(ctx, query, contactID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to query chats: %w", err)
	}
	defer rows.Close()

	var chats []ContactChat
	for rows.Next() {
		var chat ContactChat
		if err := rows.Scan(&chat.ID, &chat.ContactID, &chat.Platform, &chat.Sender, &chat.Message, &chat.SentAt, &chat.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan chat: %w", err)
		}
		chats = append(chats, chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate chats: %w", err)
	}

	return chats, nil
}

// IsServiceContact checks if a contact is marked as a service contact
func IsServiceContact(ctx context.Context, contactID string) (bool, error) {
	if pool == nil {
		return false, fmt.Errorf("database connection not initialized")
	}

	query := `SELECT is_service FROM contacts WHERE id = $1`
	var isService bool
	if err := pool.QueryRow(ctx, query, contactID).Scan(&isService); err != nil {
		return false, fmt.Errorf("failed to query contact service status: %w", err)
	}

	return isService, nil
}

// LinkCardDAV links a contact with a CardDAV contact UUID
func LinkCardDAV(ctx context.Context, contactID string, cardDAVUUID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `UPDATE contacts SET carddav_uuid = $1 WHERE id = $2`
	_, err := pool.Exec(ctx, query, cardDAVUUID, contactID)
	if err != nil {
		return fmt.Errorf("failed to link CardDAV contact: %w", err)
	}

	return nil
}

// UnlinkCardDAV removes the CardDAV link from a contact
func UnlinkCardDAV(ctx context.Context, contactID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `UPDATE contacts SET carddav_uuid = NULL WHERE id = $1`
	_, err := pool.Exec(ctx, query, contactID)
	if err != nil {
		return fmt.Errorf("failed to unlink CardDAV contact: %w", err)
	}

	return nil
}

// GetLinkedCardDAVUUIDs returns all CardDAV UUIDs that are currently linked to contacts
func GetLinkedCardDAVUUIDs(ctx context.Context) ([]string, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `SELECT carddav_uuid FROM contacts WHERE carddav_uuid IS NOT NULL`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query linked CardDAV UUIDs: %w", err)
	}
	defer rows.Close()

	var uuids []string
	for rows.Next() {
		var uuid string
		if err := rows.Scan(&uuid); err != nil {
			return nil, fmt.Errorf("failed to scan UUID: %w", err)
		}
		uuids = append(uuids, uuid)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating UUIDs: %w", err)
	}

	return uuids, nil
}

// IsCardDAVUUIDLinked checks if a CardDAV UUID is already linked to a contact
func IsCardDAVUUIDLinked(ctx context.Context, uuid string) (bool, error) {
	if pool == nil {
		return false, fmt.Errorf("database connection not initialized")
	}

	query := `SELECT EXISTS(SELECT 1 FROM contacts WHERE carddav_uuid = $1)`
	var exists bool
	err := pool.QueryRow(ctx, query, uuid).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if CardDAV UUID is linked: %w", err)
	}

	return exists, nil
}

// MigrateContactToCardDAV creates a new CardDAV contact from local data and links it
func MigrateContactToCardDAV(ctx context.Context, contactID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Fetch full contact details
	contact, err := GetContact(ctx, contactID)
	if err != nil {
		return fmt.Errorf("failed to fetch contact: %w", err)
	}
	fmt.Printf("MigrateContactToCardDAV: Fetched contact %s (%s)\n", contactID, contact.NameDisplay)

	// Check if already linked to CardDAV
	if contact.CardDAVUUID != nil && *contact.CardDAVUUID != "" {
		return fmt.Errorf("contact is already linked to CardDAV")
	}

	// Create the CardDAV contact
	newUUID, err := CreateCardDAVContact(ctx, contact)
	if err != nil {
		return fmt.Errorf("failed to create CardDAV contact: %w", err)
	}
	fmt.Printf("MigrateContactToCardDAV: Created CardDAV contact with UUID %s\n", newUUID)

	// Start a transaction to update local data
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			logger.Warn("Failed to rollback CardDAV migration", "error", err)
		}
	}()

	// Update the source of local emails to 'carddav'
	_, err = tx.Exec(ctx, `
		UPDATE contact_emails SET source = 'carddav'
		WHERE contact_id = $1 AND source = 'local'
	`, contactID)
	if err != nil {
		return fmt.Errorf("failed to update email sources: %w", err)
	}

	// Update the source of local phones to 'carddav'
	_, err = tx.Exec(ctx, `
		UPDATE contact_phones SET source = 'carddav'
		WHERE contact_id = $1 AND source = 'local'
	`, contactID)
	if err != nil {
		return fmt.Errorf("failed to update phone sources: %w", err)
	}

	// Link the contact to the new CardDAV UUID
	_, err = tx.Exec(ctx, `
		UPDATE contacts SET carddav_uuid = $1 WHERE id = $2
	`, newUUID, contactID)
	if err != nil {
		return fmt.Errorf("failed to link contact to CardDAV: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Printf("MigrateContactToCardDAV: Successfully linked contact %s to CardDAV UUID %s\n", contactID, newUUID)
	return nil
}

// OverdueContactItem represents a contact that is overdue for contact
type OverdueContactItem struct {
	ID              string  `db:"id"`
	NameDisplay     string  `db:"name_display"`
	Organization    *string `db:"organization"`
	Title           *string `db:"title"`
	Tier            Tier    `db:"tier"`
	TierLower       string  // Lowercase tier for CSS classes
	CallSign        *string `db:"call_sign"`
	PhotoURL        *string `db:"photo_url"`
	LastContactDate *string `db:"last_contact_date"`
	DaysOverdue     int     `db:"days_overdue"`
	ContactInterval int     `db:"contact_interval"`
}

// GetOverdueContacts returns contacts that are overdue for contact, sorted by most overdue first
func GetOverdueContacts(ctx context.Context) ([]OverdueContactItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		WITH contact_intervals AS (
			SELECT
				id,
				tier,
				CASE tier
					WHEN 'A' THEN 21   -- 3 weeks
					WHEN 'B' THEN 60   -- 2 months
					WHEN 'C' THEN 180  -- 6 months
					WHEN 'D' THEN 365  -- 1 year
					WHEN 'E' THEN 730  -- 2 years
					ELSE NULL
				END as interval_days
			FROM contacts
			WHERE tier != 'F'  -- Exclude F tier (no regular contact)
		),
		last_contacts AS (
			SELECT
				c.id as contact_id,
				GREATEST(
					(SELECT MAX(logged_at) FROM contact_logs WHERE contact_id = c.id),
					c.last_auto_contact
				) as last_contact_date
			FROM contacts c
		)
		SELECT
			c.id,
			c.name_display,
			c.organization,
			c.title,
			c.tier,
			c.call_sign,
			c.photo_url,
			TO_CHAR(COALESCE(lc.last_contact_date, c.created_at), 'YYYY-MM-DD') as last_contact_date,
			CASE
				WHEN lc.last_contact_date IS NULL THEN
					CAST(EXTRACT(EPOCH FROM (NOW() - c.created_at)) / 86400 AS INTEGER)
				ELSE
					CAST(EXTRACT(EPOCH FROM (NOW() - lc.last_contact_date)) / 86400 AS INTEGER)
			END as days_since_contact,
			ci.interval_days,
			CASE
				WHEN lc.last_contact_date IS NULL THEN
					CAST(EXTRACT(EPOCH FROM (NOW() - c.created_at)) / 86400 AS INTEGER) - ci.interval_days
				ELSE
					CAST(EXTRACT(EPOCH FROM (NOW() - lc.last_contact_date)) / 86400 AS INTEGER) - ci.interval_days
			END as days_overdue
		FROM contacts c
		INNER JOIN contact_intervals ci ON c.id = ci.id
		LEFT JOIN last_contacts lc ON c.id = lc.contact_id
		WHERE
			ci.interval_days IS NOT NULL
			AND c.is_service = false
			AND (
				CASE
					WHEN lc.last_contact_date IS NULL THEN
						CAST(EXTRACT(EPOCH FROM (NOW() - c.created_at)) / 86400 AS INTEGER) > ci.interval_days
					ELSE
						CAST(EXTRACT(EPOCH FROM (NOW() - lc.last_contact_date)) / 86400 AS INTEGER) > ci.interval_days
				END
			)
		ORDER BY days_overdue DESC, c.name_display ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query overdue contacts: %w", err)
	}
	defer rows.Close()

	var contacts []OverdueContactItem
	for rows.Next() {
		var contact OverdueContactItem
		var daysSinceContact int
		err := rows.Scan(
			&contact.ID,
			&contact.NameDisplay,
			&contact.Organization,
			&contact.Title,
			&contact.Tier,
			&contact.CallSign,
			&contact.PhotoURL,
			&contact.LastContactDate,
			&daysSinceContact,
			&contact.ContactInterval,
			&contact.DaysOverdue,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan overdue contact: %w", err)
		}
		// Set lowercase tier for CSS classes
		contact.TierLower = strings.ToLower(string(contact.Tier))
		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overdue contacts: %w", err)
	}

	return contacts, nil
}

// AddNoteInput represents input for adding a contact note
type AddNoteInput struct {
	ContactID string
	Content   string
	NotedAt   *string // Optional, defaults to now
}

// AddNote adds a new note to a contact
func AddNote(ctx context.Context, input AddNoteInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `
		INSERT INTO contact_notes (contact_id, content, noted_at)
		VALUES ($1, $2, COALESCE($3::timestamptz, now()))
	`
	_, err := pool.Exec(ctx, query, input.ContactID, input.Content, input.NotedAt)
	if err != nil {
		return fmt.Errorf("failed to add note: %w", err)
	}

	return nil
}

// DeleteNote removes a contact note
func DeleteNote(ctx context.Context, noteID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM contact_notes WHERE id = $1`
	_, err := pool.Exec(ctx, query, noteID)
	if err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	return nil
}

// GetContactsCount returns the total number of contacts
func GetContactsCount(ctx context.Context) (int, error) {
	if pool == nil {
		return 0, fmt.Errorf("database connection not initialized")
	}

	var count int
	query := `SELECT COUNT(*) FROM contacts`
	err := pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count contacts: %w", err)
	}

	return count, nil
}

// GetRecentContacts returns the N most recently updated contacts
func GetRecentContacts(ctx context.Context, limit int) ([]ContactListItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT
			c.id,
			c.name_display,
			c.organization,
			c.title,
			c.tier,
			c.call_sign,
			c.photo_url,
			c.is_service,
			(SELECT email FROM contact_emails WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_email,
			(SELECT phone FROM contact_phones WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_phone
		FROM contacts c
		WHERE c.is_service = false
		ORDER BY c.updated_at DESC
		LIMIT $1
	`

	rows, err := pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent contacts: %w", err)
	}
	defer rows.Close()

	var contacts []ContactListItem
	for rows.Next() {
		var contact ContactListItem
		err := rows.Scan(
			&contact.ID,
			&contact.NameDisplay,
			&contact.Organization,
			&contact.Title,
			&contact.Tier,
			&contact.CallSign,
			&contact.PhotoURL,
			&contact.IsService,
			&contact.PrimaryEmail,
			&contact.PrimaryPhone,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}
		contact.TierLower = strings.ToLower(string(contact.Tier))
		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contacts: %w", err)
	}

	return contacts, nil
}

// ListServiceContacts returns all service contacts ordered by organization
func ListServiceContacts(ctx context.Context) ([]ContactListItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT
			c.id,
			c.name_display,
			c.organization,
			c.title,
			c.tier,
			c.call_sign,
			c.photo_url,
			c.is_service,
			(SELECT email FROM contact_emails WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_email,
			(SELECT phone FROM contact_phones WHERE contact_id = c.id
			 ORDER BY is_primary DESC, created_at LIMIT 1) AS primary_phone,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'id', t.id::text,
					'name', t.name,
					'description', t.description,
					'created_at', t.created_at
				) ORDER BY t.name)
				 FROM tags t
				 INNER JOIN contact_tags ct ON t.id = ct.tag_id
				 WHERE ct.contact_id = c.id),
				'[]'::json
			) AS tags
		FROM contacts c
		WHERE c.is_service = true
		ORDER BY
			COALESCE(c.organization, c.name_display) ASC,
			c.name_display ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query service contacts: %w", err)
	}
	defer rows.Close()

	var contacts []ContactListItem
	for rows.Next() {
		var contact ContactListItem
		var tagsJSON []byte
		err := rows.Scan(
			&contact.ID,
			&contact.NameDisplay,
			&contact.Organization,
			&contact.Title,
			&contact.Tier,
			&contact.CallSign,
			&contact.PhotoURL,
			&contact.IsService,
			&contact.PrimaryEmail,
			&contact.PrimaryPhone,
			&tagsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan service contact: %w", err)
		}

		// Unmarshal tags JSON
		if len(tagsJSON) > 0 && string(tagsJSON) != "[]" {
			if err := json.Unmarshal(tagsJSON, &contact.Tags); err != nil {
				logger.Warn("Failed to unmarshal tags for contact", "contact_id", contact.ID, "error", err)
				contact.Tags = []Tag{}
			}
		} else {
			contact.Tags = []Tag{}
		}

		contact.TierLower = strings.ToLower(string(contact.Tier))
		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating service contacts: %w", err)
	}

	return contacts, nil
}

// ToggleServiceStatus converts a contact between service and personal
func ToggleServiceStatus(ctx context.Context, contactID string, isService bool) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `UPDATE contacts SET is_service = $1 WHERE id = $2`
	_, err := pool.Exec(ctx, query, isService, contactID)
	if err != nil {
		return fmt.Errorf("failed to toggle service status: %w", err)
	}

	return nil
}

// GetLinkedCardDAVUUIDsWithServiceStatus returns CardDAV UUIDs with service flag
func GetLinkedCardDAVUUIDsWithServiceStatus(ctx context.Context) (map[string]bool, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `SELECT carddav_uuid, is_service FROM contacts WHERE carddav_uuid IS NOT NULL`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query linked CardDAV UUIDs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var uuid string
		var isService bool
		if err := rows.Scan(&uuid, &isService); err != nil {
			return nil, fmt.Errorf("failed to scan UUID: %w", err)
		}
		result[strings.ToLower(uuid)] = isService
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating UUIDs: %w", err)
	}

	return result, nil
}
