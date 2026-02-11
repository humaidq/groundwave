/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
)

// ReferenceRangeDefinition represents a reference range to be synced to the database
type ReferenceRangeDefinition struct {
	TestName     string
	AgeRange     AgeRange
	Gender       Gender
	ReferenceMin *float64
	ReferenceMax *float64
	OptimalMin   *float64
	OptimalMax   *float64
}

// ptr is a helper to create pointers to float64 literals
func ptr(f float64) *float64 {
	return &f
}

// GetReferenceRangeDefinitions returns all reference ranges to be synced to the database
// This is the authoritative source of truth for reference ranges
func GetReferenceRangeDefinitions() []ReferenceRangeDefinition {
	return []ReferenceRangeDefinition{
		// ===== WHITE BLOOD CELLS (×10³/μL) =====
		// Unisex across all age ranges
		// Pediatric: 1 yr – 17 yrs range (optimal clinically dependent)
		{
			TestName: "White blood cells", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(4.5), ReferenceMax: ptr(13.0),
			OptimalMin: nil, OptimalMax: nil, // Clinically dependent
		},
		{
			TestName: "White blood cells", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(4.5), ReferenceMax: ptr(11.0),
			OptimalMin: ptr(5.0), OptimalMax: ptr(8.0),
		},
		{
			TestName: "White blood cells", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(4.5), ReferenceMax: ptr(11.0),
			OptimalMin: ptr(5.0), OptimalMax: ptr(8.0),
		},
		{
			TestName: "White blood cells", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(4.0), ReferenceMax: ptr(10.5),
			OptimalMin: ptr(4.5), OptimalMax: ptr(8.0),
		},

		// ===== RED BLOOD CELLS (×10⁶/μL) =====
		// Pediatric is unisex; Adult+ is gender-specific
		{
			TestName: "Red blood cells", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(4.0), ReferenceMax: ptr(5.5),
			OptimalMin: nil, OptimalMax: nil, // Clinically dependent
		},
		{
			TestName: "Red blood cells", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(4.35), ReferenceMax: ptr(5.65),
			OptimalMin: ptr(4.50), OptimalMax: ptr(5.50),
		},
		{
			TestName: "Red blood cells", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(3.92), ReferenceMax: ptr(5.13),
			OptimalMin: ptr(4.00), OptimalMax: ptr(4.90),
		},
		{
			TestName: "Red blood cells", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(4.30), ReferenceMax: ptr(5.60),
			OptimalMin: ptr(4.50), OptimalMax: ptr(5.50),
		},
		{
			TestName: "Red blood cells", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(3.90), ReferenceMax: ptr(5.10),
			OptimalMin: ptr(4.00), OptimalMax: ptr(4.90),
		},
		{
			TestName: "Red blood cells", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(4.20), ReferenceMax: ptr(5.50),
			OptimalMin: ptr(4.20), OptimalMax: ptr(5.50), // Standard range
		},
		{
			TestName: "Red blood cells", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(3.80), ReferenceMax: ptr(5.00),
			OptimalMin: ptr(3.80), OptimalMax: ptr(5.00), // Standard range
		},

		// ===== HEMOGLOBIN (g/dL) =====
		// Pediatric is unisex (1 mo - 17 yrs); Adult+ is gender-specific
		// Note: Newborns (0-30 days) have 14.0-24.0 range but not represented in age_range enum
		{
			TestName: "Hemoglobin", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(15.5),
			OptimalMin: nil, OptimalMax: nil, // Age dependent
		},
		{
			TestName: "Hemoglobin", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(13.2), ReferenceMax: ptr(16.6),
			OptimalMin: ptr(14.0), OptimalMax: ptr(16.0),
		},
		{
			TestName: "Hemoglobin", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(11.6), ReferenceMax: ptr(15.0),
			OptimalMin: ptr(12.5), OptimalMax: ptr(14.5),
		},
		{
			TestName: "Hemoglobin", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(13.0), ReferenceMax: ptr(16.5),
			OptimalMin: ptr(14.0), OptimalMax: ptr(16.0),
		},
		{
			TestName: "Hemoglobin", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(11.5), ReferenceMax: ptr(14.8),
			OptimalMin: ptr(12.5), OptimalMax: ptr(14.5),
		},
		{
			TestName: "Hemoglobin", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(12.4), ReferenceMax: ptr(16.0),
			OptimalMin: ptr(13.0), OptimalMax: ptr(16.0), // Maintain >13.0 to avoid anemia
		},
		{
			TestName: "Hemoglobin", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(11.7), ReferenceMax: ptr(14.5),
			OptimalMin: ptr(12.0), OptimalMax: ptr(14.5), // Maintain >12.0 to avoid anemia
		},

		// ===== HEMATOCRIT (%) =====
		// Pediatric is unisex; Adult+ is gender-specific
		{
			TestName: "Hematocrit", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(31.0), ReferenceMax: ptr(45.0),
			OptimalMin: nil, OptimalMax: nil, // Age dependent
		},
		{
			TestName: "Hematocrit", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(41.0), ReferenceMax: ptr(50.0),
			OptimalMin: ptr(42.0), OptimalMax: ptr(48.0), // Critical: avoid >50-52
		},
		{
			TestName: "Hematocrit", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(36.0), ReferenceMax: ptr(44.0),
			OptimalMin: ptr(37.0), OptimalMax: ptr(44.0),
		},
		{
			TestName: "Hematocrit", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(50.0),
			OptimalMin: ptr(42.0), OptimalMax: ptr(48.0),
		},
		{
			TestName: "Hematocrit", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(36.0), ReferenceMax: ptr(44.0),
			OptimalMin: ptr(37.0), OptimalMax: ptr(44.0),
		},
		{
			TestName: "Hematocrit", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(38.0), ReferenceMax: ptr(49.0),
			OptimalMin: ptr(38.0), OptimalMax: ptr(49.0), // Standard range
		},
		{
			TestName: "Hematocrit", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(35.0), ReferenceMax: ptr(43.0),
			OptimalMin: ptr(35.0), OptimalMax: ptr(43.0), // Standard range
		},

		// ===== MCV - Mean Corpuscular Volume (fL) =====
		// Pediatric (1-17 yrs); Note: 0-1 yr range is 70-115 but varies wildly at birth
		{
			TestName: "M.C.V", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(78.0), ReferenceMax: ptr(95.0),
			OptimalMin: ptr(78.0), OptimalMax: ptr(95.0), // Standard range
		},
		{
			TestName: "M.C.V", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(80.0), ReferenceMax: ptr(96.0),
			OptimalMin: ptr(82.0), OptimalMax: ptr(92.0),
		},
		{
			TestName: "M.C.V", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(80.0), ReferenceMax: ptr(96.0),
			OptimalMin: ptr(82.0), OptimalMax: ptr(92.0),
		},
		{
			TestName: "M.C.V", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(80.0), ReferenceMax: ptr(96.0),
			OptimalMin: ptr(82.0), OptimalMax: ptr(92.0),
		},

		// ===== MCH - Mean Corpuscular Hemoglobin (pg) =====
		// Tracks with MCV. Low = Iron issue.
		{
			TestName: "M.C.H", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(27.0), ReferenceMax: ptr(33.0),
			OptimalMin: ptr(28.0), OptimalMax: ptr(32.0),
		},
		{
			TestName: "M.C.H", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(27.0), ReferenceMax: ptr(33.0),
			OptimalMin: ptr(28.0), OptimalMax: ptr(32.0),
		},
		{
			TestName: "M.C.H", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(27.0), ReferenceMax: ptr(33.0),
			OptimalMin: ptr(28.0), OptimalMax: ptr(32.0),
		},
		{
			TestName: "M.C.H", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(27.0), ReferenceMax: ptr(33.0),
			OptimalMin: ptr(28.0), OptimalMax: ptr(32.0),
		},

		// ===== MCHC - Mean Corpuscular Hemoglobin Concentration (g/dL) =====
		// Measure of Hb concentration
		{
			TestName: "M.C.H.C", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(33.0), ReferenceMax: ptr(36.0),
			OptimalMin: ptr(33.0), OptimalMax: ptr(35.0),
		},
		{
			TestName: "M.C.H.C", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(33.0), ReferenceMax: ptr(36.0),
			OptimalMin: ptr(33.0), OptimalMax: ptr(35.0),
		},
		{
			TestName: "M.C.H.C", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(33.0), ReferenceMax: ptr(36.0),
			OptimalMin: ptr(33.0), OptimalMax: ptr(35.0),
		},
		{
			TestName: "M.C.H.C", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(33.0), ReferenceMax: ptr(36.0),
			OptimalMin: ptr(33.0), OptimalMax: ptr(35.0),
		},

		// ===== RDW-CV - Red Cell Distribution Width (%) =====
		// Critical Aging Marker
		{
			TestName: "RDW - CV", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(11.5), ReferenceMax: ptr(14.5),
			OptimalMin: nil, OptimalMax: ptr(13.0),
		},
		{
			TestName: "RDW - CV", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(11.5), ReferenceMax: ptr(14.5),
			OptimalMin: nil, OptimalMax: ptr(13.0),
		},
		{
			TestName: "RDW - CV", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(11.5), ReferenceMax: ptr(14.5),
			OptimalMin: nil, OptimalMax: ptr(13.0),
		},
		{
			TestName: "RDW - CV", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(11.5), ReferenceMax: ptr(14.5),
			OptimalMin: nil, OptimalMax: ptr(13.0),
		},

		// ===== PLATELETS (×10³/μL) =====
		{
			TestName: "Platelets", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(150.0), ReferenceMax: ptr(450.0),
			OptimalMin: ptr(150.0), OptimalMax: ptr(450.0), // Standard
		},
		{
			TestName: "Platelets", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(150.0), ReferenceMax: ptr(450.0),
			OptimalMin: ptr(200.0), OptimalMax: ptr(350.0),
		},
		{
			TestName: "Platelets", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(150.0), ReferenceMax: ptr(450.0),
			OptimalMin: ptr(200.0), OptimalMax: ptr(350.0),
		},
		{
			TestName: "Platelets", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(140.0), ReferenceMax: ptr(400.0),
			OptimalMin: ptr(140.0), OptimalMax: ptr(400.0), // Naturally declines slightly
		},

		// ===== MPV - Mean Platelet Volume (fL) =====
		// High MPV can indicate high platelet turnover/risk of clotting
		{
			TestName: "M.P.V", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(7.5), ReferenceMax: ptr(11.5),
			OptimalMin: ptr(8.5), OptimalMax: ptr(10.5), // Middle of range
		},
		{
			TestName: "M.P.V", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(7.5), ReferenceMax: ptr(11.5),
			OptimalMin: ptr(8.5), OptimalMax: ptr(10.5),
		},
		{
			TestName: "M.P.V", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(7.5), ReferenceMax: ptr(11.5),
			OptimalMin: ptr(8.5), OptimalMax: ptr(10.5),
		},
		{
			TestName: "M.P.V", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(7.5), ReferenceMax: ptr(11.5),
			OptimalMin: ptr(8.5), OptimalMax: ptr(10.5),
		},

		// ===== WHITE BLOOD CELL DIFFERENTIALS =====
		// Neutrophils (%) - Primary bacteria fighter
		{
			TestName: "Neutrophils", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(70.0),
			OptimalMin: nil, OptimalMax: nil, // High % = Bacterial infection or inflammation
		},
		{
			TestName: "Neutrophils", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(70.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Neutrophils", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(70.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Neutrophils", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(70.0),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Neutrophils (Absolute) (×10³/μL)
		{
			TestName: "Neutrophils (Absolute)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(1.8), ReferenceMax: ptr(7.8),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Neutrophils (Absolute)", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(1.8), ReferenceMax: ptr(7.8),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Neutrophils (Absolute)", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(1.8), ReferenceMax: ptr(7.8),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Neutrophils (Absolute)", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(1.8), ReferenceMax: ptr(7.8),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Lymphocytes (%) - Viral fighter. Low levels linked to poor immune resilience.
		{
			TestName: "Lymphocytes", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(20.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Lymphocytes", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(20.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Lymphocytes", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(20.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Lymphocytes", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(20.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Lymphocytes (Absolute) (×10³/μL)
		{
			TestName: "Lymphocytes (Absolute)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.8),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Lymphocytes (Absolute)", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.8),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Lymphocytes (Absolute)", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.8),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Lymphocytes (Absolute)", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.8),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Monocytes (%) - High levels often linked to recovering from infection or CVD risk
		{
			TestName: "Monocytes", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(8.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Monocytes", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(8.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Monocytes", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(8.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Monocytes", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(8.0),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Monocytes (Absolute) (×10³/μL)
		{
			TestName: "Monocytes (Absolute)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Monocytes (Absolute)", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Monocytes (Absolute)", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Monocytes (Absolute)", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Eosinophils (%) - High levels = Allergies or Parasites
		{
			TestName: "Eosinophils", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Eosinophils", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Eosinophils", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Eosinophils", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(1.0), ReferenceMax: ptr(4.0),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Eosinophils (Absolute) (×10³/μL)
		{
			TestName: "Eosinophils (Absolute)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.5),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Eosinophils (Absolute)", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.5),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Eosinophils (Absolute)", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.5),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Eosinophils (Absolute)", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.5),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Basophils (%) - High levels = Histamine response/Allergies
		{
			TestName: "Basophils", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.5), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Basophils", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.5), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Basophils", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.5), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Basophils", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.5), ReferenceMax: ptr(1.0),
			OptimalMin: nil, OptimalMax: nil,
		},

		// Basophils (Absolute) (×10³/μL)
		{
			TestName: "Basophils (Absolute)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.2),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Basophils (Absolute)", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.2),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Basophils (Absolute)", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.2),
			OptimalMin: nil, OptimalMax: nil,
		},
		{
			TestName: "Basophils (Absolute)", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(0.2),
			OptimalMin: nil, OptimalMax: nil,
		},

		// ===== GLUCOSE FASTING FBS (mg/dL) =====
		// Unisex, varies by age. Pre-diabetic: 100-125, Diabetic: >125
		{
			TestName: "Glucose fasting FBS", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(70.0), ReferenceMax: ptr(100.0),
			OptimalMin: ptr(70.0), OptimalMax: ptr(100.0), // Standard range
		},
		{
			TestName: "Glucose fasting FBS", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(70.0), ReferenceMax: ptr(99.0),
			OptimalMin: ptr(75.0), OptimalMax: ptr(90.0), // Ideally <95
		},
		{
			TestName: "Glucose fasting FBS", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(70.0), ReferenceMax: ptr(99.0),
			OptimalMin: ptr(75.0), OptimalMax: ptr(90.0),
		},
		{
			TestName: "Glucose fasting FBS", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(70.0), ReferenceMax: ptr(99.0),
			OptimalMin: ptr(70.0), OptimalMax: ptr(99.0), // Avoid hypoglycemia (<70)
		},

		// ===== URIC ACID (umol/L) =====
		// Gender-specific for adults. Attia strict: <300 umol/L (<5.0 mg/dL)
		// Pediatric is unisex
		{
			TestName: "Uric Acid", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(120.0), ReferenceMax: ptr(330.0),
			OptimalMin: ptr(120.0), OptimalMax: ptr(330.0), // Standard range
		},
		// Adult+ is gender-specific
		{
			TestName: "Uric Acid", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(200.0), ReferenceMax: ptr(420.0),
			OptimalMin: nil, OptimalMax: ptr(300.0), // <5.0 mg/dL
		},
		{
			TestName: "Uric Acid", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(140.0), ReferenceMax: ptr(360.0),
			OptimalMin: nil, OptimalMax: ptr(300.0), // <5.0 mg/dL
		},
		{
			TestName: "Uric Acid", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(200.0), ReferenceMax: ptr(420.0),
			OptimalMin: nil, OptimalMax: ptr(300.0),
		},
		{
			TestName: "Uric Acid", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(140.0), ReferenceMax: ptr(360.0),
			OptimalMin: nil, OptimalMax: ptr(300.0),
		},
		{
			TestName: "Uric Acid", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(210.0), ReferenceMax: ptr(440.0),
			OptimalMin: nil, OptimalMax: ptr(300.0),
		},
		{
			TestName: "Uric Acid", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(140.0), ReferenceMax: ptr(360.0),
			OptimalMin: nil, OptimalMax: ptr(300.0),
		},

		// ===== CREATININE (mg/dL) =====
		// Gender-specific marker of kidney function and muscle mass
		// Low creatinine in seniors may indicate sarcopenia (muscle loss)
		// Pediatric is unisex
		{
			TestName: "Creatinine", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.3), ReferenceMax: ptr(0.7),
			OptimalMin: ptr(0.3), OptimalMax: ptr(0.7), // Age dependent
		},
		// Adult+ is gender-specific
		{
			TestName: "Creatinine", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(0.74), ReferenceMax: ptr(1.35),
			OptimalMin: ptr(0.9), OptimalMax: ptr(1.2), // Good muscle mass
		},
		{
			TestName: "Creatinine", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(0.59), ReferenceMax: ptr(1.04),
			OptimalMin: ptr(0.7), OptimalMax: ptr(1.0),
		},
		{
			TestName: "Creatinine", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(0.74), ReferenceMax: ptr(1.35),
			OptimalMin: ptr(0.9), OptimalMax: ptr(1.2),
		},
		{
			TestName: "Creatinine", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(0.59), ReferenceMax: ptr(1.04),
			OptimalMin: ptr(0.7), OptimalMax: ptr(1.0),
		},
		{
			TestName: "Creatinine", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(0.70), ReferenceMax: ptr(1.30),
			OptimalMin: ptr(0.7), OptimalMax: ptr(1.30), // Avoid <0.7 = sarcopenia
		},
		{
			TestName: "Creatinine", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(0.59), ReferenceMax: ptr(1.04),
			OptimalMin: ptr(0.6), OptimalMax: ptr(1.04), // Avoid low = sarcopenia
		},

		// ===== CALCIUM (mmol/L) =====
		// Unisex across all ages
		{
			TestName: "Calcium", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(2.20), ReferenceMax: ptr(2.70),
			OptimalMin: ptr(2.20), OptimalMax: ptr(2.70), // Age dependent
		},
		{
			TestName: "Calcium", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(2.15), ReferenceMax: ptr(2.55),
			OptimalMin: ptr(2.25), OptimalMax: ptr(2.50),
		},
		{
			TestName: "Calcium", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(2.15), ReferenceMax: ptr(2.55),
			OptimalMin: ptr(2.25), OptimalMax: ptr(2.50),
		},
		{
			TestName: "Calcium", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(2.15), ReferenceMax: ptr(2.55),
			OptimalMin: ptr(2.25), OptimalMax: ptr(2.50),
		},

		// ===== BICARBONATE (mmol/L) =====
		// Unisex across all ages
		{
			TestName: "Bicarbonate", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(18.0), ReferenceMax: ptr(25.0),
			OptimalMin: ptr(18.0), OptimalMax: ptr(25.0), // Standard range
		},
		{
			TestName: "Bicarbonate", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(22.0), ReferenceMax: ptr(29.0),
			OptimalMin: ptr(24.0), OptimalMax: ptr(28.0),
		},
		{
			TestName: "Bicarbonate", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(22.0), ReferenceMax: ptr(29.0),
			OptimalMin: ptr(24.0), OptimalMax: ptr(28.0),
		},
		{
			TestName: "Bicarbonate", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(22.0), ReferenceMax: ptr(29.0),
			OptimalMin: ptr(24.0), OptimalMax: ptr(28.0),
		},

		// ===== SODIUM (mmol/L) =====
		// Unisex across all ages. Low levels (<135) common in elderly (hyponatremia)
		{
			TestName: "Sodium", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(136.0), ReferenceMax: ptr(145.0),
			OptimalMin: ptr(136.0), OptimalMax: ptr(145.0),
		},
		{
			TestName: "Sodium", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(136.0), ReferenceMax: ptr(145.0),
			OptimalMin: ptr(136.0), OptimalMax: ptr(145.0),
		},
		{
			TestName: "Sodium", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(136.0), ReferenceMax: ptr(145.0),
			OptimalMin: ptr(136.0), OptimalMax: ptr(145.0),
		},
		{
			TestName: "Sodium", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(136.0), ReferenceMax: ptr(145.0),
			OptimalMin: ptr(136.0), OptimalMax: ptr(145.0), // Watch for <135 = hyponatremia
		},

		// ===== POTASSIUM (mmol/L) =====
		// Unisex across all ages. High levels (>5.2) are dangerous for the heart
		{
			TestName: "Potassium", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(3.5), ReferenceMax: ptr(5.1),
			OptimalMin: ptr(3.5), OptimalMax: ptr(5.1),
		},
		{
			TestName: "Potassium", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(3.5), ReferenceMax: ptr(5.1),
			OptimalMin: ptr(3.5), OptimalMax: ptr(5.1), // Avoid >5.2 = cardiac risk
		},
		{
			TestName: "Potassium", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(3.5), ReferenceMax: ptr(5.1),
			OptimalMin: ptr(3.5), OptimalMax: ptr(5.1),
		},
		{
			TestName: "Potassium", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(3.5), ReferenceMax: ptr(5.1),
			OptimalMin: ptr(3.5), OptimalMax: ptr(5.1),
		},

		// ===== CHLORIDE (mmol/L) =====
		// Unisex across all ages. Tracks with Sodium
		{
			TestName: "Chloride", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(98.0), ReferenceMax: ptr(107.0),
			OptimalMin: ptr(98.0), OptimalMax: ptr(107.0),
		},
		{
			TestName: "Chloride", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(98.0), ReferenceMax: ptr(107.0),
			OptimalMin: ptr(98.0), OptimalMax: ptr(107.0),
		},
		{
			TestName: "Chloride", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(98.0), ReferenceMax: ptr(107.0),
			OptimalMin: ptr(98.0), OptimalMax: ptr(107.0),
		},
		{
			TestName: "Chloride", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(98.0), ReferenceMax: ptr(107.0),
			OptimalMin: ptr(98.0), OptimalMax: ptr(107.0),
		},

		// ===== LIPID PANEL =====

		// Total Cholesterol (mg/dL)
		// Note: Largely ignored in favor of ApoB according to Attia/Huberman
		{
			TestName: "Total Cholesterol", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(200.0),
			OptimalMin: nil, OptimalMax: nil, // Largely ignored in favor of ApoB
		},
		{
			TestName: "Total Cholesterol", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(200.0),
			OptimalMin: nil, OptimalMax: nil, // Largely ignored in favor of ApoB
		},
		{
			TestName: "Total Cholesterol", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(200.0),
			OptimalMin: nil, OptimalMax: nil, // Largely ignored in favor of ApoB
		},
		{
			TestName: "Total Cholesterol", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(200.0),
			OptimalMin: nil, OptimalMax: nil, // Largely ignored in favor of ApoB
		},

		// LDL Cholesterol (mg/dL)
		// Pediatric: Standard <110, no optimal defined
		{
			TestName: "LDL Cholesterol", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(110.0),
			OptimalMin: nil, OptimalMax: nil, // N/A for pediatric
		},
		// Adult/Senior: Reference <100 (low risk) or <70 (high risk)
		// Optimal: <70, ideally <30-40 for reversal (using 40 as upper bound)
		{
			TestName: "LDL Cholesterol", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(100.0), // Low risk
			OptimalMin: nil, OptimalMax: ptr(70.0), // If ApoB unavailable
		},
		{
			TestName: "LDL Cholesterol", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(100.0),
			OptimalMin: nil, OptimalMax: ptr(70.0),
		},
		{
			TestName: "LDL Cholesterol", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(100.0),
			OptimalMin: nil, OptimalMax: ptr(70.0),
		},

		// HDL Cholesterol (mg/dL)
		// Gender-specific: Male >40 reference, >50 optimal (functionally active)
		// Pediatric: Unisex
		{
			TestName: "HDL Cholesterol", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: nil,
			OptimalMin: ptr(50.0), OptimalMax: nil,
		},
		// Adult+ gender-specific
		{
			TestName: "HDL Cholesterol", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(40.0), ReferenceMax: nil,
			OptimalMin: ptr(50.0), OptimalMax: nil, // Functionally active
		},
		{
			TestName: "HDL Cholesterol", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(50.0), ReferenceMax: nil,
			OptimalMin: ptr(50.0), OptimalMax: nil, // Functionally active
		},
		{
			TestName: "HDL Cholesterol", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(40.0), ReferenceMax: nil,
			OptimalMin: ptr(50.0), OptimalMax: nil,
		},
		{
			TestName: "HDL Cholesterol", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(50.0), ReferenceMax: nil,
			OptimalMin: ptr(50.0), OptimalMax: nil,
		},
		{
			TestName: "HDL Cholesterol", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(40.0), ReferenceMax: nil,
			OptimalMin: ptr(50.0), OptimalMax: nil,
		},
		{
			TestName: "HDL Cholesterol", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(50.0), ReferenceMax: nil,
			OptimalMin: ptr(50.0), OptimalMax: nil,
		},

		// Triglycerides (mg/dL)
		// Reference <150, Optimal <100 (ideally <70)
		{
			TestName: "Triglycerides", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(150.0),
			OptimalMin: nil, OptimalMax: ptr(100.0),
		},
		{
			TestName: "Triglycerides", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(150.0),
			OptimalMin: nil, OptimalMax: ptr(100.0), // Ideally <70
		},
		{
			TestName: "Triglycerides", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(150.0),
			OptimalMin: nil, OptimalMax: ptr(100.0),
		},
		{
			TestName: "Triglycerides", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(150.0),
			OptimalMin: nil, OptimalMax: ptr(100.0),
		},

		// Non-HDL Cholesterol (mg/dL)
		// Proxy for ApoB when ApoB is unavailable
		// Reference <130, Optimal <90 (standard risk) or <60 (20th percentile/longevity)
		{
			TestName: "Non-HDL Cholesterol", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(130.0),
			OptimalMin: nil, OptimalMax: ptr(90.0),
		},
		{
			TestName: "Non-HDL Cholesterol", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(130.0),
			OptimalMin: nil, OptimalMax: ptr(90.0), // Proxy for ApoB
		},
		{
			TestName: "Non-HDL Cholesterol", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(130.0),
			OptimalMin: nil, OptimalMax: ptr(90.0),
		},
		{
			TestName: "Non-HDL Cholesterol", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(130.0),
			OptimalMin: nil, OptimalMax: ptr(90.0),
		},

		// Apolipoprotein B (ApoB) (mg/dL)
		// Gold standard for cardiovascular risk assessment
		// Reference: <90 (standard risk), <80 (high risk)
		// Optimal: <60 (20th percentile), <30-40 (physiologic/infant level for reversal)
		{
			TestName: "Apolipoprotein B", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(90.0),
			OptimalMin: nil, OptimalMax: ptr(60.0),
		},
		{
			TestName: "Apolipoprotein B", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(90.0), // Standard risk
			OptimalMin: nil, OptimalMax: ptr(60.0), // 20th percentile
		},
		{
			TestName: "Apolipoprotein B", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(90.0),
			OptimalMin: nil, OptimalMax: ptr(60.0),
		},
		{
			TestName: "Apolipoprotein B", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(90.0),
			OptimalMin: nil, OptimalMax: ptr(60.0),
		},

		// TG/HDL Ratio (calculated, unitless ratio)
		// Marker for insulin resistance and cardiovascular risk
		// Reference: <3.0 (high risk threshold)
		// Optimal: <1.0 (insulin sensitive), <2.0 (normal/good)
		{
			TestName: "TG/HDL (Calc)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0),
			OptimalMin: nil, OptimalMax: ptr(1.0), // Insulin sensitive
		},
		{
			TestName: "TG/HDL (Calc)", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0), // >3.0 = insulin resistant
			OptimalMin: nil, OptimalMax: ptr(1.0), // <1.0 = insulin sensitive
		},
		{
			TestName: "TG/HDL (Calc)", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0),
			OptimalMin: nil, OptimalMax: ptr(1.0),
		},
		{
			TestName: "TG/HDL (Calc)", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0),
			OptimalMin: nil, OptimalMax: ptr(1.0),
		},

		// Atherogenic Coefficient (calculated, unitless ratio)
		// Formula: (Total Cholesterol - HDL Cholesterol) / HDL Cholesterol
		// Reference: <3.0 (low risk), 3.0-4.0 (moderate risk), >4.0 (high risk)
		// Optimal: <2.0 (longevity target)
		{
			TestName: "Atherogenic Coefficient", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0), // Low risk
			OptimalMin: nil, OptimalMax: ptr(2.0), // Longevity
		},
		{
			TestName: "Atherogenic Coefficient", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0), // Low risk
			OptimalMin: nil, OptimalMax: ptr(2.0), // Longevity
		},
		{
			TestName: "Atherogenic Coefficient", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0),
			OptimalMin: nil, OptimalMax: ptr(2.0),
		},
		{
			TestName: "Atherogenic Coefficient", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(3.0),
			OptimalMin: nil, OptimalMax: ptr(2.0),
		},

		// ===== LIVER FUNCTION TESTS =====

		// SGPT (ALT) - Alanine Aminotransferase (IU/L)
		// Gender-specific. Attia strict optimal: Male <30 (ideally <22), Female <20 (ideally <17)
		// Note: Pediatric 0-12 months has higher range (10-55) than 1-17 yrs
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgePediatric, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(35.0), // 1-17 yrs range
			OptimalMin: nil, OptimalMax: ptr(25.0),
		},
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgePediatric, Gender: GenderFemale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(30.0), // 1-17 yrs range
			OptimalMin: nil, OptimalMax: ptr(22.0),
		},
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(50.0),
			OptimalMin: nil, OptimalMax: ptr(30.0), // Ideally <22
		},
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(35.0),
			OptimalMin: nil, OptimalMax: ptr(20.0), // Ideally <17
		},
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(50.0),
			OptimalMin: nil, OptimalMax: ptr(30.0),
		},
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(35.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(50.0),
			OptimalMin: nil, OptimalMax: ptr(30.0),
		},
		{
			TestName: "SGPT (ALT), Serum", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(35.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},

		// SGOT (AST) - Aspartate Aminotransferase (IU/L)
		// Gender-specific for adults. Attia strict: Male <25, Female <20
		{
			TestName: "SGOT (AST)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(15.0), ReferenceMax: ptr(50.0),
			OptimalMin: ptr(15.0), OptimalMax: ptr(50.0), // Standard range
		},
		{
			TestName: "SGOT (AST)", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: ptr(25.0),
		},
		{
			TestName: "SGOT (AST)", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(35.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},
		{
			TestName: "SGOT (AST)", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: ptr(25.0),
		},
		{
			TestName: "SGOT (AST)", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(35.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},
		{
			TestName: "SGOT (AST)", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: ptr(25.0),
		},
		{
			TestName: "SGOT (AST)", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(35.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},

		// GGT - Gamma-Glutamyl Transferase (IU/L)
		// Gender-specific. Optimal: Male <20 (strict), Female <15 (strict)
		{
			TestName: "GGT", AgeRange: AgePediatric, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(71.0),
			OptimalMin: nil, OptimalMax: ptr(20.0), // Strict: <20, Lenient: <30
		},
		{
			TestName: "GGT", AgeRange: AgePediatric, Gender: GenderFemale,
			ReferenceMin: ptr(6.0), ReferenceMax: ptr(42.0),
			OptimalMin: nil, OptimalMax: ptr(15.0), // Strict: <15, Lenient: <20
		},
		{
			TestName: "GGT", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(71.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},
		{
			TestName: "GGT", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(6.0), ReferenceMax: ptr(42.0),
			OptimalMin: nil, OptimalMax: ptr(15.0),
		},
		{
			TestName: "GGT", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(71.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},
		{
			TestName: "GGT", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(6.0), ReferenceMax: ptr(42.0),
			OptimalMin: nil, OptimalMax: ptr(15.0),
		},
		{
			TestName: "GGT", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(71.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},
		{
			TestName: "GGT", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(6.0), ReferenceMax: ptr(42.0),
			OptimalMin: nil, OptimalMax: ptr(15.0),
		},

		// Bilirubin Total (mg/dL)
		// Unisex across all ages. <1.0 unless Gilbert's syndrome
		{
			TestName: "Bilirubin Total", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.3), ReferenceMax: ptr(1.2),
			OptimalMin: nil, OptimalMax: ptr(1.0),
		},
		{
			TestName: "Bilirubin Total", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.3), ReferenceMax: ptr(1.2),
			OptimalMin: nil, OptimalMax: ptr(1.0), // Unless Gilbert's
		},
		{
			TestName: "Bilirubin Total", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.3), ReferenceMax: ptr(1.2),
			OptimalMin: nil, OptimalMax: ptr(1.0),
		},
		{
			TestName: "Bilirubin Total", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.3), ReferenceMax: ptr(1.2),
			OptimalMin: nil, OptimalMax: ptr(1.0),
		},

		// Bilirubin Direct (mg/dL)
		// Unisex across all ages. Must be low <0.2
		{
			TestName: "Bilirubin Direct", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(0.3),
			OptimalMin: nil, OptimalMax: ptr(0.2),
		},
		{
			TestName: "Bilirubin Direct", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(0.3),
			OptimalMin: nil, OptimalMax: ptr(0.2),
		},
		{
			TestName: "Bilirubin Direct", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(0.3),
			OptimalMin: nil, OptimalMax: ptr(0.2),
		},
		{
			TestName: "Bilirubin Direct", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(0.3),
			OptimalMin: nil, OptimalMax: ptr(0.2),
		},

		// Bilirubin Indirect (mg/dL)
		// Unisex across all ages. If >1.0 with normal Direct = benign Gilbert's
		{
			TestName: "Bilirubin Indirect", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(0.8),
			OptimalMin: ptr(0.2), OptimalMax: ptr(0.8),
		},
		{
			TestName: "Bilirubin Indirect", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(0.8),
			OptimalMin: ptr(0.2), OptimalMax: ptr(0.8), // Watch for >1.0
		},
		{
			TestName: "Bilirubin Indirect", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(0.8),
			OptimalMin: ptr(0.2), OptimalMax: ptr(0.8),
		},
		{
			TestName: "Bilirubin Indirect", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.2), ReferenceMax: ptr(0.8),
			OptimalMin: ptr(0.2), OptimalMax: ptr(0.8),
		},

		// Alkaline Phosphatase (ALP) (IU/L)
		// Unisex. Pediatric range much higher due to growth spurts (100-500+)
		{
			TestName: "Alkaline Phosphatase (ALP)", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(100.0), ReferenceMax: ptr(500.0),
			OptimalMin: ptr(100.0), OptimalMax: ptr(500.0), // Don't panic if high in kids
		},
		{
			TestName: "Alkaline Phosphatase (ALP)", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(130.0),
			OptimalMin: ptr(60.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Alkaline Phosphatase (ALP)", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(130.0),
			OptimalMin: ptr(60.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Alkaline Phosphatase (ALP)", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(40.0), ReferenceMax: ptr(130.0),
			OptimalMin: ptr(60.0), OptimalMax: ptr(100.0),
		},

		// Albumin (g/dL)
		// Unisex. Higher is better for longevity. Critical to maintain >4.0 in seniors
		{
			TestName: "Albumin", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(3.5), ReferenceMax: ptr(5.0),
			OptimalMin: ptr(3.5), OptimalMax: ptr(5.0), // Standard range
		},
		{
			TestName: "Albumin", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(3.5), ReferenceMax: ptr(5.2),
			OptimalMin: ptr(4.5), OptimalMax: ptr(5.0), // Higher is better
		},
		{
			TestName: "Albumin", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(3.5), ReferenceMax: ptr(5.2),
			OptimalMin: ptr(4.5), OptimalMax: ptr(5.0),
		},
		{
			TestName: "Albumin", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(3.2), ReferenceMax: ptr(4.8),
			OptimalMin: ptr(4.0), OptimalMax: ptr(4.8), // Critical to maintain >4.0
		},

		// Total Protein (g/dL)
		// Unisex across all ages
		{
			TestName: "Total Protein", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(6.4), ReferenceMax: ptr(8.3),
			OptimalMin: ptr(6.8), OptimalMax: ptr(7.8),
		},
		{
			TestName: "Total Protein", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(6.4), ReferenceMax: ptr(8.3),
			OptimalMin: ptr(6.8), OptimalMax: ptr(7.8),
		},
		{
			TestName: "Total Protein", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(6.4), ReferenceMax: ptr(8.3),
			OptimalMin: ptr(6.8), OptimalMax: ptr(7.8),
		},
		{
			TestName: "Total Protein", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(6.4), ReferenceMax: ptr(8.3),
			OptimalMin: ptr(6.8), OptimalMax: ptr(7.8),
		},

		// Globulin (g/dL)
		// Unisex across all ages
		{
			TestName: "Globulin", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(3.5),
			OptimalMin: ptr(2.2), OptimalMax: ptr(2.8),
		},
		{
			TestName: "Globulin", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(3.5),
			OptimalMin: ptr(2.2), OptimalMax: ptr(2.8),
		},
		{
			TestName: "Globulin", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(3.5),
			OptimalMin: ptr(2.2), OptimalMax: ptr(2.8),
		},
		{
			TestName: "Globulin", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(2.0), ReferenceMax: ptr(3.5),
			OptimalMin: ptr(2.2), OptimalMax: ptr(2.8),
		},

		// ===== VITAMINS & MINERALS =====

		// Vitamin D (nmol/L)
		// Unisex across all ages. Target 100-150 for immunity/testosterone
		// <50 = deficient, 50-75 = insufficient, 75-250 = optimal
		{
			TestName: "Vitamin D", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(75.0), ReferenceMax: ptr(250.0),
			OptimalMin: ptr(100.0), OptimalMax: ptr(150.0), // Immunity/testosterone
		},
		{
			TestName: "Vitamin D", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(75.0), ReferenceMax: ptr(250.0),
			OptimalMin: ptr(100.0), OptimalMax: ptr(150.0),
		},
		{
			TestName: "Vitamin D", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(75.0), ReferenceMax: ptr(250.0),
			OptimalMin: ptr(100.0), OptimalMax: ptr(150.0),
		},
		{
			TestName: "Vitamin D", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(75.0), ReferenceMax: ptr(250.0),
			OptimalMin: ptr(100.0), OptimalMax: ptr(150.0),
		},

		// Vitamin B12 (pmol/L)
		// Unisex across all ages. <150 = deficient, 150-300 = grey zone (monitor closely)
		// Optimal: >370, ideally >450
		{
			TestName: "Vitamin B12", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(150.0), ReferenceMax: ptr(650.0),
			OptimalMin: ptr(370.0), OptimalMax: ptr(650.0), // Ideally >450
		},
		{
			TestName: "Vitamin B12", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(150.0), ReferenceMax: ptr(650.0),
			OptimalMin: ptr(370.0), OptimalMax: ptr(650.0),
		},
		{
			TestName: "Vitamin B12", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(150.0), ReferenceMax: ptr(650.0),
			OptimalMin: ptr(370.0), OptimalMax: ptr(650.0),
		},
		{
			TestName: "Vitamin B12", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(150.0), ReferenceMax: ptr(650.0),
			OptimalMin: ptr(370.0), OptimalMax: ptr(650.0),
		},

		// Magnesium, Serum (mmol/L)
		// Unisex across all ages
		{
			TestName: "Magnesium, Serum", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.65), ReferenceMax: ptr(1.05),
			OptimalMin: ptr(0.85), OptimalMax: ptr(1.10),
		},
		{
			TestName: "Magnesium, Serum", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.65), ReferenceMax: ptr(1.05),
			OptimalMin: ptr(0.85), OptimalMax: ptr(1.10),
		},
		{
			TestName: "Magnesium, Serum", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.65), ReferenceMax: ptr(1.05),
			OptimalMin: ptr(0.85), OptimalMax: ptr(1.10),
		},
		{
			TestName: "Magnesium, Serum", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.65), ReferenceMax: ptr(1.05),
			OptimalMin: ptr(0.85), OptimalMax: ptr(1.10),
		},

		// Iron, Serum (umol/L)
		// Gender-specific for adults
		{
			TestName: "Iron, Serum", AgeRange: AgePediatric, Gender: GenderMale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(28.0),
			OptimalMin: ptr(12.5), OptimalMax: ptr(23.0),
		},
		{
			TestName: "Iron, Serum", AgeRange: AgePediatric, Gender: GenderFemale,
			ReferenceMin: ptr(6.6), ReferenceMax: ptr(26.0),
			OptimalMin: ptr(11.0), OptimalMax: ptr(21.5),
		},
		{
			TestName: "Iron, Serum", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(28.0),
			OptimalMin: ptr(12.5), OptimalMax: ptr(23.0),
		},
		{
			TestName: "Iron, Serum", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(6.6), ReferenceMax: ptr(26.0),
			OptimalMin: ptr(11.0), OptimalMax: ptr(21.5),
		},
		{
			TestName: "Iron, Serum", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(28.0),
			OptimalMin: ptr(12.5), OptimalMax: ptr(23.0),
		},
		{
			TestName: "Iron, Serum", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(6.6), ReferenceMax: ptr(26.0),
			OptimalMin: ptr(11.0), OptimalMax: ptr(21.5),
		},
		{
			TestName: "Iron, Serum", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(28.0),
			OptimalMin: ptr(12.5), OptimalMax: ptr(23.0),
		},
		{
			TestName: "Iron, Serum", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(6.6), ReferenceMax: ptr(26.0),
			OptimalMin: ptr(11.0), OptimalMax: ptr(21.5),
		},

		// Ferritin (ng/mL)
		// Gender-specific. Target 30-100 for both sexes (functional medicine optimal)
		{
			TestName: "Ferritin", AgeRange: AgePediatric, Gender: GenderMale,
			ReferenceMin: ptr(24.0), ReferenceMax: ptr(336.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Ferritin", AgeRange: AgePediatric, Gender: GenderFemale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(307.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Ferritin", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(24.0), ReferenceMax: ptr(336.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Ferritin", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(307.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Ferritin", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(24.0), ReferenceMax: ptr(336.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Ferritin", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(307.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Ferritin", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(24.0), ReferenceMax: ptr(336.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},
		{
			TestName: "Ferritin", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(11.0), ReferenceMax: ptr(307.0),
			OptimalMin: ptr(30.0), OptimalMax: ptr(100.0),
		},

		// Zinc (umol/L)
		// Unisex across all ages. Target upper half of range (14.0-18.0)
		{
			TestName: "Zinc", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(18.0),
			OptimalMin: ptr(14.0), OptimalMax: ptr(18.0), // Upper half
		},
		{
			TestName: "Zinc", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(18.0),
			OptimalMin: ptr(14.0), OptimalMax: ptr(18.0),
		},
		{
			TestName: "Zinc", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(18.0),
			OptimalMin: ptr(14.0), OptimalMax: ptr(18.0),
		},
		{
			TestName: "Zinc", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(10.0), ReferenceMax: ptr(18.0),
			OptimalMin: ptr(14.0), OptimalMax: ptr(18.0),
		},

		// ===== ENDOCRINE & OTHER =====

		// TSH - Thyroid Stimulating Hormone (uIU/mL)
		// Unisex. Functional optimal: 0.5-2.5 (adult), 0.5-3.0 (senior - naturally drifts up)
		{
			TestName: "TSH", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.7), ReferenceMax: ptr(6.0),
			OptimalMin: ptr(0.7), OptimalMax: ptr(6.0), // Age dependent
		},
		{
			TestName: "TSH", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: ptr(0.40), ReferenceMax: ptr(4.50),
			OptimalMin: ptr(0.50), OptimalMax: ptr(2.50),
		},
		{
			TestName: "TSH", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: ptr(0.40), ReferenceMax: ptr(4.50),
			OptimalMin: ptr(0.50), OptimalMax: ptr(2.50),
		},
		{
			TestName: "TSH", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: ptr(0.40), ReferenceMax: ptr(5.80),
			OptimalMin: ptr(0.50), OptimalMax: ptr(3.00), // Naturally drifts up
		},

		// Haemoglobin HbA1c (%)
		// Unisex across all ages. <5.7% = normal, 5.7-6.4% = pre-diabetes
		// Strict optimal: 4.8-5.1%
		{
			TestName: "Haemoglobin HbA1c", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(5.7),
			OptimalMin: ptr(4.8), OptimalMax: ptr(5.1),
		},
		{
			TestName: "Haemoglobin HbA1c", AgeRange: AgeAdult, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(5.7),
			OptimalMin: ptr(4.8), OptimalMax: ptr(5.1),
		},
		{
			TestName: "Haemoglobin HbA1c", AgeRange: AgeMiddleAge, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(5.7),
			OptimalMin: ptr(4.8), OptimalMax: ptr(5.1),
		},
		{
			TestName: "Haemoglobin HbA1c", AgeRange: AgeSenior, Gender: GenderUnisex,
			ReferenceMin: nil, ReferenceMax: ptr(5.7),
			OptimalMin: ptr(4.8), OptimalMax: ptr(5.1),
		},

		// ESR - Erythrocyte Sedimentation Rate (mm/h)
		// Gender-specific, age-dependent. General inflammatory marker
		{
			TestName: "ESR", AgeRange: AgePediatric, Gender: GenderUnisex,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(10.0),
			OptimalMin: nil, OptimalMax: ptr(5.0),
		},
		{
			TestName: "ESR", AgeRange: AgeAdult, Gender: GenderMale,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(15.0),
			OptimalMin: nil, OptimalMax: ptr(10.0),
		},
		{
			TestName: "ESR", AgeRange: AgeAdult, Gender: GenderFemale,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(20.0),
			OptimalMin: nil, OptimalMax: ptr(10.0),
		},
		{
			TestName: "ESR", AgeRange: AgeMiddleAge, Gender: GenderMale,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(20.0),
			OptimalMin: nil, OptimalMax: ptr(10.0),
		},
		{
			TestName: "ESR", AgeRange: AgeMiddleAge, Gender: GenderFemale,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(30.0),
			OptimalMin: nil, OptimalMax: ptr(15.0),
		},
		{
			TestName: "ESR", AgeRange: AgeSenior, Gender: GenderMale,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(30.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},
		{
			TestName: "ESR", AgeRange: AgeSenior, Gender: GenderFemale,
			ReferenceMin: ptr(0.0), ReferenceMax: ptr(40.0),
			OptimalMin: nil, OptimalMax: ptr(20.0),
		},

		// TODO: Add remaining lab tests here with appropriate ranges
		// Follow the same pattern for all predefined tests
	}
}

// SyncReferenceRanges synchronizes reference ranges from Go code to the database
// This is called on application startup to ensure database has latest ranges
func SyncReferenceRanges(ctx context.Context) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	definitions := GetReferenceRangeDefinitions()
	logger.Infof("Syncing %d reference range definitions to database...", len(definitions))

	// Use UPSERT (INSERT ... ON CONFLICT DO UPDATE) for each range
	query := `
		INSERT INTO reference_ranges (test_name, age_range, gender, reference_min, reference_max, optimal_min, optimal_max)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (test_name, age_range, gender)
		DO UPDATE SET
			reference_min = EXCLUDED.reference_min,
			reference_max = EXCLUDED.reference_max,
			optimal_min = EXCLUDED.optimal_min,
			optimal_max = EXCLUDED.optimal_max,
			updated_at = now()
	`

	syncCount := 0

	for _, def := range definitions {
		_, err := pool.Exec(ctx, query,
			def.TestName, def.AgeRange, def.Gender,
			def.ReferenceMin, def.ReferenceMax,
			def.OptimalMin, def.OptimalMax,
		)
		if err != nil {
			return fmt.Errorf("failed to sync reference range for %s/%s/%s: %w",
				def.TestName, def.AgeRange, def.Gender, err)
		}

		syncCount++
	}

	logger.Infof("Successfully synced %d reference ranges", syncCount)

	return nil
}

// GetReferenceRange retrieves the reference range for a given test, age, and gender
func GetReferenceRange(ctx context.Context, testName string, ageRange AgeRange, gender Gender) (*ReferenceRange, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	var rr ReferenceRange

	// First try exact match (test + age + gender)
	query := `
		SELECT id, test_name, age_range, gender, reference_min, reference_max, optimal_min, optimal_max, created_at, updated_at
		FROM reference_ranges
		WHERE test_name = $1 AND age_range = $2 AND gender = $3
	`

	err := pool.QueryRow(ctx, query, testName, ageRange, gender).Scan(
		&rr.ID, &rr.TestName, &rr.AgeRange, &rr.Gender,
		&rr.ReferenceMin, &rr.ReferenceMax,
		&rr.OptimalMin, &rr.OptimalMax,
		&rr.CreatedAt, &rr.UpdatedAt,
	)
	if err == nil {
		return &rr, nil
	}

	// If no gender-specific match, try unisex
	err = pool.QueryRow(ctx, query, testName, ageRange, GenderUnisex).Scan(
		&rr.ID, &rr.TestName, &rr.AgeRange, &rr.Gender,
		&rr.ReferenceMin, &rr.ReferenceMax,
		&rr.OptimalMin, &rr.OptimalMax,
		&rr.CreatedAt, &rr.UpdatedAt,
	)
	if err == nil {
		return &rr, nil
	}

	// No matching range found
	return nil, nil //nolint:nilnil // Missing reference ranges are expected for some tests.
}
