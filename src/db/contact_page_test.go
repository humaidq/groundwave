// SPDX-FileCopyrightText: 2026 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetContactPageNote(t *testing.T) {
	resetDatabase(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contact.org" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("#+TITLE: Contact\n* Reach me\nEmail me."))
	}))
	defer server.Close()

	t.Setenv("WEBDAV_CONTACT_PAGE", server.URL+"/contact.org")
	t.Setenv("WEBDAV_USERNAME", "")
	t.Setenv("WEBDAV_PASSWORD", "")

	note, err := GetContactPageNote(testContext())
	if err != nil {
		t.Fatalf("GetContactPageNote failed: %v", err)
	}

	if note.Title != "Contact" {
		t.Fatalf("expected title Contact, got %q", note.Title)
	}
}

func TestGetContactPageNoteMissingEnv(t *testing.T) {
	resetDatabase(t)

	t.Setenv("WEBDAV_CONTACT_PAGE", "")

	_, err := GetContactPageNote(testContext())
	if !errors.Is(err, ErrWebDAVContactPageNotConfigured) {
		t.Fatalf("expected ErrWebDAVContactPageNotConfigured, got %v", err)
	}
}

func TestGetContactPageNoteRejectsNonOrgPath(t *testing.T) {
	resetDatabase(t)

	t.Setenv("WEBDAV_CONTACT_PAGE", "https://example.test/contact.txt")

	_, err := GetContactPageNote(testContext())
	if !errors.Is(err, ErrWebDAVContactPageMustBeOrgFile) {
		t.Fatalf("expected ErrWebDAVContactPageMustBeOrgFile, got %v", err)
	}
}
