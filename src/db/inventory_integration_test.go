// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestInventoryLifecycle(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	location := "Warehouse"
	description := "Test item"
	itemType := "appliance"
	inspection := time.Now().UTC().AddDate(0, 0, 7)

	inventoryID, err := CreateInventoryItem(ctx, "Test Item", &location, &description, InventoryStatusActive, &itemType, &inspection)
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

	if item.ItemType == nil || *item.ItemType != itemType {
		t.Fatalf("expected item type %q, got %#v", itemType, item.ItemType)
	}

	newLocation := "Lab"
	newDescription := "Updated"
	newType := "networking"

	newInspection := time.Now().UTC().AddDate(0, 0, 14)
	if err := UpdateInventoryItem(ctx, inventoryID, "Updated Item", &newLocation, &newDescription, InventoryStatusStored, &newType, &newInspection); err != nil {
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

	if _, err := CreateInventoryItem(ctx, "", nil, nil, "", nil, nil); err == nil {
		t.Fatalf("expected error for missing inventory name")
	}

	if err := UpdateInventoryItem(ctx, "missing", "", nil, nil, InventoryStatusActive, nil, nil); err == nil {
		t.Fatalf("expected error for missing name on update")
	}

	if err := UpdateInventoryItem(ctx, "missing", "Name", nil, nil, InventoryStatusActive, nil, nil); err == nil {
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

func TestUpdateInventoryItemPreservesAndClearsOptionalFields(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	location := "Shelf A"
	description := "HF transceiver"
	itemType := "computing"
	inspectionDate := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)

	inventoryID, err := CreateInventoryItem(ctx, "Radio", &location, &description, InventoryStatusActive, &itemType, &inspectionDate)
	if err != nil {
		t.Fatalf("CreateInventoryItem failed: %v", err)
	}

	if err := UpdateInventoryItem(ctx, inventoryID, "Radio Updated", &location, &description, InventoryStatusStored, &itemType, &inspectionDate); err != nil {
		t.Fatalf("UpdateInventoryItem preserve/update failed: %v", err)
	}

	updatedItem, err := GetInventoryItem(ctx, inventoryID)
	if err != nil {
		t.Fatalf("GetInventoryItem failed: %v", err)
	}

	if updatedItem.Location == nil || *updatedItem.Location != location {
		t.Fatalf("expected location %q to remain set, got %#v", location, updatedItem.Location)
	}

	if updatedItem.Description == nil || *updatedItem.Description != description {
		t.Fatalf("expected description %q to remain set, got %#v", description, updatedItem.Description)
	}

	if updatedItem.InspectionDate == nil || updatedItem.InspectionDate.Format("2006-01-02") != inspectionDate.Format("2006-01-02") {
		t.Fatalf("expected inspection_date %s to remain set, got %#v", inspectionDate.Format("2006-01-02"), updatedItem.InspectionDate)
	}

	if updatedItem.ItemType == nil || *updatedItem.ItemType != itemType {
		t.Fatalf("expected item_type %q to remain set, got %#v", itemType, updatedItem.ItemType)
	}

	if err := UpdateInventoryItem(ctx, inventoryID, "Radio Updated", nil, nil, InventoryStatusStored, nil, nil); err != nil {
		t.Fatalf("UpdateInventoryItem explicit clear failed: %v", err)
	}

	clearedItem, err := GetInventoryItem(ctx, inventoryID)
	if err != nil {
		t.Fatalf("GetInventoryItem failed: %v", err)
	}

	if clearedItem.Location != nil {
		t.Fatalf("expected location to be cleared, got %#v", clearedItem.Location)
	}

	if clearedItem.Description != nil {
		t.Fatalf("expected description to be cleared, got %#v", clearedItem.Description)
	}

	if clearedItem.InspectionDate != nil {
		t.Fatalf("expected inspection_date to be cleared, got %#v", clearedItem.InspectionDate)
	}

	if clearedItem.ItemType != nil {
		t.Fatalf("expected item_type to be cleared, got %#v", clearedItem.ItemType)
	}
}

func TestInventoryTypeAndTagFilters(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	networkingType := "networking"
	toolType := "tool"
	consumableType := "consumable"

	routerID, err := CreateInventoryItem(ctx, "Router", nil, nil, InventoryStatusActive, &networkingType, nil)
	if err != nil {
		t.Fatalf("CreateInventoryItem router failed: %v", err)
	}

	drillID, err := CreateInventoryItem(ctx, "Drill", nil, nil, InventoryStatusStored, &toolType, nil)
	if err != nil {
		t.Fatalf("CreateInventoryItem drill failed: %v", err)
	}

	batteryID, err := CreateInventoryItem(ctx, "Battery", nil, nil, InventoryStatusActive, &consumableType, nil)
	if err != nil {
		t.Fatalf("CreateInventoryItem battery failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, routerID, "critical"); err != nil {
		t.Fatalf("AddTagToInventoryItem router critical failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, routerID, "outdoor"); err != nil {
		t.Fatalf("AddTagToInventoryItem router outdoor failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, drillID, "critical"); err != nil {
		t.Fatalf("AddTagToInventoryItem drill critical failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, batteryID, "warranty"); err != nil {
		t.Fatalf("AddTagToInventoryItem battery warranty failed: %v", err)
	}

	typeFilter := " networking "

	typedItems, err := ListInventoryItemsWithFilters(ctx, InventoryListOptions{ItemType: &typeFilter})
	if err != nil {
		t.Fatalf("ListInventoryItemsWithFilters type failed: %v", err)
	}

	if len(typedItems) != 1 || typedItems[0].InventoryID != routerID {
		t.Fatalf("expected router item for type filter, got %#v", typedItems)
	}

	allTags, err := ListAllInventoryTags(ctx)
	if err != nil {
		t.Fatalf("ListAllInventoryTags failed: %v", err)
	}

	tagIDsByName := map[string]string{}
	for _, tag := range allTags {
		tagIDsByName[tag.Name] = tag.ID.String()
	}

	criticalTagID, ok := tagIDsByName["critical"]
	if !ok {
		t.Fatal("missing critical tag id")
	}

	outdoorTagID, ok := tagIDsByName["outdoor"]
	if !ok {
		t.Fatal("missing outdoor tag id")
	}

	statusFilter := InventoryStatusActive

	criticalActiveItems, err := ListInventoryItemsWithFilters(ctx, InventoryListOptions{
		Status: &statusFilter,
		TagIDs: []string{criticalTagID},
	})
	if err != nil {
		t.Fatalf("ListInventoryItemsWithFilters status+tag failed: %v", err)
	}

	if len(criticalActiveItems) != 1 || criticalActiveItems[0].InventoryID != routerID {
		t.Fatalf("expected only active critical router item, got %#v", criticalActiveItems)
	}

	andFiltered, err := ListInventoryItemsWithFilters(ctx, InventoryListOptions{TagIDs: []string{criticalTagID, outdoorTagID}})
	if err != nil {
		t.Fatalf("ListInventoryItemsWithFilters multi-tag failed: %v", err)
	}

	if len(andFiltered) != 1 || andFiltered[0].InventoryID != routerID {
		t.Fatalf("expected only router to match both tags, got %#v", andFiltered)
	}
}

func TestInventoryTypeAndTagValidation(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	invalidType := "tool&fixture"
	if _, err := CreateInventoryItem(ctx, "Bad Type Item", nil, nil, InventoryStatusActive, &invalidType, nil); !errors.Is(err, ErrInventoryTypeInvalid) {
		t.Fatalf("expected ErrInventoryTypeInvalid, got %v", err)
	}

	validType := "tool"

	inventoryID, err := CreateInventoryItem(ctx, "Calibrator", nil, nil, InventoryStatusActive, &validType, nil)
	if err != nil {
		t.Fatalf("CreateInventoryItem failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, inventoryID, "needs&calibration"); !errors.Is(err, ErrInventoryTagNameInvalid) {
		t.Fatalf("expected ErrInventoryTagNameInvalid, got %v", err)
	}
}

func TestInventorySearchQuerySyntax(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	applianceType := "appliance"
	networkType := "networking"

	kettleID, err := CreateInventoryItem(ctx, "Kitchen Kettle", nil, stringPtr("Stainless steel kettle"), InventoryStatusActive, &applianceType, nil)
	if err != nil {
		t.Fatalf("CreateInventoryItem kettle failed: %v", err)
	}

	routerID, err := CreateInventoryItem(ctx, "Field Router", nil, nil, InventoryStatusActive, &networkType, nil)
	if err != nil {
		t.Fatalf("CreateInventoryItem router failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, kettleID, "appliance"); err != nil {
		t.Fatalf("AddTagToInventoryItem kettle appliance failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, kettleID, "critical"); err != nil {
		t.Fatalf("AddTagToInventoryItem kettle critical failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, routerID, "critical"); err != nil {
		t.Fatalf("AddTagToInventoryItem router critical failed: %v", err)
	}

	if err := AddTagToInventoryItem(ctx, routerID, "outdoor"); err != nil {
		t.Fatalf("AddTagToInventoryItem router outdoor failed: %v", err)
	}

	byCategory, err := ListInventoryItemsWithFilters(ctx, InventoryListOptions{SearchQuery: "category:appliance"})
	if err != nil {
		t.Fatalf("ListInventoryItemsWithFilters category query failed: %v", err)
	}

	if len(byCategory) != 1 || byCategory[0].InventoryID != kettleID {
		t.Fatalf("expected only kettle for category filter, got %#v", byCategory)
	}

	byTags, err := ListInventoryItemsWithFilters(ctx, InventoryListOptions{SearchQuery: "tag:critical tag:outdoor"})
	if err != nil {
		t.Fatalf("ListInventoryItemsWithFilters tag query failed: %v", err)
	}

	if len(byTags) != 1 || byTags[0].InventoryID != routerID {
		t.Fatalf("expected only router for two-tag query, got %#v", byTags)
	}

	byText, err := ListInventoryItemsWithFilters(ctx, InventoryListOptions{SearchQuery: "kettle"})
	if err != nil {
		t.Fatalf("ListInventoryItemsWithFilters text query failed: %v", err)
	}

	if len(byText) != 1 || byText[0].InventoryID != kettleID {
		t.Fatalf("expected only kettle for text query, got %#v", byText)
	}
}
