// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
)

func TestParseADIFTimestamp(t *testing.T) {
	t.Parallel()

	t.Run("pads time", func(t *testing.T) {
		t.Parallel()

		got, err := parseADIFTimestamp("20240115", "1234")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expected := time.Date(2024, time.January, 15, 12, 34, 0, 0, time.UTC)
		if !got.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("full time", func(t *testing.T) {
		t.Parallel()

		got, err := parseADIFTimestamp("20240115", "123456")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		expected := time.Date(2024, time.January, 15, 12, 34, 56, 0, time.UTC)
		if !got.Equal(expected) {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	})

	t.Run("invalid date length", func(t *testing.T) {
		t.Parallel()

		if _, err := parseADIFTimestamp("202401", "123456"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid time length", func(t *testing.T) {
		t.Parallel()

		if _, err := parseADIFTimestamp("20240115", "1"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("invalid date value", func(t *testing.T) {
		t.Parallel()

		if _, err := parseADIFTimestamp("20240230", "120000"); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestQSOFormatters(t *testing.T) {
	t.Parallel()

	qsoDate := time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC)
	timeOn := time.Date(2024, time.March, 10, 15, 4, 5, 0, time.FixedZone("UTC+2", 2*3600))

	qso := QSO{QSODate: qsoDate, TimeOn: timeOn}
	if got := qso.FormatDate(); got != "2024-03-10" {
		t.Fatalf("expected formatted date, got %q", got)
	}

	if got := qso.FormatTime(); got != "13:04" {
		t.Fatalf("expected formatted time, got %q", got)
	}

	if got := qso.FormatQSOTime(); got != "2024-03-10 13:04:05 UTC" {
		t.Fatalf("expected formatted timestamp, got %q", got)
	}

	listItem := QSOListItem{QSODate: qsoDate, TimeOn: timeOn}
	if got := listItem.FormatDate(); got != "2024-03-10" {
		t.Fatalf("expected formatted date, got %q", got)
	}

	if got := listItem.FormatTime(); got != "13:04" {
		t.Fatalf("expected formatted time, got %q", got)
	}

	if got := listItem.FormatQSOTime(); got != "2024-03-10 13:04:05 UTC" {
		t.Fatalf("expected formatted timestamp, got %q", got)
	}
}

func TestParseADIFDate(t *testing.T) {
	t.Parallel()

	got, err := parseADIFDate("20240115")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got.Year() != 2024 || got.Month() != time.January || got.Day() != 15 {
		t.Fatalf("unexpected parsed date: %v", got)
	}

	if _, err := parseADIFDate("20241"); err == nil {
		t.Fatalf("expected error for invalid date length")
	}
}
