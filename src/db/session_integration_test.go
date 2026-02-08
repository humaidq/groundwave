// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"net/http"
	"testing"
	"time"

	"github.com/flamego/session"
)

func TestPostgresSessionStoreLifecycle(t *testing.T) {
	resetDatabase(t)
	ctx := testContext()

	initer := PostgresSessionIniter()
	store, err := initer(ctx, PostgresSessionConfig{Lifetime: time.Hour})
	if err != nil {
		t.Fatalf("PostgresSessionIniter failed: %v", err)
	}
	pgStore := store.(*PostgresSessionStore)

	noopWriter := func(_ http.ResponseWriter, _ *http.Request, _ string) {}

	sess1 := session.NewBaseSession("sess1", session.GobEncoder, noopWriter)
	sess1.Set("authenticated", true)
	sess1.Set("device_label", "Laptop")
	sess1.Set("device_ip", "127.0.0.1")
	sess1.Set("user_id", "user-1")
	sess1.Set("user_display_name", "User One")

	if err := pgStore.Save(ctx, sess1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if !pgStore.Exist(ctx, "sess1") {
		t.Fatalf("expected session to exist")
	}

	readSess, err := pgStore.Read(ctx, "sess1")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readSess.Get("user_id") != "user-1" {
		t.Fatalf("expected user_id to match")
	}

	if err := pgStore.Touch(ctx, "sess1"); err != nil {
		t.Fatalf("Touch failed: %v", err)
	}

	sess2 := session.NewBaseSession("sess2", session.GobEncoder, noopWriter)
	sess2.Set("authenticated", true)
	sess2.Set("user_id", "user-1")
	if err := pgStore.Save(ctx, sess2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	valid, err := pgStore.ListValidSessions(ctx)
	if err != nil {
		t.Fatalf("ListValidSessions failed: %v", err)
	}
	if len(valid) != 2 {
		t.Fatalf("expected 2 valid sessions, got %d", len(valid))
	}

	deleted, err := pgStore.InvalidateOtherSessions(ctx, "sess1", "user-1")
	if err != nil {
		t.Fatalf("InvalidateOtherSessions failed: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 session deleted, got %d", deleted)
	}

	if err := pgStore.Destroy(ctx, "sess1"); err != nil {
		t.Fatalf("Destroy failed: %v", err)
	}

	if pgStore.Exist(ctx, "sess1") {
		t.Fatalf("expected session to be removed")
	}
}
