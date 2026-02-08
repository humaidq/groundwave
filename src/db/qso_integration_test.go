// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/humaidq/groundwave/utils"
)

func TestQSOImportAndQueries(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	callSign := "K1ABC"
	_ = mustCreateContact(t, CreateContactInput{
		NameGiven: "Call",
		CallSign:  &callSign,
		Tier:      TierB,
	})

	qsos := []utils.QSO{
		{
			Call:    callSign,
			QSODate: "20240102",
			TimeOn:  "130501",
			Band:    "20m",
			Mode:    "SSB",
			RSTSent: "59",
			RSTRcvd: "59",
			Country: "Japan",
			DXCC:    "291",
		},
	}

	count, err := ImportADIFQSOs(ctx, qsos)
	if err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 QSO imported, got %d", count)
	}

	all, err := ListQSOs(ctx)
	if err != nil {
		t.Fatalf("ListQSOs failed: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 QSO, got %d", len(all))
	}

	recent, err := ListRecentQSOs(ctx, 1)
	if err != nil {
		t.Fatalf("ListRecentQSOs failed: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent QSO, got %d", len(recent))
	}

	detail, err := GetQSO(ctx, all[0].ID)
	if err != nil {
		t.Fatalf("GetQSO failed: %v", err)
	}
	if detail.Call != callSign {
		t.Fatalf("expected call %q, got %q", callSign, detail.Call)
	}

	byCall, err := GetQSOsByCallSign(ctx, callSign)
	if err != nil {
		t.Fatalf("GetQSOsByCallSign failed: %v", err)
	}
	if len(byCall) != 1 {
		t.Fatalf("expected 1 QSO by call, got %d", len(byCall))
	}

	countAll, err := GetQSOCount(ctx)
	if err != nil {
		t.Fatalf("GetQSOCount failed: %v", err)
	}
	if countAll != 1 {
		t.Fatalf("expected 1 QSO count, got %d", countAll)
	}

	qsos[0].Mode = "CW"
	updatedCount, err := ImportADIFQSOs(ctx, qsos)
	if err != nil {
		t.Fatalf("ImportADIFQSOs update failed: %v", err)
	}
	if updatedCount != 1 {
		t.Fatalf("expected 1 QSO processed, got %d", updatedCount)
	}
}
