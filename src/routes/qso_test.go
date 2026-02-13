// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/humaidq/groundwave/db"
)

func TestParseADIFExportDate(t *testing.T) {
	t.Parallel()

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()

		parsed, ok, err := parseADIFExportDate("   ")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if ok {
			t.Fatalf("expected no date")
		}

		if !parsed.IsZero() {
			t.Fatalf("expected zero time value, got %v", parsed)
		}
	})

	t.Run("valid date", func(t *testing.T) {
		t.Parallel()

		parsed, ok, err := parseADIFExportDate("2024-02-03")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !ok {
			t.Fatalf("expected parsed date")
		}

		if got := parsed.Format("2006-01-02"); got != "2024-02-03" {
			t.Fatalf("unexpected parsed date: %s", got)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		t.Parallel()

		if _, _, err := parseADIFExportDate("20240203"); err == nil {
			t.Fatalf("expected error for invalid format")
		}
	})
}

func TestBuildADIFFilename(t *testing.T) {
	t.Parallel()

	t.Run("with date range", func(t *testing.T) {
		t.Parallel()

		fromDate := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		toDate := time.Date(2024, time.January, 31, 0, 0, 0, 0, time.UTC)

		filename := buildADIFFilename(&fromDate, &toDate)
		if filename != "qsos-20240101-20240131.adi" {
			t.Fatalf("unexpected filename: %s", filename)
		}
	})

	t.Run("without date range", func(t *testing.T) {
		t.Parallel()

		filename := buildADIFFilename(nil, nil)

		matched, err := regexp.MatchString(`^qsos-\d{8}-\d{6}\.adi$`, filename)
		if err != nil {
			t.Fatalf("regexp match failed: %v", err)
		}

		if !matched {
			t.Fatalf("unexpected filename format: %s", filename)
		}
	})
}

func TestFormatQSORawJSON(t *testing.T) {
	t.Parallel()

	t.Run("nil detail", func(t *testing.T) {
		t.Parallel()

		raw, err := formatQSORawJSON(nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if raw != "" {
			t.Fatalf("expected empty raw json, got %q", raw)
		}
	})

	t.Run("renders fields", func(t *testing.T) {
		t.Parallel()

		detail := &db.QSODetail{
			QSO: &db.QSO{
				Call:    "A66H",
				QSODate: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.UTC),
				TimeOn:  time.Date(2026, time.March, 5, 12, 34, 56, 0, time.UTC),
				Mode:    "SSB",
			},
		}

		raw, err := formatQSORawJSON(detail)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !strings.Contains(raw, "\"Call\": \"A66H\"") {
			t.Fatalf("expected call sign in raw json, got %q", raw)
		}

		if !strings.Contains(raw, "\"Band\": null") {
			t.Fatalf("expected null band in raw json, got %q", raw)
		}

		if !strings.Contains(raw, "\"ContactName\": null") {
			t.Fatalf("expected contact name in raw json, got %q", raw)
		}
	})
}
