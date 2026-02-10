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

const (
	oqrsToleranceMinutes = 10
	oqrsLatestLimit      = 30
)

// OQRSIndex renders the public OQRS search page.
func OQRSIndex(c flamego.Context, t template.Template, data template.Data) {
	populateOQRSData(c.Request().Context(), data)
	data["HideNav"] = true
	data["OQRSDateOnly"] = true
	t.HTML(http.StatusOK, "oqrs")
}

// OQRSFind handles public OQRS search submissions.
func OQRSFind(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Failed to parse OQRS form", "error", err)
		data["Error"] = "Failed to parse form"
		populateOQRSData(ctx, data)
		data["HideNav"] = true
		data["OQRSDateOnly"] = true
		t.HTML(http.StatusBadRequest, "oqrs")
		return
	}

	callsign := strings.ToUpper(strings.TrimSpace(c.Request().FormValue("callsign")))
	yearStr := strings.TrimSpace(c.Request().FormValue("year"))
	monthStr := strings.TrimSpace(c.Request().FormValue("month"))
	dayStr := strings.TrimSpace(c.Request().FormValue("day"))
	hourStr := strings.TrimSpace(c.Request().FormValue("hour"))
	minuteStr := strings.TrimSpace(c.Request().FormValue("minute"))

	if callsign == "" {
		data["Error"] = "Call sign is required"
		populateOQRSData(ctx, data)
		data["HideNav"] = true
		data["OQRSDateOnly"] = true
		t.HTML(http.StatusBadRequest, "oqrs")
		return
	}

	if yearStr == "" || monthStr == "" || dayStr == "" || hourStr == "" || minuteStr == "" {
		data["Error"] = "All date and time fields are required"
		populateOQRSData(ctx, data)
		data["HideNav"] = true
		data["OQRSDateOnly"] = true
		t.HTML(http.StatusBadRequest, "oqrs")
		return
	}

	searchTime, err := parseOQRSTime(yearStr, monthStr, dayStr, hourStr, minuteStr)
	if err != nil {
		data["Error"] = "Invalid date and time values"
		populateOQRSData(ctx, data)
		data["HideNav"] = true
		data["OQRSDateOnly"] = true
		t.HTML(http.StatusBadRequest, "oqrs")
		return
	}

	qso, err := db.FindClosestQSOByCallAndTime(ctx, callsign, searchTime, oqrsToleranceMinutes)
	if err != nil {
		logger.Error("Failed to search OQRS", "callsign", callsign, "error", err)
		data["Error"] = "Failed to search QSO logs"
		populateOQRSData(ctx, data)
		data["HideNav"] = true
		data["OQRSDateOnly"] = true
		t.HTML(http.StatusInternalServerError, "oqrs")
		return
	}

	if qso == nil {
		data["Error"] = fmt.Sprintf("No QSO found for %s around %s UTC", callsign, searchTime.Format("2006-01-02 15:04"))
		populateOQRSData(ctx, data)
		data["HideNav"] = true
		data["OQRSDateOnly"] = true
		t.HTML(http.StatusOK, "oqrs")
		return
	}

	qsoTimestamp := qsoTimestampUTC(qso.QSO)
	encodedCallsign := url.QueryEscape(qso.Call)
	redirectURL := fmt.Sprintf("/oqrs/%s-%d", encodedCallsign, qsoTimestamp.Unix())
	c.Redirect(redirectURL, http.StatusFound)
}

// OQRSView renders the public QSL result page and serves map images.
func OQRSView(c flamego.Context, t template.Template, data template.Data, w http.ResponseWriter) {
	path := c.Param("path")
	if path == "" {
		path = c.Param("id")
	}
	if path == "" {
		c.Redirect("/oqrs", http.StatusFound)
		return
	}

	if strings.HasSuffix(path, ".png") {
		serveOQRSMap(c, w, strings.TrimSuffix(path, ".png"))
		return
	}

	callsign, timestampStr, timestamp, ok := parseOQRSPath(path)
	if !ok {
		c.Redirect("/oqrs", http.StatusFound)
		return
	}

	searchTime := time.Unix(timestamp, 0).UTC()
	qso, err := db.FindClosestQSOByCallAndTime(c.Request().Context(), callsign, searchTime, oqrsToleranceMinutes)
	if err != nil {
		logger.Error("Failed to load OQRS result", "callsign", callsign, "error", err)
		c.Redirect("/oqrs", http.StatusFound)
		return
	}
	if qso == nil {
		c.Redirect("/oqrs", http.StatusFound)
		return
	}

	hasOpenCardRequest, err := db.HasOpenQSLCardRequestForQSO(c.Request().Context(), qso.ID.String())
	if err != nil {
		logger.Error("Failed to load open card request state", "qso_id", qso.ID.String(), "error", err)
		hasOpenCardRequest = false
	}

	allQSOs, err := db.GetQSOsByCallSign(c.Request().Context(), callsign)
	if err != nil {
		logger.Error("Failed to load OQRS call history", "callsign", callsign, "error", err)
	} else {
		data["AllQSOs"] = allQSOs
	}

	mapURL := ""
	if qso.MyGridSquare != nil && qso.GridSquare != nil && *qso.MyGridSquare != "" && *qso.GridSquare != "" {
		mapURL = fmt.Sprintf("/oqrs/%s-%s.png", url.QueryEscape(callsign), timestampStr)
	}

	data["QSO"] = qso
	data["MapURL"] = mapURL
	data["OQRSPath"] = path
	data["HasOpenCardRequest"] = hasOpenCardRequest
	data["HideNav"] = true
	t.HTML(http.StatusOK, "oqrs_result")
}

// OQRSRequestCard stores a public physical QSL card request.
func OQRSRequestCard(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Failed to parse OQRS card request form", "error", err)
		SetErrorFlash(s, "Failed to submit QSL card request")
		c.Redirect("/oqrs", http.StatusSeeOther)
		return
	}

	path := strings.TrimSpace(c.Request().FormValue("path"))
	if path == "" {
		SetErrorFlash(s, "Missing QSO reference for QSL request")
		c.Redirect("/oqrs", http.StatusSeeOther)
		return
	}

	if strings.HasPrefix(path, "/oqrs/") {
		path = strings.TrimPrefix(path, "/oqrs/")
	}

	callsign, _, timestamp, ok := parseOQRSPath(path)
	if !ok {
		SetErrorFlash(s, "Invalid QSO reference for QSL request")
		c.Redirect("/oqrs", http.StatusSeeOther)
		return
	}

	searchTime := time.Unix(timestamp, 0).UTC()
	qso, err := db.FindClosestQSOByCallAndTime(c.Request().Context(), callsign, searchTime, oqrsToleranceMinutes)
	if err != nil {
		logger.Error("Failed to resolve QSO for card request", "callsign", callsign, "error", err)
		SetErrorFlash(s, "Failed to submit QSL card request")
		c.Redirect("/oqrs/"+path, http.StatusSeeOther)
		return
	}
	if qso == nil {
		SetErrorFlash(s, "QSO not found for card request")
		c.Redirect("/oqrs", http.StatusSeeOther)
		return
	}

	hasOpenCardRequest, err := db.HasOpenQSLCardRequestForQSO(c.Request().Context(), qso.ID.String())
	if err != nil {
		logger.Error("Failed checking for existing qsl card request", "qso_id", qso.ID.String(), "error", err)
		SetErrorFlash(s, "Failed to submit QSL card request")
		c.Redirect("/oqrs/"+path, http.StatusSeeOther)
		return
	}
	if hasOpenCardRequest {
		SetInfoFlash(s, "Physical QSL card request already received")
		c.Redirect("/oqrs/"+path, http.StatusSeeOther)
		return
	}

	mailingAddress := strings.TrimSpace(c.Request().FormValue("mailing_address"))
	if mailingAddress == "" {
		SetErrorFlash(s, "Mailing address is required")
		c.Redirect("/oqrs/"+path, http.StatusSeeOther)
		return
	}

	err = db.CreateQSLCardRequest(c.Request().Context(), db.CreateQSLCardRequestInput{
		QSOID:          qso.ID.String(),
		RequesterName:  strings.TrimSpace(c.Request().FormValue("requester_name")),
		MailingAddress: mailingAddress,
		Note:           strings.TrimSpace(c.Request().FormValue("note")),
	})
	if err != nil {
		logger.Error("Failed to create qsl card request", "qso_id", qso.ID.String(), "error", err)
		SetErrorFlash(s, "Failed to submit QSL card request")
		c.Redirect("/oqrs/"+path, http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, "Physical QSL card requested successfully")
	c.Redirect("/oqrs/"+path, http.StatusSeeOther)
}

// QRZ renders the public QRZ iframe page.
func QRZ(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	latestQSOs, err := db.ListRecentQSOs(ctx, oqrsLatestLimit)
	if err != nil {
		logger.Error("Failed to load QRZ latest QSOs", "error", err)
	} else {
		data["LatestQSOs"] = latestQSOs
	}

	hallOfFame, err := db.GetPaperQSLHallOfFame(ctx)
	if err != nil {
		logger.Error("Failed to load QRZ hall of fame", "error", err)
	} else {
		data["PaperQSLHallOfFame"] = hallOfFame
	}

	data["HideNav"] = true
	t.HTML(http.StatusOK, "qrz")
}

func populateOQRSData(ctx context.Context, data template.Data) {
	totalQSOs, err := db.GetQSOCount(ctx)
	if err != nil {
		logger.Error("Failed to load QSO count", "error", err)
		totalQSOs = 0
	}
	data["TotalQSOs"] = totalQSOs

	uniqueCountries, err := db.GetUniqueCountriesCount(ctx)
	if err != nil {
		logger.Error("Failed to load unique countries", "error", err)
		uniqueCountries = 0
	}
	data["UniqueCountries"] = uniqueCountries

	latestQSOs, err := db.ListRecentQSOs(ctx, oqrsLatestLimit)
	if err != nil {
		logger.Error("Failed to load latest QSOs", "error", err)
	} else {
		data["LatestQSOs"] = latestQSOs
	}

	hallOfFame, err := db.GetPaperQSLHallOfFame(ctx)
	if err != nil {
		logger.Error("Failed to load paper QSL hall of fame", "error", err)
	} else {
		data["PaperQSLHallOfFame"] = hallOfFame
	}

	latestTime, err := db.GetLatestQSOTime(ctx)
	if err != nil {
		logger.Error("Failed to load latest QSO time", "error", err)
		latestTime = nil
	}
	if latestTime != nil {
		data["LatestQSODate"] = latestTime.Format("2006-01-02")
		data["LatestQSOTimeAgo"] = formatTimeAgo(*latestTime)
	}
}

func parseOQRSPath(path string) (string, string, int64, bool) {
	lastDash := strings.LastIndex(path, "-")
	if lastDash == -1 {
		return "", "", 0, false
	}

	encodedCallsign := path[:lastDash]
	timestampStr := path[lastDash+1:]

	callsign, err := url.QueryUnescape(encodedCallsign)
	if err != nil {
		return "", "", 0, false
	}
	callsign = strings.ToUpper(callsign)

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", "", 0, false
	}

	return callsign, timestampStr, timestamp, true
}

func serveOQRSMap(c flamego.Context, w http.ResponseWriter, path string) {
	callsign, timestampStr, timestamp, ok := parseOQRSPath(path)
	if !ok {
		http.NotFound(w, c.Request().Request)
		return
	}

	safeCallsign := strings.ReplaceAll(callsign, "/", "_")
	mapFileName := fmt.Sprintf("%s-%s.png", safeCallsign, timestampStr)
	mapPath := filepath.Join("maps", mapFileName)

	if _, err := os.Stat(mapPath); os.IsNotExist(err) {
		searchTime := time.Unix(timestamp, 0).UTC()
		qso, err := db.FindClosestQSOByCallAndTime(c.Request().Context(), callsign, searchTime, oqrsToleranceMinutes)
		if err != nil {
			logger.Error("Failed to generate OQRS map", "callsign", callsign, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if qso == nil || qso.MyGridSquare == nil || qso.GridSquare == nil || *qso.MyGridSquare == "" || *qso.GridSquare == "" {
			http.NotFound(w, c.Request().Request)
			return
		}

		config := utils.MapConfig{
			Width:      600,
			Height:     400,
			Zoom:       0,
			OutputPath: mapPath,
		}
		if err := utils.CreateGridMap(*qso.MyGridSquare, *qso.GridSquare, config); err != nil {
			logger.Error("Failed to render OQRS map", "file", mapFileName, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		logger.Error("Failed to stat OQRS map", "file", mapFileName, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, c.Request().Request, mapPath)
}

func parseOQRSTime(yearStr, monthStr, dayStr, hourStr, minuteStr string) (time.Time, error) {
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return time.Time{}, err
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		return time.Time{}, err
	}
	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return time.Time{}, err
	}
	hour, err := strconv.Atoi(hourStr)
	if err != nil {
		return time.Time{}, err
	}
	minute, err := strconv.Atoi(minuteStr)
	if err != nil {
		return time.Time{}, err
	}

	if year < 2000 || year > 2100 {
		return time.Time{}, fmt.Errorf("year out of range")
	}
	if month < 1 || month > 12 {
		return time.Time{}, fmt.Errorf("month out of range")
	}
	if day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("day out of range")
	}
	if hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("hour out of range")
	}
	if minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("minute out of range")
	}

	parsed := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
	if parsed.Year() != year || int(parsed.Month()) != month || parsed.Day() != day {
		return time.Time{}, fmt.Errorf("invalid date")
	}

	return parsed, nil
}

func formatTimeAgo(ts time.Time) string {
	diff := time.Since(ts)
	if diff < 0 {
		diff = -diff
	}

	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	if diff < 30*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
	if diff < 365*24*time.Hour {
		return fmt.Sprintf("%dmo ago", int(diff.Hours()/(24*30)))
	}
	return fmt.Sprintf("%dy ago", int(diff.Hours()/(24*365)))
}

func qsoTimestampUTC(qso *db.QSO) time.Time {
	if qso == nil {
		return time.Time{}
	}
	timeOn := qso.TimeOn.UTC()
	return time.Date(
		qso.QSODate.Year(),
		qso.QSODate.Month(),
		qso.QSODate.Day(),
		timeOn.Hour(),
		timeOn.Minute(),
		timeOn.Second(),
		0,
		time.UTC,
	)
}
