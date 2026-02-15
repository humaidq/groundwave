// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

var errFilesListShouldNotBeCalled = errors.New("should not be called")

type filesTemplateStub struct {
	called bool
	status int
	name   string
}

func (s *filesTemplateStub) HTML(status int, name string) {
	s.called = true
	s.status = status
	s.name = name
}

func newFilesListTestApp(s session.Session, t template.Template, data template.Data) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(s, (*session.Session)(nil))
		c.MapTo(t, (*template.Template)(nil))
		c.Map(data)
		c.Next()
	})

	f.Get("/files", func(c flamego.Context, sess session.Session, tmpl template.Template, d template.Data) {
		FilesList(c, sess, tmpl, d)
	})

	return f
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestFilesListRedirectsMissingPathToRoot(t *testing.T) {
	originalAdminOnlyFn := filesIsPathAdminOnlyFn
	originalRestrictedFn := filesIsPathRestrictedFn
	originalListEntriesFn := filesListEntriesFn

	t.Cleanup(func() {
		filesIsPathAdminOnlyFn = originalAdminOnlyFn
		filesIsPathRestrictedFn = originalRestrictedFn
		filesListEntriesFn = originalListEntriesFn
	})

	filesIsPathAdminOnlyFn = func(context.Context, string) (bool, error) {
		return false, db.ErrWebDAVFilesEntryNotFound
	}
	filesIsPathRestrictedFn = func(context.Context, string) (bool, error) {
		return false, errFilesListShouldNotBeCalled
	}
	filesListEntriesFn = func(context.Context, string) ([]db.WebDAVEntry, error) {
		return nil, errFilesListShouldNotBeCalled
	}

	s := newTestSession()
	tpl := &filesTemplateStub{}
	data := template.Data{}
	f := newFilesListTestApp(s, tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/files?path=missing", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "/files" {
		t.Fatalf("expected redirect to /files, got %q", got)
	}

	msg, ok := s.flash.(FlashMessage)
	if !ok {
		t.Fatalf("expected flash message, got %T", s.flash)
	}

	if msg.Type != FlashError || msg.Message != filesFolderNotFoundMessage {
		t.Fatalf("unexpected flash message: %#v", msg)
	}

	if tpl.called {
		t.Fatal("did not expect files template render when redirecting")
	}
}

//nolint:paralleltest // Overrides package-level DB function variables.
func TestFilesListRendersErrorAtRootWhenBackendUnavailable(t *testing.T) {
	originalAdminOnlyFn := filesIsPathAdminOnlyFn
	originalRestrictedFn := filesIsPathRestrictedFn
	originalListEntriesFn := filesListEntriesFn

	t.Cleanup(func() {
		filesIsPathAdminOnlyFn = originalAdminOnlyFn
		filesIsPathRestrictedFn = originalRestrictedFn
		filesListEntriesFn = originalListEntriesFn
	})

	filesIsPathAdminOnlyFn = func(context.Context, string) (bool, error) {
		return false, db.ErrWebDAVFilesEntryNotFound
	}
	filesIsPathRestrictedFn = func(context.Context, string) (bool, error) {
		return false, nil
	}
	filesListEntriesFn = func(context.Context, string) ([]db.WebDAVEntry, error) {
		return []db.WebDAVEntry{}, nil
	}

	s := newTestSession()
	tpl := &filesTemplateStub{}
	data := template.Data{}
	f := newFilesListTestApp(s, tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/files", nil)
	rec := httptest.NewRecorder()

	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("expected no redirect, got %q", got)
	}

	if s.flash != nil {
		t.Fatalf("expected no flash, got %#v", s.flash)
	}

	if !tpl.called || tpl.name != "files" || tpl.status != http.StatusOK {
		t.Fatalf("unexpected template render: %#v", tpl)
	}

	errorMsg, _ := data["Error"].(string)
	if errorMsg != filesLoadErrorMessage {
		t.Fatalf("expected data error %q, got %q", filesLoadErrorMessage, errorMsg)
	}
}
