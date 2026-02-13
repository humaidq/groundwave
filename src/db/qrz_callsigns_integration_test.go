// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/humaidq/groundwave/utils"
)

func TestSyncMissingQRZCallsignProfilesContextCanceled(t *testing.T) {
	resetDatabase(t)

	processed, err := ImportADIFQSOs(testContext(), []utils.QSO{{
		Call:    "A65RW",
		QSODate: "20250714",
		TimeOn:  "192149",
		Band:    "20m",
		Mode:    "SSB",
	}})
	if err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}

	if processed != 1 {
		t.Fatalf("expected 1 processed QSO, got %d", processed)
	}

	t.Setenv("QRZ_XML_USERNAME", "demo")
	t.Setenv("QRZ_XML_PASSWORD", "demo-pass")

	canceledCtx, cancel := context.WithCancel(testContext())
	cancel()

	_, err = SyncMissingQRZCallsignProfiles(canceledCtx, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}

func TestGetQSLCallsignProfileDetail(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	qsos := []utils.QSO{
		{
			Call:    "B2BBB",
			QSODate: "20250102",
			TimeOn:  "120000",
			Band:    "20m",
			Mode:    "SSB",
			Country: "Spain",
		},
		{
			Call:    "A1AAA",
			QSODate: "20250101",
			TimeOn:  "123000",
			Band:    "40m",
			Mode:    "CW",
			Country: "Japan",
		},
	}

	processed, err := ImportADIFQSOs(ctx, qsos)
	if err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}

	if processed != len(qsos) {
		t.Fatalf("expected %d processed QSOs, got %d", len(qsos), processed)
	}

	bCallsign := "B2BBB"
	contactID := mustCreateContact(t, CreateContactInput{
		NameGiven: "Linked Contact",
		CallSign:  &bCallsign,
		Tier:      TierB,
	})

	noQSOCallsign := "C3CCC"
	mustCreateContact(t, CreateContactInput{
		NameGiven: "No QSO Contact",
		CallSign:  &noQSOCallsign,
		Tier:      TierC,
	})

	_, err = pool.Exec(ctx, `
		INSERT INTO qrz_callsign_profiles (
			callsign,
			name_fmt,
			nickname,
			addr1,
			addr2,
			state,
			zip,
			country,
			grid,
			county,
			qslmgr,
			email,
			qrz_user,
			aliases,
			xref,
			lat,
			lon,
			dxcc
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`,
		"A1AAA",
		"Alpha One",
		"Alpha",
		"123 Beacon Road",
		"Ajman",
		"Ajman",
		"00000",
		"United Arab Emirates",
		"LL75tj",
		"Ajman",
		"QSL Manager",
		"alpha@example.com",
		"alphauser",
		"A1AAA",
		"XREF1",
		25.1234,
		55.5678,
		391,
	)
	if err != nil {
		t.Fatalf("failed to insert A1AAA profile row: %v", err)
	}

	detail, err := GetQSLCallsignProfileDetail(ctx, "a1aaa")
	if err != nil {
		t.Fatalf("GetQSLCallsignProfileDetail failed: %v", err)
	}

	if detail.Callsign != "A1AAA" {
		t.Fatalf("expected callsign A1AAA, got %q", detail.Callsign)
	}

	if detail.DisplayName() != "Alpha One" {
		t.Fatalf("expected display name Alpha One, got %q", detail.DisplayName())
	}

	if !detail.HasAddress() {
		t.Fatalf("expected A1AAA to have address lines")
	}

	if detail.QSLMgr == nil || *detail.QSLMgr != "QSL Manager" {
		t.Fatalf("expected qsl manager QSL Manager, got %v", detail.QSLMgr)
	}

	if detail.Email == nil || *detail.Email != "alpha@example.com" {
		t.Fatalf("expected email alpha@example.com, got %v", detail.Email)
	}

	if detail.DXCCString() != "391" {
		t.Fatalf("expected dxcc 391, got %q", detail.DXCCString())
	}

	if detail.CoordinateString() != "25.12340, 55.56780" {
		t.Fatalf("unexpected coordinate string: %q", detail.CoordinateString())
	}

	if !detail.HasProfileData() {
		t.Fatalf("expected A1AAA to have profile data")
	}

	contactDetail, err := GetQSLCallsignProfileDetail(ctx, "B2BBB")
	if err != nil {
		t.Fatalf("GetQSLCallsignProfileDetail failed: %v", err)
	}

	if !contactDetail.HasContact() {
		t.Fatalf("expected B2BBB to have linked contact")
	}

	if contactDetail.ContactIDValue() != contactID {
		t.Fatalf("expected contact id %q, got %q", contactID, contactDetail.ContactIDValue())
	}

	if contactDetail.HasProfileData() {
		t.Fatalf("expected B2BBB to have no profile data")
	}
}

func TestSyncQRZCallsignProfile(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	t.Setenv("QRZ_XML_USERNAME", "demo")
	t.Setenv("QRZ_XML_PASSWORD", "demo-pass")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/xml")

		if r.Form.Get("username") != "" {
			_, _ = w.Write([]byte(`<?xml version="1.0"?><QRZDatabase version="1.34"><Session><Key>test-session-key</Key><GMTime>Fri Feb 13 15:00:00 2026</GMTime></Session></QRZDatabase>`))
			return
		}

		if r.Form.Get("s") != "test-session-key" {
			_, _ = w.Write([]byte(`<?xml version="1.0"?><QRZDatabase version="1.34"><Session><Error>Session Timeout</Error></Session></QRZDatabase>`))
			return
		}

		if strings.EqualFold(r.Form.Get("callsign"), "A65RW") {
			_, _ = w.Write([]byte(`<?xml version="1.0"?><QRZDatabase version="1.34"><Callsign><call>A65RW</call><fname>Reiner</fname><name>Klohn</name><name_fmt>Reiner Klohn</name_fmt><addr1>PO Box 1</addr1><addr2>Ajman</addr2><state></state><zip></zip><country>United Arab Emirates</country><lat>25.407897</lat><lon>55.590072</lon><grid>LL75tj</grid><dxcc>391</dxcc><ccode>268</ccode><email>A65RW@proton.me</email><nickname></nickname><attn></attn><user></user></Callsign><Session><Key>test-session-key</Key></Session></QRZDatabase>`))
			return
		}

		_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><QRZDatabase version="1.34"><Session><Key>test-session-key</Key><Error>Not found: %s</Error></Session></QRZDatabase>`, r.Form.Get("callsign"))
	}))
	defer server.Close()

	originalServiceURL := qrzXMLServiceURL
	qrzXMLServiceURL = server.URL

	t.Cleanup(func() {
		qrzXMLServiceURL = originalServiceURL
	})

	if err := SyncQRZCallsignProfile(ctx, "a65rw"); err != nil {
		t.Fatalf("SyncQRZCallsignProfile failed: %v", err)
	}

	detail, err := GetQSLCallsignProfileDetail(ctx, "A65RW")
	if err != nil {
		t.Fatalf("GetQSLCallsignProfileDetail failed: %v", err)
	}

	if detail.DisplayName() != "Reiner Klohn" {
		t.Fatalf("expected display name Reiner Klohn, got %q", detail.DisplayName())
	}

	if detail.Email == nil || *detail.Email != "A65RW@proton.me" {
		t.Fatalf("expected email A65RW@proton.me, got %v", detail.Email)
	}

	if detail.Grid == nil || *detail.Grid != "LL75tj" {
		t.Fatalf("expected grid LL75tj, got %v", detail.Grid)
	}
}

func TestListQSLCallsignProfiles(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	qsos := []utils.QSO{
		{
			Call:    "B2BBB",
			QSODate: "20250102",
			TimeOn:  "120000",
			Band:    "20m",
			Mode:    "SSB",
			Country: "Spain",
		},
		{
			Call:    "A1AAA",
			QSODate: "20250101",
			TimeOn:  "123000",
			Band:    "40m",
			Mode:    "CW",
			Country: "Japan",
		},
		{
			Call:    "B2BBB",
			QSODate: "20250103",
			TimeOn:  "070000",
			Band:    "15m",
			Mode:    "FT8",
			Country: "Spain",
		},
	}

	processed, err := ImportADIFQSOs(ctx, qsos)
	if err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}

	if processed != len(qsos) {
		t.Fatalf("expected %d processed QSOs, got %d", len(qsos), processed)
	}

	bCallsign := "B2BBB"
	contactID := mustCreateContact(t, CreateContactInput{
		NameGiven: "Linked Contact",
		CallSign:  &bCallsign,
		Tier:      TierB,
	})

	payload := map[string]any{
		"xml_version": "1.34",
		"session": map[string]string{
			"Key": "test-key",
		},
		"callsign": map[string]string{
			"call":     "A1AAA",
			"name_fmt": "Alpha One",
			"addr1":    "123 Beacon Road",
			"addr2":    "Ajman",
			"country":  "United Arab Emirates",
		},
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO qrz_callsign_profiles (
			callsign,
			name_fmt,
			addr1,
			addr2,
			country,
			payload_json,
			last_lookup_at,
			last_success_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, NOW(), NOW())
	`, "A1AAA", "Alpha One", "123 Beacon Road", "Ajman", "United Arab Emirates", payload)
	if err != nil {
		t.Fatalf("failed to insert A1AAA profile row: %v", err)
	}

	profiles, err := ListQSLCallsignProfiles(ctx)
	if err != nil {
		t.Fatalf("ListQSLCallsignProfiles failed: %v", err)
	}

	if len(profiles) != 3 {
		t.Fatalf("expected 3 grouped callsigns, got %d", len(profiles))
	}

	if profiles[0].Callsign != "A1AAA" || profiles[1].Callsign != "B2BBB" || profiles[2].Callsign != "C3CCC" {
		t.Fatalf("expected alphabetical order [A1AAA, B2BBB, C3CCC], got [%s, %s, %s]", profiles[0].Callsign, profiles[1].Callsign, profiles[2].Callsign)
	}

	if profiles[0].DisplayName() != "Alpha One" {
		t.Fatalf("expected A1AAA display name Alpha One, got %q", profiles[0].DisplayName())
	}

	if !profiles[0].HasAddress() {
		t.Fatalf("expected A1AAA to have address lines")
	}

	if !profiles[1].HasContact() {
		t.Fatalf("expected B2BBB to have linked contact")
	}

	if profiles[1].ContactIDValue() != contactID {
		t.Fatalf("expected contact id %q, got %q", contactID, profiles[1].ContactIDValue())
	}

	if len(profiles[1].QSOs) != 2 {
		t.Fatalf("expected 2 QSOs for B2BBB, got %d", len(profiles[1].QSOs))
	}

	if len(profiles[0].QSOs) != 1 {
		t.Fatalf("expected 1 QSO for A1AAA, got %d", len(profiles[0].QSOs))
	}

	if len(profiles[2].QSOs) != 0 {
		t.Fatalf("expected 0 QSOs for C3CCC, got %d", len(profiles[2].QSOs))
	}
}

func TestSyncMissingQRZCallsignProfiles(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	qsos := []utils.QSO{
		{
			Call:    "A65RW",
			QSODate: "20250714",
			TimeOn:  "192149",
			Band:    "20m",
			Mode:    "SSB",
		},
	}

	processed, err := ImportADIFQSOs(ctx, qsos)
	if err != nil {
		t.Fatalf("ImportADIFQSOs failed: %v", err)
	}

	if processed != 1 {
		t.Fatalf("expected 1 processed QSO, got %d", processed)
	}

	t.Setenv("QRZ_XML_USERNAME", "demo")
	t.Setenv("QRZ_XML_PASSWORD", "demo-pass")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/xml")

		if r.Form.Get("username") != "" {
			_, _ = w.Write([]byte(`<?xml version="1.0"?><QRZDatabase version="1.34"><Session><Key>test-session-key</Key><GMTime>Fri Feb 13 15:00:00 2026</GMTime></Session></QRZDatabase>`))

			return
		}

		if r.Form.Get("s") != "test-session-key" {
			_, _ = w.Write([]byte(`<?xml version="1.0"?><QRZDatabase version="1.34"><Session><Error>Session Timeout</Error></Session></QRZDatabase>`))

			return
		}

		if strings.EqualFold(r.Form.Get("callsign"), "A65RW") {
			_, _ = w.Write([]byte(`<?xml version="1.0"?><QRZDatabase version="1.34"><Callsign><call>A65RW</call><fname>Reiner</fname><name>Klohn</name><name_fmt>Reiner Klohn</name_fmt><addr1></addr1><addr2>Ajman</addr2><state></state><zip></zip><country>United Arab Emirates</country><lat>25.407897</lat><lon>55.590072</lon><grid>LL75tj</grid><dxcc>391</dxcc><ccode>268</ccode><email>A65RW@proton.me</email><nickname></nickname><attn></attn><user></user></Callsign><Session><Key>test-session-key</Key></Session></QRZDatabase>`))

			return
		}

		_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><QRZDatabase version="1.34"><Session><Key>test-session-key</Key><Error>Not found: %s</Error></Session></QRZDatabase>`, r.Form.Get("callsign"))
	}))
	defer server.Close()

	originalServiceURL := qrzXMLServiceURL
	qrzXMLServiceURL = server.URL

	t.Cleanup(func() {
		qrzXMLServiceURL = originalServiceURL
	})

	result, err := SyncMissingQRZCallsignProfiles(ctx, 10)
	if err != nil {
		t.Fatalf("SyncMissingQRZCallsignProfiles failed: %v", err)
	}

	if result.Updated != 1 {
		t.Fatalf("expected 1 updated callsign profile, got %d", result.Updated)
	}

	if result.Failed != 0 {
		t.Fatalf("expected 0 failed lookups, got %d", result.Failed)
	}

	var (
		nameFmt   *string
		addr2     *string
		country   *string
		payload   []byte
		lastError *string
	)

	err = pool.QueryRow(ctx, `
		SELECT name_fmt, addr2, country, payload_json, last_lookup_error
		FROM qrz_callsign_profiles
		WHERE callsign = $1
	`, "A65RW").Scan(&nameFmt, &addr2, &country, &payload, &lastError)
	if err != nil {
		t.Fatalf("failed to query synced callsign profile: %v", err)
	}

	if nameFmt == nil || *nameFmt != "Reiner Klohn" {
		t.Fatalf("expected name_fmt Reiner Klohn, got %+v", nameFmt)
	}

	if addr2 == nil || *addr2 != "Ajman" {
		t.Fatalf("expected addr2 Ajman, got %+v", addr2)
	}

	if country == nil || *country != "United Arab Emirates" {
		t.Fatalf("expected country United Arab Emirates, got %+v", country)
	}

	if lastError != nil {
		t.Fatalf("expected no lookup error, got %q", *lastError)
	}

	var payloadMap map[string]any
	if err := json.Unmarshal(payload, &payloadMap); err != nil {
		t.Fatalf("failed to unmarshal payload_json: %v", err)
	}

	callsignDataRaw, ok := payloadMap["callsign"]
	if !ok {
		t.Fatalf("expected payload_json.callsign to exist")
	}

	callsignData, ok := callsignDataRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected payload_json.callsign to be object, got %T", callsignDataRaw)
	}

	if got := callsignData["call"]; got != "A65RW" {
		t.Fatalf("expected payload call A65RW, got %#v", got)
	}

	if _, ok := callsignData["nickname"]; !ok {
		t.Fatalf("expected payload to preserve empty nickname field")
	}
}
