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
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"

	"github.com/humaidq/groundwave/db"
)

// ========== Health Breadcrumb Helpers ==========

// BreadcrumbItem represents a single breadcrumb navigation item
type BreadcrumbItem struct {
	Name      string
	URL       string
	IsCurrent bool
}

// healthBreadcrumb returns the base "Health" breadcrumb
func healthBreadcrumb(isCurrent bool) BreadcrumbItem {
	return BreadcrumbItem{Name: "Health", URL: "/health", IsCurrent: isCurrent}
}

// profileBreadcrumb returns a breadcrumb for a health profile
func profileBreadcrumb(profileID, profileName string, isCurrent bool) BreadcrumbItem {
	return BreadcrumbItem{Name: profileName, URL: "/health/" + profileID, IsCurrent: isCurrent}
}

// followupBreadcrumb returns a breadcrumb for a followup
func followupBreadcrumb(profileID, followupID, label string, isCurrent bool) BreadcrumbItem {
	return BreadcrumbItem{
		Name:      label,
		URL:       "/health/" + profileID + "/followup/" + followupID,
		IsCurrent: isCurrent,
	}
}

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

	// Deduplicate results by date (keep most recent entry for each date)
	// This prevents "double points" when multiple followups exist on the same day
	dateMap := make(map[string]db.LabResultWithDate)
	for _, result := range results {
		dateKey := result.FollowupDate.Format("2006-01-02")
		existing, exists := dateMap[dateKey]
		if !exists || result.CreatedAt.After(existing.CreatedAt) {
			dateMap[dateKey] = result
		}
	}

	// Convert map back to slice and sort by date
	dedupedResults := make([]db.LabResultWithDate, 0, len(dateMap))
	for _, result := range dateMap {
		dedupedResults = append(dedupedResults, result)
	}
	// Sort by followup date ascending
	for i := 0; i < len(dedupedResults)-1; i++ {
		for j := i + 1; j < len(dedupedResults); j++ {
			if dedupedResults[i].FollowupDate.After(dedupedResults[j].FollowupDate) {
				dedupedResults[i], dedupedResults[j] = dedupedResults[j], dedupedResults[i]
			}
		}
	}

	// Prepare data and track min/max values
	xAxis := make([]string, 0, len(dedupedResults))
	yData := make([]opts.LineData, 0, len(dedupedResults))
	var dataMin, dataMax float64

	for i, result := range dedupedResults {
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
		mostRecentDate := dedupedResults[len(dedupedResults)-1].FollowupDate
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
	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(true),
	}

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
	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(false),
		{Name: "New Profile", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "health_profile_new")
}

// CreateHealthProfile handles profile creation
func CreateHealthProfile(c flamego.Context, s session.Session) {
	ctx := c.Request().Context()

	if err := c.Request().ParseForm(); err != nil {
		log.Printf("Error parsing form: %v", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/health", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(c.Request().Form.Get("name"))
	if name == "" {
		log.Printf("Profile name is required")
		SetErrorFlash(s, "Profile name is required")
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
			SetErrorFlash(s, "Invalid date of birth format")
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

	// Parse description (optional)
	var description *string
	descStr := strings.TrimSpace(c.Request().Form.Get("description"))
	if descStr != "" {
		description = &descStr
	}

	isPrimary := c.Request().Form.Get("is_primary") == "on"

	profileID, err := db.CreateHealthProfile(ctx, name, dob, gender, description, isPrimary)
	if err != nil {
		log.Printf("Error creating health profile: %v", err)
		SetErrorFlash(s, "Failed to create health profile")
		c.Redirect("/health/new", http.StatusSeeOther)
		return
	}

	log.Printf("Created health profile: %s", profileID)
	SetSuccessFlash(s, "Health profile created successfully")
	c.Redirect("/health/"+profileID, http.StatusSeeOther)
}

// ViewHealthProfile displays a single profile with all follow-ups
func ViewHealthProfile(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["IsHealth"] = true
	profileID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		SetErrorFlash(s, "Profile not found")
		c.Redirect("/health", http.StatusSeeOther)
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
	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(false),
		profileBreadcrumb(profileID, profile.Name, true),
	}
	t.HTML(http.StatusOK, "health_profile_view")
}

// EditHealthProfileForm renders the edit profile form
func EditHealthProfileForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["IsHealth"] = true
	profileID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		SetErrorFlash(s, "Profile not found")
		c.Redirect("/health", http.StatusSeeOther)
		return
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	if profile.Gender != nil {
		data["GenderValue"] = string(*profile.Gender)
	} else {
		data["GenderValue"] = ""
	}

	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(false),
		profileBreadcrumb(profileID, profile.Name, false),
		{Name: "Edit", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "health_profile_edit")
}

// UpdateHealthProfile handles profile update
func UpdateHealthProfile(c flamego.Context, s session.Session) {
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

	// Parse description (optional)
	var description *string
	descStr := strings.TrimSpace(c.Request().Form.Get("description"))
	if descStr != "" {
		description = &descStr
	}

	isPrimary := c.Request().Form.Get("is_primary") == "on"

	err := db.UpdateHealthProfile(ctx, profileID, name, dob, gender, description, isPrimary)
	if err != nil {
		log.Printf("Error updating health profile %s: %v", profileID, err)
		c.Redirect("/health/"+profileID+"/edit", http.StatusSeeOther)
		return
	}

	log.Printf("Updated health profile: %s", profileID)
	SetSuccessFlash(s, "Health profile updated successfully")
	c.Redirect("/health/"+profileID, http.StatusSeeOther)
}

// DeleteHealthProfile handles profile deletion
func DeleteHealthProfile(c flamego.Context, s session.Session) {
	profileID := c.Param("id")
	ctx := c.Request().Context()

	err := db.DeleteHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error deleting health profile %s: %v", profileID, err)
		SetErrorFlash(s, "Failed to delete health profile")
	} else {
		SetSuccessFlash(s, "Health profile deleted successfully")
	}

	c.Redirect("/health", http.StatusSeeOther)
}

// ========== Health Follow-up Handlers ==========

// NewFollowupForm renders the add follow-up form
func NewFollowupForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["IsHealth"] = true
	profileID := c.Param("profile_id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		SetErrorFlash(s, "Profile not found")
		c.Redirect("/health", http.StatusSeeOther)
		return
	}

	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(false),
		profileBreadcrumb(profileID, profile.Name, false),
		{Name: "New Follow-up", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "health_followup_new")
}

// CreateFollowup handles follow-up creation
func CreateFollowup(c flamego.Context, s session.Session) {
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
	SetSuccessFlash(s, "Follow-up created successfully")
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// ViewFollowup displays a single follow-up with all lab results
func ViewFollowup(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["IsHealth"] = true

	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		SetErrorFlash(s, "Profile not found")
		c.Redirect("/health", http.StatusSeeOther)
		return
	}

	followup, err := db.GetFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching follow-up %s: %v", followupID, err)
		SetErrorFlash(s, "Follow-up not found")
		c.Redirect("/health/"+profileID, http.StatusSeeOther)
		return
	}

	results, err := db.GetLabResultsByFollowupWithCalculated(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching lab results for follow-up %s: %v", followupID, err)
		data["Error"] = "Failed to load lab results"
	} else {
		// Enrich results with range status for color coding
		enrichedResults := make(map[db.LabTestCategory][]map[string]interface{})

		// Get age range for reference lookup
		var ageRange db.AgeRange
		if profile.DateOfBirth != nil {
			ageRange = profile.GetAgeRange(followup.FollowupDate)
		}

		for category, categoryResults := range results {
			for _, result := range categoryResults {
				enrichedResult := map[string]interface{}{
					"ID":           result.ID,
					"TestName":     result.TestName,
					"TestUnit":     result.TestUnit,
					"TestValue":    result.TestValue,
					"CreatedAt":    result.CreatedAt,
					"IsCalculated": result.IsCalculated,
					"RangeStatus":  "normal", // default
					"HasRanges":    false,
				}

				// Check against reference ranges if we have age and gender
				if profile.DateOfBirth != nil && profile.Gender != nil {
					refRange, err := db.GetReferenceRange(ctx, result.TestName, ageRange, *profile.Gender)
					if err == nil && refRange != nil {
						refMin, refMax, optMin, optMax, hasOptimal := refRange.GetDisplayRange()

						enrichedResult["HasRanges"] = true
						// Dereference pointers for template use
						if refMin != nil {
							enrichedResult["RefMin"] = *refMin
						}
						if refMax != nil {
							enrichedResult["RefMax"] = *refMax
						}
						if hasOptimal {
							if optMin != nil {
								enrichedResult["OptMin"] = *optMin
							}
							if optMax != nil {
								enrichedResult["OptMax"] = *optMax
							}
							enrichedResult["HasOptimal"] = true
						} else {
							enrichedResult["HasOptimal"] = false
						}

						// Check if outside reference range (RED)
						outOfReference := false
						var arrowDirection string
						if refMin != nil && result.TestValue < *refMin {
							outOfReference = true
							arrowDirection = "down"
						}
						if refMax != nil && result.TestValue > *refMax {
							outOfReference = true
							arrowDirection = "up"
						}

						// Check if outside optimal range (GOLD)
						outOfOptimal := false
						if hasOptimal {
							if optMin != nil && result.TestValue < *optMin {
								outOfOptimal = true
								if arrowDirection == "" {
									arrowDirection = "down"
								}
							}
							if optMax != nil && result.TestValue > *optMax {
								outOfOptimal = true
								if arrowDirection == "" {
									arrowDirection = "up"
								}
							}
						}

						// Set status (red takes priority over gold)
						if outOfReference {
							enrichedResult["RangeStatus"] = "out_of_reference"
							enrichedResult["ArrowDirection"] = arrowDirection
						} else if outOfOptimal {
							enrichedResult["RangeStatus"] = "out_of_optimal"
							enrichedResult["ArrowDirection"] = arrowDirection
						}
					}
				}

				enrichedResults[category] = append(enrichedResults[category], enrichedResult)
			}
		}

		data["Results"] = enrichedResults

		// Collect existing test names to disable them in the dropdown
		existingTests := make(map[string]bool)
		for _, categoryResults := range enrichedResults {
			for _, result := range categoryResults {
				if testName, ok := result["TestName"].(string); ok {
					existingTests[testName] = true
				}
			}
		}
		data["ExistingTests"] = existingTests
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

	followupLabel := followup.FollowupDate.Format("Jan 2, 2006") + " - " + followup.HospitalName
	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(false),
		profileBreadcrumb(profileID, profile.Name, false),
		followupBreadcrumb(profileID, followupID, followupLabel, true),
	}

	t.HTML(http.StatusOK, "health_followup_view")
}

// EditFollowupForm renders the edit follow-up form
func EditFollowupForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["IsHealth"] = true
	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		SetErrorFlash(s, "Profile not found")
		c.Redirect("/health", http.StatusSeeOther)
		return
	}

	followup, err := db.GetFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching follow-up %s: %v", followupID, err)
		SetErrorFlash(s, "Follow-up not found")
		c.Redirect("/health/"+profileID, http.StatusSeeOther)
		return
	}

	followupLabel := followup.FollowupDate.Format("Jan 2, 2006")
	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	data["Followup"] = followup
	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(false),
		profileBreadcrumb(profileID, profile.Name, false),
		followupBreadcrumb(profileID, followupID, followupLabel, false),
		{Name: "Edit", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "health_followup_edit")
}

// UpdateFollowup handles follow-up update
func UpdateFollowup(c flamego.Context, s session.Session) {
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
	SetSuccessFlash(s, "Follow-up updated successfully")
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// DeleteFollowup handles follow-up deletion
func DeleteFollowup(c flamego.Context, s session.Session) {
	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	err := db.DeleteFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error deleting follow-up %s: %v", followupID, err)
		SetErrorFlash(s, "Failed to delete follow-up")
	} else {
		SetSuccessFlash(s, "Follow-up deleted successfully")
	}

	c.Redirect("/health/"+profileID, http.StatusSeeOther)
}

// ========== Lab Result Handlers ==========

// AddLabResult handles adding a lab result via inline form
func AddLabResult(c flamego.Context, s session.Session) {
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
	SetSuccessFlash(s, "Lab result added successfully")
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// EditLabResultForm renders the edit lab result form
func EditLabResultForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["IsHealth"] = true
	profileID := c.Param("profile_id")
	followupID := c.Param("followup_id")
	resultID := c.Param("id")
	ctx := c.Request().Context()

	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		SetErrorFlash(s, "Profile not found")
		c.Redirect("/health", http.StatusSeeOther)
		return
	}

	followup, err := db.GetFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching follow-up %s: %v", followupID, err)
		SetErrorFlash(s, "Follow-up not found")
		c.Redirect("/health/"+profileID, http.StatusSeeOther)
		return
	}

	result, err := db.GetLabResult(ctx, resultID)
	if err != nil {
		log.Printf("Error fetching lab result %s: %v", resultID, err)
		SetErrorFlash(s, "Lab result not found")
		c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
		return
	}

	followupLabel := followup.FollowupDate.Format("Jan 2, 2006")
	data["Profile"] = profile
	data["ProfileName"] = profile.Name
	data["Followup"] = followup
	data["Result"] = result
	data["Breadcrumbs"] = []BreadcrumbItem{
		healthBreadcrumb(false),
		profileBreadcrumb(profileID, profile.Name, false),
		followupBreadcrumb(profileID, followupID, followupLabel, false),
		{Name: "Edit Result", URL: "", IsCurrent: true},
	}
	t.HTML(http.StatusOK, "health_result_edit")
}

// UpdateLabResult handles lab result update
func UpdateLabResult(c flamego.Context, s session.Session) {
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
	SetSuccessFlash(s, "Lab result updated successfully")
	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// DeleteLabResult handles lab result deletion
func DeleteLabResult(c flamego.Context, s session.Session) {
	profileID := c.Param("profile_id")
	followupID := c.Param("followup_id")
	resultID := c.Param("id")
	ctx := c.Request().Context()

	err := db.DeleteLabResult(ctx, resultID)
	if err != nil {
		log.Printf("Error deleting lab result %s: %v", resultID, err)
		SetErrorFlash(s, "Failed to delete lab result")
	} else {
		SetSuccessFlash(s, "Lab result deleted successfully")
	}

	c.Redirect("/health/"+profileID+"/followup/"+followupID, http.StatusSeeOther)
}

// ========== AI Summary Handler ==========

// GenerateAISummary handles AI-powered lab result summarization using Server-Sent Events (SSE).
// SSE keeps data flowing, preventing reverse proxy timeouts during long AI generation.
func GenerateAISummary(c flamego.Context) {
	profileID := c.Param("profile_id")
	followupID := c.Param("id")
	ctx := c.Request().Context()

	w := c.ResponseWriter()

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Helper to send SSE event
	sendEvent := func(event, data string) {
		if event != "" {
			w.Write([]byte("event: " + event + "\n"))
		}
		// Escape newlines in data for SSE format
		escapedData := strings.ReplaceAll(data, "\n", "\ndata: ")
		w.Write([]byte("data: " + escapedData + "\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Helper to send error
	sendError := func(message string) {
		sendEvent("error", message)
	}

	// Get profile
	profile, err := db.GetHealthProfile(ctx, profileID)
	if err != nil {
		log.Printf("Error fetching health profile %s: %v", profileID, err)
		sendError("Profile not found")
		return
	}

	// Get followup
	followup, err := db.GetFollowup(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching follow-up %s: %v", followupID, err)
		sendError("Follow-up not found")
		return
	}

	// Get lab results
	results, err := db.GetLabResultsByFollowupWithCalculated(ctx, followupID)
	if err != nil {
		log.Printf("Error fetching lab results for follow-up %s: %v", followupID, err)
		sendError("Failed to load lab results")
		return
	}

	// Build lab result summaries for the AI
	var labSummaries []db.LabResultSummary

	// Get age range for reference lookup
	var ageRange db.AgeRange
	if profile.DateOfBirth != nil {
		ageRange = profile.GetAgeRange(followup.FollowupDate)
	}

	for _, categoryResults := range results {
		for _, result := range categoryResults {
			summary := db.LabResultSummary{
				TestName:    result.TestName,
				TestValue:   result.TestValue,
				RangeStatus: "normal",
			}

			if result.TestUnit != nil {
				summary.TestUnit = *result.TestUnit
			}

			// Get reference ranges if available
			if profile.DateOfBirth != nil && profile.Gender != nil {
				refRange, err := db.GetReferenceRange(ctx, result.TestName, ageRange, *profile.Gender)
				if err == nil && refRange != nil {
					refMin, refMax, _, _, _ := refRange.GetDisplayRange()
					summary.RefMin = refMin
					summary.RefMax = refMax

					// Determine status
					if refMin != nil && result.TestValue < *refMin {
						summary.RangeStatus = "out_of_reference"
					}
					if refMax != nil && result.TestValue > *refMax {
						summary.RangeStatus = "out_of_reference"
					}
				}
			}

			labSummaries = append(labSummaries, summary)
		}
	}

	if len(labSummaries) == 0 {
		sendError("No lab results to summarize")
		return
	}

	// Stream the summary using Ollama
	err = db.StreamLabSummary(ctx, profile, followup, labSummaries, func(chunk string) error {
		sendEvent("chunk", chunk)
		return nil
	})

	if err != nil {
		log.Printf("Error generating AI summary: %v", err)
		sendError("Failed to generate summary: " + err.Error())
		return
	}

	// Signal completion
	sendEvent("done", "")
}
