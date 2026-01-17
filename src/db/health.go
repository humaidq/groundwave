/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ========== Health Profile Operations ==========

// ListHealthProfiles returns all health profiles with follow-up counts
func ListHealthProfiles(ctx context.Context) ([]HealthProfileSummary, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, name, date_of_birth, gender, description, is_primary, created_at, updated_at, followup_count, last_followup_date
		FROM health_profiles_summary
		ORDER BY name ASC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list health profiles: %w", err)
	}
	defer rows.Close()

	var profiles []HealthProfileSummary
	for rows.Next() {
		var profile HealthProfileSummary
		err := rows.Scan(
			&profile.ID, &profile.Name, &profile.DateOfBirth, &profile.Gender, &profile.Description,
			&profile.IsPrimary,
			&profile.CreatedAt, &profile.UpdatedAt,
			&profile.FollowupCount, &profile.LastFollowupDate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan profile: %w", err)
		}
		profiles = append(profiles, profile)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating profiles: %w", err)
	}

	return profiles, nil
}

// GetHealthProfile returns a single health profile by ID
func GetHealthProfile(ctx context.Context, id string) (*HealthProfile, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var profile HealthProfile
	query := `
		SELECT id, name, date_of_birth, gender, description, is_primary, created_at, updated_at
		FROM health_profiles
		WHERE id = $1
	`

	err := pool.QueryRow(ctx, query, id).Scan(
		&profile.ID, &profile.Name, &profile.DateOfBirth, &profile.Gender, &profile.Description,
		&profile.IsPrimary,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get health profile: %w", err)
	}

	return &profile, nil
}

// GetPrimaryHealthProfile returns the primary health profile, if any.
func GetPrimaryHealthProfile(ctx context.Context) (*HealthProfile, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var profile HealthProfile
	query := `
		SELECT id, name, date_of_birth, gender, description, is_primary, created_at, updated_at
		FROM health_profiles
		WHERE is_primary = true
		LIMIT 1
	`

	err := pool.QueryRow(ctx, query).Scan(
		&profile.ID, &profile.Name, &profile.DateOfBirth, &profile.Gender, &profile.Description,
		&profile.IsPrimary,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get primary health profile: %w", err)
	}

	return &profile, nil
}

// CreateHealthProfile creates a new health profile
func CreateHealthProfile(ctx context.Context, name string, dob *time.Time, gender *Gender, description *string, isPrimary bool) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if isPrimary {
		_, err = tx.Exec(ctx, `UPDATE health_profiles SET is_primary = false WHERE is_primary = true`)
		if err != nil {
			return "", fmt.Errorf("failed to clear existing primary profile: %w", err)
		}
	}

	var id string
	query := `
		INSERT INTO health_profiles (name, date_of_birth, gender, description, is_primary)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	err = tx.QueryRow(ctx, query, name, dob, gender, description, isPrimary).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create health profile: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit health profile creation: %w", err)
	}

	return id, nil
}

// UpdateHealthProfile updates a health profile
func UpdateHealthProfile(ctx context.Context, id, name string, dob *time.Time, gender *Gender, description *string, isPrimary bool) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if isPrimary {
		_, err = tx.Exec(ctx, `UPDATE health_profiles SET is_primary = false WHERE is_primary = true AND id <> $1`, id)
		if err != nil {
			return fmt.Errorf("failed to clear existing primary profile: %w", err)
		}
	}

	query := `
		UPDATE health_profiles
		SET name = $1, date_of_birth = $2, gender = $3, description = $4, is_primary = $5
		WHERE id = $6
	`

	_, err = tx.Exec(ctx, query, name, dob, gender, description, isPrimary, id)
	if err != nil {
		return fmt.Errorf("failed to update health profile: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit health profile update: %w", err)
	}

	return nil
}

// DeleteHealthProfile deletes a health profile (cascades to follow-ups and results)
func DeleteHealthProfile(ctx context.Context, id string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM health_profiles WHERE id = $1`
	_, err := pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete health profile: %w", err)
	}

	return nil
}

// ========== Health Follow-up Operations ==========

// CreateFollowupInput represents input for creating a follow-up
type CreateFollowupInput struct {
	ProfileID    string
	FollowupDate time.Time
	HospitalName string
	Notes        *string
}

// UpdateFollowupInput represents input for updating a follow-up
type UpdateFollowupInput struct {
	FollowupDate time.Time
	HospitalName string
	Notes        *string
}

// ListFollowups returns all follow-ups for a profile
func ListFollowups(ctx context.Context, profileID string) ([]HealthFollowupSummary, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, profile_id, followup_date, hospital_name, notes,
		       created_at, updated_at, result_count
		FROM health_followups_summary
		WHERE profile_id = $1
		ORDER BY followup_date DESC
	`

	rows, err := pool.Query(ctx, query, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to list follow-ups: %w", err)
	}
	defer rows.Close()

	var followups []HealthFollowupSummary
	for rows.Next() {
		var followup HealthFollowupSummary
		err := rows.Scan(
			&followup.ID, &followup.ProfileID, &followup.FollowupDate,
			&followup.HospitalName, &followup.Notes, &followup.CreatedAt,
			&followup.UpdatedAt, &followup.ResultCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan follow-up: %w", err)
		}
		followups = append(followups, followup)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating follow-ups: %w", err)
	}

	return followups, nil
}

// GetFollowup returns a single follow-up by ID
func GetFollowup(ctx context.Context, id string) (*HealthFollowup, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var followup HealthFollowup
	query := `
		SELECT id, profile_id, followup_date, hospital_name, notes,
		       created_at, updated_at
		FROM health_followups
		WHERE id = $1
	`

	err := pool.QueryRow(ctx, query, id).Scan(
		&followup.ID, &followup.ProfileID, &followup.FollowupDate,
		&followup.HospitalName, &followup.Notes, &followup.CreatedAt,
		&followup.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get follow-up: %w", err)
	}

	return &followup, nil
}

// CreateFollowup creates a new follow-up
func CreateFollowup(ctx context.Context, input CreateFollowupInput) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	var id string
	query := `
		INSERT INTO health_followups (profile_id, followup_date, hospital_name, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	err := pool.QueryRow(ctx, query,
		input.ProfileID, input.FollowupDate, input.HospitalName, input.Notes,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create follow-up: %w", err)
	}

	return id, nil
}

// UpdateFollowup updates a follow-up's details
func UpdateFollowup(ctx context.Context, id string, input UpdateFollowupInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `
		UPDATE health_followups
		SET followup_date = $1, hospital_name = $2, notes = $3
		WHERE id = $4
	`

	_, err := pool.Exec(ctx, query,
		input.FollowupDate, input.HospitalName, input.Notes, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update follow-up: %w", err)
	}

	return nil
}

// DeleteFollowup deletes a follow-up (cascades to lab results)
func DeleteFollowup(ctx context.Context, id string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM health_followups WHERE id = $1`
	_, err := pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete follow-up: %w", err)
	}

	return nil
}

// ========== Lab Result Operations ==========

// CreateLabResultInput represents input for creating a lab result
type CreateLabResultInput struct {
	FollowupID string
	TestName   string
	TestUnit   *string
	TestValue  float64
}

// UpdateLabResultInput represents input for updating a lab result
type UpdateLabResultInput struct {
	TestValue float64
}

// calculateAbsoluteCounts generates calculated absolute counts from WBC and percentage values
// Also calculates TG/HDL ratio (always calculated, never uses stored values)
func calculateAbsoluteCounts(results []HealthLabResult) []HealthLabResultDisplay {
	calculated := make([]HealthLabResultDisplay, 0)

	// Find values needed for calculations
	var wbcValue, triglycerides, hdlCholesterol *float64
	var triglyceridesCreatedAt, hdlCreatedAt time.Time
	var triglyceridesFollowupID uuid.UUID

	for _, r := range results {
		switch r.TestName {
		case "White blood cells":
			wbcValue = &r.TestValue
		case "Triglycerides":
			triglycerides = &r.TestValue
			triglyceridesCreatedAt = r.CreatedAt
			triglyceridesFollowupID = r.FollowupID
		case "HDL Cholesterol":
			hdlCholesterol = &r.TestValue
			hdlCreatedAt = r.CreatedAt
		}
	}

	// Calculate absolute counts from WBC and percentages
	if wbcValue != nil {
		// Map of percentage test names to their absolute count equivalents
		percentageToAbsolute := map[string]string{
			"Neutrophils": "Neutrophils (Absolute)",
			"Lymphocytes": "Lymphocytes (Absolute)",
			"Monocytes":   "Monocytes (Absolute)",
			"Eosinophils": "Eosinophils (Absolute)",
			"Basophils":   "Basophils (Absolute)",
		}

		for _, r := range results {
			// Check if this is a percentage test that has an absolute equivalent
			if absoluteName, ok := percentageToAbsolute[r.TestName]; ok {
				// Check if the absolute value was already entered manually
				alreadyExists := false
				for _, existing := range results {
					if existing.TestName == absoluteName {
						alreadyExists = true
						break
					}
				}

				// Only calculate if not already entered
				if !alreadyExists {
					absoluteValue := (*wbcValue * r.TestValue) / 100.0
					unit := "×10³/μL"
					calculated = append(calculated, HealthLabResultDisplay{
						HealthLabResult: HealthLabResult{
							ID:         r.ID, // Use same ID as percentage (but won't be used for edit)
							FollowupID: r.FollowupID,
							TestName:   absoluteName,
							TestUnit:   &unit,
							TestValue:  absoluteValue,
							CreatedAt:  r.CreatedAt,
						},
						IsCalculated: true,
					})
				}
			}
		}
	}

	// Calculate TG/HDL ratio - ALWAYS calculated, never use stored value
	if triglycerides != nil && hdlCholesterol != nil && *hdlCholesterol != 0 {
		ratio := *triglycerides / *hdlCholesterol
		unit := "ratio"
		// Use the more recent created_at date
		createdAt := triglyceridesCreatedAt
		if hdlCreatedAt.After(triglyceridesCreatedAt) {
			createdAt = hdlCreatedAt
		}
		calculated = append(calculated, HealthLabResultDisplay{
			HealthLabResult: HealthLabResult{
				FollowupID: triglyceridesFollowupID,
				TestName:   "TG/HDL (Calc)",
				TestUnit:   &unit,
				TestValue:  ratio,
				CreatedAt:  createdAt,
			},
			IsCalculated: true,
		})
	}

	// Calculate Atherogenic Coefficient - ALWAYS calculated, never use stored value
	// Formula: (Total Cholesterol - HDL Cholesterol) / HDL Cholesterol
	var totalCholesterol *float64
	var totalCholCreatedAt time.Time
	var totalCholFollowupID uuid.UUID

	for _, r := range results {
		if r.TestName == "Total Cholesterol" {
			totalCholesterol = &r.TestValue
			totalCholCreatedAt = r.CreatedAt
			totalCholFollowupID = r.FollowupID
			break
		}
	}

	if totalCholesterol != nil && hdlCholesterol != nil && *hdlCholesterol != 0 {
		coefficient := (*totalCholesterol - *hdlCholesterol) / *hdlCholesterol
		unit := "ratio"
		// Use the more recent created_at date
		createdAt := totalCholCreatedAt
		if hdlCreatedAt.After(totalCholCreatedAt) {
			createdAt = hdlCreatedAt
		}
		calculated = append(calculated, HealthLabResultDisplay{
			HealthLabResult: HealthLabResult{
				FollowupID: totalCholFollowupID,
				TestName:   "Atherogenic Coefficient",
				TestUnit:   &unit,
				TestValue:  coefficient,
				CreatedAt:  createdAt,
			},
			IsCalculated: true,
		})
	}

	return calculated
}

// GetLabResultsByFollowup returns all lab results for a follow-up, grouped by category
func GetLabResultsByFollowup(ctx context.Context, followupID string) (map[LabTestCategory][]HealthLabResult, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, followup_id, test_name, test_unit, test_value, created_at
		FROM health_lab_results
		WHERE followup_id = $1
		ORDER BY created_at ASC
	`

	rows, err := pool.Query(ctx, query, followupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lab results: %w", err)
	}
	defer rows.Close()

	var allResults []HealthLabResult
	for rows.Next() {
		var result HealthLabResult
		err := rows.Scan(
			&result.ID, &result.FollowupID, &result.TestName,
			&result.TestUnit, &result.TestValue, &result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lab result: %w", err)
		}
		allResults = append(allResults, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating lab results: %w", err)
	}

	// Group results by category
	grouped := make(map[LabTestCategory][]HealthLabResult)
	predefinedTests := GetPredefinedLabTests()
	testCategoryMap := make(map[string]LabTestCategory)
	for _, test := range predefinedTests {
		testCategoryMap[test.Name] = test.Category
	}

	for _, result := range allResults {
		category, found := testCategoryMap[result.TestName]
		if !found {
			// Unknown test, put in "Other" category
			category = CategoryEndocrineOther
		}
		grouped[category] = append(grouped[category], result)
	}

	return grouped, nil
}

// GetLabResultsByFollowupWithCalculated returns all lab results including calculated absolute counts
func GetLabResultsByFollowupWithCalculated(ctx context.Context, followupID string) (map[LabTestCategory][]HealthLabResultDisplay, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, followup_id, test_name, test_unit, test_value, created_at
		FROM health_lab_results
		WHERE followup_id = $1
		ORDER BY created_at ASC
	`

	rows, err := pool.Query(ctx, query, followupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lab results: %w", err)
	}
	defer rows.Close()

	var allResults []HealthLabResult
	for rows.Next() {
		var result HealthLabResult
		err := rows.Scan(
			&result.ID, &result.FollowupID, &result.TestName,
			&result.TestUnit, &result.TestValue, &result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lab result: %w", err)
		}
		allResults = append(allResults, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating lab results: %w", err)
	}

	// Filter out any manually-entered calculated values (like TG/HDL ratio, Atherogenic Coefficient)
	// These should ALWAYS be calculated, never stored
	filteredResults := make([]HealthLabResult, 0, len(allResults))
	for _, r := range allResults {
		if r.TestName != "TG/HDL (Calc)" && r.TestName != "Atherogenic Coefficient" {
			filteredResults = append(filteredResults, r)
		}
	}

	// Calculate absolute counts and ratios
	calculatedResults := calculateAbsoluteCounts(filteredResults)

	// Convert all actual results to display format
	allDisplayResults := make([]HealthLabResultDisplay, 0, len(filteredResults)+len(calculatedResults))
	for _, r := range filteredResults {
		allDisplayResults = append(allDisplayResults, HealthLabResultDisplay{
			HealthLabResult: r,
			IsCalculated:    false,
		})
	}
	allDisplayResults = append(allDisplayResults, calculatedResults...)

	// Group results by category
	grouped := make(map[LabTestCategory][]HealthLabResultDisplay)
	predefinedTests := GetPredefinedLabTests()
	testCategoryMap := make(map[string]LabTestCategory)
	for _, test := range predefinedTests {
		testCategoryMap[test.Name] = test.Category
	}

	for _, result := range allDisplayResults {
		category, found := testCategoryMap[result.TestName]
		if !found {
			// Unknown test, put in "Other" category
			category = CategoryEndocrineOther
		}
		grouped[category] = append(grouped[category], result)
	}

	return grouped, nil
}

// CreateLabResult creates a new lab result
func CreateLabResult(ctx context.Context, input CreateLabResultInput) (string, error) {
	if pool == nil {
		return "", fmt.Errorf("database connection not initialized")
	}

	var id string
	query := `
		INSERT INTO health_lab_results (followup_id, test_name, test_unit, test_value)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	err := pool.QueryRow(ctx, query,
		input.FollowupID, input.TestName, input.TestUnit, input.TestValue,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to create lab result: %w", err)
	}

	return id, nil
}

// UpdateLabResult updates a lab result's value
func UpdateLabResult(ctx context.Context, id string, input UpdateLabResultInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `
		UPDATE health_lab_results
		SET test_value = $1
		WHERE id = $2
	`

	_, err := pool.Exec(ctx, query, input.TestValue, id)
	if err != nil {
		return fmt.Errorf("failed to update lab result: %w", err)
	}

	return nil
}

// DeleteLabResult deletes a lab result
func DeleteLabResult(ctx context.Context, id string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `DELETE FROM health_lab_results WHERE id = $1`
	_, err := pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete lab result: %w", err)
	}

	return nil
}

// GetLabResult returns a single lab result by ID
func GetLabResult(ctx context.Context, id string) (*HealthLabResult, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var result HealthLabResult
	query := `
		SELECT id, followup_id, test_name, test_unit, test_value, created_at
		FROM health_lab_results
		WHERE id = $1
	`

	err := pool.QueryRow(ctx, query, id).Scan(
		&result.ID, &result.FollowupID, &result.TestName,
		&result.TestUnit, &result.TestValue, &result.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get lab result: %w", err)
	}

	return &result, nil
}

// LabResultWithDate represents a lab result with its follow-up date
type LabResultWithDate struct {
	HealthLabResult
	FollowupDate time.Time
}

// HealthLabResultDisplay represents a lab result for display, including calculated values
type HealthLabResultDisplay struct {
	HealthLabResult
	IsCalculated bool // True if this value was calculated (not directly entered)
}

// GetLabResultsByTestName returns all results for a specific test across all follow-ups for a profile
func GetLabResultsByTestName(ctx context.Context, profileID, testName string) ([]LabResultWithDate, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT r.id, r.followup_id, r.test_name, r.test_unit, r.test_value, r.created_at,
		       f.followup_date
		FROM health_lab_results r
		INNER JOIN health_followups f ON r.followup_id = f.id
		WHERE f.profile_id = $1 AND r.test_name = $2
		ORDER BY f.followup_date ASC
	`

	rows, err := pool.Query(ctx, query, profileID, testName)
	if err != nil {
		return nil, fmt.Errorf("failed to get lab results by test name: %w", err)
	}
	defer rows.Close()

	var results []LabResultWithDate
	for rows.Next() {
		var result LabResultWithDate
		err := rows.Scan(
			&result.ID, &result.FollowupID, &result.TestName,
			&result.TestUnit, &result.TestValue, &result.CreatedAt,
			&result.FollowupDate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lab result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating lab results: %w", err)
	}

	return results, nil
}

// GetLabResultsByTestNameWithCalculated returns results for a specific test including calculated values
func GetLabResultsByTestNameWithCalculated(ctx context.Context, profileID, testName string) ([]LabResultWithDate, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Check if this is TG/HDL ratio (always calculated, never stored)
	if testName == "TG/HDL (Calc)" {
		return calculateTGHDLRatioTimeSeries(ctx, profileID)
	}

	// Check if this is Atherogenic Coefficient (always calculated, never stored)
	if testName == "Atherogenic Coefficient" {
		return calculateAtherogenicCoefficientTimeSeries(ctx, profileID)
	}

	// Check if this is an absolute count test that can be calculated
	absoluteToPercentage := map[string]string{
		"Neutrophils (Absolute)": "Neutrophils",
		"Lymphocytes (Absolute)": "Lymphocytes",
		"Monocytes (Absolute)":   "Monocytes",
		"Eosinophils (Absolute)": "Eosinophils",
		"Basophils (Absolute)":   "Basophils",
	}

	percentageTestName, isCalculatable := absoluteToPercentage[testName]

	// If it's not a calculatable test, just return the regular results
	if !isCalculatable {
		return GetLabResultsByTestName(ctx, profileID, testName)
	}

	// Get all follow-ups for this profile
	query := `
		SELECT id, followup_date
		FROM health_followups
		WHERE profile_id = $1
		ORDER BY followup_date ASC
	`

	rows, err := pool.Query(ctx, query, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get follow-ups: %w", err)
	}
	defer rows.Close()

	var followupIDs []string
	var followupDates []time.Time
	for rows.Next() {
		var id string
		var date time.Time
		if err := rows.Scan(&id, &date); err != nil {
			return nil, fmt.Errorf("failed to scan follow-up: %w", err)
		}
		followupIDs = append(followupIDs, id)
		followupDates = append(followupDates, date)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating follow-ups: %w", err)
	}

	// For each follow-up, calculate or get the absolute value
	var results []LabResultWithDate
	for i, followupID := range followupIDs {
		// First check if there's a manually entered value
		manualQuery := `
			SELECT id, followup_id, test_name, test_unit, test_value, created_at
			FROM health_lab_results
			WHERE followup_id = $1 AND test_name = $2
		`
		var manualResult LabResultWithDate
		err := pool.QueryRow(ctx, manualQuery, followupID, testName).Scan(
			&manualResult.ID, &manualResult.FollowupID, &manualResult.TestName,
			&manualResult.TestUnit, &manualResult.TestValue, &manualResult.CreatedAt,
		)

		if err == nil {
			// Manual value exists, use it
			manualResult.FollowupDate = followupDates[i]
			results = append(results, manualResult)
		} else {
			// No manual value, try to calculate it
			// Get WBC and percentage values
			var wbc, percentage *float64
			var percentageCreatedAt time.Time

			wbcQuery := `SELECT test_value FROM health_lab_results WHERE followup_id = $1 AND test_name = 'White blood cells'`
			var wbcVal float64
			if err := pool.QueryRow(ctx, wbcQuery, followupID).Scan(&wbcVal); err == nil {
				wbc = &wbcVal
			}

			percentageQuery := `SELECT test_value, created_at FROM health_lab_results WHERE followup_id = $1 AND test_name = $2`
			var percentageVal float64
			if err := pool.QueryRow(ctx, percentageQuery, followupID, percentageTestName).Scan(&percentageVal, &percentageCreatedAt); err == nil {
				percentage = &percentageVal
			}

			// If both exist, calculate the absolute count
			if wbc != nil && percentage != nil {
				absoluteValue := (*wbc * *percentage) / 100.0
				unit := "×10³/μL"

				// Parse followupID string to UUID
				followupUUID, err := uuid.Parse(followupID)
				if err != nil {
					continue // Skip this entry if UUID is invalid
				}

				results = append(results, LabResultWithDate{
					HealthLabResult: HealthLabResult{
						FollowupID: followupUUID,
						TestName:   testName,
						TestUnit:   &unit,
						TestValue:  absoluteValue,
						CreatedAt:  percentageCreatedAt,
					},
					FollowupDate: followupDates[i],
				})
			}
		}
	}

	return results, nil
}

// calculateTGHDLRatioTimeSeries calculates TG/HDL ratio for all followups in a profile
func calculateTGHDLRatioTimeSeries(ctx context.Context, profileID string) ([]LabResultWithDate, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Get all follow-ups for this profile
	query := `
		SELECT id, followup_date
		FROM health_followups
		WHERE profile_id = $1
		ORDER BY followup_date ASC
	`

	rows, err := pool.Query(ctx, query, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get follow-ups: %w", err)
	}
	defer rows.Close()

	var followupIDs []string
	var followupDates []time.Time
	for rows.Next() {
		var id string
		var date time.Time
		if err := rows.Scan(&id, &date); err != nil {
			return nil, fmt.Errorf("failed to scan follow-up: %w", err)
		}
		followupIDs = append(followupIDs, id)
		followupDates = append(followupDates, date)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating follow-ups: %w", err)
	}

	// For each follow-up, calculate TG/HDL ratio if both values exist
	var results []LabResultWithDate
	for i, followupID := range followupIDs {
		// Get Triglycerides and HDL Cholesterol values
		var tg, hdl *float64
		var tgCreatedAt, hdlCreatedAt time.Time

		tgQuery := `SELECT test_value, created_at FROM health_lab_results WHERE followup_id = $1 AND test_name = 'Triglycerides'`
		var tgVal float64
		if err := pool.QueryRow(ctx, tgQuery, followupID).Scan(&tgVal, &tgCreatedAt); err == nil {
			tg = &tgVal
		}

		hdlQuery := `SELECT test_value, created_at FROM health_lab_results WHERE followup_id = $1 AND test_name = 'HDL Cholesterol'`
		var hdlVal float64
		if err := pool.QueryRow(ctx, hdlQuery, followupID).Scan(&hdlVal, &hdlCreatedAt); err == nil {
			hdl = &hdlVal
		}

		// If both exist and HDL is not zero, calculate the ratio
		if tg != nil && hdl != nil && *hdl != 0 {
			ratio := *tg / *hdl
			unit := "ratio"

			// Use the more recent created_at date
			createdAt := tgCreatedAt
			if hdlCreatedAt.After(tgCreatedAt) {
				createdAt = hdlCreatedAt
			}

			// Parse followupID string to UUID
			followupUUID, err := uuid.Parse(followupID)
			if err != nil {
				continue // Skip this entry if UUID is invalid
			}

			results = append(results, LabResultWithDate{
				HealthLabResult: HealthLabResult{
					FollowupID: followupUUID,
					TestName:   "TG/HDL (Calc)",
					TestUnit:   &unit,
					TestValue:  ratio,
					CreatedAt:  createdAt,
				},
				FollowupDate: followupDates[i],
			})
		}
	}

	return results, nil
}

// calculateAtherogenicCoefficientTimeSeries calculates Atherogenic Coefficient for all followups in a profile
// Formula: (Total Cholesterol - HDL Cholesterol) / HDL Cholesterol
func calculateAtherogenicCoefficientTimeSeries(ctx context.Context, profileID string) ([]LabResultWithDate, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	// Get all follow-ups for this profile
	query := `
		SELECT id, followup_date
		FROM health_followups
		WHERE profile_id = $1
		ORDER BY followup_date ASC
	`

	rows, err := pool.Query(ctx, query, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get follow-ups: %w", err)
	}
	defer rows.Close()

	var followupIDs []string
	var followupDates []time.Time
	for rows.Next() {
		var id string
		var date time.Time
		if err := rows.Scan(&id, &date); err != nil {
			return nil, fmt.Errorf("failed to scan follow-up: %w", err)
		}
		followupIDs = append(followupIDs, id)
		followupDates = append(followupDates, date)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating follow-ups: %w", err)
	}

	// For each follow-up, calculate Atherogenic Coefficient if both values exist
	var results []LabResultWithDate
	for i, followupID := range followupIDs {
		// Get Total Cholesterol and HDL Cholesterol values
		var totalChol, hdl *float64
		var totalCholCreatedAt, hdlCreatedAt time.Time

		totalCholQuery := `SELECT test_value, created_at FROM health_lab_results WHERE followup_id = $1 AND test_name = 'Total Cholesterol'`
		var totalCholVal float64
		if err := pool.QueryRow(ctx, totalCholQuery, followupID).Scan(&totalCholVal, &totalCholCreatedAt); err == nil {
			totalChol = &totalCholVal
		}

		hdlQuery := `SELECT test_value, created_at FROM health_lab_results WHERE followup_id = $1 AND test_name = 'HDL Cholesterol'`
		var hdlVal float64
		if err := pool.QueryRow(ctx, hdlQuery, followupID).Scan(&hdlVal, &hdlCreatedAt); err == nil {
			hdl = &hdlVal
		}

		// If both exist and HDL is not zero, calculate the coefficient
		if totalChol != nil && hdl != nil && *hdl != 0 {
			coefficient := (*totalChol - *hdl) / *hdl
			unit := "ratio"

			// Use the more recent created_at date
			createdAt := totalCholCreatedAt
			if hdlCreatedAt.After(totalCholCreatedAt) {
				createdAt = hdlCreatedAt
			}

			// Parse followupID string to UUID
			followupUUID, err := uuid.Parse(followupID)
			if err != nil {
				continue // Skip this entry if UUID is invalid
			}

			results = append(results, LabResultWithDate{
				HealthLabResult: HealthLabResult{
					FollowupID: followupUUID,
					TestName:   "Atherogenic Coefficient",
					TestUnit:   &unit,
					TestValue:  coefficient,
					CreatedAt:  createdAt,
				},
				FollowupDate: followupDates[i],
			})
		}
	}

	return results, nil
}

// TestNameCount represents a test name with its count
type TestNameCount struct {
	TestName string
	Count    int
}

// GetTestNamesWithCounts returns all unique test names for a profile with their result counts
func GetTestNamesWithCounts(ctx context.Context, profileID string) ([]TestNameCount, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT r.test_name, COUNT(*) as count
		FROM health_lab_results r
		INNER JOIN health_followups f ON r.followup_id = f.id
		WHERE f.profile_id = $1
		GROUP BY r.test_name
		ORDER BY r.test_name ASC
	`

	rows, err := pool.Query(ctx, query, profileID)
	if err != nil {
		return nil, fmt.Errorf("failed to get test names with counts: %w", err)
	}
	defer rows.Close()

	var tests []TestNameCount
	for rows.Next() {
		var test TestNameCount
		err := rows.Scan(&test.TestName, &test.Count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan test name count: %w", err)
		}
		tests = append(tests, test)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating test names: %w", err)
	}

	return tests, nil
}

// GetTestNamesWithCountsIncludingCalculated returns test names including calculated absolute counts and ratios
func GetTestNamesWithCountsIncludingCalculated(ctx context.Context, profileID string) ([]TestNameCount, error) {
	// Get actual test counts
	tests, err := GetTestNamesWithCounts(ctx, profileID)
	if err != nil {
		return nil, err
	}

	// Filter out manually-entered calculated values (like TG/HDL ratio, Atherogenic Coefficient)
	filteredTests := make([]TestNameCount, 0, len(tests))
	for _, test := range tests {
		if test.TestName != "TG/HDL (Calc)" && test.TestName != "Atherogenic Coefficient" {
			filteredTests = append(filteredTests, test)
		}
	}
	tests = filteredTests

	// Build a map for quick lookup
	testMap := make(map[string]int)
	for _, test := range tests {
		testMap[test.TestName] = test.Count
	}

	// Check if we have WBC and percentage tests to calculate absolute counts
	wbcCount, hasWBC := testMap["White blood cells"]
	if hasWBC && wbcCount > 0 {
		// Add calculated absolute counts for any percentage tests that exist
		percentageToAbsolute := map[string]string{
			"Neutrophils": "Neutrophils (Absolute)",
			"Lymphocytes": "Lymphocytes (Absolute)",
			"Monocytes":   "Monocytes (Absolute)",
			"Eosinophils": "Eosinophils (Absolute)",
			"Basophils":   "Basophils (Absolute)",
		}

		for percentageName, absoluteName := range percentageToAbsolute {
			percentageCount, hasPercentage := testMap[percentageName]
			_, alreadyHasAbsolute := testMap[absoluteName]

			// If we have the percentage test but not the absolute test, add it to the list
			if hasPercentage && percentageCount > 0 && !alreadyHasAbsolute {
				tests = append(tests, TestNameCount{
					TestName: absoluteName,
					Count:    percentageCount, // Same count as percentage test
				})
			}
		}
	}

	// Check if we have Triglycerides and HDL Cholesterol to calculate TG/HDL ratio
	tgCount, hasTG := testMap["Triglycerides"]
	hdlCount, hasHDL := testMap["HDL Cholesterol"]
	if hasTG && hasHDL && tgCount > 0 && hdlCount > 0 {
		// Add TG/HDL ratio - use minimum of the two counts
		ratioCount := tgCount
		if hdlCount < tgCount {
			ratioCount = hdlCount
		}
		tests = append(tests, TestNameCount{
			TestName: "TG/HDL (Calc)",
			Count:    ratioCount,
		})
	}

	// Check if we have Total Cholesterol and HDL Cholesterol to calculate Atherogenic Coefficient
	totalCholCount, hasTotalChol := testMap["Total Cholesterol"]
	if hasTotalChol && hasHDL && totalCholCount > 0 && hdlCount > 0 {
		// Add Atherogenic Coefficient - use minimum of the two counts
		coefficientCount := totalCholCount
		if hdlCount < totalCholCount {
			coefficientCount = hdlCount
		}
		tests = append(tests, TestNameCount{
			TestName: "Atherogenic Coefficient",
			Count:    coefficientCount,
		})
	}

	return tests, nil
}
