// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
)

func TestHealthProfileLifecycle(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	profileID, err := CreateHealthProfile(ctx, "Alice", nil, nil, nil, true)
	if err != nil {
		t.Fatalf("CreateHealthProfile failed: %v", err)
	}

	profile, err := GetHealthProfile(ctx, profileID)
	if err != nil {
		t.Fatalf("GetHealthProfile failed: %v", err)
	}
	if profile.Name != "Alice" {
		t.Fatalf("expected profile name Alice, got %q", profile.Name)
	}

	primary, err := GetPrimaryHealthProfile(ctx)
	if err != nil {
		t.Fatalf("GetPrimaryHealthProfile failed: %v", err)
	}
	if primary == nil || primary.ID.String() != profileID {
		t.Fatalf("expected primary profile to match")
	}

	updatedName := "Alice Updated"
	if err := UpdateHealthProfile(ctx, profileID, updatedName, nil, nil, nil, true); err != nil {
		t.Fatalf("UpdateHealthProfile failed: %v", err)
	}

	profiles, err := ListHealthProfiles(ctx)
	if err != nil {
		t.Fatalf("ListHealthProfiles failed: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}

	if err := DeleteHealthProfile(ctx, profileID); err != nil {
		t.Fatalf("DeleteHealthProfile failed: %v", err)
	}
}

func TestHealthFollowupsAndLabResults(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	profileID := mustCreateHealthProfile(t, "Lab Profile", true)

	followupDate1 := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	followupDate2 := time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC)
	followupID1 := mustCreateFollowup(t, profileID, followupDate1)
	followupID2 := mustCreateFollowup(t, profileID, followupDate2)

	followups, err := ListFollowups(ctx, profileID)
	if err != nil {
		t.Fatalf("ListFollowups failed: %v", err)
	}
	if len(followups) != 2 {
		t.Fatalf("expected 2 followups, got %d", len(followups))
	}

	if _, err := GetFollowup(ctx, followupID1); err != nil {
		t.Fatalf("GetFollowup failed: %v", err)
	}

	updateInput := UpdateFollowupInput{
		FollowupDate: followupDate1.AddDate(0, 0, 1),
		HospitalName: "Updated Hospital",
		Notes:        stringPtr("updated"),
	}
	if err := UpdateFollowup(ctx, followupID1, updateInput); err != nil {
		t.Fatalf("UpdateFollowup failed: %v", err)
	}

	results := []CreateLabResultInput{
		{FollowupID: followupID1, TestName: "White blood cells", TestUnit: stringPtr("x10^3/uL"), TestValue: 5.0},
		{FollowupID: followupID1, TestName: "Neutrophils", TestUnit: stringPtr("%"), TestValue: 40.0},
		{FollowupID: followupID1, TestName: "Lymphocytes", TestUnit: stringPtr("%"), TestValue: 20.0},
		{FollowupID: followupID1, TestName: "Triglycerides", TestUnit: stringPtr("mg/dL"), TestValue: 150.0},
		{FollowupID: followupID1, TestName: "HDL Cholesterol", TestUnit: stringPtr("mg/dL"), TestValue: 50.0},
		{FollowupID: followupID1, TestName: "Total Cholesterol", TestUnit: stringPtr("mg/dL"), TestValue: 200.0},
		{FollowupID: followupID2, TestName: "Triglycerides", TestUnit: stringPtr("mg/dL"), TestValue: 120.0},
		{FollowupID: followupID2, TestName: "HDL Cholesterol", TestUnit: stringPtr("mg/dL"), TestValue: 60.0},
		{FollowupID: followupID2, TestName: "Total Cholesterol", TestUnit: stringPtr("mg/dL"), TestValue: 180.0},
	}

	var firstResultID string
	for i, input := range results {
		resultID, err := CreateLabResult(ctx, input)
		if err != nil {
			t.Fatalf("CreateLabResult failed: %v", err)
		}
		if i == 0 {
			firstResultID = resultID
		}
	}

	if _, err := GetLabResult(ctx, firstResultID); err != nil {
		t.Fatalf("GetLabResult failed: %v", err)
	}

	if err := UpdateLabResult(ctx, firstResultID, UpdateLabResultInput{TestValue: 5.5}); err != nil {
		t.Fatalf("UpdateLabResult failed: %v", err)
	}

	grouped, err := GetLabResultsByFollowup(ctx, followupID1)
	if err != nil {
		t.Fatalf("GetLabResultsByFollowup failed: %v", err)
	}
	if len(grouped) == 0 {
		t.Fatalf("expected grouped results")
	}

	withCalculated, err := GetLabResultsByFollowupWithCalculated(ctx, followupID1)
	if err != nil {
		t.Fatalf("GetLabResultsByFollowupWithCalculated failed: %v", err)
	}
	if len(withCalculated) == 0 {
		t.Fatalf("expected calculated results")
	}

	nameResults, err := GetLabResultsByTestName(ctx, profileID, "Triglycerides")
	if err != nil {
		t.Fatalf("GetLabResultsByTestName failed: %v", err)
	}
	if len(nameResults) != 2 {
		t.Fatalf("expected 2 triglycerides results, got %d", len(nameResults))
	}

	calcSeries, err := GetLabResultsByTestNameWithCalculated(ctx, profileID, "TG/HDL (Calc)")
	if err != nil {
		t.Fatalf("GetLabResultsByTestNameWithCalculated failed: %v", err)
	}
	if len(calcSeries) != 2 {
		t.Fatalf("expected 2 ratio results, got %d", len(calcSeries))
	}

	coeffSeries, err := GetLabResultsByTestNameWithCalculated(ctx, profileID, "Atherogenic Coefficient")
	if err != nil {
		t.Fatalf("GetLabResultsByTestNameWithCalculated failed: %v", err)
	}
	if len(coeffSeries) != 2 {
		t.Fatalf("expected 2 coefficient results, got %d", len(coeffSeries))
	}

	counts, err := GetTestNamesWithCounts(ctx, profileID)
	if err != nil {
		t.Fatalf("GetTestNamesWithCounts failed: %v", err)
	}
	if len(counts) == 0 {
		t.Fatalf("expected test name counts")
	}

	countsWithCalculated, err := GetTestNamesWithCountsIncludingCalculated(ctx, profileID)
	if err != nil {
		t.Fatalf("GetTestNamesWithCountsIncludingCalculated failed: %v", err)
	}
	if len(countsWithCalculated) <= len(counts) {
		t.Fatalf("expected additional calculated tests")
	}

	if err := DeleteLabResult(ctx, firstResultID); err != nil {
		t.Fatalf("DeleteLabResult failed: %v", err)
	}

	if err := DeleteFollowup(ctx, followupID1); err != nil {
		t.Fatalf("DeleteFollowup failed: %v", err)
	}
}
