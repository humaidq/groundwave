// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"testing"
	"time"
)

func testContext() context.Context {
	return context.Background()
}

func stringPtr(value string) *string {
	return &value
}

func mustCreateUser(t *testing.T, displayName string) *User {
	t.Helper()
	user, err := CreateUser(testContext(), CreateUserInput{DisplayName: displayName, IsAdmin: false})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func mustCreateContact(t *testing.T, input CreateContactInput) string {
	t.Helper()
	contactID, err := CreateContact(testContext(), input)
	if err != nil {
		t.Fatalf("failed to create contact: %v", err)
	}
	return contactID
}

func mustCreateHealthProfile(t *testing.T, name string, isPrimary bool) string {
	t.Helper()
	profileID, err := CreateHealthProfile(testContext(), name, nil, nil, nil, isPrimary)
	if err != nil {
		t.Fatalf("failed to create health profile: %v", err)
	}
	return profileID
}

func mustCreateFollowup(t *testing.T, profileID string, date time.Time) string {
	t.Helper()
	input := CreateFollowupInput{
		ProfileID:    profileID,
		FollowupDate: date,
		HospitalName: "General Hospital",
	}
	followupID, err := CreateFollowup(testContext(), input)
	if err != nil {
		t.Fatalf("failed to create followup: %v", err)
	}
	return followupID
}
