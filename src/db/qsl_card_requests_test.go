// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/google/uuid"

	"github.com/humaidq/groundwave/utils"
)

func TestQSLCardRequestLifecycle(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	qsos := []utils.QSO{
		{
			Call:    "K1ABC",
			QSODate: "20240202",
			TimeOn:  "130501",
			Mode:    "SSB",
			Band:    "20m",
			Country: "Japan",
		},
	}

	if _, err := ImportADIFQSOs(ctx, qsos); err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}

	allQSOs, err := ListQSOs(ctx)
	if err != nil {
		t.Fatalf("ListQSOs failed: %v", err)
	}

	if len(allQSOs) != 1 {
		t.Fatalf("expected 1 QSO, got %d", len(allQSOs))
	}

	qsoID := allQSOs[0].ID

	err = CreateQSLCardRequest(ctx, CreateQSLCardRequestInput{
		QSOID:          qsoID,
		RequesterName:  "Alice Operator",
		MailingAddress: "123 DX Lane\nTokyo\nJapan",
		Note:           "Please sign the card.",
	})
	if err != nil {
		t.Fatalf("CreateQSLCardRequest failed: %v", err)
	}

	hasOpen, err := HasOpenQSLCardRequestForQSO(ctx, qsoID)
	if err != nil {
		t.Fatalf("HasOpenQSLCardRequestForQSO failed: %v", err)
	}

	if !hasOpen {
		t.Fatalf("expected open request to be detected")
	}

	requests, err := ListOpenQSLCardRequests(ctx)
	if err != nil {
		t.Fatalf("ListOpenQSLCardRequests failed: %v", err)
	}

	if len(requests) != 1 {
		t.Fatalf("expected 1 open request, got %d", len(requests))
	}

	request := requests[0]
	if request.Call != "K1ABC" {
		t.Fatalf("expected call K1ABC, got %q", request.Call)
	}

	if request.QSOID.String() != qsoID {
		t.Fatalf("expected qso id %q, got %q", qsoID, request.QSOID.String())
	}

	if request.RequesterName == nil || *request.RequesterName != "Alice Operator" {
		t.Fatalf("expected requester name to be stored")
	}

	if request.MailingAddress != "123 DX Lane\nTokyo\nJapan" {
		t.Fatalf("unexpected mailing address: %q", request.MailingAddress)
	}

	if request.Note == nil || *request.Note != "Please sign the card." {
		t.Fatalf("expected note to be stored")
	}

	if err := DismissQSLCardRequest(ctx, request.ID.String()); err != nil {
		t.Fatalf("DismissQSLCardRequest failed: %v", err)
	}

	requests, err = ListOpenQSLCardRequests(ctx)
	if err != nil {
		t.Fatalf("ListOpenQSLCardRequests after dismiss failed: %v", err)
	}

	if len(requests) != 0 {
		t.Fatalf("expected 0 open requests after dismiss, got %d", len(requests))
	}

	hasOpen, err = HasOpenQSLCardRequestForQSO(ctx, qsoID)
	if err != nil {
		t.Fatalf("HasOpenQSLCardRequestForQSO after dismiss failed: %v", err)
	}

	if hasOpen {
		t.Fatalf("expected no open request after dismiss")
	}

	qsoDetail, err := GetQSO(ctx, qsoID)
	if err != nil {
		t.Fatalf("GetQSO failed: %v", err)
	}

	if qsoDetail.QSLSent != nil {
		t.Fatalf("expected qsl_sent to remain unchanged")
	}

	if qsoDetail.QSLRcvd != nil {
		t.Fatalf("expected qsl_rcvd to remain unchanged")
	}
}

func TestQSLCardRequestErrors(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	if err := CreateQSLCardRequest(ctx, CreateQSLCardRequestInput{}); err == nil {
		t.Fatalf("expected error for missing qso id and address")
	}

	if _, err := HasOpenQSLCardRequestForQSO(ctx, ""); err == nil {
		t.Fatalf("expected error for missing qso id in open request check")
	}

	if err := DismissQSLCardRequest(ctx, uuid.New().String()); err == nil {
		t.Fatalf("expected error when dismissing missing request")
	}
}
