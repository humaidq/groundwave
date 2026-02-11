/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
)

// LinkedInURLContact stores a LinkedIn URL with contact ID.
type LinkedInURLContact struct {
	URL       string
	ContactID string
}

// ContactName contains a contact identifier and display name.
type ContactName struct {
	ID          string
	NameDisplay string
}

// ListLinkedInURLs returns all LinkedIn URLs stored for contacts.
func ListLinkedInURLs(ctx context.Context) ([]LinkedInURLContact, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT url, contact_id
		FROM contact_urls
		WHERE url_type = $1
		AND url IS NOT NULL
	`

	rows, err := pool.Query(ctx, query, URLLinkedIn)
	if err != nil {
		return nil, fmt.Errorf("failed to query LinkedIn URLs: %w", err)
	}
	defer rows.Close()

	var urls []LinkedInURLContact

	for rows.Next() {
		var url LinkedInURLContact
		if err := rows.Scan(&url.URL, &url.ContactID); err != nil {
			return nil, fmt.Errorf("failed to scan LinkedIn URL: %w", err)
		}

		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating LinkedIn URLs: %w", err)
	}

	return urls, nil
}

// ListContactsWithoutLinkedIn returns contacts missing LinkedIn URLs.
func ListContactsWithoutLinkedIn(ctx context.Context) ([]ContactName, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT c.id, c.name_display
		FROM contacts c
		WHERE c.is_service = false
		AND NOT EXISTS (
			SELECT 1 FROM contact_urls
			WHERE contact_id = c.id AND url_type = $1
		)
		ORDER BY c.name_display ASC
	`

	rows, err := pool.Query(ctx, query, URLLinkedIn)
	if err != nil {
		return nil, fmt.Errorf("failed to query contacts without LinkedIn: %w", err)
	}
	defer rows.Close()

	var contacts []ContactName

	for rows.Next() {
		var contact ContactName
		if err := rows.Scan(&contact.ID, &contact.NameDisplay); err != nil {
			return nil, fmt.Errorf("failed to scan contact: %w", err)
		}

		contacts = append(contacts, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contacts without LinkedIn: %w", err)
	}

	return contacts, nil
}
