/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

// LedgerBudgetUsage represents a budget with monthly usage details.
type LedgerBudgetUsage struct {
	LedgerBudget
	Used         float64
	Remaining    float64
	RemainingAbs float64
	IsOver       bool
	Progress     float64
}

// LedgerAccountSummary represents an account with balance and last reconciliation.
type LedgerAccountSummary struct {
	LedgerAccount
	Balance          float64
	LastReconciledAt *time.Time
}

// LedgerTransactionWithBudget represents a transaction with optional budget info.
type LedgerTransactionWithBudget struct {
	LedgerTransaction
	BudgetName        *string
	BudgetPeriodStart *time.Time
}

// CreateLedgerBudgetInput represents input for creating a budget.
type CreateLedgerBudgetInput struct {
	CategoryName string
	Amount       float64
	Currency     string
	PeriodStart  time.Time
}

// CreateLedgerAccountInput represents input for creating an account.
type CreateLedgerAccountInput struct {
	Name           string
	AccountType    LedgerAccountType
	OpeningBalance float64
	IBAN           *string
	BankName       *string
	AccountNumber  *string
	Description    *string
}

// CreateLedgerTransactionInput represents input for creating a transaction.
type CreateLedgerTransactionInput struct {
	AccountID  uuid.UUID
	BudgetID   *uuid.UUID
	Amount     float64
	Merchant   string
	Status     LedgerTransactionStatus
	OccurredAt time.Time
	Note       *string
}

// CreateLedgerReconciliationInput represents input for creating a reconciliation.
type CreateLedgerReconciliationInput struct {
	AccountID    uuid.UUID
	Balance      float64
	ReconciledAt time.Time
	Note         *string
}

// UpdateLedgerAccountInput represents input for updating an account.
type UpdateLedgerAccountInput struct {
	ID             uuid.UUID
	Name           string
	AccountType    LedgerAccountType
	OpeningBalance float64
	IBAN           *string
	BankName       *string
	AccountNumber  *string
	Description    *string
}

// UpdateLedgerTransactionInput represents input for updating a transaction.
type UpdateLedgerTransactionInput struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	BudgetID   *uuid.UUID
	Amount     float64
	Merchant   string
	Status     LedgerTransactionStatus
	OccurredAt time.Time
	Note       *string
}

// UpdateLedgerReconciliationInput represents input for updating a reconciliation.
type UpdateLedgerReconciliationInput struct {
	ID           uuid.UUID
	AccountID    uuid.UUID
	Balance      float64
	ReconciledAt time.Time
	Note         *string
}

// ListLedgerBudgetsForPeriod returns all budgets for a given month (period_start).
func ListLedgerBudgetsForPeriod(ctx context.Context, periodStart time.Time) ([]LedgerBudget, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, category_name, amount, currency, period_start, created_at, updated_at
		FROM ledger_budgets
		WHERE period_start = $1
		ORDER BY category_name ASC
	`

	rows, err := pool.Query(ctx, query, periodStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query ledger budgets: %w", err)
	}
	defer rows.Close()

	var budgets []LedgerBudget
	for rows.Next() {
		var budget LedgerBudget
		if err := rows.Scan(
			&budget.ID,
			&budget.CategoryName,
			&budget.Amount,
			&budget.Currency,
			&budget.PeriodStart,
			&budget.CreatedAt,
			&budget.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ledger budget: %w", err)
		}
		budgets = append(budgets, budget)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger budgets: %w", err)
	}

	return budgets, nil
}

// ListLedgerBudgetsWithUsage returns budgets with usage totals for the month.
func ListLedgerBudgetsWithUsage(ctx context.Context, periodStart time.Time) ([]LedgerBudgetUsage, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT b.id, b.category_name, b.amount, b.currency, b.period_start, b.created_at, b.updated_at,
			COALESCE(SUM(t.amount), 0)
		FROM ledger_budgets b
		LEFT JOIN ledger_transactions t
			ON t.budget_id = b.id
			AND t.status IN ('cleared', 'refunded')
			AND t.occurred_at >= b.period_start
			AND t.occurred_at < (b.period_start + INTERVAL '1 month')
		WHERE b.period_start = $1
		GROUP BY b.id
		ORDER BY b.category_name ASC
	`

	rows, err := pool.Query(ctx, query, periodStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query ledger budgets with usage: %w", err)
	}
	defer rows.Close()

	var budgets []LedgerBudgetUsage
	for rows.Next() {
		var budget LedgerBudget
		var totalAmount float64
		if err := rows.Scan(
			&budget.ID,
			&budget.CategoryName,
			&budget.Amount,
			&budget.Currency,
			&budget.PeriodStart,
			&budget.CreatedAt,
			&budget.UpdatedAt,
			&totalAmount,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ledger budget usage: %w", err)
		}

		used := -totalAmount
		if used < 0 {
			used = 0
		}
		remaining := budget.Amount - used
		isOver := remaining < 0
		remainingAbs := math.Abs(remaining)
		progress := 0.0
		if budget.Amount > 0 {
			progress = (used / budget.Amount) * 100
		}
		progress = math.Max(0, math.Min(progress, 100))

		budgets = append(budgets, LedgerBudgetUsage{
			LedgerBudget: budget,
			Used:         used,
			Remaining:    remaining,
			RemainingAbs: remainingAbs,
			IsOver:       isOver,
			Progress:     progress,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger budgets: %w", err)
	}

	return budgets, nil
}

// CreateLedgerBudget creates a new budget row.
func CreateLedgerBudget(ctx context.Context, input CreateLedgerBudgetInput) (uuid.UUID, error) {
	if pool == nil {
		return uuid.UUID{}, fmt.Errorf("database connection not initialized")
	}
	if input.CategoryName == "" {
		return uuid.UUID{}, fmt.Errorf("category name is required")
	}
	if input.Amount <= 0 {
		return uuid.UUID{}, fmt.Errorf("amount must be greater than zero")
	}
	if input.Currency == "" {
		input.Currency = "AED"
	}

	query := `
		INSERT INTO ledger_budgets (category_name, amount, currency, period_start)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var id uuid.UUID
	if err := pool.QueryRow(ctx, query, input.CategoryName, input.Amount, input.Currency, input.PeriodStart).Scan(&id); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to create ledger budget: %w", err)
	}
	return id, nil
}

// GetLedgerBudget fetches a single budget by ID.
func GetLedgerBudget(ctx context.Context, budgetID uuid.UUID) (*LedgerBudget, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, category_name, amount, currency, period_start, created_at, updated_at
		FROM ledger_budgets
		WHERE id = $1
	`

	var budget LedgerBudget
	if err := pool.QueryRow(ctx, query, budgetID).Scan(
		&budget.ID,
		&budget.CategoryName,
		&budget.Amount,
		&budget.Currency,
		&budget.PeriodStart,
		&budget.CreatedAt,
		&budget.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to get ledger budget: %w", err)
	}

	return &budget, nil
}

// UpdateLedgerBudget updates an existing budget.
func UpdateLedgerBudget(ctx context.Context, budgetID uuid.UUID, categoryName string, amount float64, currency string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}
	if categoryName == "" {
		return fmt.Errorf("category name is required")
	}
	if amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	if currency == "" {
		currency = "AED"
	}

	query := `
		UPDATE ledger_budgets
		SET category_name = $1, amount = $2, currency = $3
		WHERE id = $4
	`

	result, err := pool.Exec(ctx, query, categoryName, amount, currency, budgetID)
	if err != nil {
		return fmt.Errorf("failed to update ledger budget: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("budget not found")
	}

	return nil
}

// DeleteLedgerBudget removes a budget.
func DeleteLedgerBudget(ctx context.Context, budgetID uuid.UUID) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	result, err := pool.Exec(ctx, `DELETE FROM ledger_budgets WHERE id = $1`, budgetID)
	if err != nil {
		return fmt.Errorf("failed to delete ledger budget: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("budget not found")
	}

	return nil
}

// ListLedgerAccountsWithBalances returns all accounts with derived balances.
func ListLedgerAccountsWithBalances(ctx context.Context) ([]LedgerAccountSummary, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT a.id, a.name, a.account_type, a.opening_balance, a.iban, a.bank_name, a.account_number,
			a.description, a.created_at, a.updated_at,
			COALESCE(r.balance, a.opening_balance) AS base_balance,
			r.reconciled_at,
			COALESCE(t.delta, 0) AS delta
		FROM ledger_accounts a
		LEFT JOIN LATERAL (
			SELECT balance, reconciled_at
			FROM ledger_reconciliations
			WHERE account_id = a.id
			ORDER BY reconciled_at DESC, created_at DESC
			LIMIT 1
		) r ON true
		LEFT JOIN LATERAL (
			SELECT SUM(amount) AS delta
			FROM ledger_transactions
			WHERE account_id = a.id
				AND status IN ('cleared', 'refunded')
				AND occurred_at > COALESCE(r.reconciled_at, 'epoch')
		) t ON true
		ORDER BY a.account_type, a.name
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query ledger accounts: %w", err)
	}
	defer rows.Close()

	var accounts []LedgerAccountSummary
	for rows.Next() {
		var account LedgerAccount
		var baseBalance float64
		var delta float64
		var reconciledAt sql.NullTime
		if err := rows.Scan(
			&account.ID,
			&account.Name,
			&account.AccountType,
			&account.OpeningBalance,
			&account.IBAN,
			&account.BankName,
			&account.AccountNumber,
			&account.Description,
			&account.CreatedAt,
			&account.UpdatedAt,
			&baseBalance,
			&reconciledAt,
			&delta,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ledger account: %w", err)
		}

		var lastReconciledAt *time.Time
		if reconciledAt.Valid {
			lastReconciledAt = &reconciledAt.Time
		}
		accounts = append(accounts, LedgerAccountSummary{
			LedgerAccount:    account,
			Balance:          baseBalance + delta,
			LastReconciledAt: lastReconciledAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger accounts: %w", err)
	}

	return accounts, nil
}

// GetLedgerAccount fetches a single account by ID.
func GetLedgerAccount(ctx context.Context, accountID uuid.UUID) (*LedgerAccount, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, name, account_type, opening_balance, iban, bank_name, account_number, description, created_at, updated_at
		FROM ledger_accounts
		WHERE id = $1
	`

	var account LedgerAccount
	if err := pool.QueryRow(ctx, query, accountID).Scan(
		&account.ID,
		&account.Name,
		&account.AccountType,
		&account.OpeningBalance,
		&account.IBAN,
		&account.BankName,
		&account.AccountNumber,
		&account.Description,
		&account.CreatedAt,
		&account.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to get ledger account: %w", err)
	}

	return &account, nil
}

// GetLedgerAccountSummary returns the account with its derived balance.
func GetLedgerAccountSummary(ctx context.Context, accountID uuid.UUID) (*LedgerAccountSummary, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT a.id, a.name, a.account_type, a.opening_balance, a.iban, a.bank_name, a.account_number,
			a.description, a.created_at, a.updated_at,
			COALESCE(r.balance, a.opening_balance) AS base_balance,
			r.reconciled_at,
			COALESCE(t.delta, 0) AS delta
		FROM ledger_accounts a
		LEFT JOIN LATERAL (
			SELECT balance, reconciled_at
			FROM ledger_reconciliations
			WHERE account_id = a.id
			ORDER BY reconciled_at DESC, created_at DESC
			LIMIT 1
		) r ON true
		LEFT JOIN LATERAL (
			SELECT SUM(amount) AS delta
			FROM ledger_transactions
			WHERE account_id = a.id
				AND status IN ('cleared', 'refunded')
				AND occurred_at > COALESCE(r.reconciled_at, 'epoch')
		) t ON true
		WHERE a.id = $1
	`

	var account LedgerAccount
	var baseBalance float64
	var delta float64
	var reconciledAt sql.NullTime
	if err := pool.QueryRow(ctx, query, accountID).Scan(
		&account.ID,
		&account.Name,
		&account.AccountType,
		&account.OpeningBalance,
		&account.IBAN,
		&account.BankName,
		&account.AccountNumber,
		&account.Description,
		&account.CreatedAt,
		&account.UpdatedAt,
		&baseBalance,
		&reconciledAt,
		&delta,
	); err != nil {
		return nil, fmt.Errorf("failed to get ledger account summary: %w", err)
	}

	var lastReconciledAt *time.Time
	if reconciledAt.Valid {
		lastReconciledAt = &reconciledAt.Time
	}

	return &LedgerAccountSummary{
		LedgerAccount:    account,
		Balance:          baseBalance + delta,
		LastReconciledAt: lastReconciledAt,
	}, nil
}

// CreateLedgerAccount creates a new account and returns its ID.
func CreateLedgerAccount(ctx context.Context, input CreateLedgerAccountInput) (uuid.UUID, error) {
	if pool == nil {
		return uuid.UUID{}, fmt.Errorf("database connection not initialized")
	}
	if input.Name == "" {
		return uuid.UUID{}, fmt.Errorf("account name is required")
	}
	if input.AccountType == "" {
		return uuid.UUID{}, fmt.Errorf("account type is required")
	}

	query := `
		INSERT INTO ledger_accounts (name, account_type, opening_balance, iban, bank_name, account_number, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	var id uuid.UUID
	if err := pool.QueryRow(
		ctx,
		query,
		input.Name,
		input.AccountType,
		input.OpeningBalance,
		input.IBAN,
		input.BankName,
		input.AccountNumber,
		input.Description,
	).Scan(&id); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to create ledger account: %w", err)
	}

	return id, nil
}

// UpdateLedgerAccount updates an existing account.
func UpdateLedgerAccount(ctx context.Context, input UpdateLedgerAccountInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}
	if input.Name == "" {
		return fmt.Errorf("account name is required")
	}
	if input.AccountType == "" {
		return fmt.Errorf("account type is required")
	}

	query := `
		UPDATE ledger_accounts
		SET name = $1, account_type = $2, opening_balance = $3,
			iban = $4, bank_name = $5, account_number = $6, description = $7
		WHERE id = $8
	`

	result, err := pool.Exec(
		ctx,
		query,
		input.Name,
		input.AccountType,
		input.OpeningBalance,
		input.IBAN,
		input.BankName,
		input.AccountNumber,
		input.Description,
		input.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update ledger account: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

// DeleteLedgerAccount removes an account and its related records.
func DeleteLedgerAccount(ctx context.Context, accountID uuid.UUID) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	result, err := pool.Exec(ctx, `DELETE FROM ledger_accounts WHERE id = $1`, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete ledger account: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("account not found")
	}

	return nil
}

// ListLedgerAccountTransactions returns all transactions for an account.
func ListLedgerAccountTransactions(ctx context.Context, accountID uuid.UUID) ([]LedgerTransactionWithBudget, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT t.id, t.account_id, t.budget_id, t.amount, t.merchant, t.status, t.occurred_at,
			t.note, t.created_at, t.updated_at,
			b.category_name, b.period_start
		FROM ledger_transactions t
		LEFT JOIN ledger_budgets b ON b.id = t.budget_id
		WHERE t.account_id = $1
		ORDER BY t.occurred_at DESC, t.created_at DESC
	`

	rows, err := pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query ledger transactions: %w", err)
	}
	defer rows.Close()

	var transactions []LedgerTransactionWithBudget
	for rows.Next() {
		var tx LedgerTransaction
		var budgetName sql.NullString
		var budgetPeriod sql.NullTime
		if err := rows.Scan(
			&tx.ID,
			&tx.AccountID,
			&tx.BudgetID,
			&tx.Amount,
			&tx.Merchant,
			&tx.Status,
			&tx.OccurredAt,
			&tx.Note,
			&tx.CreatedAt,
			&tx.UpdatedAt,
			&budgetName,
			&budgetPeriod,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ledger transaction: %w", err)
		}
		var budgetNamePtr *string
		if budgetName.Valid {
			budgetNamePtr = &budgetName.String
		}
		var budgetPeriodPtr *time.Time
		if budgetPeriod.Valid {
			budgetPeriodPtr = &budgetPeriod.Time
		}
		transactions = append(transactions, LedgerTransactionWithBudget{
			LedgerTransaction: tx,
			BudgetName:        budgetNamePtr,
			BudgetPeriodStart: budgetPeriodPtr,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger transactions: %w", err)
	}

	return transactions, nil
}

// GetLedgerTransaction fetches a single transaction by ID for an account.
func GetLedgerTransaction(ctx context.Context, accountID uuid.UUID, transactionID uuid.UUID) (*LedgerTransaction, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, account_id, budget_id, amount, merchant, status, occurred_at, note, created_at, updated_at
		FROM ledger_transactions
		WHERE id = $1 AND account_id = $2
	`

	var tx LedgerTransaction
	if err := pool.QueryRow(ctx, query, transactionID, accountID).Scan(
		&tx.ID,
		&tx.AccountID,
		&tx.BudgetID,
		&tx.Amount,
		&tx.Merchant,
		&tx.Status,
		&tx.OccurredAt,
		&tx.Note,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to get ledger transaction: %w", err)
	}

	return &tx, nil
}

// UpdateLedgerTransaction updates an existing transaction.
func UpdateLedgerTransaction(ctx context.Context, input UpdateLedgerTransactionInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}
	if input.Merchant == "" {
		return fmt.Errorf("merchant is required")
	}
	if input.Amount == 0 {
		return fmt.Errorf("amount must be non-zero")
	}
	if input.Status == "" {
		input.Status = LedgerTransactionCleared
	}

	query := `
		UPDATE ledger_transactions
		SET budget_id = $1, amount = $2, merchant = $3, status = $4, occurred_at = $5, note = $6
		WHERE id = $7 AND account_id = $8
	`

	result, err := pool.Exec(
		ctx,
		query,
		input.BudgetID,
		input.Amount,
		input.Merchant,
		input.Status,
		input.OccurredAt,
		input.Note,
		input.ID,
		input.AccountID,
	)
	if err != nil {
		return fmt.Errorf("failed to update ledger transaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}

	return nil
}

// DeleteLedgerTransaction removes a transaction.
func DeleteLedgerTransaction(ctx context.Context, accountID uuid.UUID, transactionID uuid.UUID) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	result, err := pool.Exec(
		ctx,
		`DELETE FROM ledger_transactions WHERE id = $1 AND account_id = $2`,
		transactionID,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete ledger transaction: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("transaction not found")
	}

	return nil
}

// CreateLedgerTransaction creates a new transaction.
func CreateLedgerTransaction(ctx context.Context, input CreateLedgerTransactionInput) (uuid.UUID, error) {
	if pool == nil {
		return uuid.UUID{}, fmt.Errorf("database connection not initialized")
	}
	if input.Merchant == "" {
		return uuid.UUID{}, fmt.Errorf("merchant is required")
	}
	if input.Amount == 0 {
		return uuid.UUID{}, fmt.Errorf("amount must be non-zero")
	}
	if input.Status == "" {
		input.Status = LedgerTransactionCleared
	}

	query := `
		INSERT INTO ledger_transactions (account_id, budget_id, amount, merchant, status, occurred_at, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	var id uuid.UUID
	if err := pool.QueryRow(
		ctx,
		query,
		input.AccountID,
		input.BudgetID,
		input.Amount,
		input.Merchant,
		input.Status,
		input.OccurredAt,
		input.Note,
	).Scan(&id); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to create ledger transaction: %w", err)
	}

	return id, nil
}

// ListLedgerReconciliations returns reconciliations for an account.
func ListLedgerReconciliations(ctx context.Context, accountID uuid.UUID) ([]LedgerReconciliation, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, account_id, balance, reconciled_at, note, created_at
		FROM ledger_reconciliations
		WHERE account_id = $1
		ORDER BY reconciled_at DESC, created_at DESC
	`

	rows, err := pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query ledger reconciliations: %w", err)
	}
	defer rows.Close()

	var reconciliations []LedgerReconciliation
	for rows.Next() {
		var rec LedgerReconciliation
		if err := rows.Scan(
			&rec.ID,
			&rec.AccountID,
			&rec.Balance,
			&rec.ReconciledAt,
			&rec.Note,
			&rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ledger reconciliation: %w", err)
		}
		reconciliations = append(reconciliations, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ledger reconciliations: %w", err)
	}

	return reconciliations, nil
}

// GetLedgerReconciliation fetches a reconciliation by ID for an account.
func GetLedgerReconciliation(ctx context.Context, accountID uuid.UUID, reconciliationID uuid.UUID) (*LedgerReconciliation, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, account_id, balance, reconciled_at, note, created_at
		FROM ledger_reconciliations
		WHERE id = $1 AND account_id = $2
	`

	var rec LedgerReconciliation
	if err := pool.QueryRow(ctx, query, reconciliationID, accountID).Scan(
		&rec.ID,
		&rec.AccountID,
		&rec.Balance,
		&rec.ReconciledAt,
		&rec.Note,
		&rec.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to get ledger reconciliation: %w", err)
	}

	return &rec, nil
}

// UpdateLedgerReconciliation updates a reconciliation snapshot.
func UpdateLedgerReconciliation(ctx context.Context, input UpdateLedgerReconciliationInput) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	query := `
		UPDATE ledger_reconciliations
		SET balance = $1, reconciled_at = $2, note = $3
		WHERE id = $4 AND account_id = $5
	`

	result, err := pool.Exec(ctx, query, input.Balance, input.ReconciledAt, input.Note, input.ID, input.AccountID)
	if err != nil {
		return fmt.Errorf("failed to update ledger reconciliation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("reconciliation not found")
	}

	return nil
}

// DeleteLedgerReconciliation removes a reconciliation snapshot.
func DeleteLedgerReconciliation(ctx context.Context, accountID uuid.UUID, reconciliationID uuid.UUID) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	result, err := pool.Exec(
		ctx,
		`DELETE FROM ledger_reconciliations WHERE id = $1 AND account_id = $2`,
		reconciliationID,
		accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete ledger reconciliation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("reconciliation not found")
	}

	return nil
}

// CreateLedgerReconciliation creates a new reconciliation.
func CreateLedgerReconciliation(ctx context.Context, input CreateLedgerReconciliationInput) (uuid.UUID, error) {
	if pool == nil {
		return uuid.UUID{}, fmt.Errorf("database connection not initialized")
	}

	query := `
		INSERT INTO ledger_reconciliations (account_id, balance, reconciled_at, note)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var id uuid.UUID
	if err := pool.QueryRow(
		ctx,
		query,
		input.AccountID,
		input.Balance,
		input.ReconciledAt,
		input.Note,
	).Scan(&id); err != nil {
		return uuid.UUID{}, fmt.Errorf("failed to create ledger reconciliation: %w", err)
	}

	return id, nil
}
