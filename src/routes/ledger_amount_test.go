package routes

import (
	"testing"

	"github.com/humaidq/groundwave/db"
)

func TestHasExplicitLedgerSign(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "whitespace", value: "  ", want: false},
		{name: "plain number", value: "100", want: false},
		{name: "negative", value: "-100", want: true},
		{name: "positive", value: "+100", want: true},
		{name: "trimmed sign", value: "  +100", want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := hasExplicitLedgerSign(tt.value)
			if got != tt.want {
				t.Fatalf("hasExplicitLedgerSign(%q) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}

func TestNormalizeLedgerTransactionAmount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rawValue    string
		amount      float64
		accountType db.LedgerAccountType
		want        float64
	}{
		{
			name:        "regular unsigned defaults negative",
			rawValue:    "100",
			amount:      100,
			accountType: db.LedgerAccountRegular,
			want:        -100,
		},
		{
			name:        "regular explicit positive stays positive",
			rawValue:    "+100",
			amount:      100,
			accountType: db.LedgerAccountRegular,
			want:        100,
		},
		{
			name:        "regular explicit negative stays negative",
			rawValue:    "-100",
			amount:      -100,
			accountType: db.LedgerAccountRegular,
			want:        -100,
		},
		{
			name:        "debt unsigned defaults negative",
			rawValue:    "100",
			amount:      100,
			accountType: db.LedgerAccountDebt,
			want:        -100,
		},
		{
			name:        "debt explicit positive means payment",
			rawValue:    "+100",
			amount:      100,
			accountType: db.LedgerAccountDebt,
			want:        100,
		},
		{
			name:        "tracking unsigned remains unchanged",
			rawValue:    "100",
			amount:      100,
			accountType: db.LedgerAccountTracking,
			want:        100,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeLedgerTransactionAmount(tt.rawValue, tt.amount, tt.accountType)
			if got != tt.want {
				t.Fatalf("normalizeLedgerTransactionAmount(%q, %.2f, %q) = %.2f, want %.2f", tt.rawValue, tt.amount, tt.accountType, got, tt.want)
			}
		})
	}
}

func TestNormalizeLedgerDebtBalanceInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rawValue    string
		amount      float64
		accountType db.LedgerAccountType
		want        float64
	}{
		{
			name:        "debt unsigned defaults negative",
			rawValue:    "500",
			amount:      500,
			accountType: db.LedgerAccountDebt,
			want:        -500,
		},
		{
			name:        "debt explicit positive stays positive",
			rawValue:    "+500",
			amount:      500,
			accountType: db.LedgerAccountDebt,
			want:        500,
		},
		{
			name:        "debt explicit negative stays negative",
			rawValue:    "-500",
			amount:      -500,
			accountType: db.LedgerAccountDebt,
			want:        -500,
		},
		{
			name:        "regular remains unchanged",
			rawValue:    "500",
			amount:      500,
			accountType: db.LedgerAccountRegular,
			want:        500,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeLedgerDebtBalanceInput(tt.rawValue, tt.amount, tt.accountType)
			if got != tt.want {
				t.Fatalf("normalizeLedgerDebtBalanceInput(%q, %.2f, %q) = %.2f, want %.2f", tt.rawValue, tt.amount, tt.accountType, got, tt.want)
			}
		})
	}
}
