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
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var inventoryLabelPattern = regexp.MustCompile(`^[a-z0-9._:/-]+( [a-z0-9._:/-]+)*$`)

// InventoryListOptions holds filtering options for inventory list queries.
type InventoryListOptions struct {
	Status      *InventoryStatus
	ItemType    *string
	TagIDs      []string
	SearchQuery string
}

// ListInventoryItems returns inventory items ordered by inspection due first, then newest.
// If status is provided, filters by that status.
func ListInventoryItems(ctx context.Context, status ...InventoryStatus) ([]InventoryItem, error) {
	opts := InventoryListOptions{}

	if len(status) > 0 && status[0] != "" {
		statusFilter := status[0]
		opts.Status = &statusFilter
	}

	return ListInventoryItemsWithFilters(ctx, opts)
}

// ListInventoryItemsWithFilters returns inventory items matching provided filters.
func ListInventoryItemsWithFilters(ctx context.Context, opts InventoryListOptions) ([]InventoryItem, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	var (
		whereClauses = make([]string, 0, 10)
		args         = make([]any, 0, 10)
	)

	argNum := 1

	if opts.Status != nil && *opts.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("i.status = $%d", argNum))
		args = append(args, *opts.Status)
		argNum++
	}

	if opts.ItemType != nil {
		normalizedType, err := normalizeInventoryTypeValue(*opts.ItemType)
		if err != nil {
			return nil, err
		}

		if normalizedType != "" {
			whereClauses = append(whereClauses, fmt.Sprintf("i.item_type = $%d", argNum))
			args = append(args, normalizedType)
			argNum++
		}
	}

	if len(opts.TagIDs) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf(
			`i.id IN (
				SELECT item_id
				FROM inventory_item_tags
				WHERE tag_id = ANY($%d::uuid[])
				GROUP BY item_id
				HAVING COUNT(DISTINCT tag_id) = $%d
			)`, argNum, argNum+1))
		args = append(args, opts.TagIDs, len(opts.TagIDs))
		argNum += 2
	}

	parsed := parseSearchQuery(opts.SearchQuery)

	for _, category := range parsed.categories {
		normalizedCategory, err := normalizeInventoryTypeValue(category)
		if err != nil {
			return nil, err
		}

		if normalizedCategory == "" {
			continue
		}

		whereClauses = append(whereClauses, fmt.Sprintf("i.item_type = $%d", argNum))
		args = append(args, normalizedCategory)
		argNum++
	}

	if len(parsed.tagNames) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf(
			`i.id IN (
				SELECT iit.item_id
				FROM inventory_item_tags iit
				INNER JOIN inventory_tags t ON t.id = iit.tag_id
				WHERE lower(t.name) = ANY($%d::text[])
				GROUP BY iit.item_id
				HAVING COUNT(DISTINCT lower(t.name)) = $%d
			)`, argNum, argNum+1))
		args = append(args, parsed.tagNames, len(parsed.tagNames))
		argNum += 2
	}

	for _, term := range parsed.freeTerms {
		whereClauses = append(whereClauses, fmt.Sprintf(`(
			i.inventory_id ILIKE $%d OR
			i.name ILIKE $%d OR
			COALESCE(i.location, '') ILIKE $%d OR
			COALESCE(i.description, '') ILIKE $%d OR
			COALESCE(i.item_type, '') ILIKE $%d OR
			EXISTS (
				SELECT 1
				FROM inventory_item_tags iit
				INNER JOIN inventory_tags t ON t.id = iit.tag_id
				WHERE iit.item_id = i.id AND t.name ILIKE $%d
			)
		)`, argNum, argNum, argNum, argNum, argNum, argNum))
		args = append(args, "%"+term+"%")
		argNum++
	}

	query := `
		SELECT
			i.id,
			i.inventory_id,
			i.name,
			i.location,
			i.description,
			i.status,
			i.item_type,
			i.inspection_date,
			(i.inspection_date IS NOT NULL AND i.inspection_date <= CURRENT_DATE) AS inspection_due,
			i.created_at,
			i.updated_at,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'id', t.id::text,
					'name', t.name,
					'created_at', t.created_at
				) ORDER BY t.name)
				 FROM inventory_tags t
				 INNER JOIN inventory_item_tags iit ON t.id = iit.tag_id
				 WHERE iit.item_id = i.id),
				'[]'::json
			) AS tags
		FROM inventory_items i
	`

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	query += " ORDER BY inspection_due DESC, i.id DESC"

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventory items: %w", err)
	}
	defer rows.Close()

	var items []InventoryItem

	for rows.Next() {
		var (
			item     InventoryItem
			tagsJSON []byte
		)

		err := rows.Scan(
			&item.ID,
			&item.InventoryID,
			&item.Name,
			&item.Location,
			&item.Description,
			&item.Status,
			&item.ItemType,
			&item.InspectionDate,
			&item.InspectionDue,
			&item.CreatedAt,
			&item.UpdatedAt,
			&tagsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory item: %w", err)
		}

		if len(tagsJSON) > 0 && string(tagsJSON) != "[]" {
			if err := json.Unmarshal(tagsJSON, &item.Tags); err != nil {
				logger.Warn("Failed to unmarshal tags for inventory item", "inventory_id", item.InventoryID, "error", err)
				item.Tags = []InventoryTag{}
			}
		} else {
			item.Tags = []InventoryTag{}
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inventory items: %w", err)
	}

	return items, nil
}

// GetInventoryItem fetches a single inventory item by its formatted inventory_id (GW-00001)
func GetInventoryItem(ctx context.Context, inventoryID string) (*InventoryItem, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT
			i.id,
			i.inventory_id,
			i.name,
			i.location,
			i.description,
			i.status,
			i.item_type,
			i.inspection_date,
			(i.inspection_date IS NOT NULL AND i.inspection_date <= CURRENT_DATE) AS inspection_due,
			i.created_at,
			i.updated_at,
			COALESCE(
				(SELECT json_agg(json_build_object(
					'id', t.id::text,
					'name', t.name,
					'created_at', t.created_at
				) ORDER BY t.name)
				 FROM inventory_tags t
				 INNER JOIN inventory_item_tags iit ON t.id = iit.tag_id
				 WHERE iit.item_id = i.id),
				'[]'::json
			) AS tags
		FROM inventory_items i
		WHERE i.inventory_id = $1
	`

	var (
		item     InventoryItem
		tagsJSON []byte
	)

	err := pool.QueryRow(ctx, query, inventoryID).Scan(
		&item.ID,
		&item.InventoryID,
		&item.Name,
		&item.Location,
		&item.Description,
		&item.Status,
		&item.ItemType,
		&item.InspectionDate,
		&item.InspectionDue,
		&item.CreatedAt,
		&item.UpdatedAt,
		&tagsJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}

	if len(tagsJSON) > 0 && string(tagsJSON) != "[]" {
		if err := json.Unmarshal(tagsJSON, &item.Tags); err != nil {
			return nil, fmt.Errorf("failed to decode inventory item tags: %w", err)
		}
	} else {
		item.Tags = []InventoryTag{}
	}

	return &item, nil
}

// CreateInventoryItem creates a new inventory item (inventory_id auto-generated).
func CreateInventoryItem(ctx context.Context, name string, location *string, description *string, status InventoryStatus, itemType *string, inspectionDate *time.Time) (string, error) {
	if pool == nil {
		return "", ErrDatabaseConnectionNotInitialized
	}

	// Validate name is not empty
	if name == "" {
		return "", ErrNameRequired
	}

	// Default to active if not specified
	if status == "" {
		status = InventoryStatusActive
	}

	var normalizedType *string

	if itemType != nil {
		normalized, err := normalizeInventoryTypeValue(*itemType)
		if err != nil {
			return "", err
		}

		if normalized != "" {
			normalizedType = &normalized
		}
	}

	query := `
		INSERT INTO inventory_items (name, location, description, status, item_type, inspection_date)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING inventory_id
	`

	var inventoryID string

	err := pool.QueryRow(ctx, query, name, location, description, status, normalizedType, inspectionDate).Scan(&inventoryID)
	if err != nil {
		return "", fmt.Errorf("failed to create inventory item: %w", err)
	}

	return inventoryID, nil
}

// UpdateInventoryItem updates an existing inventory item.
func UpdateInventoryItem(ctx context.Context, inventoryID string, name string, location *string, description *string, status InventoryStatus, itemType *string, inspectionDate *time.Time) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	if name == "" {
		return ErrNameRequired
	}

	var normalizedType *string

	if itemType != nil {
		normalized, err := normalizeInventoryTypeValue(*itemType)
		if err != nil {
			return err
		}

		if normalized != "" {
			normalizedType = &normalized
		}
	}

	query := `
		UPDATE inventory_items
		SET name = $1, location = $2, description = $3, status = $4, item_type = $5, inspection_date = $6
		WHERE inventory_id = $7
	`

	result, err := pool.Exec(ctx, query, name, location, description, status, normalizedType, inspectionDate, inventoryID)
	if err != nil {
		return fmt.Errorf("failed to update inventory item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrInventoryItemNotFound, inventoryID)
	}

	return nil
}

// DeleteInventoryItem deletes an inventory item and its comments (CASCADE)
func DeleteInventoryItem(ctx context.Context, inventoryID string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	query := `DELETE FROM inventory_items WHERE inventory_id = $1`

	result, err := pool.Exec(ctx, query, inventoryID)
	if err != nil {
		return fmt.Errorf("failed to delete inventory item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrInventoryItemNotFound, inventoryID)
	}

	return nil
}

// GetDistinctLocations returns all unique location values for autocomplete
func GetDistinctLocations(ctx context.Context) ([]string, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT DISTINCT location
		FROM inventory_items
		WHERE location IS NOT NULL AND location != ''
		ORDER BY location ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %w", err)
	}
	defer rows.Close()

	var locations []string

	for rows.Next() {
		var loc string
		if err := rows.Scan(&loc); err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}

		locations = append(locations, loc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	return locations, nil
}

// GetDistinctInventoryTypes returns all unique inventory type values for autocomplete.
func GetDistinctInventoryTypes(ctx context.Context) ([]string, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT DISTINCT item_type
		FROM inventory_items
		WHERE item_type IS NOT NULL AND item_type != ''
		ORDER BY item_type ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventory types: %w", err)
	}
	defer rows.Close()

	var itemTypes []string

	for rows.Next() {
		var itemType string
		if err := rows.Scan(&itemType); err != nil {
			return nil, fmt.Errorf("failed to scan inventory type: %w", err)
		}

		itemTypes = append(itemTypes, itemType)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inventory types: %w", err)
	}

	return itemTypes, nil
}

// ListAllInventoryTags returns all inventory tags with their usage counts.
func ListAllInventoryTags(ctx context.Context) ([]InventoryTagWithUsage, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT
			t.id,
			t.name,
			t.created_at,
			COUNT(iit.item_id) AS usage_count
		FROM inventory_tags t
		LEFT JOIN inventory_item_tags iit ON t.id = iit.tag_id
		GROUP BY t.id, t.name, t.created_at
		ORDER BY t.name ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventory tags: %w", err)
	}
	defer rows.Close()

	var tags []InventoryTagWithUsage

	for rows.Next() {
		var tag InventoryTagWithUsage

		err := rows.Scan(
			&tag.ID,
			&tag.Name,
			&tag.CreatedAt,
			&tag.UsageCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory tag: %w", err)
		}

		tags = append(tags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating inventory tags: %w", err)
	}

	return tags, nil
}

// AddTagToInventoryItem adds a tag to an inventory item (creates tag if it does not exist).
func AddTagToInventoryItem(ctx context.Context, inventoryID string, tagName string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	normalizedTagName, err := normalizeInventoryTagNameValue(tagName)
	if err != nil {
		return err
	}

	var itemID int

	err = pool.QueryRow(ctx, `SELECT id FROM inventory_items WHERE inventory_id = $1`, inventoryID).Scan(&itemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %s", ErrInventoryItemNotFound, inventoryID)
		}

		return fmt.Errorf("failed to look up inventory item: %w", err)
	}

	query := `
		WITH upserted_tag AS (
			INSERT INTO inventory_tags (name)
			VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id
		)
		INSERT INTO inventory_item_tags (item_id, tag_id)
		SELECT $2, id FROM upserted_tag
		ON CONFLICT (item_id, tag_id) DO NOTHING
	`

	_, err = pool.Exec(ctx, query, normalizedTagName, itemID)
	if err != nil {
		return fmt.Errorf("failed to add tag to inventory item: %w", err)
	}

	return nil
}

// RemoveTagFromInventoryItem removes a tag from an inventory item.
func RemoveTagFromInventoryItem(ctx context.Context, inventoryID string, tagID string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	var itemID int

	err := pool.QueryRow(ctx, `SELECT id FROM inventory_items WHERE inventory_id = $1`, inventoryID).Scan(&itemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %s", ErrInventoryItemNotFound, inventoryID)
		}

		return fmt.Errorf("failed to look up inventory item: %w", err)
	}

	query := `DELETE FROM inventory_item_tags WHERE item_id = $1 AND tag_id = $2`

	_, err = pool.Exec(ctx, query, itemID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from inventory item: %w", err)
	}

	return nil
}

func normalizeInventoryTypeValue(rawType string) (string, error) {
	normalizedType := normalizeInventoryLabel(rawType)
	if normalizedType == "" {
		return "", nil
	}

	if !inventoryLabelPattern.MatchString(normalizedType) {
		return "", ErrInventoryTypeInvalid
	}

	return normalizedType, nil
}

func normalizeInventoryTagNameValue(tagName string) (string, error) {
	normalizedTagName := normalizeInventoryLabel(tagName)
	if normalizedTagName == "" {
		return "", ErrInventoryTagNameInvalid
	}

	if !inventoryLabelPattern.MatchString(normalizedTagName) {
		return "", ErrInventoryTagNameInvalid
	}

	return normalizedTagName, nil
}

func normalizeInventoryLabel(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	return strings.ToLower(strings.Join(strings.Fields(trimmed), " "))
}

// GetCommentsForItem fetches all comments for an inventory item (newest first)
func GetCommentsForItem(ctx context.Context, itemID int) ([]InventoryComment, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT id, item_id, content, created_at, updated_at
		FROM inventory_comments
		WHERE item_id = $1
		ORDER BY created_at DESC
	`

	rows, err := pool.Query(ctx, query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventory comments: %w", err)
	}
	defer rows.Close()

	var comments []InventoryComment

	for rows.Next() {
		var comment InventoryComment

		err := rows.Scan(
			&comment.ID,
			&comment.ItemID,
			&comment.Content,
			&comment.CreatedAt,
			&comment.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan comment: %w", err)
		}

		comments = append(comments, comment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating comments: %w", err)
	}

	return comments, nil
}

// CreateInventoryComment creates a new comment for an inventory item
func CreateInventoryComment(ctx context.Context, itemID int, content string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	if content == "" {
		return ErrCommentContentEmpty
	}

	query := `
		INSERT INTO inventory_comments (item_id, content)
		VALUES ($1, $2)
	`

	_, err := pool.Exec(ctx, query, itemID, content)
	if err != nil {
		return fmt.Errorf("failed to create inventory comment: %w", err)
	}

	return nil
}

// DeleteInventoryComment deletes a comment by UUID
func DeleteInventoryComment(ctx context.Context, commentID uuid.UUID) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	query := `DELETE FROM inventory_comments WHERE id = $1`

	result, err := pool.Exec(ctx, query, commentID)
	if err != nil {
		return fmt.Errorf("failed to delete inventory comment: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrCommentNotFound, commentID)
	}

	return nil
}

// GetInventoryCount returns the total number of inventory items
func GetInventoryCount(ctx context.Context) (int, error) {
	if pool == nil {
		return 0, ErrDatabaseConnectionNotInitialized
	}

	var count int

	query := `SELECT COUNT(*) FROM inventory_items`

	err := pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count inventory items: %w", err)
	}

	return count, nil
}
