// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"io/fs"
	"testing"
)

func TestReferenceRangeLookup(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	if err := SyncReferenceRanges(ctx); err != nil {
		t.Fatalf("SyncReferenceRanges failed: %v", err)
	}

	if _, err := fs.ReadDir(GetEmbeddedMigrations(), "migrations"); err != nil {
		t.Fatalf("expected embedded migrations: %v", err)
	}

	defs := GetReferenceRangeDefinitions()
	if len(defs) == 0 {
		t.Fatalf("expected reference range definitions")
	}

	first := defs[0]
	rangeResult, err := GetReferenceRange(ctx, first.TestName, first.AgeRange, first.Gender)
	if err != nil {
		t.Fatalf("GetReferenceRange failed: %v", err)
	}
	if rangeResult == nil {
		t.Fatalf("expected reference range result")
	}
}

func TestGetReferenceRangeFallbacks(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	if err := SyncReferenceRanges(ctx); err != nil {
		t.Fatalf("SyncReferenceRanges failed: %v", err)
	}

	// Gender-specific lookup should fall back to unisex when missing.
	rr, err := GetReferenceRange(ctx, "White blood cells", AgeAdult, GenderMale)
	if err != nil {
		t.Fatalf("GetReferenceRange failed: %v", err)
	}
	if rr == nil || rr.Gender != GenderUnisex {
		t.Fatalf("expected unisex fallback range")
	}

	missing, err := GetReferenceRange(ctx, "Nonexistent Test", AgeAdult, GenderMale)
	if err != nil {
		t.Fatalf("GetReferenceRange failed: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil range for missing test")
	}
}
