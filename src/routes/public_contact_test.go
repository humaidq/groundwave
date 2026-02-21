// SPDX-FileCopyrightText: 2026 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	flamegoTemplate "github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

var (
	errTestContactPageFetch = errors.New("contact page fetch failed")
	errTestUnexpectedFetch  = errors.New("unexpected contact page fetch")
)

type publicContactTemplateStub struct {
	called bool
	status int
	name   string
}

func (s *publicContactTemplateStub) HTML(status int, name string) {
	s.called = true
	s.status = status
	s.name = name
}

func newPublicContactTestApp(tmpl flamegoTemplate.Template, data flamegoTemplate.Data) *flamego.Flame {
	f := flamego.New()
	f.Use(func(c flamego.Context) {
		c.MapTo(tmpl, (*flamegoTemplate.Template)(nil))
		c.Map(data)
		c.Next()
	})

	f.Get("/contact", func(c flamego.Context, t flamegoTemplate.Template, d flamegoTemplate.Data) {
		PublicContactForm(c, t, d)
	})
	f.Post("/contact", func(c flamego.Context, t flamegoTemplate.Template, d flamegoTemplate.Data) {
		SubmitPublicContact(c, t, d)
	})

	return f
}

func TestPublicContactFormRenders(t *testing.T) {
	t.Setenv(publicSiteTitleEnvVar, "Public Contact")

	tpl := &publicContactTemplateStub{}
	data := flamegoTemplate.Data{}
	f := newPublicContactTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodGet, "/contact", nil)
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !tpl.called || tpl.name != "contact_public_form" {
		t.Fatalf("unexpected template render: %#v", tpl)
	}

	hideNav, _ := data["HideNav"].(bool)
	if !hideNav {
		t.Fatal("expected HideNav to be true")
	}

	if got, _ := data["PageTitle"].(string); got != "Public Contact" {
		t.Fatalf("expected page title %q, got %q", "Public Contact", got)
	}
}

//nolint:paralleltest // Overrides package-level DB function variable.
func TestSubmitPublicContactRendersContactOrgPage(t *testing.T) {
	originalGetContactPageNoteFn := getContactPageNoteFn

	t.Cleanup(func() {
		getContactPageNoteFn = originalGetContactPageNoteFn
	})

	getContactPageNoteFn = func(context.Context) (*db.ContactPageNote, error) {
		return &db.ContactPageNote{
			Title:    "Contact",
			HTMLBody: template.HTML("<h1>Contact</h1><p>hello@huma.id</p>"),
		}, nil
	}

	tpl := &publicContactTemplateStub{}
	data := flamegoTemplate.Data{}
	f := newPublicContactTestApp(tpl, data)

	form := url.Values{}
	form.Set("full_name", "Alice Smith")
	form.Set("email", "alice@huma.id")

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if !tpl.called || tpl.name != "contact_public_view" {
		t.Fatalf("unexpected template render: %#v", tpl)
	}

	note, ok := data["Note"].(*db.ContactPageNote)
	if !ok || note == nil {
		t.Fatal("expected Note in template data")
	}

	if note.Title != "Contact" {
		t.Fatalf("expected note title %q, got %q", "Contact", note.Title)
	}
}

//nolint:paralleltest // Overrides package-level DB function variable.
func TestSubmitPublicContactRequiresFields(t *testing.T) {
	originalGetContactPageNoteFn := getContactPageNoteFn

	t.Cleanup(func() {
		getContactPageNoteFn = originalGetContactPageNoteFn
	})

	fetchCalled := false
	getContactPageNoteFn = func(context.Context) (*db.ContactPageNote, error) {
		fetchCalled = true

		return nil, errTestUnexpectedFetch
	}

	tpl := &publicContactTemplateStub{}
	data := flamegoTemplate.Data{}
	f := newPublicContactTestApp(tpl, data)

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader("full_name=&email="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/contact" {
		t.Fatalf("expected redirect location %q, got %q", "/contact", location)
	}

	if tpl.called {
		t.Fatalf("expected no template render, got %#v", tpl)
	}

	if fetchCalled {
		t.Fatal("expected contact page fetch to be skipped for invalid form")
	}
}

//nolint:paralleltest // Overrides package-level DB function variable.
func TestSubmitPublicContactRejectsFakeName(t *testing.T) {
	originalGetContactPageNoteFn := getContactPageNoteFn

	t.Cleanup(func() {
		getContactPageNoteFn = originalGetContactPageNoteFn
	})

	fetchCalled := false
	getContactPageNoteFn = func(context.Context) (*db.ContactPageNote, error) {
		fetchCalled = true

		return nil, errTestUnexpectedFetch
	}

	tpl := &publicContactTemplateStub{}
	data := flamegoTemplate.Data{}
	f := newPublicContactTestApp(tpl, data)

	form := url.Values{}
	form.Set("full_name", "test")
	form.Set("email", "alice@huma.id")

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/contact" {
		t.Fatalf("expected redirect location %q, got %q", "/contact", location)
	}

	if tpl.called {
		t.Fatalf("expected no template render, got %#v", tpl)
	}

	if fetchCalled {
		t.Fatal("expected contact page fetch to be skipped for fake name")
	}
}

//nolint:paralleltest // Overrides package-level DB function variable.
func TestSubmitPublicContactRejectsFakeEmail(t *testing.T) {
	originalGetContactPageNoteFn := getContactPageNoteFn

	t.Cleanup(func() {
		getContactPageNoteFn = originalGetContactPageNoteFn
	})

	fetchCalled := false
	getContactPageNoteFn = func(context.Context) (*db.ContactPageNote, error) {
		fetchCalled = true

		return nil, errTestUnexpectedFetch
	}

	tpl := &publicContactTemplateStub{}
	data := flamegoTemplate.Data{}
	f := newPublicContactTestApp(tpl, data)

	form := url.Values{}
	form.Set("full_name", "Alice Smith")
	form.Set("email", "alice@example.com")

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	if location := rec.Header().Get("Location"); location != "/contact" {
		t.Fatalf("expected redirect location %q, got %q", "/contact", location)
	}

	if tpl.called {
		t.Fatalf("expected no template render, got %#v", tpl)
	}

	if fetchCalled {
		t.Fatal("expected contact page fetch to be skipped for fake email")
	}
}

//nolint:paralleltest // Overrides package-level DB function variable.
func TestSubmitPublicContactHandlesContactPageFetchFailure(t *testing.T) {
	originalGetContactPageNoteFn := getContactPageNoteFn

	t.Cleanup(func() {
		getContactPageNoteFn = originalGetContactPageNoteFn
	})

	getContactPageNoteFn = func(context.Context) (*db.ContactPageNote, error) {
		return nil, errTestContactPageFetch
	}

	tpl := &publicContactTemplateStub{}
	data := flamegoTemplate.Data{}
	f := newPublicContactTestApp(tpl, data)

	form := url.Values{}
	form.Set("full_name", "Alice Smith")
	form.Set("email", "alice@huma.id")

	req := httptest.NewRequest(http.MethodPost, "/contact", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, req)

	if !tpl.called || tpl.name != "contact_public_form" {
		t.Fatalf("unexpected template render: %#v", tpl)
	}

	if tpl.status != http.StatusInternalServerError {
		t.Fatalf("expected template status %d, got %d", http.StatusInternalServerError, tpl.status)
	}

	if got, _ := data["Error"].(string); got != "Failed to load contact page" {
		t.Fatalf("expected error %q, got %q", "Failed to load contact page", got)
	}
}
