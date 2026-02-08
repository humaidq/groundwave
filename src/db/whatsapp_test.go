// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
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
