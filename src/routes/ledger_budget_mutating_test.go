// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

func performLedgerBudgetFormPOST(
	t *testing.T,
	f *flamego.Flame,
	path string,
	form url.Values,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	return rec
}

func assertLedgerBudgetRedirect(t *testing.T, rec *httptest.ResponseRecorder, wantLocation string) {
	t.Helper()

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Fatalf("expected redirect %q, got %q", wantLocation, got)
	}
}

func assertLedgerBudgetFlash(t *testing.T, s *testSession, wantType FlashType, wantMessage string) {
	t.Helper()

	msg, ok := s.flash.(FlashMessage)
	if !ok {
		t.Fatalf("expected flash message, got %T", s.flash)
	}

	if msg.Type != wantType || msg.Message != wantMessage {
		t.Fatalf("unexpected flash message: %#v", msg)
	}
}

func newLedgerBudgetMutatingTestApp(s session.Session) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.Next()
	})

	f.Post("/ledger/budgets/new", func(c flamego.Context, sess session.Session) {
		CreateLedgerBudget(c, sess)
	})
	f.Post("/ledger/budgets/{id}/edit", func(c flamego.Context, sess session.Session) {
		UpdateLedgerBudget(c, sess)
	})

	return f
}

func TestCreateLedgerBudgetRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		amount     string
		wantFlash  string
		wantStatus FlashType
	}{
		{
			name:       "zero",
			amount:     "0",
			wantFlash:  "Budget amount must be greater than zero",
			wantStatus: FlashError,
		},
		{
			name:       "negative",
			amount:     "-10",
			wantFlash:  "Budget amount must be greater than zero",
			wantStatus: FlashError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newTestSession()
			f := newLedgerBudgetMutatingTestApp(s)

			rec := performLedgerBudgetFormPOST(
				t,
				f,
				"/ledger/budgets/new",
				url.Values{
					"category_name": {"Groceries"},
					"amount":        {tt.amount},
				},
			)

			assertLedgerBudgetRedirect(t, rec, "/ledger/budgets/new")
			assertLedgerBudgetFlash(t, s, tt.wantStatus, tt.wantFlash)
		})
	}
}

func TestUpdateLedgerBudgetRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	budgetID := uuid.NewString()
	tests := []struct {
		name      string
		amount    string
		wantFlash string
	}{
		{name: "zero", amount: "0", wantFlash: "Budget amount must be greater than zero"},
		{name: "negative", amount: "-1", wantFlash: "Budget amount must be greater than zero"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newTestSession()
			f := newLedgerBudgetMutatingTestApp(s)

			rec := performLedgerBudgetFormPOST(
				t,
				f,
				"/ledger/budgets/"+budgetID+"/edit",
				url.Values{
					"category_name": {"Groceries"},
					"amount":        {tt.amount},
					"currency":      {"AED"},
				},
			)

			assertLedgerBudgetRedirect(t, rec, "/ledger/budgets/"+budgetID+"/edit")
			assertLedgerBudgetFlash(t, s, FlashError, tt.wantFlash)
		})
	}
}

func TestLedgerMutationErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		want    string
		wantHit bool
	}{
		{
			name:    "amount must be greater than zero",
			err:     db.ErrAmountMustBeGreaterThanZero,
			want:    "Budget amount must be greater than zero",
			wantHit: true,
		},
		{
			name:    "wrapped not found",
			err:     fmt.Errorf("wrapped: %w", db.ErrTransactionNotFound),
			want:    "Transaction not found",
			wantHit: true,
		},
		{
			name:    "unknown error",
			err:     fmt.Errorf("boom"),
			want:    "",
			wantHit: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, gotHit := ledgerMutationErrorMessage(tt.err)
			if got != tt.want || gotHit != tt.wantHit {
				t.Fatalf("ledgerMutationErrorMessage(%v) = (%q, %v), want (%q, %v)", tt.err, got, gotHit, tt.want, tt.wantHit)
			}
		})
	}
}
