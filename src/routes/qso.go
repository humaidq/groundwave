/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
	"github.com/humaidq/groundwave/utils"
)

// QSL renders the QSL contacts list
func QSL(c flamego.Context, t template.Template, data template.Data) {
	// Fetch QSOs from database
	qsos, err := db.ListQSOs(c.Request().Context())
	if err != nil {
		logger.Error("Error fetching QSOs", "error", err)
		data["Error"] = "Failed to load QSOs"
	} else {
		data["QSOs"] = qsos
	}

	data["IsQSL"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "QSL", URL: "/qsl", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "qsl")
}

// ViewQSO renders the QSO detail page
func ViewQSO(c flamego.Context, t template.Template, data template.Data) {
	qsoID := c.Param("id")
	if qsoID == "" {
		c.Redirect("/qsl", http.StatusSeeOther)
		return
	}

	// Fetch QSO details
	qso, err := db.GetQSO(c.Request().Context(), qsoID)
	if err != nil {
		logger.Error("Error fetching QSO", "error", err)
		data["Error"] = "Failed to load QSO details"
		c.Redirect("/qsl", http.StatusSeeOther)
		return
	}

	// Fetch all QSOs with the same call sign
	allQSOs, err := db.GetQSOsByCallSign(c.Request().Context(), qso.Call)
	if err != nil {
		logger.Error("Error fetching all QSOs for call sign", "call_sign", qso.Call, "error", err)
	} else {
		data["AllQSOs"] = allQSOs
	}

	// Generate map if both grid squares are available
	mapURL := ""
	if qso.MyGridSquare != nil && *qso.MyGridSquare != "" && qso.GridSquare != nil && *qso.GridSquare != "" {
		safeCallsign := strings.ReplaceAll(qso.Call, "/", "_")
		mapFileName := fmt.Sprintf("%s-%s.png", safeCallsign, qsoID)
		encodedCallsign := url.QueryEscape(qso.Call)
		mapURL = fmt.Sprintf("/%s-%s.png", encodedCallsign, qsoID)

		// Generate map in background if it doesn't exist
		go generateMapIfNeeded(mapFileName, *qso.MyGridSquare, *qso.GridSquare)
	}

	data["QSO"] = qso
	data["MapURL"] = mapURL
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "QSL", URL: "/qsl", IsCurrent: false},
		{Name: qso.Call, URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "qso_view")
}

// generateMapIfNeeded creates a map if it doesn't already exist
func generateMapIfNeeded(fileName, myGrid, theirGrid string) {
	mapPath := filepath.Join("maps", fileName)

	// Check if map already exists
	if _, err := os.Stat(mapPath); err == nil {
		return
	}

	// Generate the map
	config := utils.MapConfig{
		Width:      600,
		Height:     400,
		Zoom:       0, // Will be auto-calculated
		OutputPath: mapPath,
	}

	if err := utils.CreateGridMap(myGrid, theirGrid, config); err != nil {
		logger.Error("Failed to generate map", "file", fileName, "error", err)
	}
}

// ImportADIF handles ADIF file upload and import
func ImportADIF(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	// Parse multipart form (max 10MB)
	err := c.Request().ParseMultipartForm(10 << 20)
	if err != nil {
		logger.Error("Error parsing form", "error", err)
		data["Error"] = "Failed to parse upload form"
		data["IsQSL"] = true

		// Fetch QSOs to show list again
		qsos, _ := db.ListQSOs(c.Request().Context())
		data["QSOs"] = qsos
		t.HTML(http.StatusBadRequest, "qsl")
		return
	}

	// Get file from form
	file, header, err := c.Request().FormFile("adif_file")
	if err != nil {
		logger.Error("Error getting file", "error", err)
		data["Error"] = "No file uploaded or invalid file"
		data["IsQSL"] = true

		// Fetch QSOs to show list again
		qsos, _ := db.ListQSOs(c.Request().Context())
		data["QSOs"] = qsos
		t.HTML(http.StatusBadRequest, "qsl")
		return
	}
	defer file.Close()

	logger.Info("Uploading file", "filename", header.Filename, "bytes", header.Size)

	// Parse ADIF file
	parser := utils.NewADIFParser()
	err = parser.ParseFile(file)
	if err != nil {
		logger.Error("Error parsing ADIF file", "error", err)
		data["Error"] = "Failed to parse ADIF file: " + err.Error()
		data["IsQSL"] = true

		// Fetch QSOs to show list again
		qsos, _ := db.ListQSOs(c.Request().Context())
		data["QSOs"] = qsos
		t.HTML(http.StatusBadRequest, "qsl")
		return
	}

	logger.Info("Parsed QSOs from ADIF file", "count", len(parser.QSOs))

	// Import/merge QSOs into database
	processed, err := db.ImportADIFQSOs(c.Request().Context(), parser.QSOs)
	if err != nil {
		logger.Error("Error importing QSOs", "error", err)
		data["Error"] = "Failed to import QSOs: " + err.Error()
		data["IsQSL"] = true

		// Fetch QSOs to show list again
		qsos, _ := db.ListQSOs(c.Request().Context())
		data["QSOs"] = qsos
		t.HTML(http.StatusInternalServerError, "qsl")
		return
	}

	skipped := len(parser.QSOs) - processed
	logger.Info("Successfully processed QSOs from ADIF file", "processed", processed, "skipped", skipped)

	// Redirect to QSL page with success message
	if skipped > 0 {
		SetSuccessFlash(s, fmt.Sprintf("Successfully imported %d QSOs (%d skipped)", processed, skipped))
	} else {
		SetSuccessFlash(s, fmt.Sprintf("Successfully imported %d QSOs", processed))
	}
	c.Redirect("/qsl", http.StatusSeeOther)
}
