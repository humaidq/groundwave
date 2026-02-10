/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"

	"github.com/humaidq/groundwave/db"
)

type ledgerHistoryPoint struct {
	MonthStart time.Time
	Label      string
	Balance    float64
}

type ledgerAccountHistorySeries struct {
	Account db.LedgerAccountSummary
	Points  []ledgerHistoryPoint
}

type ledgerHistoryRangePreset struct {
	Value  string
	Label  string
	Months int
}

type ledgerHistoryRangeOption struct {
	Label      string
	URL        string
	IsSelected bool
}

var ledgerHistoryRangePresets = []ledgerHistoryRangePreset{
	{Value: "12m", Label: "12M", Months: 12},
	{Value: "24m", Label: "24M", Months: 24},
	{Value: "36m", Label: "36M", Months: 36},
	{Value: "all", Label: "All", Months: 0},
}

const defaultLedgerHistoryRange = "all"

// LedgerHistoryView renders end-of-month balance history charts.
func LedgerHistoryView(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()
	now := time.Now()
	selectedRange := resolveLedgerHistoryRange(c.Query("range"))
	rangeCutoff := ledgerHistoryCutoffMonth(selectedRange, now)
	data["HistoryRangeOptions"] = ledgerHistoryRangeOptions(selectedRange)

	accounts, err := db.ListLedgerAccountsWithBalances(ctx)
	if err != nil {
		logger.Error("Error fetching ledger accounts", "error", err)
		data["Error"] = "Failed to load ledger history"
		accounts = []db.LedgerAccountSummary{}
	}

	series := make([]ledgerAccountHistorySeries, 0, len(accounts))
	hasHistoryLoadError := false

	for _, account := range accounts {
		transactions, err := db.ListLedgerAccountTransactions(ctx, account.ID)
		if err != nil {
			logger.Error("Error fetching ledger transactions for history", "account_id", account.ID, "error", err)
			hasHistoryLoadError = true
			continue
		}

		reconciliations, err := db.ListLedgerReconciliations(ctx, account.ID)
		if err != nil {
			logger.Error("Error fetching ledger reconciliations for history", "account_id", account.ID, "error", err)
			hasHistoryLoadError = true
			continue
		}

		points := buildLedgerMonthlySeries(account, transactions, reconciliations, now)
		if len(points) == 0 {
			continue
		}

		series = append(series, ledgerAccountHistorySeries{
			Account: account,
			Points:  points,
		})
	}

	if hasHistoryLoadError && data["Error"] == nil {
		data["Error"] = "Some account history could not be loaded"
	}

	series = filterLedgerAccountSeriesByCutoff(series, rangeCutoff)

	netWorthPoints := buildLedgerNetWorthSeries(series)
	if len(netWorthPoints) > 0 {
		netWorthChart, err := renderLedgerHistoryChart(
			"Net Worth",
			"ledger_net_worth_history",
			netWorthPoints,
		)
		if err != nil {
			logger.Error("Error rendering net worth history chart", "error", err)
			if data["Error"] == nil {
				data["Error"] = "Failed to render net worth chart"
			}
		} else {
			data["NetWorthChart"] = htmltemplate.HTML(netWorthChart)
		}
	}

	var regularCharts []map[string]interface{}
	var trackingCharts []map[string]interface{}

	for _, accountSeries := range series {
		switch accountSeries.Account.AccountType {
		case db.LedgerAccountDebt:
			continue
		case db.LedgerAccountRegular, db.LedgerAccountTracking:
			chart, err := renderLedgerHistoryChart(
				accountSeries.Account.Name,
				ledgerHistoryChartID(accountSeries.Account.ID.String()),
				accountSeries.Points,
			)
			if err != nil {
				logger.Error(
					"Error rendering account history chart",
					"account_id", accountSeries.Account.ID,
					"error", err,
				)
				continue
			}

			chartData := map[string]interface{}{
				"AccountName": accountSeries.Account.Name,
				"HTML":        htmltemplate.HTML(chart),
			}

			if accountSeries.Account.AccountType == db.LedgerAccountRegular {
				regularCharts = append(regularCharts, chartData)
			} else {
				trackingCharts = append(trackingCharts, chartData)
			}
		}
	}

	var chartCategories []map[string]interface{}
	if len(regularCharts) > 0 {
		chartCategories = append(chartCategories, map[string]interface{}{
			"CategoryName": "Regular Accounts",
			"Charts":       regularCharts,
		})
	}
	if len(trackingCharts) > 0 {
		chartCategories = append(chartCategories, map[string]interface{}{
			"CategoryName": "Tracking Accounts",
			"Charts":       trackingCharts,
		})
	}
	if len(chartCategories) > 0 {
		data["AccountChartCategories"] = chartCategories
	}

	data["AsOfMonth"] = ledgerMonthStart(now).Format("Jan 2006")
	data["SelectedHistoryRangeLabel"] = selectedRange.Label
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: "History", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_history")
}

func buildLedgerMonthlySeries(
	account db.LedgerAccountSummary,
	transactions []db.LedgerTransactionWithBudget,
	reconciliations []db.LedgerReconciliation,
	now time.Time,
) []ledgerHistoryPoint {
	if account.CreatedAt.IsZero() {
		return nil
	}

	firstMonth := ledgerMonthStart(account.CreatedAt)
	lastMonth := ledgerMonthStart(now)
	if firstMonth.After(lastMonth) {
		return nil
	}

	var points []ledgerHistoryPoint
	for month := firstMonth; !month.After(lastMonth); month = month.AddDate(0, 1, 0) {
		monthEndExclusive := month.AddDate(0, 1, 0)
		balance := ledgerHistoryBalanceAt(account, transactions, reconciliations, monthEndExclusive)

		points = append(points, ledgerHistoryPoint{
			MonthStart: month,
			Label:      month.Format("Jan 2006"),
			Balance:    balance,
		})
	}

	return points
}

func ledgerHistoryBalanceAt(
	account db.LedgerAccountSummary,
	transactions []db.LedgerTransactionWithBudget,
	reconciliations []db.LedgerReconciliation,
	monthEndExclusive time.Time,
) float64 {
	balance := account.OpeningBalance

	var latestReconciliation *db.LedgerReconciliation
	for i := range reconciliations {
		rec := reconciliations[i]
		if rec.ReconciledAt.Before(account.CreatedAt) {
			continue
		}
		if !rec.ReconciledAt.Before(monthEndExclusive) {
			continue
		}

		if latestReconciliation == nil ||
			rec.ReconciledAt.After(latestReconciliation.ReconciledAt) ||
			(rec.ReconciledAt.Equal(latestReconciliation.ReconciledAt) && rec.CreatedAt.After(latestReconciliation.CreatedAt)) {
			recCopy := rec
			latestReconciliation = &recCopy
		}
	}

	cutoff := account.CreatedAt
	if latestReconciliation != nil {
		balance = latestReconciliation.Balance
		cutoff = latestReconciliation.ReconciledAt
	}

	for _, tx := range transactions {
		if tx.Status != db.LedgerTransactionCleared && tx.Status != db.LedgerTransactionRefunded {
			continue
		}
		if !tx.OccurredAt.Before(monthEndExclusive) {
			continue
		}

		if latestReconciliation != nil {
			if !tx.OccurredAt.After(cutoff) {
				continue
			}
		} else if tx.OccurredAt.Before(cutoff) {
			continue
		}

		balance += tx.Amount
	}

	return balance
}

func buildLedgerNetWorthSeries(accountSeries []ledgerAccountHistorySeries) []ledgerHistoryPoint {
	if len(accountSeries) == 0 {
		return nil
	}

	totals := make(map[string]float64)
	monthsByKey := make(map[string]time.Time)

	for _, series := range accountSeries {
		for _, point := range series.Points {
			key := point.MonthStart.Format("2006-01")
			totals[key] += point.Balance
			monthsByKey[key] = point.MonthStart
		}
	}

	months := make([]time.Time, 0, len(monthsByKey))
	for _, month := range monthsByKey {
		months = append(months, month)
	}

	sort.Slice(months, func(i, j int) bool {
		return months[i].Before(months[j])
	})

	points := make([]ledgerHistoryPoint, 0, len(months))
	for _, month := range months {
		key := month.Format("2006-01")
		points = append(points, ledgerHistoryPoint{
			MonthStart: month,
			Label:      month.Format("Jan 2006"),
			Balance:    totals[key],
		})
	}

	return points
}

func renderLedgerHistoryChart(title, chartID string, points []ledgerHistoryPoint) (string, error) {
	if len(points) == 0 {
		return "", nil
	}

	xAxis := make([]string, 0, len(points))
	yData := make([]opts.LineData, 0, len(points))
	for _, point := range points {
		xAxis = append(xAxis, point.Label)
		yData = append(yData, opts.LineData{Value: point.Balance})
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:   "100%",
			Height:  "320px",
			ChartID: chartID,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: title,
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(false),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			AxisLabel: &opts.AxisLabel{
				Rotate:      35,
				HideOverlap: opts.Bool(true),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:  "AED",
			Scale: opts.Bool(true),
		}),
	)

	line.SetXAxis(xAxis).
		AddSeries(title, yData).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth:     opts.Bool(true),
				ShowSymbol: opts.Bool(true),
			}),
		)

	var buf bytes.Buffer
	if err := line.Render(&buf); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func ledgerHistoryChartID(accountID string) string {
	sanitized := strings.ReplaceAll(accountID, "-", "")
	return fmt.Sprintf("ledger_account_history_%s", sanitized)
}

func resolveLedgerHistoryRange(rawRange string) ledgerHistoryRangePreset {
	normalized := strings.ToLower(strings.TrimSpace(rawRange))
	for _, preset := range ledgerHistoryRangePresets {
		if preset.Value == normalized {
			return preset
		}
	}

	for _, preset := range ledgerHistoryRangePresets {
		if preset.Value == defaultLedgerHistoryRange {
			return preset
		}
	}

	return ledgerHistoryRangePreset{Value: defaultLedgerHistoryRange, Label: "All"}
}

func ledgerHistoryCutoffMonth(selectedRange ledgerHistoryRangePreset, now time.Time) *time.Time {
	if selectedRange.Months <= 0 {
		return nil
	}

	lastMonth := ledgerMonthStart(now)
	cutoff := lastMonth.AddDate(0, -(selectedRange.Months - 1), 0)
	return &cutoff
}

func filterLedgerAccountSeriesByCutoff(series []ledgerAccountHistorySeries, cutoff *time.Time) []ledgerAccountHistorySeries {
	if cutoff == nil {
		return series
	}

	filteredSeries := make([]ledgerAccountHistorySeries, 0, len(series))
	for _, item := range series {
		filteredPoints := filterLedgerHistoryPointsByCutoff(item.Points, cutoff)
		if len(filteredPoints) == 0 {
			continue
		}

		filteredSeries = append(filteredSeries, ledgerAccountHistorySeries{
			Account: item.Account,
			Points:  filteredPoints,
		})
	}

	return filteredSeries
}

func filterLedgerHistoryPointsByCutoff(points []ledgerHistoryPoint, cutoff *time.Time) []ledgerHistoryPoint {
	if cutoff == nil {
		copied := make([]ledgerHistoryPoint, len(points))
		copy(copied, points)
		return copied
	}

	filtered := make([]ledgerHistoryPoint, 0, len(points))
	for _, point := range points {
		if point.MonthStart.Before(*cutoff) {
			continue
		}
		filtered = append(filtered, point)
	}

	return filtered
}

func ledgerHistoryRangeOptions(selectedRange ledgerHistoryRangePreset) []ledgerHistoryRangeOption {
	options := make([]ledgerHistoryRangeOption, 0, len(ledgerHistoryRangePresets))
	for _, preset := range ledgerHistoryRangePresets {
		url := "/ledger/history"
		if preset.Value != defaultLedgerHistoryRange {
			url = "/ledger/history?range=" + preset.Value
		}

		options = append(options, ledgerHistoryRangeOption{
			Label:      preset.Label,
			URL:        url,
			IsSelected: preset.Value == selectedRange.Value,
		})
	}

	return options
}
