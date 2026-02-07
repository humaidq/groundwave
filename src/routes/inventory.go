/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

// InventoryList renders the inventory list page
func InventoryList(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Get status filter from query parameter
	statusFilter := c.Query("status")
	var items []db.InventoryItem
	var err error

	if statusFilter != "" {
		items, err = db.ListInventoryItems(ctx, db.InventoryStatus(statusFilter))
	} else {
		items, err = db.ListInventoryItems(ctx)
	}

	if err != nil {
		logger.Error("Error fetching inventory items", "error", err)
		SetErrorFlash(s, "Failed to load inventory items")
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	data["Items"] = items
	data["IsInventory"] = true
	data["StatusFilter"] = statusFilter
	data["StatusOptions"] = getInventoryStatusOptions()
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Inventory", URL: "/inventory", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "inventory")
}

// ViewInventoryItem renders a single inventory item with comments and files
func ViewInventoryItem(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	if inventoryID == "" {
		SetErrorFlash(s, "Inventory ID is required")
		c.Redirect("/inventory", http.StatusSeeOther)
		return
	}

	ctx := c.Request().Context()
	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil {
		logger.Error("Error resolving admin state", "error", err)
		isAdmin = false
	}
	data["IsAdmin"] = isAdmin

	// Fetch item
	item, err := db.GetInventoryItem(ctx, inventoryID)
	if err != nil {
		logger.Error("Error fetching inventory item", "inventory_id", inventoryID, "error", err)
		SetErrorFlash(s, "Inventory item not found")
		c.Redirect("/inventory", http.StatusSeeOther)
		return
	}

	// Fetch comments (admin only)
	comments := []db.InventoryComment{}
	if isAdmin {
		comments, err = db.GetCommentsForItem(ctx, item.ID)
		if err != nil {
			logger.Error("Error fetching comments for item", "inventory_id", inventoryID, "error", err)
			// Don't fail the page, just show empty comments
			comments = []db.InventoryComment{}
		}
	}

	// Fetch WebDAV files (gracefully handle errors)
	var files []db.WebDAVFile
	webdavFiles, err := db.ListInventoryFiles(ctx, inventoryID)
	if err != nil {
		logger.Warn("Could not list WebDAV files", "inventory_id", inventoryID, "error", err)
		files = []db.WebDAVFile{} // Empty list if WebDAV not configured/available
	} else {
		files = webdavFiles
	}

	data["Item"] = item
	data["Comments"] = comments
	data["Files"] = files
	data["IsInventory"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Inventory", URL: "/inventory", IsCurrent: false},
		{Name: item.Name, URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "inventory_view")
}

// InventoryStatusOption represents a status option for templates
type InventoryStatusOption struct {
	Value string
	Label string
}

// getInventoryStatusOptions returns all available inventory status options
func getInventoryStatusOptions() []InventoryStatusOption {
	return []InventoryStatusOption{
		{Value: string(db.InventoryStatusActive), Label: db.InventoryStatusLabel(db.InventoryStatusActive)},
		{Value: string(db.InventoryStatusStored), Label: db.InventoryStatusLabel(db.InventoryStatusStored)},
		{Value: string(db.InventoryStatusDamaged), Label: db.InventoryStatusLabel(db.InventoryStatusDamaged)},
		{Value: string(db.InventoryStatusMaintenanceRequired), Label: db.InventoryStatusLabel(db.InventoryStatusMaintenanceRequired)},
		{Value: string(db.InventoryStatusGiven), Label: db.InventoryStatusLabel(db.InventoryStatusGiven)},
		{Value: string(db.InventoryStatusDisposed), Label: db.InventoryStatusLabel(db.InventoryStatusDisposed)},
		{Value: string(db.InventoryStatusLost), Label: db.InventoryStatusLabel(db.InventoryStatusLost)},
	}
}

// NewInventoryItemForm renders the create inventory item form
func NewInventoryItemForm(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	// Fetch distinct locations for autocomplete
	locations, err := db.GetDistinctLocations(ctx)
	if err != nil {
		logger.Error("Error fetching locations", "error", err)
		locations = []string{} // Continue with empty list
	}

	data["Locations"] = locations
	data["StatusOptions"] = getInventoryStatusOptions()
	data["IsInventory"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Inventory", URL: "/inventory", IsCurrent: false},
		{Name: "New Item", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "inventory_new")
}

// CreateInventoryItem handles inventory item creation
func CreateInventoryItem(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/inventory/new", http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		SetErrorFlash(s, "Name is required")
		c.Redirect("/inventory/new", http.StatusSeeOther)
		return
	}

	location := getOptionalString(form.Get("location"))
	description := getOptionalString(form.Get("description"))
	status := db.InventoryStatus(form.Get("status"))
	if status == "" {
		status = db.InventoryStatusActive
	} else if !isValidInventoryStatus(status) {
		SetErrorFlash(s, "Invalid status value")
		c.Redirect("/inventory/new", http.StatusSeeOther)
		return
	}

	var inspectionDate *time.Time
	inspectionDateStr := strings.TrimSpace(form.Get("inspection_date"))
	if inspectionDateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", inspectionDateStr)
		if err != nil {
			SetErrorFlash(s, "Invalid inspection date")
			c.Redirect("/inventory/new", http.StatusSeeOther)
			return
		}
		inspectionDate = &parsedDate
	}

	ctx := c.Request().Context()

	// Create item
	inventoryID, err := db.CreateInventoryItem(ctx, name, location, description, status, inspectionDate)
	if err != nil {
		logger.Error("Error creating inventory item", "error", err)
		SetErrorFlash(s, "Failed to create inventory item")
		c.Redirect("/inventory/new", http.StatusSeeOther)
		return
	}

	// Redirect to view page with success message
	SetSuccessFlash(s, "Inventory item created successfully")
	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// EditInventoryItemForm renders the edit form
func EditInventoryItemForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	ctx := c.Request().Context()

	item, err := db.GetInventoryItem(ctx, inventoryID)
	if err != nil {
		logger.Error("Error fetching inventory item", "error", err)
		SetErrorFlash(s, "Inventory item not found")
		c.Redirect("/inventory", http.StatusSeeOther)
		return
	}

	// Fetch locations for autocomplete
	locations, err := db.GetDistinctLocations(ctx)
	if err != nil {
		logger.Error("Error fetching locations", "error", err)
		locations = []string{}
	}

	data["Item"] = item
	data["Locations"] = locations
	data["StatusOptions"] = getInventoryStatusOptions()
	data["IsInventory"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Inventory", URL: "/inventory", IsCurrent: false},
		{Name: item.Name, URL: "/inventory/" + inventoryID, IsCurrent: false},
		{Name: "Edit", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "inventory_edit")
}

// UpdateInventoryItem handles inventory item updates
func UpdateInventoryItem(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	inventoryID := c.Param("id")

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/inventory/"+inventoryID+"/edit", http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		SetErrorFlash(s, "Name is required")
		c.Redirect("/inventory/"+inventoryID+"/edit", http.StatusSeeOther)
		return
	}

	location := getOptionalString(form.Get("location"))
	description := getOptionalString(form.Get("description"))
	status := db.InventoryStatus(form.Get("status"))

	// Validate status if provided
	if status != "" && !isValidInventoryStatus(status) {
		SetErrorFlash(s, "Invalid status value")
		c.Redirect("/inventory/"+inventoryID+"/edit", http.StatusSeeOther)
		return
	}

	var inspectionDate *time.Time
	inspectionDateStr := strings.TrimSpace(form.Get("inspection_date"))
	if inspectionDateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", inspectionDateStr)
		if err != nil {
			SetErrorFlash(s, "Invalid inspection date")
			c.Redirect("/inventory/"+inventoryID+"/edit", http.StatusSeeOther)
			return
		}
		inspectionDate = &parsedDate
	}

	ctx := c.Request().Context()

	if err := db.UpdateInventoryItem(ctx, inventoryID, name, location, description, status, inspectionDate); err != nil {
		logger.Error("Error updating inventory item", "error", err)
		SetErrorFlash(s, "Failed to update inventory item")
		c.Redirect("/inventory/"+inventoryID+"/edit", http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Inventory item updated successfully")
	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// DeleteInventoryItem handles inventory item deletion
func DeleteInventoryItem(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	ctx := c.Request().Context()

	if err := db.DeleteInventoryItem(ctx, inventoryID); err != nil {
		logger.Error("Error deleting inventory item", "error", err)
		SetErrorFlash(s, "Failed to delete inventory item")
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Inventory item deleted successfully")
	c.Redirect("/inventory", http.StatusSeeOther)
}

// AddInventoryComment handles adding a comment
func AddInventoryComment(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	ctx := c.Request().Context()

	// Get item to retrieve numeric ID
	item, err := db.GetInventoryItem(ctx, inventoryID)
	if err != nil {
		logger.Error("Error fetching inventory item", "error", err)
		SetErrorFlash(s, "Inventory item not found")
		c.Redirect("/inventory", http.StatusSeeOther)
		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing form", "error", err)
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	content := strings.TrimSpace(c.Request().Form.Get("content"))
	if content == "" {
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	if err := db.CreateInventoryComment(ctx, item.ID, content); err != nil {
		logger.Error("Error creating comment", "error", err)
		SetErrorFlash(s, "Failed to add comment")
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Comment added successfully")
	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// DeleteInventoryComment handles comment deletion
func DeleteInventoryComment(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	commentIDStr := c.Param("comment_id")

	commentID, err := uuid.Parse(commentIDStr)
	if err != nil {
		logger.Warn("Invalid comment ID", "error", err)
		SetErrorFlash(s, "Invalid comment ID")
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	ctx := c.Request().Context()

	if err := db.DeleteInventoryComment(ctx, commentID); err != nil {
		logger.Error("Error deleting comment", "error", err)
		SetErrorFlash(s, "Failed to delete comment")
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Comment deleted successfully")
	c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
}

// DownloadInventoryFile proxies a file download from WebDAV
func DownloadInventoryFile(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	inventoryID := c.Param("id")
	filename := c.Param("filename")

	// Validate filename to prevent path traversal attacks
	if !isValidFilename(filename) {
		SetErrorFlash(s, "Invalid filename")
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	ctx := c.Request().Context()

	// Fetch file from WebDAV
	fileData, contentType, err := db.FetchInventoryFile(ctx, inventoryID, filename)
	if err != nil {
		logger.Error("Error fetching file", "error", err)
		SetErrorFlash(s, "File not found")
		c.Redirect("/inventory/"+inventoryID, http.StatusSeeOther)
		return
	}

	// Set headers and serve file
	c.ResponseWriter().Header().Set("Content-Type", contentType)
	c.ResponseWriter().Header().Set("Content-Disposition", "inline; filename=\""+sanitizeFilenameForHeader(filename)+"\"")
	c.ResponseWriter().Header().Set("Content-Length", strconv.Itoa(len(fileData)))

	c.ResponseWriter().WriteHeader(http.StatusOK)
	if _, err := c.ResponseWriter().Write(fileData); err != nil {
		logger.Error("Error writing inventory file", "error", err)
	}
}

// Helper function
func getOptionalString(val string) *string {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// isValidFilename checks for path traversal attempts in filenames
func isValidFilename(filename string) bool {
	return !strings.Contains(filename, "..") &&
		!strings.Contains(filename, "/") &&
		!strings.Contains(filename, "\\")
}

// isValidInventoryStatus validates inventory status values
func isValidInventoryStatus(s db.InventoryStatus) bool {
	switch s {
	case db.InventoryStatusActive, db.InventoryStatusStored,
		db.InventoryStatusDamaged, db.InventoryStatusMaintenanceRequired,
		db.InventoryStatusGiven, db.InventoryStatusDisposed,
		db.InventoryStatusLost:
		return true
	}
	return false
}

// sanitizeFilenameForHeader escapes special chars for Content-Disposition header
func sanitizeFilenameForHeader(filename string) string {
	s := strings.ReplaceAll(filename, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}
