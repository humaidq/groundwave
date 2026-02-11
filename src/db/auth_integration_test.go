// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

func setupTestCredential(id string) webauthn.Credential {
	return webauthn.Credential{
		ID:              []byte(id),
		PublicKey:       []byte("public-key-" + id),
		AttestationType: "none",
	}
}

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

func TestFinalizeSetupRegistrationBootstrapIsAtomic(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	inputs := []FinalizeSetupRegistrationInput{
		{
			UserID:      uuid.New(),
			DisplayName: "Admin A",
			IsAdmin:     true,
			Credential:  setupTestCredential("bootstrap-a"),
		},
		{
			UserID:      uuid.New(),
			DisplayName: "Admin B",
			IsAdmin:     true,
			Credential:  setupTestCredential("bootstrap-b"),
		},
	}

	type setupResult struct {
		user *User
		err  error
	}

	results := make(chan setupResult, len(inputs))
	start := make(chan struct{})

	var waitGroup sync.WaitGroup

	for _, input := range inputs {
		waitGroup.Add(1)

		go func(input FinalizeSetupRegistrationInput) {
			defer waitGroup.Done()

			<-start

			user, err := FinalizeSetupRegistration(ctx, input)
			results <- setupResult{user: user, err: err}
		}(input)
	}

	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	setupCompletedFailures := 0

	for result := range results {
		if result.err == nil {
			successes++
			continue
		}

		if errors.Is(result.err, ErrSetupAlreadyCompleted) {
			setupCompletedFailures++
			continue
		}

		t.Fatalf("unexpected setup finalize error: %v", result.err)
	}

	if successes != 1 || setupCompletedFailures != 1 {
		t.Fatalf("expected one success and one setup-completed failure, got success=%d failure=%d", successes, setupCompletedFailures)
	}

	users, err := ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if len(users) != 1 {
		t.Fatalf("expected 1 user after concurrent bootstrap finalize, got %d", len(users))
	}

	if !users[0].IsAdmin {
		t.Fatalf("expected bootstrap user to be admin")
	}

	passkeyCount, err := CountUserPasskeys(ctx, users[0].ID.String())
	if err != nil {
		t.Fatalf("CountUserPasskeys failed: %v", err)
	}

	if passkeyCount != 1 {
		t.Fatalf("expected 1 passkey after concurrent bootstrap finalize, got %d", passkeyCount)
	}
}

func TestFinalizeSetupRegistrationInviteIsAtomic(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	owner := mustCreateUser(t, "Invite Owner")

	invite, err := CreateUserInvite(ctx, owner.ID.String(), "Invitee")
	if err != nil {
		t.Fatalf("CreateUserInvite failed: %v", err)
	}

	inviteID := invite.ID.String()
	inputs := []FinalizeSetupRegistrationInput{
		{
			UserID:      uuid.New(),
			DisplayName: "Invitee A",
			IsAdmin:     false,
			InviteID:    &inviteID,
			Credential:  setupTestCredential("invite-a"),
		},
		{
			UserID:      uuid.New(),
			DisplayName: "Invitee B",
			IsAdmin:     false,
			InviteID:    &inviteID,
			Credential:  setupTestCredential("invite-b"),
		},
	}

	type setupResult struct {
		user *User
		err  error
	}

	results := make(chan setupResult, len(inputs))
	start := make(chan struct{})

	var waitGroup sync.WaitGroup

	for _, input := range inputs {
		waitGroup.Add(1)

		go func(input FinalizeSetupRegistrationInput) {
			defer waitGroup.Done()

			<-start

			user, err := FinalizeSetupRegistration(ctx, input)
			results <- setupResult{user: user, err: err}
		}(input)
	}

	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	inviteFailures := 0

	for result := range results {
		if result.err == nil {
			successes++
			continue
		}

		if errors.Is(result.err, ErrInviteInvalidOrUsed) {
			inviteFailures++
			continue
		}

		t.Fatalf("unexpected invite finalize error: %v", result.err)
	}

	if successes != 1 || inviteFailures != 1 {
		t.Fatalf("expected one success and one invite failure, got success=%d failure=%d", successes, inviteFailures)
	}

	users, err := ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("expected owner plus one invited user, got %d users", len(users))
	}

	pendingInvites, err := ListPendingUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListPendingUserInvites failed: %v", err)
	}

	if len(pendingInvites) != 0 {
		t.Fatalf("expected invite to be consumed exactly once")
	}
}

func TestFinalizeSetupRegistrationRollsBackOnPasskeyFailure(t *testing.T) {
	resetDatabase(t)

	ctx := testContext()

	existingUser := mustCreateUser(t, "Existing User")

	duplicateCredential := setupTestCredential("duplicate-credential")
	if _, err := AddUserPasskey(ctx, existingUser.ID.String(), duplicateCredential, nil); err != nil {
		t.Fatalf("AddUserPasskey failed: %v", err)
	}

	invite, err := CreateUserInvite(ctx, existingUser.ID.String(), "New Invitee")
	if err != nil {
		t.Fatalf("CreateUserInvite failed: %v", err)
	}

	inviteID := invite.ID.String()
	newUserID := uuid.New()

	_, err = FinalizeSetupRegistration(ctx, FinalizeSetupRegistrationInput{
		UserID:      newUserID,
		DisplayName: "New Invitee",
		IsAdmin:     false,
		InviteID:    &inviteID,
		Credential:  duplicateCredential,
	})
	if err == nil {
		t.Fatalf("expected setup finalization to fail when passkey insert fails")
	}

	count, err := CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected user creation to be rolled back, got %d users", count)
	}

	pendingInvites, err := ListPendingUserInvites(ctx)
	if err != nil {
		t.Fatalf("ListPendingUserInvites failed: %v", err)
	}

	if len(pendingInvites) != 1 {
		t.Fatalf("expected invite consume to be rolled back, got %d pending invites", len(pendingInvites))
	}

	if _, err := GetUserByID(ctx, newUserID.String()); err == nil {
		t.Fatalf("expected rolled-back user to not exist")
	}
}

func TestFinalizeSetupRegistrationRejectsExpiredInvite(t *testing.T) {
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

	inviteID := invite.ID.String()

	_, err = FinalizeSetupRegistration(ctx, FinalizeSetupRegistrationInput{
		UserID:      uuid.New(),
		DisplayName: "Late Invitee",
		IsAdmin:     false,
		InviteID:    &inviteID,
		Credential:  setupTestCredential("expired-invite"),
	})
	if !errors.Is(err, ErrInviteInvalidOrUsed) {
		t.Fatalf("expected ErrInviteInvalidOrUsed for expired invite, got %v", err)
	}

	users, err := ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}

	if len(users) != 1 {
		t.Fatalf("expected only owner user after expired invite finalize, got %d users", len(users))
	}
}
