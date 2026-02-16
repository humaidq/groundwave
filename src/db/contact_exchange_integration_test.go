// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"errors"
	"testing"
	"time"
)

func TestMeContactLifecycle(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	contactOne := mustCreateContact(t, CreateContactInput{NameGiven: "Me One", Tier: TierB})
	contactTwo := mustCreateContact(t, CreateContactInput{NameGiven: "Me Two", Tier: TierC})

	me, err := GetMeContact(ctx)
	if err != nil {
		t.Fatalf("GetMeContact failed: %v", err)
	}

	if me != nil {
		t.Fatalf("expected no me-contact initially, got %s", me.ID)
	}

	if err := SetContactAsMe(ctx, contactOne); err != nil {
		t.Fatalf("SetContactAsMe failed: %v", err)
	}

	me, err = GetMeContact(ctx)
	if err != nil {
		t.Fatalf("GetMeContact failed: %v", err)
	}

	if me == nil || me.ID.String() != contactOne {
		t.Fatalf("expected contact %s as me-contact, got %#v", contactOne, me)
	}

	if err := SetContactAsMe(ctx, contactTwo); err != nil {
		t.Fatalf("SetContactAsMe second contact failed: %v", err)
	}

	var (
		isMeOne bool
		isMeTwo bool
	)

	if err := pool.QueryRow(ctx, `SELECT is_me FROM contacts WHERE id = $1`, contactOne).Scan(&isMeOne); err != nil {
		t.Fatalf("failed to read is_me for first contact: %v", err)
	}

	if err := pool.QueryRow(ctx, `SELECT is_me FROM contacts WHERE id = $1`, contactTwo).Scan(&isMeTwo); err != nil {
		t.Fatalf("failed to read is_me for second contact: %v", err)
	}

	if isMeOne {
		t.Fatalf("expected first contact to be cleared as me-contact")
	}

	if !isMeTwo {
		t.Fatalf("expected second contact to be marked as me-contact")
	}

	if err := ClearContactAsMe(ctx, contactTwo); err != nil {
		t.Fatalf("ClearContactAsMe failed: %v", err)
	}

	me, err = GetMeContact(ctx)
	if err != nil {
		t.Fatalf("GetMeContact failed: %v", err)
	}

	if me != nil {
		t.Fatalf("expected no me-contact after clear, got %s", me.ID)
	}

	if err := SetContactAsMe(ctx, ""); !errors.Is(err, ErrContactNotFound) {
		t.Fatalf("expected ErrContactNotFound for empty contact id, got %v", err)
	}
}

func TestContactExchangeLinkLifecycle(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()
	contactID := mustCreateContact(t, CreateContactInput{NameGiven: "Exchange Contact", Tier: TierB})

	if _, err := CreateContactExchangeLink(ctx, contactID, false, false, "", time.Hour); !errors.Is(err, ErrContactExchangeCollectFieldEmpty) {
		t.Fatalf("expected ErrContactExchangeCollectFieldEmpty, got %v", err)
	}

	linkOne, err := CreateContactExchangeLink(ctx, contactID, true, false, "  X University  ", time.Hour)
	if err != nil {
		t.Fatalf("CreateContactExchangeLink first link failed: %v", err)
	}

	if !linkOne.CollectPhone || linkOne.CollectEmail {
		t.Fatalf("unexpected first link flags: %#v", linkOne)
	}

	if linkOne.AdditionalNote != "X University" {
		t.Fatalf("expected trimmed additional note on first link, got %q", linkOne.AdditionalNote)
	}

	active, err := GetActiveContactExchangeLink(ctx, contactID)
	if err != nil {
		t.Fatalf("GetActiveContactExchangeLink failed: %v", err)
	}

	if active == nil || active.Token != linkOne.Token {
		t.Fatalf("expected first link to be active, got %#v", active)
	}

	linkTwo, err := CreateContactExchangeLink(ctx, contactID, false, true, "Club Night", time.Hour)
	if err != nil {
		t.Fatalf("CreateContactExchangeLink second link failed: %v", err)
	}

	if linkTwo.Token == linkOne.Token {
		t.Fatalf("expected regenerated link to rotate token")
	}

	if linkTwo.AdditionalNote != "Club Night" {
		t.Fatalf("expected additional note on second link, got %q", linkTwo.AdditionalNote)
	}

	var firstUsedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT used_at FROM contact_exchange_links WHERE id = $1`, linkOne.ID).Scan(&firstUsedAt); err != nil {
		t.Fatalf("failed to read first link used_at: %v", err)
	}

	if firstUsedAt == nil {
		t.Fatalf("expected first link to be invalidated when second was created")
	}

	byToken, err := GetContactExchangeLinkByToken(ctx, linkTwo.Token)
	if err != nil {
		t.Fatalf("GetContactExchangeLinkByToken failed: %v", err)
	}

	if byToken == nil || byToken.ID != linkTwo.ID {
		t.Fatalf("expected second link by token, got %#v", byToken)
	}

	if err := MarkContactExchangeLinkUsed(ctx, linkTwo.Token); err != nil {
		t.Fatalf("MarkContactExchangeLinkUsed failed: %v", err)
	}

	if err := MarkContactExchangeLinkUsed(ctx, linkTwo.Token); !errors.Is(err, ErrContactExchangeLinkInvalid) {
		t.Fatalf("expected ErrContactExchangeLinkInvalid on second consume, got %v", err)
	}

	active, err = GetActiveContactExchangeLink(ctx, contactID)
	if err != nil {
		t.Fatalf("GetActiveContactExchangeLink after consume failed: %v", err)
	}

	if active != nil {
		t.Fatalf("expected no active link after consume, got %#v", active)
	}

	byToken, err = GetContactExchangeLinkByToken(ctx, linkTwo.Token)
	if err != nil {
		t.Fatalf("GetContactExchangeLinkByToken after consume failed: %v", err)
	}

	if byToken != nil {
		t.Fatalf("expected consumed token lookup to return nil, got %#v", byToken)
	}

	byTokenAllowUsed, err := GetContactExchangeLinkByTokenAllowUsed(ctx, linkTwo.Token)
	if err != nil {
		t.Fatalf("GetContactExchangeLinkByTokenAllowUsed after consume failed: %v", err)
	}

	if byTokenAllowUsed == nil || byTokenAllowUsed.ID != linkTwo.ID {
		t.Fatalf("expected consumed token lookup with allow-used to return link, got %#v", byTokenAllowUsed)
	}
}

func TestContactExchangeLinkExpiry(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()
	contactID := mustCreateContact(t, CreateContactInput{NameGiven: "Expiry Contact", Tier: TierD})

	link, err := CreateContactExchangeLink(ctx, contactID, true, true, "", time.Hour)
	if err != nil {
		t.Fatalf("CreateContactExchangeLink failed: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE contact_exchange_links
		SET expires_at = NOW() - INTERVAL '1 minute'
		WHERE id = $1
	`, link.ID); err != nil {
		t.Fatalf("failed to expire link: %v", err)
	}

	byToken, err := GetContactExchangeLinkByToken(ctx, link.Token)
	if err != nil {
		t.Fatalf("GetContactExchangeLinkByToken failed: %v", err)
	}

	if byToken != nil {
		t.Fatalf("expected expired token lookup to return nil, got %#v", byToken)
	}

	byTokenAllowUsed, err := GetContactExchangeLinkByTokenAllowUsed(ctx, link.Token)
	if err != nil {
		t.Fatalf("GetContactExchangeLinkByTokenAllowUsed failed: %v", err)
	}

	if byTokenAllowUsed != nil {
		t.Fatalf("expected expired token allow-used lookup to return nil, got %#v", byTokenAllowUsed)
	}

	active, err := GetActiveContactExchangeLink(ctx, contactID)
	if err != nil {
		t.Fatalf("GetActiveContactExchangeLink failed: %v", err)
	}

	if active != nil {
		t.Fatalf("expected no active link when expired, got %#v", active)
	}

	if err := MarkContactExchangeLinkUsed(ctx, link.Token); !errors.Is(err, ErrContactExchangeLinkInvalid) {
		t.Fatalf("expected ErrContactExchangeLinkInvalid for expired token, got %v", err)
	}
}
