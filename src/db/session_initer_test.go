// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"net/http"
	"testing"
	"time"

	"github.com/flamego/session"
)

func TestPostgresSessionIniterDefaults(t *testing.T) {
	t.Parallel()

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

	if pgStore.idWriter == nil {
		t.Fatalf("expected idWriter to be set")
	}
}

func TestPostgresSessionIniterInvalidConfig(t *testing.T) {
	t.Parallel()

	initer := PostgresSessionIniter()
	if _, err := initer(testContext(), "invalid"); err == nil {
		t.Fatalf("expected invalid config error")
	}
}

func TestPostgresSessionIniterIDWriter(t *testing.T) {
	t.Parallel()

	initer := PostgresSessionIniter()

	called := false
	writer := session.IDWriter(func(http.ResponseWriter, *http.Request, string) {
		called = true
	})

	store, err := initer(testContext(), PostgresSessionConfig{}, writer)
	if err != nil {
		t.Fatalf("PostgresSessionIniter failed: %v", err)
	}

	pgStore, ok := store.(*PostgresSessionStore)
	if !ok {
		t.Fatalf("expected PostgresSessionStore")
	}

	pgStore.idWriter(nil, nil, "new-session-id")

	if !called {
		t.Fatalf("expected idWriter to be wired into store")
	}
}
