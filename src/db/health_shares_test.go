// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
)

func TestHealthProfileShares(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	user := mustCreateUser(t, "Share User")
	profileID := mustCreateHealthProfile(t, "Profile", false)
	followupID := mustCreateFollowup(t, profileID, time.Now().UTC())

	if err := SetHealthProfileShares(ctx, user.ID.String(), []string{profileID}, user.ID.String()); err != nil {
		t.Fatalf("SetHealthProfileShares failed: %v", err)
	}

	profiles, err := ListHealthProfilesForUser(ctx, user.ID.String())
	if err != nil {
		t.Fatalf("ListHealthProfilesForUser failed: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 shared profile, got %d", len(profiles))
	}

	allowed, err := UserHasHealthProfileAccess(ctx, user.ID.String(), profileID)
	if err != nil {
		t.Fatalf("UserHasHealthProfileAccess failed: %v", err)
	}
	if !allowed {
		t.Fatalf("expected access to shared profile")
	}

	allowed, err = UserHasHealthFollowupAccess(ctx, user.ID.String(), followupID)
	if err != nil {
		t.Fatalf("UserHasHealthFollowupAccess failed: %v", err)
	}
	if !allowed {
		t.Fatalf("expected access to shared followup")
	}

	shares, err := ListHealthProfileShares(ctx)
	if err != nil {
		t.Fatalf("ListHealthProfileShares failed: %v", err)
	}
	if len(shares) != 1 {
		t.Fatalf("expected 1 share, got %d", len(shares))
	}
}
