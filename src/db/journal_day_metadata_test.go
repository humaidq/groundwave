// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
)

func TestJournalDayLocations(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	day := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	if err := CreateJournalDayLocation(ctx, day, 25.1, 55.2); err != nil {
		t.Fatalf("CreateJournalDayLocation failed: %v", err)
	}

	locations, err := ListJournalDayLocations(ctx, day)
	if err != nil {
		t.Fatalf("ListJournalDayLocations failed: %v", err)
	}

	if len(locations) != 1 {
		t.Fatalf("expected 1 location, got %d", len(locations))
	}

	if err := DeleteJournalDayLocation(ctx, locations[0].ID); err != nil {
		t.Fatalf("DeleteJournalDayLocation failed: %v", err)
	}

	locations, err = ListJournalDayLocations(ctx, day)
	if err != nil {
		t.Fatalf("ListJournalDayLocations failed: %v", err)
	}

	if len(locations) != 0 {
		t.Fatalf("expected 0 locations, got %d", len(locations))
	}
}
