// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import "testing"

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
