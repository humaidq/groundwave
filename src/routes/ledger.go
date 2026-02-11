/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

type ledgerActivityType string

const (
	ledgerActivityTransaction    ledgerActivityType = "transaction"
	ledgerActivityReconciliation ledgerActivityType = "reconciliation"
)

type ledgerAccountActivity struct {
	Type           ledgerActivityType
	OccurredAt     time.Time
	Transaction    *db.LedgerTransactionWithBudget
	Reconciliation *db.LedgerReconciliation
}

// LedgerIndex renders the ledger dashboard.
func LedgerIndex(c flamego.Context, _ session.Session, t template.Template, data template.Data) {
	ctx := c.Request().Context()
	periodStart := ledgerMonthStart(time.Now())
	periodLabel := periodStart.Format("Jan 2006")

	budgets, err := db.ListLedgerBudgetsWithUsage(ctx, periodStart)
	if err != nil {
		logger.Error("Error fetching ledger budgets", "error", err)

		data["Error"] = "Failed to load budgets"
		budgets = []db.LedgerBudgetUsage{}
	}

	accounts, err := db.ListLedgerAccountsWithBalances(ctx)
	if err != nil {
		logger.Error("Error fetching ledger accounts", "error", err)

		data["Error"] = "Failed to load accounts"
		accounts = []db.LedgerAccountSummary{}
	}

	var (
		regularAccounts   []db.LedgerAccountSummary
		debtAccounts      []db.LedgerAccountSummary
		trackingAccounts  []db.LedgerAccountSummary
		netWorth          float64
		cashAvailable     float64
		debtBalanceTotal  float64
		budgetUsedTotal   float64
		budgetAmountTotal float64
	)

	for _, budget := range budgets {
		budgetUsedTotal += budget.Used
		budgetAmountTotal += budget.Amount
	}

	for _, account := range accounts {
		switch account.AccountType {
		case db.LedgerAccountRegular:
			regularAccounts = append(regularAccounts, account)
			cashAvailable += account.Balance
		case db.LedgerAccountDebt:
			debtAccounts = append(debtAccounts, account)
			debtBalanceTotal += account.Balance
		case db.LedgerAccountTracking:
			trackingAccounts = append(trackingAccounts, account)
		default:
			regularAccounts = append(regularAccounts, account)
			cashAvailable += account.Balance
		}

		netWorth += account.Balance
	}

	totalDebt := 0.0
	if debtBalanceTotal < 0 {
		totalDebt = -debtBalanceTotal
	}

	data["Budgets"] = budgets
	data["AccountsRegular"] = regularAccounts
	data["AccountsDebt"] = debtAccounts
	data["AccountsTracking"] = trackingAccounts
	data["NetWorth"] = netWorth
	data["CashAvailable"] = cashAvailable
	data["TotalDebt"] = totalDebt
	data["BudgetUsageUsed"] = budgetUsedTotal
	data["BudgetUsageTotal"] = budgetAmountTotal
	data["BudgetUsageCurrency"] = "AED"
	data["PeriodLabel"] = periodLabel
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger")
}

// LedgerAccountView renders a single ledger account with transactions.
func LedgerAccountView(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	accountIDStr := c.Param("id")
	if accountIDStr == "" {
		SetErrorFlash(s, "Account ID is required")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	accountSummary, err := db.GetLedgerAccountSummary(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	transactions, err := db.ListLedgerAccountTransactions(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger transactions", "error", err)

		data["Error"] = "Failed to load transactions"
		transactions = []db.LedgerTransactionWithBudget{}
	}

	reconciliations, err := db.ListLedgerReconciliations(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger reconciliations", "error", err)

		data["Error"] = "Failed to load reconciliations"
		reconciliations = []db.LedgerReconciliation{}
	}

	activity := buildLedgerAccountActivity(transactions, reconciliations)

	data["Account"] = accountSummary
	data["AccountActivity"] = activity
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: accountSummary.Name, URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_account")
}

// CreateLedgerBudget handles creating a new monthly budget.
func CreateLedgerBudget(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing ledger budget form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/budgets/new", http.StatusSeeOther)

		return
	}

	form := c.Request().Form

	category := strings.TrimSpace(form.Get("category_name"))
	if category == "" {
		SetErrorFlash(s, "Category name is required")
		c.Redirect("/ledger/budgets/new", http.StatusSeeOther)

		return
	}

	amountStr := strings.TrimSpace(form.Get("amount"))

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		SetErrorFlash(s, "Budget amount must be a number")
		c.Redirect("/ledger/budgets/new", http.StatusSeeOther)

		return
	}

	if amount <= 0 {
		SetErrorFlash(s, "Budget amount must be greater than zero")
		c.Redirect("/ledger/budgets/new", http.StatusSeeOther)

		return
	}

	periodStart := ledgerMonthStart(time.Now())
	ctx := c.Request().Context()

	_, err = db.CreateLedgerBudget(ctx, db.CreateLedgerBudgetInput{
		CategoryName: category,
		Amount:       amount,
		Currency:     "AED",
		PeriodStart:  periodStart,
	})
	if err != nil {
		logger.Error("Error creating ledger budget", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to create budget")
		c.Redirect("/ledger/budgets/new", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Budget created")
	c.Redirect("/ledger", http.StatusSeeOther)
}

// CreateLedgerAccount handles creating a new ledger account.
func CreateLedgerAccount(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing ledger account form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/accounts/new", http.StatusSeeOther)

		return
	}

	form := c.Request().Form

	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		SetErrorFlash(s, "Account name is required")
		c.Redirect("/ledger/accounts/new", http.StatusSeeOther)

		return
	}

	accountType := db.LedgerAccountType(strings.TrimSpace(form.Get("account_type")))
	if !isValidLedgerAccountType(accountType) {
		SetErrorFlash(s, "Invalid account type")
		c.Redirect("/ledger/accounts/new", http.StatusSeeOther)

		return
	}

	openingBalance := 0.0

	openingStr := strings.TrimSpace(form.Get("opening_balance"))
	if openingStr != "" {
		value, err := strconv.ParseFloat(openingStr, 64)
		if err != nil {
			SetErrorFlash(s, "Opening balance must be a number")
			c.Redirect("/ledger/accounts/new", http.StatusSeeOther)

			return
		}

		openingBalance = normalizeLedgerDebtBalanceInput(openingStr, value, accountType)
	}

	input := db.CreateLedgerAccountInput{
		Name:           name,
		AccountType:    accountType,
		OpeningBalance: openingBalance,
		IBAN:           getOptionalString(form.Get("iban")),
		BankName:       getOptionalString(form.Get("bank_name")),
		AccountNumber:  getOptionalString(form.Get("account_number")),
		Description:    getOptionalString(form.Get("description")),
	}

	ctx := c.Request().Context()

	accountID, err := db.CreateLedgerAccount(ctx, input)
	if err != nil {
		logger.Error("Error creating ledger account", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to create account")
		c.Redirect("/ledger/accounts/new", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Account created")
	c.Redirect("/ledger/accounts/"+accountID.String(), http.StatusSeeOther)
}

// LedgerBudgetNewForm renders the create budget page.
func LedgerBudgetNewForm(_ flamego.Context, t template.Template, data template.Data) {
	periodStart := ledgerMonthStart(time.Now())
	data["PeriodLabel"] = periodStart.Format("Jan 2006")
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: "New Budget", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_budget_new")
}

// LedgerBudgetEditForm renders the edit budget page.
func LedgerBudgetEditForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	budgetIDStr := c.Param("id")

	budgetID, err := uuid.Parse(budgetIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid budget ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	budget, err := db.GetLedgerBudget(ctx, budgetID)
	if err != nil {
		logger.Error("Error fetching ledger budget", "error", err)
		SetErrorFlash(s, "Budget not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	data["Budget"] = budget
	data["PeriodLabel"] = budget.PeriodStart.Format("Jan 2006")
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: "Edit Budget", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_budget_edit")
}

// UpdateLedgerBudget handles updating a budget.
func UpdateLedgerBudget(c flamego.Context, s session.Session) {
	budgetIDStr := c.Param("id")

	budgetID, err := uuid.Parse(budgetIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid budget ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing budget form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/budgets/"+budgetIDStr+"/edit", http.StatusSeeOther)

		return
	}

	form := c.Request().Form

	category := strings.TrimSpace(form.Get("category_name"))
	if category == "" {
		SetErrorFlash(s, "Category name is required")
		c.Redirect("/ledger/budgets/"+budgetIDStr+"/edit", http.StatusSeeOther)

		return
	}

	amountStr := strings.TrimSpace(form.Get("amount"))

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		SetErrorFlash(s, "Budget amount must be a number")
		c.Redirect("/ledger/budgets/"+budgetIDStr+"/edit", http.StatusSeeOther)

		return
	}

	if amount <= 0 {
		SetErrorFlash(s, "Budget amount must be greater than zero")
		c.Redirect("/ledger/budgets/"+budgetIDStr+"/edit", http.StatusSeeOther)

		return
	}

	currency := strings.TrimSpace(form.Get("currency"))

	ctx := c.Request().Context()
	if err := db.UpdateLedgerBudget(ctx, budgetID, category, amount, currency); err != nil {
		logger.Error("Error updating ledger budget", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to update budget")
		c.Redirect("/ledger/budgets/"+budgetIDStr+"/edit", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Budget updated")
	c.Redirect("/ledger", http.StatusSeeOther)
}

// DeleteLedgerBudget handles budget deletion.
func DeleteLedgerBudget(c flamego.Context, s session.Session) {
	budgetIDStr := c.Param("id")

	budgetID, err := uuid.Parse(budgetIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid budget ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()
	if err := db.DeleteLedgerBudget(ctx, budgetID); err != nil {
		logger.Error("Error deleting ledger budget", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to delete budget")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Budget deleted")
	c.Redirect("/ledger", http.StatusSeeOther)
}

// LedgerAccountNewForm renders the create account page.
func LedgerAccountNewForm(_ flamego.Context, t template.Template, data template.Data) {
	data["AccountTypeOptions"] = ledgerAccountTypeOptions()
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: "New Account", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_account_new")
}

// LedgerAccountEditForm renders the edit account page.
func LedgerAccountEditForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	account, err := db.GetLedgerAccount(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	data["Account"] = account
	data["AccountTypeOptions"] = ledgerAccountTypeOptions()
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: account.Name, URL: "/ledger/accounts/" + accountIDStr, IsCurrent: false},
		{Name: "Edit Account", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_account_edit")
}

// UpdateLedgerAccount handles updating an account.
func UpdateLedgerAccount(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing ledger account form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/edit", http.StatusSeeOther)

		return
	}

	form := c.Request().Form

	name := strings.TrimSpace(form.Get("name"))
	if name == "" {
		SetErrorFlash(s, "Account name is required")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/edit", http.StatusSeeOther)

		return
	}

	accountType := db.LedgerAccountType(strings.TrimSpace(form.Get("account_type")))
	if !isValidLedgerAccountType(accountType) {
		SetErrorFlash(s, "Invalid account type")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/edit", http.StatusSeeOther)

		return
	}

	openingBalance := 0.0

	openingStr := strings.TrimSpace(form.Get("opening_balance"))
	if openingStr != "" {
		value, err := strconv.ParseFloat(openingStr, 64)
		if err != nil {
			SetErrorFlash(s, "Opening balance must be a number")
			c.Redirect("/ledger/accounts/"+accountIDStr+"/edit", http.StatusSeeOther)

			return
		}

		openingBalance = normalizeLedgerDebtBalanceInput(openingStr, value, accountType)
	}

	input := db.UpdateLedgerAccountInput{
		ID:             accountID,
		Name:           name,
		AccountType:    accountType,
		OpeningBalance: openingBalance,
		IBAN:           getOptionalString(form.Get("iban")),
		BankName:       getOptionalString(form.Get("bank_name")),
		AccountNumber:  getOptionalString(form.Get("account_number")),
		Description:    getOptionalString(form.Get("description")),
	}

	ctx := c.Request().Context()
	if err := db.UpdateLedgerAccount(ctx, input); err != nil {
		logger.Error("Error updating ledger account", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to update account")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/edit", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Account updated")
	c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)
}

// DeleteLedgerAccount handles account deletion.
func DeleteLedgerAccount(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()
	if err := db.DeleteLedgerAccount(ctx, accountID); err != nil {
		logger.Error("Error deleting ledger account", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to delete account")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Account deleted")
	c.Redirect("/ledger", http.StatusSeeOther)
}

// LedgerTransactionNewForm renders the create transaction page.
func LedgerTransactionNewForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	accountSummary, err := db.GetLedgerAccountSummary(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	periodStart := ledgerMonthStart(time.Now())

	budgets, err := db.ListLedgerBudgetsForPeriod(ctx, periodStart)
	if err != nil {
		logger.Error("Error fetching ledger budgets", "error", err)

		budgets = []db.LedgerBudget{}
	}

	now := time.Now()
	data["Account"] = accountSummary
	data["Budgets"] = budgets
	data["PeriodLabel"] = periodStart.Format("Jan 2006")
	data["TransactionStatusOptions"] = ledgerTransactionStatusOptions()
	data["DefaultOccurredAt"] = now.Format("2006-01-02T15:04")
	data["DefaultAmountPlaceholder"] = ledgerTransactionAmountPlaceholder(accountSummary.AccountType)
	data["AmountInputHint"] = ledgerTransactionAmountHint(accountSummary.AccountType)
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: accountSummary.Name, URL: "/ledger/accounts/" + accountIDStr, IsCurrent: false},
		{Name: "New Transaction", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_transaction_new")
}

// LedgerReconcileNewForm renders the reconciliation page.
func LedgerReconcileNewForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	accountSummary, err := db.GetLedgerAccountSummary(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	now := time.Now()
	data["Account"] = accountSummary
	data["DefaultReconciledAt"] = now.Format("2006-01-02T15:04")
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: accountSummary.Name, URL: "/ledger/accounts/" + accountIDStr, IsCurrent: false},
		{Name: "Reconcile", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_reconcile_new")
}

// CreateLedgerTransaction handles creating a new transaction.
func CreateLedgerTransaction(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	account, err := db.GetLedgerAccount(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing ledger transaction form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)

		return
	}

	form := c.Request().Form

	merchant := strings.TrimSpace(form.Get("merchant"))
	if merchant == "" {
		SetErrorFlash(s, "Merchant is required")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)

		return
	}

	amountStr := strings.TrimSpace(form.Get("amount"))

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		SetErrorFlash(s, "Amount must be a number")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)

		return
	}

	amount = normalizeLedgerTransactionAmount(amountStr, amount, account.AccountType)

	status := db.LedgerTransactionStatus(strings.TrimSpace(form.Get("status")))
	if status == "" {
		status = db.LedgerTransactionCleared
	} else if !isValidLedgerTransactionStatus(status) {
		SetErrorFlash(s, "Invalid transaction status")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)

		return
	}

	occurredAt, err := parseLedgerDateTime(form.Get("occurred_at"))
	if err != nil {
		SetErrorFlash(s, "Invalid transaction date")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)

		return
	}

	var budgetID *uuid.UUID

	budgetIDStr := strings.TrimSpace(form.Get("budget_id"))
	if budgetIDStr != "" {
		parsed, err := uuid.Parse(budgetIDStr)
		if err != nil {
			SetErrorFlash(s, "Invalid budget")
			c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)

			return
		}

		budgetID = &parsed
	}

	input := db.CreateLedgerTransactionInput{
		AccountID:  accountID,
		BudgetID:   budgetID,
		Amount:     amount,
		Merchant:   merchant,
		Status:     status,
		OccurredAt: occurredAt,
		Note:       getOptionalString(form.Get("note")),
	}

	_, err = db.CreateLedgerTransaction(ctx, input)
	if err != nil {
		logger.Error("Error creating ledger transaction", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to add transaction")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Transaction added")

	if strings.TrimSpace(form.Get("action")) == "add_another" {
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/new", http.StatusSeeOther)
		return
	}

	c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)
}

// LedgerTransactionEditForm renders the edit transaction page.
func LedgerTransactionEditForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	transactionIDStr := c.Param("tx_id")

	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid transaction ID")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	accountSummary, err := db.GetLedgerAccountSummary(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	transaction, err := db.GetLedgerTransaction(ctx, accountID, transactionID)
	if err != nil {
		logger.Error("Error fetching ledger transaction", "error", err)
		SetErrorFlash(s, "Transaction not found")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	periodStart := ledgerMonthStart(transaction.OccurredAt)

	budgets, err := db.ListLedgerBudgetsForPeriod(ctx, periodStart)
	if err != nil {
		logger.Error("Error fetching ledger budgets", "error", err)

		budgets = []db.LedgerBudget{}
	}

	data["Account"] = accountSummary
	data["Transaction"] = transaction
	data["Budgets"] = budgets
	data["PeriodLabel"] = periodStart.Format("Jan 2006")
	data["TransactionStatusOptions"] = ledgerTransactionStatusOptions()

	data["AmountInputHint"] = ledgerTransactionAmountHint(accountSummary.AccountType)
	if transaction.BudgetID != nil {
		data["SelectedBudgetID"] = transaction.BudgetID.String()
	} else {
		data["SelectedBudgetID"] = ""
	}

	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: accountSummary.Name, URL: "/ledger/accounts/" + accountIDStr, IsCurrent: false},
		{Name: "Edit Transaction", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_transaction_edit")
}

// UpdateLedgerTransaction handles updating a transaction.
func UpdateLedgerTransaction(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	transactionIDStr := c.Param("tx_id")

	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid transaction ID")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	account, err := db.GetLedgerAccount(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing ledger transaction form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/"+transactionIDStr+"/edit", http.StatusSeeOther)

		return
	}

	form := c.Request().Form

	merchant := strings.TrimSpace(form.Get("merchant"))
	if merchant == "" {
		SetErrorFlash(s, "Merchant is required")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/"+transactionIDStr+"/edit", http.StatusSeeOther)

		return
	}

	amountStr := strings.TrimSpace(form.Get("amount"))

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		SetErrorFlash(s, "Amount must be a number")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/"+transactionIDStr+"/edit", http.StatusSeeOther)

		return
	}

	amount = normalizeLedgerTransactionAmount(amountStr, amount, account.AccountType)

	status := db.LedgerTransactionStatus(strings.TrimSpace(form.Get("status")))
	if status == "" {
		status = db.LedgerTransactionCleared
	} else if !isValidLedgerTransactionStatus(status) {
		SetErrorFlash(s, "Invalid transaction status")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/"+transactionIDStr+"/edit", http.StatusSeeOther)

		return
	}

	occurredAt, err := parseLedgerDateTime(form.Get("occurred_at"))
	if err != nil {
		SetErrorFlash(s, "Invalid transaction date")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/"+transactionIDStr+"/edit", http.StatusSeeOther)

		return
	}

	var budgetID *uuid.UUID

	budgetIDStr := strings.TrimSpace(form.Get("budget_id"))
	if budgetIDStr != "" {
		parsed, err := uuid.Parse(budgetIDStr)
		if err != nil {
			SetErrorFlash(s, "Invalid budget")
			c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/"+transactionIDStr+"/edit", http.StatusSeeOther)

			return
		}

		budgetID = &parsed
	}

	input := db.UpdateLedgerTransactionInput{
		ID:         transactionID,
		AccountID:  accountID,
		BudgetID:   budgetID,
		Amount:     amount,
		Merchant:   merchant,
		Status:     status,
		OccurredAt: occurredAt,
		Note:       getOptionalString(form.Get("note")),
	}

	if err := db.UpdateLedgerTransaction(ctx, input); err != nil {
		logger.Error("Error updating ledger transaction", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to update transaction")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/transactions/"+transactionIDStr+"/edit", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Transaction updated")
	c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)
}

// DeleteLedgerTransaction handles deleting a transaction.
func DeleteLedgerTransaction(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	transactionIDStr := c.Param("tx_id")

	transactionID, err := uuid.Parse(transactionIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid transaction ID")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()
	if err := db.DeleteLedgerTransaction(ctx, accountID, transactionID); err != nil {
		logger.Error("Error deleting ledger transaction", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to delete transaction")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Transaction deleted")
	c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)
}

// CreateLedgerReconciliation handles creating a reconciliation snapshot.
func CreateLedgerReconciliation(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	account, err := db.GetLedgerAccount(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing ledger reconciliation form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconcile/new", http.StatusSeeOther)

		return
	}

	form := c.Request().Form
	balanceStr := strings.TrimSpace(form.Get("balance"))

	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		SetErrorFlash(s, "Balance must be a number")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconcile/new", http.StatusSeeOther)

		return
	}

	balance = normalizeLedgerDebtBalanceInput(balanceStr, balance, account.AccountType)

	reconciledAt, err := parseLedgerDateTime(form.Get("reconciled_at"))
	if err != nil {
		SetErrorFlash(s, "Invalid reconciliation date")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconcile/new", http.StatusSeeOther)

		return
	}

	input := db.CreateLedgerReconciliationInput{
		AccountID:    accountID,
		Balance:      balance,
		ReconciledAt: reconciledAt,
		Note:         getOptionalString(form.Get("note")),
	}

	_, err = db.CreateLedgerReconciliation(ctx, input)
	if err != nil {
		logger.Error("Error creating ledger reconciliation", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to reconcile")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconcile/new", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Reconciled")
	c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)
}

// LedgerReconciliationEditForm renders the edit reconciliation page.
func LedgerReconciliationEditForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	recIDStr := c.Param("rec_id")

	recID, err := uuid.Parse(recIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid reconciliation ID")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	accountSummary, err := db.GetLedgerAccountSummary(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	reconciliation, err := db.GetLedgerReconciliation(ctx, accountID, recID)
	if err != nil {
		logger.Error("Error fetching ledger reconciliation", "error", err)
		SetErrorFlash(s, "Reconciliation not found")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	data["Account"] = accountSummary
	data["Reconciliation"] = reconciliation
	data["IsLedger"] = true
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Ledger", URL: "/ledger", IsCurrent: false},
		{Name: accountSummary.Name, URL: "/ledger/accounts/" + accountIDStr, IsCurrent: false},
		{Name: "Edit Reconciliation", URL: "", IsCurrent: true},
	}

	t.HTML(http.StatusOK, "ledger_reconcile_edit")
}

// UpdateLedgerReconciliation handles updating a reconciliation snapshot.
func UpdateLedgerReconciliation(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	recIDStr := c.Param("rec_id")

	recID, err := uuid.Parse(recIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid reconciliation ID")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	account, err := db.GetLedgerAccount(ctx, accountID)
	if err != nil {
		logger.Error("Error fetching ledger account", "error", err)
		SetErrorFlash(s, "Account not found")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing reconciliation form", "error", err)
		SetErrorFlash(s, "Failed to parse form")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconciliations/"+recIDStr+"/edit", http.StatusSeeOther)

		return
	}

	form := c.Request().Form
	balanceStr := strings.TrimSpace(form.Get("balance"))

	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		SetErrorFlash(s, "Balance must be a number")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconciliations/"+recIDStr+"/edit", http.StatusSeeOther)

		return
	}

	balance = normalizeLedgerDebtBalanceInput(balanceStr, balance, account.AccountType)

	reconciledAt, err := parseLedgerDateTime(form.Get("reconciled_at"))
	if err != nil {
		SetErrorFlash(s, "Invalid reconciliation date")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconciliations/"+recIDStr+"/edit", http.StatusSeeOther)

		return
	}

	input := db.UpdateLedgerReconciliationInput{
		ID:           recID,
		AccountID:    accountID,
		Balance:      balance,
		ReconciledAt: reconciledAt,
		Note:         getOptionalString(form.Get("note")),
	}

	if err := db.UpdateLedgerReconciliation(ctx, input); err != nil {
		logger.Error("Error updating ledger reconciliation", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to update reconciliation")
		c.Redirect("/ledger/accounts/"+accountIDStr+"/reconciliations/"+recIDStr+"/edit", http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Reconciliation updated")
	c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)
}

// DeleteLedgerReconciliation handles deleting a reconciliation snapshot.
func DeleteLedgerReconciliation(c flamego.Context, s session.Session) {
	accountIDStr := c.Param("id")

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid account ID")
		c.Redirect("/ledger", http.StatusSeeOther)

		return
	}

	recIDStr := c.Param("rec_id")

	recID, err := uuid.Parse(recIDStr)
	if err != nil {
		SetErrorFlash(s, "Invalid reconciliation ID")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()
	if err := db.DeleteLedgerReconciliation(ctx, accountID, recID); err != nil {
		logger.Error("Error deleting ledger reconciliation", "error", err)
		setLedgerMutationErrorFlash(s, err, "Failed to delete reconciliation")
		c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Reconciliation deleted")
	c.Redirect("/ledger/accounts/"+accountIDStr, http.StatusSeeOther)
}

func setLedgerMutationErrorFlash(s session.Session, err error, fallback string) {
	if message, ok := ledgerMutationErrorMessage(err); ok {
		SetErrorFlash(s, message)

		return
	}

	SetErrorFlash(s, fallback)
}

func ledgerMutationErrorMessage(err error) (string, bool) {
	switch {
	case errors.Is(err, db.ErrCategoryNameRequired):
		return "Category name is required", true
	case errors.Is(err, db.ErrAmountMustBeGreaterThanZero):
		return "Budget amount must be greater than zero", true
	case errors.Is(err, db.ErrBudgetNotFound):
		return "Budget not found", true
	case errors.Is(err, db.ErrAccountNameRequired):
		return "Account name is required", true
	case errors.Is(err, db.ErrAccountTypeRequired):
		return "Account type is required", true
	case errors.Is(err, db.ErrAccountNotFound):
		return "Account not found", true
	case errors.Is(err, db.ErrMerchantRequired):
		return "Merchant is required", true
	case errors.Is(err, db.ErrAmountMustBeNonZero):
		return "Amount must be non-zero", true
	case errors.Is(err, db.ErrTransactionNotFound):
		return "Transaction not found", true
	case errors.Is(err, db.ErrReconciliationNotFound):
		return "Reconciliation not found", true
	default:
		return "", false
	}
}

type ledgerOption struct {
	Value string
	Label string
}

func ledgerAccountTypeOptions() []ledgerOption {
	return []ledgerOption{
		{Value: string(db.LedgerAccountRegular), Label: "Regular"},
		{Value: string(db.LedgerAccountDebt), Label: "Debt"},
		{Value: string(db.LedgerAccountTracking), Label: "Tracking"},
	}
}

func ledgerTransactionStatusOptions() []ledgerOption {
	return []ledgerOption{
		{Value: string(db.LedgerTransactionCleared), Label: "Cleared"},
		{Value: string(db.LedgerTransactionPending), Label: "Pending"},
		{Value: string(db.LedgerTransactionRefunded), Label: "Refunded"},
		{Value: string(db.LedgerTransactionRejected), Label: "Rejected"},
	}
}

func isValidLedgerAccountType(accountType db.LedgerAccountType) bool {
	switch accountType {
	case db.LedgerAccountRegular, db.LedgerAccountDebt, db.LedgerAccountTracking:
		return true
	default:
		return false
	}
}

func isValidLedgerTransactionStatus(status db.LedgerTransactionStatus) bool {
	switch status {
	case db.LedgerTransactionPending, db.LedgerTransactionCleared,
		db.LedgerTransactionRefunded, db.LedgerTransactionRejected:
		return true
	default:
		return false
	}
}

func hasExplicitLedgerSign(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	first := trimmed[0]

	return first == '+' || first == '-'
}

func normalizeLedgerTransactionAmount(rawValue string, amount float64, accountType db.LedgerAccountType) float64 {
	switch accountType {
	case db.LedgerAccountRegular, db.LedgerAccountDebt:
		if hasExplicitLedgerSign(rawValue) {
			return amount
		}

		return -math.Abs(amount)
	case db.LedgerAccountTracking:
		return amount
	default:
		return amount
	}
}

func normalizeLedgerDebtBalanceInput(rawValue string, amount float64, accountType db.LedgerAccountType) float64 {
	if accountType != db.LedgerAccountDebt {
		return amount
	}

	if hasExplicitLedgerSign(rawValue) {
		return amount
	}

	return -math.Abs(amount)
}

func ledgerTransactionAmountPlaceholder(accountType db.LedgerAccountType) string {
	switch accountType {
	case db.LedgerAccountRegular, db.LedgerAccountDebt:
		return "-0.00"
	case db.LedgerAccountTracking:
		return "0.00"
	default:
		return "0.00"
	}
}

func ledgerTransactionAmountHint(accountType db.LedgerAccountType) string {
	switch accountType {
	case db.LedgerAccountDebt:
		return "No sign defaults to negative (new debt). Use + for payments."
	case db.LedgerAccountRegular:
		return "No sign defaults to negative. Use + for deposits or refunds."
	case db.LedgerAccountTracking:
		return "Use - for outflow and + for inflow."
	default:
		return "Use - for outflow and + for inflow."
	}
}

func parseLedgerDateTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errMissingDate
	}

	if strings.Contains(trimmed, "T") {
		if parsed, err := time.ParseInLocation("2006-01-02T15:04", trimmed, time.Local); err == nil {
			return parsed, nil
		}
	}

	if parsed, err := time.ParseInLocation("2006-01-02", trimmed, time.Local); err == nil {
		return parsed, nil
	}

	return time.Time{}, errInvalidDate
}

func ledgerMonthStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func buildLedgerAccountActivity(transactions []db.LedgerTransactionWithBudget, reconciliations []db.LedgerReconciliation) []ledgerAccountActivity {
	activity := make([]ledgerAccountActivity, 0, len(transactions)+len(reconciliations))

	for i := range transactions {
		tx := transactions[i]
		activity = append(activity, ledgerAccountActivity{
			Type:        ledgerActivityTransaction,
			OccurredAt:  tx.OccurredAt,
			Transaction: &tx,
		})
	}

	for i := range reconciliations {
		rec := reconciliations[i]
		activity = append(activity, ledgerAccountActivity{
			Type:           ledgerActivityReconciliation,
			OccurredAt:     rec.ReconciledAt,
			Reconciliation: &rec,
		})
	}

	sort.Slice(activity, func(i, j int) bool {
		return activity[i].OccurredAt.After(activity[j].OccurredAt)
	})

	return activity
}
