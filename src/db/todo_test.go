// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTodoNote(t *testing.T) {
	resetDatabase(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/todo.org" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#+TITLE: Tasks\n* TODO Test"))
	}))
	defer server.Close()

	t.Setenv("WEBDAV_TODO_PATH", server.URL+"/todo.org")
	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")

	note, err := GetTodoNote(testContext())
	if err != nil {
		t.Fatalf("GetTodoNote failed: %v", err)
	}
	if note.Title != "Tasks" {
		t.Fatalf("expected title Tasks, got %q", note.Title)
	}
}
