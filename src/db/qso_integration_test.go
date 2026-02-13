// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"math"
	"strings"
	"testing"
	"time"

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

	if len(qsos) == 0 {
		t.Fatalf("expected imported QSOs to contain at least one entry")
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

func TestQSOImportAdditionalFieldsAndJSON(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	qsos := []utils.QSO{
		{
			Call:                   "A65EB",
			QSODate:                "20250121",
			TimeOn:                 "131400",
			QSODateOff:             "20250121",
			TimeOff:                "131400",
			Band:                   "2m",
			BandRx:                 "15m",
			Mode:                   "FM",
			Submode:                "FT4",
			Freq:                   "144.65",
			FreqRx:                 "21.074",
			RSTSent:                "59",
			RSTRcvd:                "59",
			QTH:                    "ajman",
			Name:                   "Rashed Shwaiki",
			Comment:                "test",
			Notes:                  "grid approx",
			GridSquare:             "LL75",
			Country:                "United Arab Emirates",
			DXCC:                   "391",
			CQZ:                    "21",
			ITUZ:                   "39",
			Cont:                   "AS",
			State:                  "AZ",
			Cnty:                   "VA,CULPEPER",
			Pfx:                    "A65",
			IOTA:                   "AS-001",
			Distance:               "55.606462659392456",
			AIndex:                 "12",
			KIndex:                 "3",
			SFI:                    "150",
			MyName:                 "A66H",
			MyCity:                 "Ajman",
			MyCountry:              "United Arab Emirates",
			MyCQZone:               "21",
			MyITUZone:              "39",
			MyDXCC:                 "391",
			MyGridSquare:           "LL75SJ",
			StationCall:            "A66H",
			Operator:               "A66H",
			MyRig:                  "Icom IC-5100",
			MyAntenna:              "DX Commander Signature 9",
			TxPwr:                  "50",
			QslSent:                utils.QslYes,
			QslRcvd:                utils.QslYes,
			QSLSDate:               "20250301",
			QSLRDate:               "20250302",
			QSLSentVia:             "D",
			QSLRcvdVia:             "E",
			QSLVia:                 "Email",
			QSLMsg:                 "tnx",
			QSLMsgRcvd:             "tnx!",
			LotwSent:               utils.QslYes,
			LotwRcvd:               utils.QslYes,
			LotwQSLSDate:           "20250303",
			LotwQSLRDate:           "20250304",
			EqslSent:               utils.QslStatus("Q"),
			EqslRcvd:               utils.QslYes,
			EqslQSLSDate:           "20250305",
			EqslQSLRDate:           "20250306",
			EqslAG:                 "Y",
			ClublogQSOUploadDate:   "20250307",
			ClublogQSOUploadStatus: "M",
			HRDLogQSOUploadDate:    "20250308",
			HRDLogQSOUploadStatus:  "Y",
			AppFields: map[string]any{
				"app_cqrlog_profile": "1|LL75sj|Home|Xiegu G90|",
			},
			UserFields: map[string]any{
				"userdef1": "hello",
			},
		},
	}

	count, err := ImportADIFQSOs(ctx, qsos)
	if err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 QSO processed, got %d", count)
	}

	all, err := ListQSOs(ctx)
	if err != nil {
		t.Fatalf("ListQSOs failed: %v", err)
	}

	if len(all) != 1 {
		t.Fatalf("expected 1 QSO, got %d", len(all))
	}

	detail, err := GetQSO(ctx, all[0].ID)
	if err != nil {
		t.Fatalf("GetQSO failed: %v", err)
	}

	if detail.Distance == nil {
		t.Fatalf("expected distance to be set")
	}

	if math.Abs(*detail.Distance-55.606462659392456) > 1e-12 {
		t.Fatalf("unexpected distance value: %v", *detail.Distance)
	}

	if detail.CQZ == nil || *detail.CQZ != 21 {
		t.Fatalf("expected CQZ 21, got %+v", detail.CQZ)
	}

	if detail.ITUZ == nil || *detail.ITUZ != 39 {
		t.Fatalf("expected ITUZ 39, got %+v", detail.ITUZ)
	}

	if detail.Cont == nil || *detail.Cont != "AS" {
		t.Fatalf("expected continent AS, got %+v", detail.Cont)
	}

	if detail.QSLSDate == nil || detail.QSLSDate.Format("20060102") != "20250301" {
		t.Fatalf("expected QSLSDate 20250301, got %+v", detail.QSLSDate)
	}

	if detail.QSLRDate == nil || detail.QSLRDate.Format("20060102") != "20250302" {
		t.Fatalf("expected QSLRDate 20250302, got %+v", detail.QSLRDate)
	}

	if detail.QSLSentVia == nil || *detail.QSLSentVia != QSLViaDirect {
		t.Fatalf("expected qsl_sent_via D, got %+v", detail.QSLSentVia)
	}

	if detail.QSLRcvdVia == nil || *detail.QSLRcvdVia != QSLViaElectronic {
		t.Fatalf("expected qsl_rcvd_via E, got %+v", detail.QSLRcvdVia)
	}

	if detail.ClublogQSOUploadStatus == nil || *detail.ClublogQSOUploadStatus != QSOModified {
		t.Fatalf("expected clublog upload status M, got %+v", detail.ClublogQSOUploadStatus)
	}

	if detail.HRDLogQSOUploadStatus == nil || *detail.HRDLogQSOUploadStatus != QSOUploaded {
		t.Fatalf("expected hrdlog upload status Y, got %+v", detail.HRDLogQSOUploadStatus)
	}

	if detail.AppFields["app_cqrlog_profile"] != "1|LL75sj|Home|Xiegu G90|" {
		t.Fatalf("expected app_fields to include app_cqrlog_profile, got %+v", detail.AppFields)
	}

	if detail.UserFields["userdef1"] != "hello" {
		t.Fatalf("expected user_fields to include userdef1, got %+v", detail.UserFields)
	}
}

func TestExportADIF(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	qsos := []utils.QSO{
		{
			Call:    "K1ABC",
			QSODate: "20240102",
			TimeOn:  "130501",
			Band:    "20m",
			Mode:    "SSB",
		},
		{
			Call:    "K2XYZ",
			QSODate: "20240203",
			TimeOn:  "101500",
			Band:    "40m",
			Mode:    "CW",
		},
	}

	count, err := ImportADIFQSOs(ctx, qsos)
	if err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 QSOs processed, got %d", count)
	}

	allExport, err := ExportADIF(ctx, nil, nil)
	if err != nil {
		t.Fatalf("ExportADIF failed: %v", err)
	}

	if !strings.Contains(allExport, "<ADIF_VER:5>3.1.6") {
		t.Fatalf("expected ADIF header, got %q", allExport)
	}

	if !strings.Contains(allExport, "<CALL:5>K1ABC") || !strings.Contains(allExport, "<CALL:5>K2XYZ") {
		t.Fatalf("expected both calls in full export, got %q", allExport)
	}

	if strings.Count(allExport, "<EOR>") != 2 {
		t.Fatalf("expected 2 records, got %d", strings.Count(allExport, "<EOR>"))
	}

	fromDate := time.Date(2024, time.February, 1, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC)

	rangeExport, err := ExportADIF(ctx, &fromDate, &toDate)
	if err != nil {
		t.Fatalf("ExportADIF with date range failed: %v", err)
	}

	if strings.Contains(rangeExport, "<CALL:5>K1ABC") {
		t.Fatalf("did not expect January QSO in filtered export, got %q", rangeExport)
	}

	if !strings.Contains(rangeExport, "<CALL:5>K2XYZ") {
		t.Fatalf("expected February QSO in filtered export, got %q", rangeExport)
	}
}
