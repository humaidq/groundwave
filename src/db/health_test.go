// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCalculateAbsoluteCounts(t *testing.T) {
	t.Parallel()

	t.Run("calculates derived results", func(t *testing.T) {
		t.Parallel()

		followupID := uuid.New()
		baseTime := time.Date(2024, time.January, 1, 10, 0, 0, 0, time.UTC)
		laterTime := baseTime.Add(2 * time.Hour)
		earlierTime := baseTime.Add(-2 * time.Hour)

		results := []HealthLabResult{
			{ID: uuid.New(), FollowupID: followupID, TestName: "White blood cells", TestValue: 5.0, CreatedAt: baseTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "Neutrophils", TestValue: 40.0, CreatedAt: baseTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "Lymphocytes", TestValue: 20.0, CreatedAt: baseTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "Triglycerides", TestValue: 150.0, CreatedAt: baseTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "HDL Cholesterol", TestValue: 50.0, CreatedAt: laterTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "Total Cholesterol", TestValue: 200.0, CreatedAt: earlierTime},
		}

		calculated := calculateAbsoluteCounts(results)

		neutrophils := findDisplayResult(calculated, "Neutrophils (Absolute)")
		if neutrophils == nil {
			t.Fatalf("expected Neutrophils (Absolute) result")
		}

		assertFloatClose(t, neutrophils.TestValue, 2.0)

		if neutrophils.TestUnit == nil {
			t.Fatalf("expected unit for Neutrophils (Absolute)")
		}

		if !neutrophils.CreatedAt.Equal(baseTime) {
			t.Fatalf("expected created_at %v, got %v", baseTime, neutrophils.CreatedAt)
		}

		lymphocytes := findDisplayResult(calculated, "Lymphocytes (Absolute)")
		if lymphocytes == nil {
			t.Fatalf("expected Lymphocytes (Absolute) result")
		}

		assertFloatClose(t, lymphocytes.TestValue, 1.0)

		ratio := findDisplayResult(calculated, "TG/HDL (Calc)")
		if ratio == nil {
			t.Fatalf("expected TG/HDL (Calc) result")
		}

		assertFloatClose(t, ratio.TestValue, 3.0)

		if ratio.TestUnit == nil || *ratio.TestUnit != "ratio" {
			t.Fatalf("expected ratio unit")
		}

		if ratio.FollowupID != followupID {
			t.Fatalf("expected followup ID %v, got %v", followupID, ratio.FollowupID)
		}

		if !ratio.CreatedAt.Equal(laterTime) {
			t.Fatalf("expected created_at %v, got %v", laterTime, ratio.CreatedAt)
		}

		coefficient := findDisplayResult(calculated, "Atherogenic Coefficient")
		if coefficient == nil {
			t.Fatalf("expected Atherogenic Coefficient result")
		}

		assertFloatClose(t, coefficient.TestValue, 3.0)

		if coefficient.TestUnit == nil || *coefficient.TestUnit != "ratio" {
			t.Fatalf("expected ratio unit")
		}

		if coefficient.FollowupID != followupID {
			t.Fatalf("expected followup ID %v, got %v", followupID, coefficient.FollowupID)
		}

		if !coefficient.CreatedAt.Equal(laterTime) {
			t.Fatalf("expected created_at %v, got %v", laterTime, coefficient.CreatedAt)
		}
	})

	t.Run("skips manual absolute", func(t *testing.T) {
		t.Parallel()

		followupID := uuid.New()
		baseTime := time.Date(2024, time.January, 1, 10, 0, 0, 0, time.UTC)

		results := []HealthLabResult{
			{ID: uuid.New(), FollowupID: followupID, TestName: "White blood cells", TestValue: 5.0, CreatedAt: baseTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "Neutrophils", TestValue: 40.0, CreatedAt: baseTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "Neutrophils (Absolute)", TestValue: 2.0, CreatedAt: baseTime},
			{ID: uuid.New(), FollowupID: followupID, TestName: "Lymphocytes", TestValue: 20.0, CreatedAt: baseTime},
		}

		calculated := calculateAbsoluteCounts(results)
		if findDisplayResult(calculated, "Neutrophils (Absolute)") != nil {
			t.Fatalf("did not expect calculated Neutrophils (Absolute) result")
		}

		if findDisplayResult(calculated, "Lymphocytes (Absolute)") == nil {
			t.Fatalf("expected Lymphocytes (Absolute) result")
		}
	})
}

func findDisplayResult(results []HealthLabResultDisplay, name string) *HealthLabResultDisplay {
	for i := range results {
		if results[i].TestName == name {
			return &results[i]
		}
	}

	return nil
}

func assertFloatClose(t *testing.T, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
