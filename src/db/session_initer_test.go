// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"
	"time"
)

func TestPostgresSessionIniterDefaults(t *testing.T) {
	initer := PostgresSessionIniter()
	store, err := initer(testContext())
	if err != nil {
		t.Fatalf("PostgresSessionIniter failed: %v", err)
	}

	pgStore, ok := store.(*PostgresSessionStore)
	if !ok {
		t.Fatalf("expected PostgresSessionStore")
	}
	if pgStore.config.TableName != "flamego_sessions" {
		t.Fatalf("expected default table name, got %q", pgStore.config.TableName)
	}
	if pgStore.config.Lifetime != 30*24*time.Hour {
		t.Fatalf("expected default lifetime, got %v", pgStore.config.Lifetime)
	}
	if pgStore.encoder == nil || pgStore.decoder == nil {
		t.Fatalf("expected encoder and decoder to be set")
	}
}

func TestPostgresSessionIniterInvalidConfig(t *testing.T) {
	initer := PostgresSessionIniter()
	if _, err := initer(testContext(), "invalid"); err == nil {
		t.Fatalf("expected invalid config error")
	}
}
