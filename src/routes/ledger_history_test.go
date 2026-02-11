// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"testing"
	"time"

	"github.com/humaidq/groundwave/db"
)

func mustParseRFC3339(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", value, err)
	}

	return parsed
}

func TestBuildLedgerMonthlySeriesUsesOpeningBalanceAndMonthEndResolution(t *testing.T) {
	t.Parallel()

	account := db.LedgerAccountSummary{
		LedgerAccount: db.LedgerAccount{
			OpeningBalance: 1000,
			CreatedAt:      mustParseRFC3339(t, "2026-01-15T00:00:00Z"),
		},
	}

	transactions := []db.LedgerTransactionWithBudget{
		{LedgerTransaction: db.LedgerTransaction{
			Amount:     -100,
			Status:     db.LedgerTransactionCleared,
			OccurredAt: mustParseRFC3339(t, "2026-01-20T10:00:00Z"),
		}},
		{LedgerTransaction: db.LedgerTransaction{
			Amount:     50,
			Status:     db.LedgerTransactionRefunded,
			OccurredAt: mustParseRFC3339(t, "2026-02-10T10:00:00Z"),
		}},
		{LedgerTransaction: db.LedgerTransaction{
			Amount:     -999,
			Status:     db.LedgerTransactionPending,
			OccurredAt: mustParseRFC3339(t, "2026-02-11T10:00:00Z"),
		}},
	}

	now := mustParseRFC3339(t, "2026-03-15T00:00:00Z")
	points := buildLedgerMonthlySeries(account, transactions, nil, now)

	if len(points) != 3 {
		t.Fatalf("expected 3 monthly points, got %d", len(points))
	}

	if points[0].Label != "Jan 2026" || points[0].Balance != 900 {
		t.Fatalf("unexpected January point: %#v", points[0])
	}

	if points[1].Label != "Feb 2026" || points[1].Balance != 950 {
		t.Fatalf("unexpected February point: %#v", points[1])
	}

	if points[2].Label != "Mar 2026" || points[2].Balance != 950 {
		t.Fatalf("unexpected March point: %#v", points[2])
	}
}

func TestBuildLedgerMonthlySeriesUsesLatestReconciliationCutoff(t *testing.T) {
	t.Parallel()

	account := db.LedgerAccountSummary{
		LedgerAccount: db.LedgerAccount{
			OpeningBalance: 1000,
			CreatedAt:      mustParseRFC3339(t, "2026-01-01T00:00:00Z"),
		},
	}

	reconciledAt := mustParseRFC3339(t, "2026-01-15T09:00:00Z")
	reconciliations := []db.LedgerReconciliation{
		{
			Balance:      700,
			ReconciledAt: reconciledAt,
			CreatedAt:    mustParseRFC3339(t, "2026-01-15T09:05:00Z"),
		},
		{
			Balance:      720,
			ReconciledAt: reconciledAt,
			CreatedAt:    mustParseRFC3339(t, "2026-01-15T09:10:00Z"),
		},
	}

	transactions := []db.LedgerTransactionWithBudget{
		{LedgerTransaction: db.LedgerTransaction{
			Amount:     -200,
			Status:     db.LedgerTransactionCleared,
			OccurredAt: mustParseRFC3339(t, "2026-01-10T00:00:00Z"),
		}},
		{LedgerTransaction: db.LedgerTransaction{
			Amount:     100,
			Status:     db.LedgerTransactionCleared,
			OccurredAt: reconciledAt,
		}},
		{LedgerTransaction: db.LedgerTransaction{
			Amount:     -50,
			Status:     db.LedgerTransactionCleared,
			OccurredAt: mustParseRFC3339(t, "2026-01-16T00:00:00Z"),
		}},
		{LedgerTransaction: db.LedgerTransaction{
			Amount:     20,
			Status:     db.LedgerTransactionCleared,
			OccurredAt: mustParseRFC3339(t, "2026-02-02T00:00:00Z"),
		}},
	}

	now := mustParseRFC3339(t, "2026-02-20T00:00:00Z")
	points := buildLedgerMonthlySeries(account, transactions, reconciliations, now)

	if len(points) != 2 {
		t.Fatalf("expected 2 monthly points, got %d", len(points))
	}

	if points[0].Label != "Jan 2026" || points[0].Balance != 670 {
		t.Fatalf("unexpected January point: %#v", points[0])
	}

	if points[1].Label != "Feb 2026" || points[1].Balance != 690 {
		t.Fatalf("unexpected February point: %#v", points[1])
	}
}

func TestBuildLedgerNetWorthSeriesAggregatesByMonth(t *testing.T) {
	t.Parallel()

	jan := mustParseRFC3339(t, "2026-01-01T00:00:00Z")
	feb := mustParseRFC3339(t, "2026-02-01T00:00:00Z")
	mar := mustParseRFC3339(t, "2026-03-01T00:00:00Z")

	series := []ledgerAccountHistorySeries{
		{
			Points: []ledgerHistoryPoint{
				{MonthStart: jan, Label: "Jan 2026", Balance: 100},
				{MonthStart: feb, Label: "Feb 2026", Balance: 110},
			},
		},
		{
			Points: []ledgerHistoryPoint{
				{MonthStart: feb, Label: "Feb 2026", Balance: 50},
				{MonthStart: mar, Label: "Mar 2026", Balance: 70},
			},
		},
	}

	netWorth := buildLedgerNetWorthSeries(series)
	if len(netWorth) != 3 {
		t.Fatalf("expected 3 net worth points, got %d", len(netWorth))
	}

	if netWorth[0].Label != "Jan 2026" || netWorth[0].Balance != 100 {
		t.Fatalf("unexpected January net worth point: %#v", netWorth[0])
	}

	if netWorth[1].Label != "Feb 2026" || netWorth[1].Balance != 160 {
		t.Fatalf("unexpected February net worth point: %#v", netWorth[1])
	}

	if netWorth[2].Label != "Mar 2026" || netWorth[2].Balance != 70 {
		t.Fatalf("unexpected March net worth point: %#v", netWorth[2])
	}
}

func TestResolveLedgerHistoryRange(t *testing.T) {
	t.Parallel()

	selected := resolveLedgerHistoryRange("24m")
	if selected.Value != "24m" || selected.Months != 24 {
		t.Fatalf("unexpected resolved range: %#v", selected)
	}

	fallback := resolveLedgerHistoryRange("invalid")
	if fallback.Value != "all" || fallback.Months != 0 {
		t.Fatalf("unexpected fallback range: %#v", fallback)
	}
}

func TestLedgerHistoryCutoffMonth(t *testing.T) {
	t.Parallel()

	now := mustParseRFC3339(t, "2026-03-15T00:00:00Z")

	cutoff := ledgerHistoryCutoffMonth(ledgerHistoryRangePreset{Value: "12m", Months: 12}, now)
	if cutoff == nil {
		t.Fatal("expected cutoff month")
	}

	if cutoff.Format("2006-01-02") != "2025-04-01" {
		t.Fatalf("unexpected cutoff month: %s", cutoff.Format("2006-01-02"))
	}

	allCutoff := ledgerHistoryCutoffMonth(ledgerHistoryRangePreset{Value: "all", Months: 0}, now)
	if allCutoff != nil {
		t.Fatalf("expected nil cutoff for all range, got %v", allCutoff)
	}
}

func TestFilterLedgerAccountSeriesByCutoff(t *testing.T) {
	t.Parallel()

	cutoff := mustParseRFC3339(t, "2026-02-01T00:00:00Z")
	jan := mustParseRFC3339(t, "2026-01-01T00:00:00Z")
	feb := mustParseRFC3339(t, "2026-02-01T00:00:00Z")
	mar := mustParseRFC3339(t, "2026-03-01T00:00:00Z")

	series := []ledgerAccountHistorySeries{
		{
			Points: []ledgerHistoryPoint{
				{MonthStart: jan, Label: "Jan 2026", Balance: 10},
				{MonthStart: feb, Label: "Feb 2026", Balance: 20},
			},
		},
		{
			Points: []ledgerHistoryPoint{
				{MonthStart: mar, Label: "Mar 2026", Balance: 30},
			},
		},
		{
			Points: []ledgerHistoryPoint{
				{MonthStart: jan, Label: "Jan 2026", Balance: 40},
			},
		},
	}

	filtered := filterLedgerAccountSeriesByCutoff(series, &cutoff)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 account series after cutoff filtering, got %d", len(filtered))
	}

	if len(filtered[0].Points) != 1 || filtered[0].Points[0].Label != "Feb 2026" {
		t.Fatalf("unexpected first filtered series: %#v", filtered[0].Points)
	}

	if len(filtered[1].Points) != 1 || filtered[1].Points[0].Label != "Mar 2026" {
		t.Fatalf("unexpected second filtered series: %#v", filtered[1].Points)
	}
}
