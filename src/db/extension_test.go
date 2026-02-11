// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import "testing"

func TestExtensionLinkedInQueries(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	contact1 := mustCreateContact(t, CreateContactInput{NameGiven: "Linked", Tier: TierB})
	contact2 := mustCreateContact(t, CreateContactInput{NameGiven: "NoLinked", Tier: TierB})

	if err := AddURL(ctx, AddURLInput{ContactID: contact1, URL: "https://linkedin.com/in/linked", URLType: URLLinkedIn}); err != nil {
		t.Fatalf("AddURL failed: %v", err)
	}

	urls, err := ListLinkedInURLs(ctx)
	if err != nil {
		t.Fatalf("ListLinkedInURLs failed: %v", err)
	}

	if len(urls) != 1 {
		t.Fatalf("expected 1 linkedIn url, got %d", len(urls))
	}

	missing, err := ListContactsWithoutLinkedIn(ctx)
	if err != nil {
		t.Fatalf("ListContactsWithoutLinkedIn failed: %v", err)
	}

	if len(missing) != 1 || missing[0].ID != contact2 {
		t.Fatalf("expected contact without LinkedIn")
	}
}
