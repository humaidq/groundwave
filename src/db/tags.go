/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// TagWithUsage represents a tag with usage count
type TagWithUsage struct {
	Tag
	UsageCount int `db:"usage_count"`
}

// ListAllTags returns all tags with their usage counts
func ListAllTags(ctx context.Context) ([]TagWithUsage, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT
			t.id,
			t.name,
			t.description,
			t.created_at,
			COUNT(ct.contact_id) as usage_count
		FROM tags t
		LEFT JOIN contact_tags ct ON t.id = ct.tag_id
		GROUP BY t.id, t.name, t.description, t.created_at
		ORDER BY t.name ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	var tags []TagWithUsage
	for rows.Next() {
		var tag TagWithUsage
		err := rows.Scan(
			&tag.ID,
			&tag.Name,
			&tag.Description,
			&tag.CreatedAt,
			&tag.UsageCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return tags, nil
}

// SearchTags searches tags by name (case-insensitive prefix match)
func SearchTags(ctx context.Context, query string) ([]TagWithUsage, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	sqlQuery := `
		SELECT
			t.id,
			t.name,
			t.description,
			t.created_at,
			COUNT(ct.contact_id) as usage_count
		FROM tags t
		LEFT JOIN contact_tags ct ON t.id = ct.tag_id
		WHERE LOWER(t.name) LIKE LOWER($1 || '%')
		GROUP BY t.id, t.name, t.description, t.created_at
		ORDER BY t.name ASC
	`

	rows, err := pool.Query(ctx, sqlQuery, query)
	if err != nil {
		return nil, fmt.Errorf("failed to search tags: %w", err)
	}
	defer rows.Close()

	var tags []TagWithUsage
	for rows.Next() {
		var tag TagWithUsage
		err := rows.Scan(
			&tag.ID,
			&tag.Name,
			&tag.Description,
			&tag.CreatedAt,
			&tag.UsageCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return tags, nil
}

// GetTag retrieves a single tag by ID
func GetTag(ctx context.Context, tagID string) (*Tag, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, name, description, created_at
		FROM tags
		WHERE id = $1
	`

	var tag Tag
	err := pool.QueryRow(ctx, query, tagID).Scan(
		&tag.ID,
		&tag.Name,
		&tag.Description,
		&tag.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query tag: %w", err)
	}

	return &tag, nil
}

// RenameTag updates a tag's name and description
func RenameTag(ctx context.Context, tagID string, newName string, description *string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Normalize tag name to lowercase
	normalizedName := strings.ToLower(newName)

	query := `
		UPDATE tags
		SET name = $1, description = $2
		WHERE id = $3
	`

	_, err := pool.Exec(ctx, query, normalizedName, description, tagID)
	if err != nil {
		return fmt.Errorf("failed to rename tag: %w", err)
	}

	return nil
}

// GetContactTags retrieves all tags for a contact
func GetContactTags(ctx context.Context, contactID string) ([]Tag, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT t.id, t.name, t.description, t.created_at
		FROM tags t
		INNER JOIN contact_tags ct ON t.id = ct.tag_id
		WHERE ct.contact_id = $1
		ORDER BY t.name ASC
	`

	rows, err := pool.Query(ctx, query, contactID)
	if err != nil {
		return nil, fmt.Errorf("failed to query contact tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.Description, &tag.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tags: %w", err)
	}

	return tags, nil
}

// AddTagToContact adds a tag to a contact (creates tag if it doesn't exist)
func AddTagToContact(ctx context.Context, contactID string, tagName string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Use the PostgreSQL get_or_create_tag function to get or create the tag
	query := `
		INSERT INTO contact_tags (contact_id, tag_id)
		VALUES ($1, get_or_create_tag($2))
		ON CONFLICT (contact_id, tag_id) DO NOTHING
	`

	_, err := pool.Exec(ctx, query, contactID, tagName)
	if err != nil {
		return fmt.Errorf("failed to add tag to contact: %w", err)
	}

	return nil
}

// RemoveTagFromContact removes a tag from a contact
func RemoveTagFromContact(ctx context.Context, contactID string, tagID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM contact_tags WHERE contact_id = $1 AND tag_id = $2`
	_, err := pool.Exec(ctx, query, contactID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from contact: %w", err)
	}

	return nil
}

// DeleteTag deletes a tag and all its associations with contacts
func DeleteTag(ctx context.Context, tagID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	// Delete tag associations first (due to foreign key constraints)
	_, err := pool.Exec(ctx, `DELETE FROM contact_tags WHERE tag_id = $1`, tagID)
	if err != nil {
		return fmt.Errorf("failed to delete tag associations: %w", err)
	}

	// Delete the tag itself
	_, err = pool.Exec(ctx, `DELETE FROM tags WHERE id = $1`, tagID)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}

	return nil
}

// GetContactsByTags returns contacts matching ALL specified tags (AND logic)
func GetContactsByTags(ctx context.Context, tagIDs []string) ([]ContactListItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	if len(tagIDs) == 0 {
		return nil, fmt.Errorf("no tag IDs provided")
	}

	query := `
		SELECT
			c.id,
			c.name_display,
			c.organization,
			c.title,
			c.tier,
			c.call_sign,
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
		WHERE c.id IN (
			SELECT contact_id
			FROM contact_tags
			WHERE tag_id = ANY($1)
			GROUP BY contact_id
			HAVING COUNT(DISTINCT tag_id) = $2
		)
		ORDER BY c.tier ASC, c.name_display ASC
	`

	rows, err := pool.Query(ctx, query, tagIDs, len(tagIDs))
	if err != nil {
		return nil, fmt.Errorf("failed to query contacts by tags: %w", err)
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
				log.Printf("Warning: failed to unmarshal tags for contact %s: %v", contact.ID, err)
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
