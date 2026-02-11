// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestUserInvitesLifecycle(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	user := mustCreateUser(t, "Invite Owner")

	invite, err := CreateUserInvite(ctx, user.ID.String(), "Invitee")
	if err != nil {
		t.Fatalf("CreateUserInvite failed: %v", err)
	}

	if invite.Token == "" {
		t.Fatalf("expected invite token")
	}

	byToken, err := GetUserInviteByToken(ctx, invite.Token)
	if err != nil {
		t.Fatalf("GetUserInviteByToken failed: %v", err)
	}

	if byToken == nil || byToken.ID != invite.ID {
		t.Fatalf("expected invite by token to match")
	}

	byID, err := GetUserInviteByID(ctx, invite.ID.String())
	if err != nil {
		t.Fatalf("GetUserInviteByID failed: %v", err)
	}

	if byID == nil || byID.Token != invite.Token {
		t.Fatalf("expected invite by id to match")
	}

	invites, err := ListPendingUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListPendingUserInvites failed: %v", err)
	}

	if len(invites) != 1 {
		t.Fatalf("expected 1 invite, got %d", len(invites))
	}

	if err := MarkUserInviteUsed(ctx, invite.ID.String()); err != nil {
		t.Fatalf("MarkUserInviteUsed failed: %v", err)
	}

	invites, err = ListPendingUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListPendingUserInvites failed: %v", err)
	}

	if len(invites) != 0 {
		t.Fatalf("expected 0 pending invites, got %d", len(invites))
	}

	if err := DeleteUserInvite(ctx, invite.ID.String()); err != nil {
		t.Fatalf("DeleteUserInvite failed: %v", err)
	}

	missing, err := GetUserInviteByToken(ctx, "missing")
	if err != nil {
		t.Fatalf("GetUserInviteByToken failed: %v", err)
	}

	if missing != nil {
		t.Fatalf("expected nil for missing invite")
	}
}

func TestUserInviteErrors(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	missingID := uuid.New().String()

	missing, err := GetUserInviteByID(ctx, missingID)
	if err != nil {
		t.Fatalf("GetUserInviteByID failed: %v", err)
	}

	if missing != nil {
		t.Fatalf("expected nil invite for missing id")
	}

	if err := MarkUserInviteUsed(ctx, missingID); err == nil {
		t.Fatalf("expected error for missing invite")
	}

	if err := DeleteUserInvite(ctx, missingID); err == nil {
		t.Fatalf("expected error for missing invite delete")
	}
}

func TestUserInviteExpiryAfter24Hours(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	owner := mustCreateUser(t, "Invite Owner")

	invite, err := CreateUserInvite(ctx, owner.ID.String(), "Invitee")
	if err != nil {
		t.Fatalf("CreateUserInvite failed: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE user_invites
		SET created_at = NOW() - INTERVAL '25 hours'
		WHERE id = $1
	`, invite.ID); err != nil {
		t.Fatalf("failed to age invite: %v", err)
	}

	byToken, err := GetUserInviteByToken(ctx, invite.Token)
	if err != nil {
		t.Fatalf("GetUserInviteByToken failed: %v", err)
	}

	if byToken != nil {
		t.Fatalf("expected expired invite to be unavailable by token")
	}

	byID, err := GetUserInviteByID(ctx, invite.ID.String())
	if err != nil {
		t.Fatalf("GetUserInviteByID failed: %v", err)
	}

	if byID != nil {
		t.Fatalf("expected expired invite to be unavailable by id")
	}

	pending, err := ListPendingUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListPendingUserInvites failed: %v", err)
	}

	if len(pending) != 0 {
		t.Fatalf("expected expired invite to be excluded from pending list")
	}

	if err := MarkUserInviteUsed(ctx, invite.ID.String()); err == nil {
		t.Fatalf("expected expired invite to fail when marked used")
	}
}

func TestListExpiredUserInvitesAndRegenerate(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	owner := mustCreateUser(t, "Invite Owner")

	invite, err := CreateUserInvite(ctx, owner.ID.String(), "Invitee")
	if err != nil {
		t.Fatalf("CreateUserInvite failed: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE user_invites
		SET created_at = NOW() - INTERVAL '25 hours'
		WHERE id = $1
	`, invite.ID); err != nil {
		t.Fatalf("failed to age invite: %v", err)
	}

	expired, err := ListExpiredUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListExpiredUserInvites failed: %v", err)
	}

	if len(expired) != 1 {
		t.Fatalf("expected 1 expired invite, got %d", len(expired))
	}

	if expired[0].ID != invite.ID {
		t.Fatalf("expected expired invite to match original invite")
	}

	regenerated, err := RegenerateExpiredUserInvite(ctx, invite.ID.String())
	if err != nil {
		t.Fatalf("RegenerateExpiredUserInvite failed: %v", err)
	}

	if regenerated.Token == "" {
		t.Fatalf("expected regenerated invite token")
	}

	if regenerated.Token == invite.Token {
		t.Fatalf("expected regenerated token to change")
	}

	oldTokenInvite, err := GetUserInviteByToken(ctx, invite.Token)
	if err != nil {
		t.Fatalf("GetUserInviteByToken failed: %v", err)
	}

	if oldTokenInvite != nil {
		t.Fatalf("expected old token to be invalid after regeneration")
	}

	newTokenInvite, err := GetUserInviteByToken(ctx, regenerated.Token)
	if err != nil {
		t.Fatalf("GetUserInviteByToken failed: %v", err)
	}

	if newTokenInvite == nil {
		t.Fatalf("expected regenerated token to be active")
	}

	pending, err := ListPendingUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListPendingUserInvites failed: %v", err)
	}

	if len(pending) != 1 {
		t.Fatalf("expected 1 pending invite after regeneration, got %d", len(pending))
	}

	if pending[0].ID != invite.ID {
		t.Fatalf("expected regenerated invite to keep same id")
	}

	expired, err = ListExpiredUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListExpiredUserInvites failed: %v", err)
	}

	if len(expired) != 0 {
		t.Fatalf("expected 0 expired invites after regeneration, got %d", len(expired))
	}
}

func TestRegenerateExpiredUserInviteRequiresExpiry(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	owner := mustCreateUser(t, "Invite Owner")

	invite, err := CreateUserInvite(ctx, owner.ID.String(), "Invitee")
	if err != nil {
		t.Fatalf("CreateUserInvite failed: %v", err)
	}

	if _, err := RegenerateExpiredUserInvite(ctx, invite.ID.String()); !errors.Is(err, ErrInviteNotExpired) {
		t.Fatalf("expected ErrInviteNotExpired, got %v", err)
	}

	if _, err := RegenerateExpiredUserInvite(ctx, uuid.New().String()); err == nil {
		t.Fatalf("expected error for missing invite")
	}
}
