/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
	"github.com/humaidq/groundwave/utils"
)

func populateQSLPageData(ctx context.Context, data template.Data) {
	requests, err := db.ListOpenQSLCardRequests(ctx)
	if err != nil {
		logger.Error("Error fetching QSL card requests", "error", err)

		data["QSLCardRequestsError"] = "Failed to load pending card requests"
	} else {
		data["QSLCardRequests"] = requests
	}

	qsos, err := db.ListQSOs(ctx)
	if err != nil {
		logger.Error("Error fetching QSOs", "error", err)

		if _, exists := data["Error"]; !exists {
			data["Error"] = "Failed to load QSOs"
		}
	} else {
		data["QSOs"] = qsos
	}

	data["IsQSL"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "QSL", URL: "/qsl", IsCurrent: true},
	}
}

// QSL renders the QSL contacts list
func QSL(c flamego.Context, t template.Template, data template.Data) {
	populateQSLPageData(c.Request().Context(), data)
	t.HTML(http.StatusOK, "qsl")
}

// DismissQSLCardRequest hides a request card from the /qsl inbox.
func DismissQSLCardRequest(c flamego.Context, s session.Session) {
	requestID := strings.TrimSpace(c.Param("id"))
	if requestID == "" {
		SetErrorFlash(s, "Invalid QSL card request")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	if err := db.DismissQSLCardRequest(c.Request().Context(), requestID); err != nil {
		logger.Error("Error dismissing QSL card request", "request_id", requestID, "error", err)
		SetErrorFlash(s, "Failed to dismiss QSL card request")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "QSL card request dismissed")
	c.Redirect("/qsl", http.StatusSeeOther)
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

		generateMapIfNeeded(mapFileName, *qso.MyGridSquare, *qso.GridSquare)
	}

	data["QSO"] = qso
	data["MapURL"] = mapURL

	qsoTimestamp := qsoTimestampUTC(qso.QSO)
	if !qsoTimestamp.IsZero() {
		encodedCallsign := url.QueryEscape(qso.Call)
		data["PublicQSLPath"] = fmt.Sprintf("/oqrs/%s-%d", encodedCallsign, qsoTimestamp.Unix())
	}

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

// ExportADIF exports QSOs as an ADIF file.
func ExportADIF(c flamego.Context, s session.Session) {
	fromDate, hasFromDate, err := parseADIFExportDate(c.Query("from"))
	if err != nil {
		SetErrorFlash(s, "Invalid export date, use YYYY-MM-DD")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	toDate, hasToDate, err := parseADIFExportDate(c.Query("to"))
	if err != nil {
		SetErrorFlash(s, "Invalid export date, use YYYY-MM-DD")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	var fromDatePtr *time.Time
	if hasFromDate {
		fromDatePtr = &fromDate
	}

	var toDatePtr *time.Time
	if hasToDate {
		toDatePtr = &toDate
	}

	if fromDatePtr != nil && toDatePtr != nil && fromDatePtr.After(*toDatePtr) {
		SetErrorFlash(s, "Export date range is invalid")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	adif, err := db.ExportADIF(c.Request().Context(), fromDatePtr, toDatePtr)
	if err != nil {
		logger.Error("Error exporting ADIF", "error", err)
		SetErrorFlash(s, "Failed to export ADIF")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	filename := buildADIFFilename(fromDatePtr, toDatePtr)

	c.ResponseWriter().Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.ResponseWriter().Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	c.ResponseWriter().Header().Set("Content-Length", strconv.Itoa(len(adif)))
	c.ResponseWriter().WriteHeader(http.StatusOK)

	if _, err := c.ResponseWriter().Write([]byte(adif)); err != nil {
		logger.Error("Error writing ADIF export response", "error", err)
	}
}

func parseADIFExportDate(raw string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false, nil
	}

	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%w: %s", errInvalidADIFExportDate, trimmed)
	}

	return parsed, true, nil
}

func buildADIFFilename(fromDate *time.Time, toDate *time.Time) string {
	if fromDate == nil && toDate == nil {
		return "qsos-" + time.Now().UTC().Format("20060102-150405") + ".adi"
	}

	fromPart := "all"
	if fromDate != nil {
		fromPart = fromDate.Format("20060102")
	}

	toPart := "all"
	if toDate != nil {
		toPart = toDate.Format("20060102")
	}

	return fmt.Sprintf("qsos-%s-%s.adi", fromPart, toPart)
}

// ImportADIF handles ADIF file upload and import
func ImportADIF(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	// Parse multipart form (max 10MB)
	err := c.Request().ParseMultipartForm(10 << 20)
	if err != nil {
		logger.Error("Error parsing form", "error", err)

		data["Error"] = "Failed to parse upload form"
		populateQSLPageData(c.Request().Context(), data)
		t.HTML(http.StatusBadRequest, "qsl")

		return
	}

	// Get file from form
	file, header, err := c.Request().FormFile("adif_file")
	if err != nil {
		logger.Error("Error getting file", "error", err)

		data["Error"] = "No file uploaded or invalid file"
		populateQSLPageData(c.Request().Context(), data)
		t.HTML(http.StatusBadRequest, "qsl")

		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			logger.Error("Error closing ADIF upload file", "error", err)
		}
	}()

	logger.Info("Uploading file", "filename", header.Filename, "bytes", header.Size)

	// Parse ADIF file
	parser := utils.NewADIFParser()

	err = parser.ParseFile(file)
	if err != nil {
		logger.Error("Error parsing ADIF file", "error", err)
		data["Error"] = "Failed to parse ADIF file: " + err.Error()
		populateQSLPageData(c.Request().Context(), data)
		t.HTML(http.StatusBadRequest, "qsl")

		return
	}

	logger.Info("Parsed QSOs from ADIF file", "count", len(parser.QSOs))

	// Import/merge QSOs into database
	processed, err := db.ImportADIFQSOs(c.Request().Context(), parser.QSOs)
	if err != nil {
		logger.Error("Error importing QSOs", "error", err)
		data["Error"] = "Failed to import QSOs: " + err.Error()
		populateQSLPageData(c.Request().Context(), data)
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

// ImportQRZLogs fetches the latest QRZ logbook entries and imports them.
func ImportQRZLogs(c flamego.Context, s session.Session) {
	apiKeys := splitEnvList(os.Getenv("QRZ_API_KEY"))
	if len(apiKeys) == 0 {
		SetErrorFlash(s, "QRZ_API_KEY is not configured")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	result, err := db.SyncQRZLogbooks(c.Request().Context(), apiKeys)
	if err != nil {
		logger.Error("Error syncing QRZ logs", "error", err)
		SetErrorFlash(s, "Failed to sync QRZ logs")
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	if result.FailedLogbooks > 0 {
		SetWarningFlash(s, fmt.Sprintf("Synced %d QSOs from %d QRZ logbook(s); %d failed", result.ProcessedQSOs, result.SyncedLogbooks, result.FailedLogbooks))
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	if result.ProcessedQSOs == 0 {
		SetInfoFlash(s, fmt.Sprintf("QRZ sync complete. No new QSOs found across %d logbook(s)", result.SyncedLogbooks))
		c.Redirect("/qsl", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, fmt.Sprintf("Synced %d QSOs from %d QRZ logbook(s)", result.ProcessedQSOs, result.SyncedLogbooks))
	c.Redirect("/qsl", http.StatusSeeOther)
}
