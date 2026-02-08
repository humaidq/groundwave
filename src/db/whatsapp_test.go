// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"

	"github.com/emersion/go-vcard"
)

func TestWhatsAppPhoneMatching(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	if normalizePhone("+1 (415) 555-1234") != "14155551234" {
		t.Fatalf("expected normalized phone")
	}
	if !phonesMatch("14155551234", "4155551234") {
		t.Fatalf("expected suffix phone match")
	}
	if phonesMatch("", "123") {
		t.Fatalf("expected empty phone to not match")
	}

	phone := "+1 415 555 1234"
	contactID := mustCreateContact(t, CreateContactInput{NameGiven: "Caller", Phone: &phone, Tier: TierB})

	found, err := FindContactByPhone(ctx, "(415) 555-1234")
	if err != nil {
		t.Fatalf("FindContactByPhone failed: %v", err)
	}
	if found == nil || *found != contactID {
		t.Fatalf("expected to find contact by phone")
	}

	if err := UpdateContactAutoTimestamp(ctx, contactID, time.Now().UTC()); err != nil {
		t.Fatalf("UpdateContactAutoTimestamp failed: %v", err)
	}
}

func TestFindContactByPhoneCardDAV(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	server := newCardDAVTestServer(t)
	defer server.close()

	card := make(vcard.Card)
	card.SetValue(vcard.FieldUID, "card-1")
	card.SetValue(vcard.FieldFormattedName, "Card User")
	card.Add(vcard.FieldTelephone, &vcard.Field{Value: "+1 555 0000", Params: vcard.Params{vcard.ParamType: []string{"cell"}}})
	server.cards["card-1.vcf"] = card

	t.Setenv("CARDDAV_URL", server.server.URL+"/addressbook/")
	t.Setenv("CARDDAV_USERNAME", "user")
	t.Setenv("CARDDAV_PASSWORD", "pass")

	carddavID := "card-1"
	contactID := mustCreateContact(t, CreateContactInput{NameGiven: "Card", CardDAVUUID: &carddavID, Tier: TierB})

	found, err := FindContactByPhone(ctx, "555-0000")
	if err != nil {
		t.Fatalf("FindContactByPhone failed: %v", err)
	}
	if found == nil || *found != contactID {
		t.Fatalf("expected to find contact by CardDAV phone")
	}
}
