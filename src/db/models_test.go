// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
)

func floatPtr(value float64) *float64 {
	return &value
}

func assertFloatPtrEqual(t *testing.T, got, want *float64) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("expected %v, got %v", want, got)
	}
	if *got != *want {
		t.Fatalf("expected %v, got %v", *want, *got)
	}
}

func TestInventoryStatusLabel(t *testing.T) {
	cases := []struct {
		status InventoryStatus
		want   string
	}{
		{status: InventoryStatusActive, want: "Active"},
		{status: InventoryStatusStored, want: "Stored"},
		{status: InventoryStatusDamaged, want: "Damaged"},
		{status: InventoryStatusMaintenanceRequired, want: "Maintenance Required"},
		{status: InventoryStatusGiven, want: "Given"},
		{status: InventoryStatusDisposed, want: "Disposed"},
		{status: InventoryStatusLost, want: "Lost"},
		{status: InventoryStatus("custom"), want: "custom"},
	}

	for _, tc := range cases {
		if got := InventoryStatusLabel(tc.status); got != tc.want {
			t.Fatalf("expected %q, got %q", tc.want, got)
		}
	}
}

func TestHealthProfileGetAge(t *testing.T) {
	dob := time.Date(2000, time.February, 10, 0, 0, 0, 0, time.UTC)
	profile := HealthProfile{DateOfBirth: &dob}

	beforeBirthday := time.Date(2024, time.February, 9, 0, 0, 0, 0, time.UTC)
	got := profile.GetAge(beforeBirthday)
	if got == nil || *got != 23 {
		t.Fatalf("expected age 23, got %v", got)
	}

	onBirthday := time.Date(2024, time.February, 10, 0, 0, 0, 0, time.UTC)
	got = profile.GetAge(onBirthday)
	if got == nil || *got != 24 {
		t.Fatalf("expected age 24, got %v", got)
	}

	profile = HealthProfile{}
	if got := profile.GetAge(onBirthday); got != nil {
		t.Fatalf("expected nil age, got %v", got)
	}
}

func TestHealthProfileGetAgeRange(t *testing.T) {
	atDate := time.Date(2024, time.June, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		dob  *time.Time
		want AgeRange
	}{
		{name: "missing dob", dob: nil, want: AgeAdult},
		{name: "pediatric", dob: floatPtrTime(time.Date(2007, time.June, 1, 0, 0, 0, 0, time.UTC)), want: AgePediatric},
		{name: "adult lower", dob: floatPtrTime(time.Date(2006, time.June, 1, 0, 0, 0, 0, time.UTC)), want: AgeAdult},
		{name: "adult upper", dob: floatPtrTime(time.Date(1975, time.June, 1, 0, 0, 0, 0, time.UTC)), want: AgeAdult},
		{name: "middle age lower", dob: floatPtrTime(time.Date(1974, time.June, 1, 0, 0, 0, 0, time.UTC)), want: AgeMiddleAge},
		{name: "middle age upper", dob: floatPtrTime(time.Date(1960, time.June, 1, 0, 0, 0, 0, time.UTC)), want: AgeMiddleAge},
		{name: "senior", dob: floatPtrTime(time.Date(1959, time.June, 1, 0, 0, 0, 0, time.UTC)), want: AgeSenior},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := HealthProfile{DateOfBirth: tc.dob}
			if got := profile.GetAgeRange(atDate); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func floatPtrTime(value time.Time) *time.Time {
	return &value
}

func TestReferenceRangeGetDisplayRange(t *testing.T) {
	cases := []struct {
		name       string
		refMin     *float64
		refMax     *float64
		optMin     *float64
		optMax     *float64
		hasOptimal bool
		wantOptMin *float64
		wantOptMax *float64
	}{
		{
			name:       "no optimal",
			refMin:     floatPtr(1),
			refMax:     floatPtr(2),
			optMin:     nil,
			optMax:     nil,
			hasOptimal: false,
			wantOptMin: nil,
			wantOptMax: nil,
		},
		{
			name:       "only optimal max",
			refMin:     floatPtr(1),
			refMax:     floatPtr(2),
			optMin:     nil,
			optMax:     floatPtr(1.8),
			hasOptimal: true,
			wantOptMin: floatPtr(1),
			wantOptMax: floatPtr(1.8),
		},
		{
			name:       "only optimal min",
			refMin:     floatPtr(1),
			refMax:     floatPtr(2),
			optMin:     floatPtr(1.2),
			optMax:     nil,
			hasOptimal: true,
			wantOptMin: floatPtr(1.2),
			wantOptMax: floatPtr(2),
		},
		{
			name:       "both optimal",
			refMin:     floatPtr(1),
			refMax:     floatPtr(2),
			optMin:     floatPtr(1.1),
			optMax:     floatPtr(1.9),
			hasOptimal: true,
			wantOptMin: floatPtr(1.1),
			wantOptMax: floatPtr(1.9),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := ReferenceRange{
				ReferenceMin: tc.refMin,
				ReferenceMax: tc.refMax,
				OptimalMin:   tc.optMin,
				OptimalMax:   tc.optMax,
			}
			refMin, refMax, optMin, optMax, hasOptimal := rr.GetDisplayRange()
			assertFloatPtrEqual(t, refMin, tc.refMin)
			assertFloatPtrEqual(t, refMax, tc.refMax)
			if hasOptimal != tc.hasOptimal {
				t.Fatalf("expected hasOptimal=%v, got %v", tc.hasOptimal, hasOptimal)
			}
			assertFloatPtrEqual(t, optMin, tc.wantOptMin)
			assertFloatPtrEqual(t, optMax, tc.wantOptMax)
		})
	}
}
