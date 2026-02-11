// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"math"
	"testing"
	"time"
)

func TestListLedgerBudgetsWithUsageNoTransactionsUsesPositiveZero(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()
	periodStart := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	if _, err := CreateLedgerBudget(ctx, CreateLedgerBudgetInput{
		CategoryName: "Groceries",
		Amount:       1000,
		Currency:     "AED",
		PeriodStart:  periodStart,
	}); err != nil {
		t.Fatalf("failed to create budget: %v", err)
	}

	budgets, err := ListLedgerBudgetsWithUsage(ctx, periodStart)
	if err != nil {
		t.Fatalf("failed to list budgets with usage: %v", err)
	}

	if len(budgets) != 1 {
		t.Fatalf("expected 1 budget, got %d", len(budgets))
	}

	if budgets[0].Used != 0 {
		t.Fatalf("expected used amount 0, got %.2f", budgets[0].Used)
	}

	if math.Signbit(budgets[0].Used) {
		t.Fatalf("expected positive zero used amount, got negative zero")
	}
}

func TestListLedgerBudgetsWithUsageZeroNetSpendingUsesPositiveZero(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()
	periodStart := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

	budgetID, err := CreateLedgerBudget(ctx, CreateLedgerBudgetInput{
		CategoryName: "Travel",
		Amount:       500,
		Currency:     "AED",
		PeriodStart:  periodStart,
	})
	if err != nil {
		t.Fatalf("failed to create budget: %v", err)
	}

	accountID, err := CreateLedgerAccount(ctx, CreateLedgerAccountInput{
		Name:        "Checking",
		AccountType: LedgerAccountRegular,
	})
	if err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	if _, err := CreateLedgerTransaction(ctx, CreateLedgerTransactionInput{
		AccountID:  accountID,
		BudgetID:   &budgetID,
		Amount:     -200,
		Merchant:   "Airline",
		Status:     LedgerTransactionCleared,
		OccurredAt: periodStart.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("failed to create expense transaction: %v", err)
	}

	if _, err := CreateLedgerTransaction(ctx, CreateLedgerTransactionInput{
		AccountID:  accountID,
		BudgetID:   &budgetID,
		Amount:     200,
		Merchant:   "Refund",
		Status:     LedgerTransactionRefunded,
		OccurredAt: periodStart.Add(48 * time.Hour),
	}); err != nil {
		t.Fatalf("failed to create refund transaction: %v", err)
	}

	budgets, err := ListLedgerBudgetsWithUsage(ctx, periodStart)
	if err != nil {
		t.Fatalf("failed to list budgets with usage: %v", err)
	}

	if len(budgets) != 1 {
		t.Fatalf("expected 1 budget, got %d", len(budgets))
	}

	if budgets[0].Used != 0 {
		t.Fatalf("expected used amount 0, got %.2f", budgets[0].Used)
	}

	if math.Signbit(budgets[0].Used) {
		t.Fatalf("expected positive zero used amount, got negative zero")
	}
}
