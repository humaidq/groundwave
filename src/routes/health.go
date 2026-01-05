/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"bytes"
	htmltemplate "html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/template"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"

	"github.com/humaidq/groundwave/db"
)

// ========== Health Chart Helpers ==========

// generateLabTestChart creates a line chart for a specific lab test with reference ranges
func generateLabTestChart(ctx flamego.Context, profileID, testName string) (string, error) {
	results, err := db.GetLabResultsByTestNameWithCalculated(ctx.Request().Context(), profileID, testName)
	if err != nil {
		return "", err
	}

	// If no data, return empty string
	if len(results) == 0 {
		return "", nil
	}

	// Get profile to determine age and gender
	profile, err := db.GetHealthProfile(ctx.Request().Context(), profileID)
	if err != nil {
		return "", err
	}

	// Extract unit from first result (if available)
	var unitLabel string
	if results[0].TestUnit != nil && *results[0].TestUnit != "" {
		unitLabel = *results[0].TestUnit
	}

	// Prepare data and track min/max values
	xAxis := make([]string, 0, len(results))
	yData := make([]opts.LineData, 0, len(results))
	var dataMin, dataMax float64

	for i, result := range results {
		xAxis = append(xAxis, result.FollowupDate.Format("Jan 2, 2006"))
		yData = append(yData, opts.LineData{Value: result.TestValue})

		if i == 0 {
			dataMin = result.TestValue
			dataMax = result.TestValue
		} else {
			if result.TestValue < dataMin {
				dataMin = result.TestValue
			}
			if result.TestValue > dataMax {
				dataMax = result.TestValue
			}
		}
	}

	// Calculate y-axis range to include reference ranges
	var yAxisMin, yAxisMax interface{}

	// Get reference range for scaling
	if profile.DateOfBirth != nil && profile.Gender != nil {
		mostRecentDate := results[len(results)-1].FollowupDate
		ageRange := profile.GetAgeRange(mostRecentDate)
		refRange, err := db.GetReferenceRange(ctx.Request().Context(), testName, ageRange, *profile.Gender)

		if err == nil && refRange != nil {
			refMin, refMax, _, _, _ := refRange.GetDisplayRange()

			// Set y-axis to include reference range with some padding
			if refMin != nil && refMax != nil {
				padding := (*refMax - *refMin) * 0.1 // 10% padding
				minVal := *refMin - padding
				maxVal := *refMax + padding

				// Expand range if data goes beyond reference range
				if dataMin < minVal {
					minVal = dataMin - (dataMax-dataMin)*0.05 // Add 5% padding below
				}
				if dataMax > maxVal {
					maxVal = dataMax + (dataMax-dataMin)*0.05 // Add 5% padding above
				}

				yAxisMin = minVal
				yAxisMax = maxVal
			}
		}
	}

	// Create line chart
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: testName,
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show: opts.Bool(true),
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(false),
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: unitLabel,
			Min:  yAxisMin,
			Max:  yAxisMax,
		}),
	)

	// Add series with mark points
	seriesOpts := []charts.SeriesOpts{
		charts.WithLineChartOpts(opts.LineChart{
			Smooth:     opts.Bool(true),
			ShowSymbol: opts.Bool(true),
		}),
		charts.WithMarkPointNameTypeItemOpts(
			opts.MarkPointNameTypeItem{Name: "Max", Type: "max"},
			opts.MarkPointNameTypeItem{Name: "Min", Type: "min"},
		),
		charts.WithMarkLineNameTypeItemOpts(
			opts.MarkLineNameTypeItem{Name: "Average", Type: "average"},
		),
	}

	// Add reference range bands if profile has DOB and gender
	if profile.DateOfBirth != nil && profile.Gender != nil {
		// Use the most recent follow-up date to determine age range
		mostRecentDate := results[len(results)-1].FollowupDate
		ageRange := profile.GetAgeRange(mostRecentDate)

		// Get reference range for this test from database
		refRange, err := db.GetReferenceRange(ctx.Request().Context(), testName, ageRange, *profile.Gender)
		if err != nil {
			log.Printf("Warning: failed to get reference range for %s: %v", testName, err)
		}

		if refRange != nil {
			refMin, refMax, optMin, optMax, hasOptimal := refRange.GetDisplayRange()

			var markLineItems []interface{}

			// Add reference range lines
			if refMin != nil {
				markLineItems = append(markLineItems, opts.MarkLineNameYAxisItem{
					Name:  "Ref Min",
					YAxis: *refMin,
				})
			}
			if refMax != nil {
				markLineItems = append(markLineItems, opts.MarkLineNameYAxisItem{
					Name:  "Ref Max",
					YAxis: *refMax,
				})
			}

			// Add optimal range lines if it exists
			if hasOptimal {
				if optMin != nil && *optMin != 0 { // Don't show if it's just 0
					markLineItems = append(markLineItems, opts.MarkLineNameYAxisItem{
						Name:  "Opt Min",
						YAxis: *optMin,
					})
				}
				if optMax != nil {
					markLineItems = append(markLineItems, opts.MarkLineNameYAxisItem{
						Name:  "Opt Max",
						YAxis: *optMax,
					})
				}
			}

			if len(markLineItems) > 0 {
				// Add mark lines with styling (no arrows, dashed gray lines)
				seriesOpts = append(seriesOpts, func(s *charts.SingleSeries) {
					s.MarkLines = &opts.MarkLines{
						Data: markLineItems,
						MarkLineStyle: opts.MarkLineStyle{
							Symbol: []string{"none", "none"}, // No arrows on either end
							LineStyle: &opts.LineStyle{
								Color: "rgba(128, 128, 128, 0.6)", // Gray
								Type:  "dashed",
								Width: 1.5,
							},
						},
					}
				})
			}
		}
	}

	line.SetXAxis(xAxis).
		AddSeries(testName, yData).
		SetSeriesOptions(seriesOpts...)

	// Render to HTML string
	var buf bytes.Buffer
	err = line.Render(&buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ========== Health Profile Handlers ==========

// ListHealthProfiles displays all health profiles
func ListHealthProfiles(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true

	ctx := c.Request().Context()
	profiles, err := db.ListHealthProfiles(ctx)
	if err != nil {
		log.Printf("Error fetching health profiles: %v", err)
		data["Error"] = "Failed to load health profiles"
	} else {
		data["Profiles"] = profiles
	}

	t.HTML(http.StatusOK, "health_list")
}

// NewHealthProfileForm renders the add profile form
func NewHealthProfileForm(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true
	t.HTML(http.StatusOK, "health_profile_new")
}

// CreateHealthProfile handles profile creation
func CreateHealthProfile(c flamego.Context) {
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/health", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(c.Request().Form.Get("name"))
	if name == "" {
		log.Printf("Profile name is required")
		c.Redirect("/health/new", http.StatusSeeOther)
		return
	}

	// Parse DOB (optional)
	var dob *time.Time
	dobStr := strings.TrimSpace(c.Request().Form.Get("date_of_birth"))
	if dobStr != "" {
		parsed, err := time.Parse("2006-01-02", dobStr)
		if err != nil {
			log.Printf("Invalid date of birth format: %v", err)
			c.Redirect("/health/new", http.StatusSeeOther)
			return
		}
		dob = &parsed
	}

	// Parse gender (optional)
	var gender *db.Gender
	genderStr := strings.TrimSpace(c.Request().Form.Get("gender"))
	if genderStr != "" {
		g := db.Gender(genderStr)
		gender = &g
	}

	profileID, err := db.CreateHealthProfile(ctx, name, dob, gender)
	if err != nil {
		log.Printf("Error creating health profile: %v", err)
		c.Redirect("/health/new", http.StatusSeeOther)
		return
	}

	log.Printf("Created health profile: %s", profileID)
	c.Redirect("/health/"+profileID, http.StatusSeeOther)
}

// ViewHealthProfile displays a single profile with all follow-ups
func ViewHealthProfile(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true

	profileID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		data["Error"] = "Profile not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	// Calculate current age for display
	if profile.DateOfBirth != nil {
		age := profile.GetAge(time.Now())
		if age != nil {
			data["CurrentAge"] = *age
		}
	}

	followups, err := db.ListFollowups(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching follow-ups for profile %s: %v", profileID, err)
		data["Error"] = "Failed to load follow-ups"
	} else {
		data["Followups"] = followups
	}

	// Get all test names with counts (including calculated absolute counts)
	testCounts, err := db.GetTestNamesWithCountsIncludingCalculated(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching test names: %v", err)
	}

	// Build a map of test names to categories
	predefinedTests := db.GetPredefinedLabTests()
	testCategoryMap := make(map[string]db.LabTestCategory)
	for _, test := range predefinedTests {
		testCategoryMap[test.Name] = test.Category
	}

	// Generate charts for all tests with at least 1 data point, grouped by category
	chartsByCategory := make(map[db.LabTestCategory][]map[string]interface{})
	for _, test := range testCounts {
		if test.Count > 0 {
			chart, err := generateLabTestChart(c, profileID, test.TestName)
			if err != nil {
				log.Printf("Error generating chart for %s: %v", test.TestName, err)
				continue
			}
			if chart != "" {
				category, found := testCategoryMap[test.TestName]
				if !found {
					// Unknown test, put in "Endocrine & Other" category
					category = db.CategoryEndocrineOther
				}

				chartsByCategory[category] = append(chartsByCategory[category], map[string]interface{}{
					"TestName": test.TestName,
					"HTML":     htmltemplate.HTML(chart),
				})
			}
		}
	}

	// Prepare data for template - categories in order
	categories := []db.LabTestCategory{
		db.CategoryBloodCounts,
		db.CategoryLipidPanel,
		db.CategoryMetabolic,
		db.CategoryLiverFunction,
		db.CategoryVitaminsMinerals,
		db.CategoryEndocrineOther,
	}

	var chartCategories []map[string]interface{}
	for _, category := range categories {
		if charts, exists := chartsByCategory[category]; exists {
			chartCategories = append(chartCategories, map[string]interface{}{
				"CategoryName": string(category),
				"Charts":       charts,
			})
		}
	}

	if len(chartCategories) > 0 {
		data["ChartCategories"] = chartCategories
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	t.HTML(http.StatusOK, "health_profile_view")
}

// EditHealthProfileForm renders the edit profile form
func EditHealthProfileForm(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true

	profileID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		data["Error"] = "Profile not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	t.HTML(http.StatusOK, "health_profile_edit")
}

// UpdateHealthProfile handles profile update
func UpdateHealthProfile(c flamego.Context) {
	profileID := c.Param("id")
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/health/"+profileID, http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(c.Request().Form.Get("name"))
	if name == "" {
		log.Printf("Profile name is required")
		c.Redirect("/health/"+profileID+"/edit", http.StatusSeeOther)
		return
	}

	// Parse DOB (optional)
	var dob *time.Time
	dobStr := strings.TrimSpace(c.Request().Form.Get("date_of_birth"))
	if dobStr != "" {
		parsed, err := time.Parse("2006-01-02", dobStr)
		if err != nil {
			log.Printf("Invalid date of birth format: %v", err)
			c.Redirect("/health/"+profileID+"/edit", http.StatusSeeOther)
			return
		}
		dob = &parsed
	}

	// Parse gender (optional)
	var gender *db.Gender
	genderStr := strings.TrimSpace(c.Request().Form.Get("gender"))
	if genderStr != "" {
		g := db.Gender(genderStr)
		gender = &g
	}

	err := db.UpdateHealthProfile(ctx, profileID, name, dob, gender)
	if err != nil {
		log.Printf("Error updating health profile %s: %v", profileID, err)
		c.Redirect("/health/"+profileID+"/edit", http.StatusSeeOther)
		return
	}

	log.Printf("Updated health profile: %s", profileID)
	c.Redirect("/health/"+profileID, http.StatusSeeOther)
}

// DeleteHealthProfile handles profile deletion
func DeleteHealthProfile(c flamego.Context) {
	profileID := c.Param("id")
	ctx := c.Request().Context()

	err := db.DeleteHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error deleting health profile %s: %v", profileID, err)
	}

	c.Redirect("/health", http.StatusSeeOther)
}

// ========== Health Follow-up Handlers ==========

// NewFollowupForm renders the add follow-up form
func NewFollowupForm(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true

	profileID := c.Param("profile_id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		data["Error"] = "Profile not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	t.HTML(http.StatusOK, "health_followup_new")
}

// CreateFollowup handles follow-up creation
func CreateFollowup(c flamego.Context) {
	profileID := c.Param("profile_id")
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/health/"+profileID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	// Parse date
	dateStr := strings.TrimSpace(form.Get("followup_date"))
	if dateStr == "" {
		log.Printf("Follow-up date is required")
		c.Redirect("/health/"+profileID+"/followup/new", http.StatusSeeOther)
		return
	}
	followupDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		log.Printf("Invalid date format: %v", err)
		c.Redirect("/health/"+profileID+"/followup/new", http.StatusSeeOther)
		return
	}

	hospitalName := strings.TrimSpace(form.Get("hospital_name"))
	if hospitalName == "" {
		log.Printf("Hospital name is required")
		c.Redirect("/health/"+profileID+"/followup/new", http.StatusSeeOther)
		return
	}

	// Optional notes
	var notes *string
	notesVal := strings.TrimSpace(form.Get("notes"))
	if notesVal != "" {
		notes = &notesVal
	}

	input := db.CreateFollowupInput{
		ProfileID:    profileID,
		FollowupDate: followupDate,
		HospitalName: hospitalName,
		Notes:        notes,
	}

	followupID, err := db.CreateFollowup(ctx, input)
	if err != nil {
		log.Printf("Error creating follow-up: %v", err)
		c.Redirect("/health/"+profileID+"/followup/new", http.StatusSeeOther)
		return
	}

	log.Printf("Created follow-up: %s", followupID)
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// ViewFollowup displays a single follow-up with all lab results
func ViewFollowup(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true

	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		data["Error"] = "Profile not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	followup, err := db.GetFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching follow-up %s: %v", followupID, err)
		data["Error"] = "Follow-up not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	results, err := db.GetLabResultsByFollowupWithCalculated(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching lab results for follow-up %s: %v", followupID, err)
		data["Error"] = "Failed to load lab results"
	} else {
		data["Results"] = results
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	data["Followup"] = followup
	data["PredefinedTests"] = db.GetPredefinedLabTests()

	// Create a map of categories for template iteration
	categories := []db.LabTestCategory{
		db.CategoryBloodCounts,
		db.CategoryLipidPanel,
		db.CategoryMetabolic,
		db.CategoryLiverFunction,
		db.CategoryVitaminsMinerals,
		db.CategoryEndocrineOther,
	}
	data["Categories"] = categories

	t.HTML(http.StatusOK, "health_followup_view")
}

// EditFollowupForm renders the edit follow-up form
func EditFollowupForm(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true

	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		data["Error"] = "Profile not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	followup, err := db.GetFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching follow-up %s: %v", followupID, err)
		data["Error"] = "Follow-up not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	data["Followup"] = followup
	t.HTML(http.StatusOK, "health_followup_edit")
}

// UpdateFollowup handles follow-up update
func UpdateFollowup(c flamego.Context) {
	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	// Parse date
	dateStr := strings.TrimSpace(form.Get("followup_date"))
	if dateStr == "" {
		log.Printf("Follow-up date is required")
		c.Redirect("/health/"+profileID+"/followup/"+followupID+"/edit", http.StatusSeeOther)
		return
	}
	followupDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		log.Printf("Invalid date format: %v", err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID+"/edit", http.StatusSeeOther)
		return
	}

	hospitalName := strings.TrimSpace(form.Get("hospital_name"))
	if hospitalName == "" {
		log.Printf("Hospital name is required")
		c.Redirect("/health/"+profileID+"/followup/"+followupID+"/edit", http.StatusSeeOther)
		return
	}

	// Optional notes
	var notes *string
	notesVal := strings.TrimSpace(form.Get("notes"))
	if notesVal != "" {
		notes = &notesVal
	}

	input := db.UpdateFollowupInput{
		FollowupDate: followupDate,
		HospitalName: hospitalName,
		Notes:        notes,
	}

	err = db.UpdateFollowup(ctx, followupID, input)
	if err != nil {
		log.Printf("Error updating follow-up %s: %v", followupID, err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID+"/edit", http.StatusSeeOther)
		return
	}

	log.Printf("Updated follow-up: %s", followupID)
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// DeleteFollowup handles follow-up deletion
func DeleteFollowup(c flamego.Context) {
	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	err := db.DeleteFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error deleting follow-up %s: %v", followupID, err)
	}

	c.Redirect("/health/"+profileID, http.StatusSeeOther)
}

// ========== Lab Result Handlers ==========

// AddLabResult handles adding a lab result via inline form
func AddLabResult(c flamego.Context) {
	profileID := c.Param("profile_id")
	followupID := c.Param("followup_id")
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}

	form := c.Request().Form

	testName := strings.TrimSpace(form.Get("test_name"))
	if testName == "" {
		log.Printf("Test name is required")
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}

	// Get unit (may be auto-filled from dropdown)
	var testUnit *string
	unitVal := strings.TrimSpace(form.Get("test_unit"))
	if unitVal != "" {
		testUnit = &unitVal
	}

	// Parse value
	valueStr := strings.TrimSpace(form.Get("test_value"))
	if valueStr == "" {
		log.Printf("Test value is required")
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}
	testValue, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		log.Printf("Invalid test value: %v", err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}

	input := db.CreateLabResultInput{
		FollowupID: followupID,
		TestName:   testName,
		TestUnit:   testUnit,
		TestValue:  testValue,
	}

	resultID, err := db.CreateLabResult(ctx, input)
	if err != nil {
		log.Printf("Error creating lab result: %v", err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}

	log.Printf("Created lab result: %s", resultID)
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// EditLabResultForm renders the edit lab result form
func EditLabResultForm(c flamego.Context, t template.Template, data template.Data) {
	data["IsHealth"] = true

	profileID := c.Param("profile_id")
	followupID := c.Param("followup_id")
	resultID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		data["Error"] = "Profile not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	followup, err := db.GetFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching follow-up %s: %v", followupID, err)
		data["Error"] = "Follow-up not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	result, err := db.GetLabResult(ctx, resultID)
	if err != nil {
		log.Printf("Error fetching lab result %s: %v", resultID, err)
		data["Error"] = "Lab result not found"
		t.HTML(http.StatusNotFound, "error")
		return
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	data["Followup"] = followup
	data["Result"] = result
	t.HTML(http.StatusOK, "health_result_edit")
}

// UpdateLabResult handles lab result update
func UpdateLabResult(c flamego.Context) {
	profileID := c.Param("profile_id")
	followupID := c.Param("followup_id")
	resultID := c.Param("id")
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}

	// Parse value
	valueStr := strings.TrimSpace(c.Request().Form.Get("test_value"))
	if valueStr == "" {
		log.Printf("Test value is required")
		c.Redirect("/health/"+profileID+"/followup/"+followupID+"/result/"+resultID+"/edit", http.StatusSeeOther)
		return
	}
	testValue, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		log.Printf("Invalid test value: %v", err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID+"/result/"+resultID+"/edit", http.StatusSeeOther)
		return
	}

	input := db.UpdateLabResultInput{
		TestValue: testValue,
	}

	err = db.UpdateLabResult(ctx, resultID, input)
	if err != nil {
		log.Printf("Error updating lab result %s: %v", resultID, err)
		c.Redirect("/health/"+profileID+"/followup/"+followupID+"/result/"+resultID+"/edit", http.StatusSeeOther)
		return
	}

	log.Printf("Updated lab result: %s", resultID)
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// DeleteLabResult handles lab result deletion
func DeleteLabResult(c flamego.Context) {
	profileID := c.Param("profile_id")
	followupID := c.Param("followup_id")
	resultID := c.Param("id")
	ctx := c.Request().Context()

	err := db.DeleteLabResult(ctx, resultID)
	if err != nil {
		log.Printf("Error deleting lab result %s: %v", resultID, err)
	}

	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}
