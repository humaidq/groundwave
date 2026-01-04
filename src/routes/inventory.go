/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/flamego/flamego"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

// InventoryList renders the inventory list page
func InventoryList(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch all inventory items
	items, err := db.ListInventoryItems(ctx)
	if err != nil {
		log.Printf("Error fetching inventory items: %v", err)
		data["Error"] = "Failed to load inventory items"
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	data["Items"] = items
	data["IsInventory"] = true
	t.HTML(http.StatusOK, "inventory")
}

// ViewInventoryItem renders a single inventory item with comments and files
func ViewInventoryItem(c flamego.Context, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	if inventoryID == "" {
		data["Error"] = "Inventory ID is required"
		t.HTML(http.StatusBadRequest, "error")
		return
	}

	ctx := c.Request().Context()

	// Fetch item
	item, err := db.GetInventoryItem(ctx, inventoryID)
	if err != nil {
		log.Printf("Error fetching inventory item %s: %v", inventoryID, err)
		data["Error"] = "Inventory item not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	// Fetch comments
	comments, err := db.GetCommentsForItem(ctx, item.ID)
	if err != nil {
		log.Printf("Error fetching comments for item %s: %v", inventoryID, err)
		// Don't fail the page, just show empty comments
		comments = []db.InventoryComment{}
	}

	// Fetch WebDAV files (gracefully handle errors)
	var files []db.WebDAVFile
	webdavFiles, err := db.ListInventoryFiles(ctx, inventoryID)
	if err != nil {
		log.Printf("Warning: could not list WebDAV files for %s: %v", inventoryID, err)
		files = []db.WebDAVFile{} // Empty list if WebDAV not configured/available
	} else {
		files = webdavFiles
	}

	data["Item"] = item
	data["Comments"] = comments
	data["Files"] = files
	data["IsInventory"] = true
	t.HTML(http.StatusOK, "inventory_view")
}

// NewInventoryItemForm renders the create inventory item form
func NewInventoryItemForm(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch distinct locations for autocomplete
	locations, err := db.GetDistinctLocations(ctx)
	if err != nil {
		log.Printf("Error fetching locations: %v", err)
		locations = []string{} // Continue with empty list
	}

	data["Locations"] = locations
	data["IsInventory"] = true
	t.HTML(http.StatusOK, "inventory_new")
}

// CreateInventoryItem handles inventory item creation
func CreateInventoryItem(c flamego.Context, t template.Template, data template.Data) {
	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		data["Error"] = "Failed to parse form data"
		t.HTML(http.StatusBadRequest, "inventory_new")
		return
	}

	form := c.Request().Form

	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		data["Error"] = "Name is required"
		data["FormData"] = form
		// Re-fetch locations for the form
		ctx := c.Request().Context()
		locations, _ := db.GetDistinctLocations(ctx)
		data["Locations"] = locations
		data["IsInventory"] = true
		t.HTML(http.StatusBadRequest, "inventory_new")
		return
	}

	location := getOptionalString(form.Get("location"))
	description := getOptionalString(form.Get("description"))

	ctx := c.Request().Context()

	// Create item
	inventoryID, err := db.CreateInventoryItem(ctx, name, location, description)
	if err != nil {
		log.Printf("Error creating inventory item: %v", err)
		data["Error"] = "Failed to create inventory item"
		data["FormData"] = form
		// Re-fetch locations for the form
		locations, _ := db.GetDistinctLocations(ctx)
		data["Locations"] = locations
		data["IsInventory"] = true
		t.HTML(http.StatusInternalServerError, "inventory_new")
		return
	}

	// Redirect to view page
	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// EditInventoryItemForm renders the edit form
func EditInventoryItemForm(c flamego.Context, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	ctx := c.Request().Context()

	item, err := db.GetInventoryItem(ctx, inventoryID)
	if err != nil {
		log.Printf("Error fetching inventory item: %v", err)
		data["Error"] = "Inventory item not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	// Fetch locations for autocomplete
	locations, err := db.GetDistinctLocations(ctx)
	if err != nil {
		log.Printf("Error fetching locations: %v", err)
		locations = []string{}
	}

	data["Item"] = item
	data["Locations"] = locations
	data["IsInventory"] = true
	t.HTML(http.StatusOK, "inventory_edit")
}

// UpdateInventoryItem handles inventory item updates
func UpdateInventoryItem(c flamego.Context, t template.Template, data template.Data) {
	inventoryID := c.Param("id")

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		data["Error"] = "Name is required"
		c.Redirect("/inventory/"+inventoryID+"/edit", http.StatusSeeOther)
		return
	}

	location := getOptionalString(form.Get("location"))
	description := getOptionalString(form.Get("description"))

	ctx := c.Request().Context()

	if err := db.UpdateInventoryItem(ctx, inventoryID, name, location, description); err != nil {
		log.Printf("Error updating inventory item: %v", err)
		data["Error"] = "Failed to update inventory item"
		c.Redirect("/inventory/"+inventoryID+"/edit", http.StatusSeeOther)
		return
	}

	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// DeleteInventoryItem handles inventory item deletion
func DeleteInventoryItem(c flamego.Context, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	ctx := c.Request().Context()

	if err := db.DeleteInventoryItem(ctx, inventoryID); err != nil {
		log.Printf("Error deleting inventory item: %v", err)
		data["Error"] = "Failed to delete inventory item"
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	c.Redirect("/inventory", http.StatusSeeOther)
}

// AddInventoryComment handles adding a comment
func AddInventoryComment(c flamego.Context, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	ctx := c.Request().Context()

	// Get item to retrieve numeric ID
	item, err := db.GetInventoryItem(ctx, inventoryID)
	if err != nil {
		log.Printf("Error fetching inventory item: %v", err)
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	content := strings.TrimSpace(c.Request().Form.Get("content"))
	if content == "" {
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	if err := db.CreateInventoryComment(ctx, item.ID, content); err != nil {
		log.Printf("Error creating comment: %v", err)
		data["Error"] = "Failed to add comment"
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// DeleteInventoryComment handles comment deletion
func DeleteInventoryComment(c flamego.Context, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	commentIDStr := c.Param("comment_id")

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		log.Printf("Invalid comment ID: %v", err)
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	ctx := c.Request().Context()

	if err := db.DeleteInventoryComment(ctx, commentID); err != nil {
		log.Printf("Error deleting comment: %v", err)
		data["Error"] = "Failed to delete comment"
		t.HTML(http.StatusInternalServerError, "error")
		return
	}

	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// DownloadInventoryFile proxies a file download from WebDAV
func DownloadInventoryFile(c flamego.Context, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	filename := c.Param("filename")

	ctx := c.Request().Context()

	// Fetch file from WebDAV
	fileData, contentType, err := db.FetchInventoryFile(ctx, inventoryID, filename)
	if err != nil {
		log.Printf("Error fetching file: %v", err)
		data["Error"] = "File not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	// Set headers and serve file
	c.ResponseWriter().Header().Set("Content-Type", contentType)
	c.ResponseWriter().Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")
	c.ResponseWriter().Header().Set("Content-Length", strconv.Itoa(len(fileData)))

	c.ResponseWriter().WriteHeader(http.StatusOK)
	c.ResponseWriter().Write(fileData)
}

// Helper function
func getOptionalString(val string) *string {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
