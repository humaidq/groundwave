// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"os"
	"testing"
)

func TestInitRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if err := Init(testContext()); err == nil {
		t.Fatalf("expected error when DATABASE_URL is missing")
	}
}

func TestInitInvalidDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://")

	if err := Init(testContext()); err == nil {
		t.Fatalf("expected error for invalid database url")
	}
}

func TestGetPoolAndClose(t *testing.T) {
	if GetPool() == nil {
		t.Fatalf("expected pool to be initialized")
	}

	baseURL := os.Getenv("DATABASE_URL")
	if baseURL == "" {
		t.Fatalf("DATABASE_URL not set")
	}

	Close()

	if err := initTestPool(testContext(), baseURL, testSchemaName); err != nil {
		t.Fatalf("failed to re-init pool: %v", err)
	}
}

func TestSyncSchema(t *testing.T) {
	baseURL := os.Getenv("DATABASE_URL")
	if baseURL == "" {
		t.Fatalf("DATABASE_URL not set")
	}

	searchPathURL, err := withSearchPath(baseURL, testSchemaName)
	if err != nil {
		t.Fatalf("withSearchPath failed: %v", err)
	}

	t.Setenv("DATABASE_URL", searchPathURL)

	if err := SyncSchema(testContext()); err != nil {
		t.Fatalf("SyncSchema failed: %v", err)
	}
}

func TestInitSuccess(t *testing.T) {
	baseURL := os.Getenv("DATABASE_URL")
	if baseURL == "" {
		t.Fatalf("DATABASE_URL not set")
	}

	Close()

	if err := Init(testContext()); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if GetPool() == nil {
		t.Fatalf("expected pool to be initialized")
	}

	Close()

	if err := initTestPool(testContext(), baseURL, testSchemaName); err != nil {
		t.Fatalf("failed to re-init pool: %v", err)
	}
}
