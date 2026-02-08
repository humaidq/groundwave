// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

func TestUserLifecycle(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	if _, err := CreateUser(ctx, CreateUserInput{}); err == nil {
		t.Fatalf("expected error for missing display name")
	}

	count, err := CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero users, got %d", count)
	}

	user, err := CreateUser(ctx, CreateUserInput{DisplayName: "Alice", IsAdmin: true})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	byID, err := GetUserByID(ctx, user.ID.String())
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if byID.DisplayName != "Alice" {
		t.Fatalf("expected display name Alice, got %q", byID.DisplayName)
	}

	first, err := GetFirstUser(ctx)
	if err != nil {
		t.Fatalf("GetFirstUser failed: %v", err)
	}
	if first == nil || first.ID != user.ID {
		t.Fatalf("expected first user to match")
	}

	users, err := ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	byHandle, err := GetUserByWebAuthnID(ctx, user.ID[:])
	if err != nil {
		t.Fatalf("GetUserByWebAuthnID failed: %v", err)
	}
	if byHandle == nil || byHandle.ID != user.ID {
		t.Fatalf("expected user by handle to match")
	}

	if err := DeleteUser(ctx, user.ID.String()); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	count, err = CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero users, got %d", count)
	}
}

func TestUserPasskeyLifecycle(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	user := mustCreateUser(t, "Passkey User")

	credential := webauthn.Credential{
		ID:              []byte("credential-id"),
		PublicKey:       []byte("public-key"),
		AttestationType: "none",
	}
	label := "laptop"
	passkey, err := AddUserPasskey(ctx, user.ID.String(), credential, &label)
	if err != nil {
		t.Fatalf("AddUserPasskey failed: %v", err)
	}
	if passkey.Label == nil || *passkey.Label != label {
		t.Fatalf("expected label %q", label)
	}

	count, err := CountUserPasskeys(ctx, user.ID.String())
	if err != nil {
		t.Fatalf("CountUserPasskeys failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 passkey, got %d", count)
	}

	passkeys, err := ListUserPasskeys(ctx, user.ID.String())
	if err != nil {
		t.Fatalf("ListUserPasskeys failed: %v", err)
	}
	if len(passkeys) != 1 {
		t.Fatalf("expected 1 passkey, got %d", len(passkeys))
	}

	lastUsed := time.Now().UTC().Add(-time.Minute)
	if err := UpdateUserPasskeyCredential(ctx, user.ID.String(), credential, lastUsed); err != nil {
		t.Fatalf("UpdateUserPasskeyCredential failed: %v", err)
	}

	creds, err := LoadUserCredentials(ctx, user.ID.String())
	if err != nil {
		t.Fatalf("LoadUserCredentials failed: %v", err)
	}
	if len(creds) != 1 || string(creds[0].ID) != string(credential.ID) {
		t.Fatalf("expected stored credential to match")
	}

	if err := DeleteUserPasskey(ctx, user.ID.String(), passkey.ID.String()); err != nil {
		t.Fatalf("DeleteUserPasskey failed: %v", err)
	}

	count, err = CountUserPasskeys(ctx, user.ID.String())
	if err != nil {
		t.Fatalf("CountUserPasskeys failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero passkeys, got %d", count)
	}
}

func TestUserQueriesNoResults(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	first, err := GetFirstUser(ctx)
	if err != nil {
		t.Fatalf("GetFirstUser failed: %v", err)
	}
	if first != nil {
		t.Fatalf("expected no first user")
	}

	if err := DeleteUser(ctx, uuid.New().String()); err == nil {
		t.Fatalf("expected error for missing user")
	}

	if _, err := GetUserByWebAuthnID(ctx, []byte("short")); err == nil {
		t.Fatalf("expected error for invalid user handle")
	}
}
