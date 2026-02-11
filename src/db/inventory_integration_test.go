// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestInventoryLifecycle(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	location := "Warehouse"
	description := "Test item"
	inspection := time.Now().UTC().AddDate(0, 0, 7)

	inventoryID, err := CreateInventoryItem(ctx, "Test Item", &location, &description, InventoryStatusActive, &inspection)
	if err != nil {
		t.Fatalf("CreateInventoryItem failed: %v", err)
	}

	items, err := ListInventoryItems(ctx)
	if err != nil {
		t.Fatalf("ListInventoryItems failed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 inventory item, got %d", len(items))
	}

	filtered, err := ListInventoryItems(ctx, InventoryStatusActive)
	if err != nil {
		t.Fatalf("ListInventoryItems filtered failed: %v", err)
	}

	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered item, got %d", len(filtered))
	}

	item, err := GetInventoryItem(ctx, inventoryID)
	if err != nil {
		t.Fatalf("GetInventoryItem failed: %v", err)
	}

	if item.Name != "Test Item" {
		t.Fatalf("expected name Test Item, got %q", item.Name)
	}

	newLocation := "Lab"
	newDescription := "Updated"

	newInspection := time.Now().UTC().AddDate(0, 0, 14)
	if err := UpdateInventoryItem(ctx, inventoryID, "Updated Item", &newLocation, &newDescription, InventoryStatusStored, &newInspection); err != nil {
		t.Fatalf("UpdateInventoryItem failed: %v", err)
	}

	locations, err := GetDistinctLocations(ctx)
	if err != nil {
		t.Fatalf("GetDistinctLocations failed: %v", err)
	}

	if len(locations) != 1 || locations[0] != newLocation {
		t.Fatalf("expected location %q, got %v", newLocation, locations)
	}

	if err := CreateInventoryComment(ctx, item.ID, "Needs inspection"); err != nil {
		t.Fatalf("CreateInventoryComment failed: %v", err)
	}

	comments, err := GetCommentsForItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("GetCommentsForItem failed: %v", err)
	}

	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}

	if err := DeleteInventoryComment(ctx, comments[0].ID); err != nil {
		t.Fatalf("DeleteInventoryComment failed: %v", err)
	}

	count, err := GetInventoryCount(ctx)
	if err != nil {
		t.Fatalf("GetInventoryCount failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 inventory item, got %d", count)
	}

	if err := DeleteInventoryItem(ctx, inventoryID); err != nil {
		t.Fatalf("DeleteInventoryItem failed: %v", err)
	}
}

func TestInventoryErrors(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	if _, err := CreateInventoryItem(ctx, "", nil, nil, "", nil); err == nil {
		t.Fatalf("expected error for missing inventory name")
	}

	if err := UpdateInventoryItem(ctx, "missing", "", nil, nil, InventoryStatusActive, nil); err == nil {
		t.Fatalf("expected error for missing name on update")
	}

	if err := UpdateInventoryItem(ctx, "missing", "Name", nil, nil, InventoryStatusActive, nil); err == nil {
		t.Fatalf("expected error for missing inventory item")
	}

	if err := DeleteInventoryItem(ctx, "missing"); err == nil {
		t.Fatalf("expected error for missing inventory item delete")
	}

	if err := CreateInventoryComment(ctx, 1, ""); err == nil {
		t.Fatalf("expected error for empty inventory comment")
	}

	if err := DeleteInventoryComment(ctx, uuid.New()); err == nil {
		t.Fatalf("expected error for missing inventory comment")
	}
}
