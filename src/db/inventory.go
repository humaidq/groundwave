/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ListInventoryItems returns all inventory items ordered by id DESC (newest first)
func ListInventoryItems(ctx context.Context) ([]InventoryItem, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, inventory_id, name, location, description, created_at, updated_at
		FROM inventory_items
		ORDER BY id DESC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query inventory items: %w", err)
	}
	defer rows.Close()

	var items []InventoryItem
	for rows.Next() {
		var item InventoryItem
		err := rows.Scan(
			&item.ID,
			&item.InventoryID,
			&item.Name,
			&item.Location,
			&item.Description,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan inventory item: %w", err)
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
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, inventory_id, name, location, description, created_at, updated_at
		FROM inventory_items
		WHERE inventory_id = $1
	`

	var item InventoryItem
	err := pool.QueryRow(ctx, query, inventoryID).Scan(
		&item.ID,
		&item.InventoryID,
		&item.Name,
		&item.Location,
		&item.Description,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get inventory item: %w", err)
	}

	return &item, nil
}

// CreateInventoryItem creates a new inventory item (inventory_id auto-generated)
func CreateInventoryItem(ctx context.Context, name string, location *string, description *string) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	// Validate name is not empty
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	query := `
		INSERT INTO inventory_items (name, location, description)
		VALUES ($1, $2, $3)
		RETURNING inventory_id
	`

	var inventoryID string
	err := pool.QueryRow(ctx, query, name, location, description).Scan(&inventoryID)
	if err != nil {
		return "", fmt.Errorf("failed to create inventory item: %w", err)
	}

	return inventoryID, nil
}

// UpdateInventoryItem updates an existing inventory item
func UpdateInventoryItem(ctx context.Context, inventoryID string, name string, location *string, description *string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	if name == "" {
		return fmt.Errorf("name is required")
	}

	query := `
		UPDATE inventory_items
		SET name = $1, location = $2, description = $3
		WHERE inventory_id = $4
	`

	result, err := pool.Exec(ctx, query, name, location, description, inventoryID)
	if err != nil {
		return fmt.Errorf("failed to update inventory item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("inventory item not found: %s", inventoryID)
	}

	return nil
}

// DeleteInventoryItem deletes an inventory item and its comments (CASCADE)
func DeleteInventoryItem(ctx context.Context, inventoryID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM inventory_items WHERE inventory_id = $1`

	result, err := pool.Exec(ctx, query, inventoryID)
	if err != nil {
		return fmt.Errorf("failed to delete inventory item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("inventory item not found: %s", inventoryID)
	}

	return nil
}

// GetDistinctLocations returns all unique location values for autocomplete
func GetDistinctLocations(ctx context.Context) ([]string, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
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

// GetCommentsForItem fetches all comments for an inventory item (newest first)
func GetCommentsForItem(ctx context.Context, itemID int) ([]InventoryComment, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
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
		return fmt.Errorf("database connection not initialized")
	}

	if content == "" {
		return fmt.Errorf("comment content cannot be empty")
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
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM inventory_comments WHERE id = $1`

	result, err := pool.Exec(ctx, query, commentID)
	if err != nil {
		return fmt.Errorf("failed to delete inventory comment: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("comment not found: %s", commentID)
	}

	return nil
}
