/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

var nonDigitRegex = regexp.MustCompile(`[^\d]`)

// normalizePhone removes all non-digit characters from a phone number
func normalizePhone(phone string) string {
	return nonDigitRegex.ReplaceAllString(phone, "")
}

// UpdateContactAutoTimestamp updates the last_auto_contact timestamp for a contact.
// This is used for automatic contact tracking (e.g., WhatsApp messages).
func UpdateContactAutoTimestamp(ctx context.Context, contactID string, timestamp time.Time) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	query := `
		UPDATE contacts
		SET last_auto_contact = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := pool.Exec(ctx, query, timestamp, contactID)
	if err != nil {
		return fmt.Errorf("failed to update auto contact timestamp: %w", err)
	}

	return nil
}

// FindContactByPhone finds a contact by phone number using normalized matching.
// Returns the contact ID if found, nil otherwise.
// If multiple contacts have the same phone number, returns the one with the highest tier priority.
// Also checks CardDAV phone numbers for contacts linked to CardDAV.
func FindContactByPhone(ctx context.Context, phoneNumber string) (*string, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	normalized := normalizePhone(phoneNumber)
	if normalized == "" {
		return nil, nil //nolint:nilnil // Empty input is treated as no lookup result.
	}

	// First, try to find in local contact_phones table
	// Query for contacts with matching phone numbers
	// Uses suffix matching to handle country code differences
	// Orders by tier (A first) to prefer higher-priority contacts
	query := `
		SELECT c.id
		FROM contacts c
		JOIN contact_phones cp ON c.id = cp.contact_id
		WHERE
			-- Exact match after normalization
			REGEXP_REPLACE(cp.phone, '[^\d]', '', 'g') = $1
			OR
			-- Suffix match (for country code differences)
			-- Check if normalized phone ends with the stored phone or vice versa
			(
				LENGTH(REGEXP_REPLACE(cp.phone, '[^\d]', '', 'g')) >= 7
				AND (
					REGEXP_REPLACE(cp.phone, '[^\d]', '', 'g') LIKE '%' || RIGHT($1, 9)
					OR $1 LIKE '%' || RIGHT(REGEXP_REPLACE(cp.phone, '[^\d]', '', 'g'), 9)
				)
			)
		ORDER BY
			CASE c.tier
				WHEN 'A' THEN 1
				WHEN 'B' THEN 2
				WHEN 'C' THEN 3
				WHEN 'D' THEN 4
				WHEN 'E' THEN 5
				WHEN 'F' THEN 6
			END,
			c.created_at
		LIMIT 1
	`

	var contactID string

	err := pool.QueryRow(ctx, query, normalized).Scan(&contactID)
	if err == nil {
		return &contactID, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to find contact by phone: %w", err)
	}

	// Not found in local database, check CardDAV
	cardDAVContactID, cardDAVErr := findContactByCardDAVPhone(ctx, normalized)
	if cardDAVErr == nil && cardDAVContactID != "" {
		return &cardDAVContactID, nil
	}

	return nil, nil //nolint:nilnil // No match is a valid, non-error outcome.
}

// findContactByCardDAVPhone searches for a contact by checking CardDAV phone numbers.
// Returns the contact ID if found, empty string otherwise.
func findContactByCardDAVPhone(ctx context.Context, normalizedPhone string) (string, error) {
	// Check if CardDAV is configured
	_, err := GetCardDAVConfig()
	if err != nil {
		// CardDAV not configured, skip
		return "", nil
	}

	// Get all contacts with CardDAV UUIDs
	query := `
		SELECT id, carddav_uuid, tier
		FROM contacts
		WHERE carddav_uuid IS NOT NULL
		ORDER BY
			CASE tier
				WHEN 'A' THEN 1
				WHEN 'B' THEN 2
				WHEN 'C' THEN 3
				WHEN 'D' THEN 4
				WHEN 'E' THEN 5
				WHEN 'F' THEN 6
			END,
			created_at
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to query CardDAV-linked contacts: %w", err)
	}
	defer rows.Close()

	type cardDAVContact struct {
		id          string
		cardDAVUUID string
		tier        string
	}

	var contacts []cardDAVContact

	for rows.Next() {
		var c cardDAVContact
		if err := rows.Scan(&c.id, &c.cardDAVUUID, &c.tier); err != nil {
			continue
		}

		contacts = append(contacts, c)
	}

	if len(contacts) == 0 {
		return "", nil
	}

	// Fetch all CardDAV contacts once
	cardDAVContacts, err := ListCardDAVContacts(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch CardDAV contacts: %w", err)
	}

	// Build a map of CardDAV UUID to phones for quick lookup
	cardDAVPhones := make(map[string][]string)

	for _, cdContact := range cardDAVContacts {
		for _, phone := range cdContact.Phones {
			cardDAVPhones[cdContact.UUID] = append(cardDAVPhones[cdContact.UUID], phone.Phone)
		}
	}

	// Check each local contact's CardDAV phones
	for _, c := range contacts {
		phones, ok := cardDAVPhones[c.cardDAVUUID]
		if !ok {
			continue
		}

		for _, phone := range phones {
			if phonesMatch(normalizedPhone, normalizePhone(phone)) {
				return c.id, nil
			}
		}
	}

	return "", nil
}

// phonesMatch checks if two normalized phone numbers match.
// Uses suffix matching to handle country code differences.
func phonesMatch(normalized1, normalized2 string) bool {
	if normalized1 == "" || normalized2 == "" {
		return false
	}

	// Exact match
	if normalized1 == normalized2 {
		return true
	}

	// Suffix match (for country code differences)
	// Only match if both have at least 7 digits
	if len(normalized1) >= 7 && len(normalized2) >= 7 {
		// Get last 9 digits for comparison
		suffix1 := normalized1
		if len(suffix1) > 9 {
			suffix1 = suffix1[len(suffix1)-9:]
		}

		suffix2 := normalized2
		if len(suffix2) > 9 {
			suffix2 = suffix2[len(suffix2)-9:]
		}

		// Check if one ends with the other's suffix
		if len(normalized1) >= len(suffix2) && normalized1[len(normalized1)-len(suffix2):] == suffix2 {
			return true
		}

		if len(normalized2) >= len(suffix1) && normalized2[len(normalized2)-len(suffix1):] == suffix1 {
			return true
		}
	}

	return false
}
