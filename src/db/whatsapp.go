/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"regexp"
	"time"
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
		return fmt.Errorf("database connection not initialized")
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
func FindContactByPhone(ctx context.Context, phoneNumber string) (*string, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	normalized := normalizePhone(phoneNumber)
	if normalized == "" {
		return nil, nil
	}

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
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find contact by phone: %w", err)
	}

	return &contactID, nil
}
