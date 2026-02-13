// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"regexp"
	"testing"
	"time"
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
