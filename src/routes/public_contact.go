/*
 * Copyright 2026 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/flamego/flamego"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

var getContactPageNoteFn = db.GetContactPageNote

const (
	publicContactMinNameRunes = 2
	publicContactMaxNameRunes = 80
	publicContactMaxEmailLen  = 254
)

var blockedPublicContactNameTokens = map[string]struct{}{
	"asdf":    {},
	"fake":    {},
	"na":      {},
	"none":    {},
	"null":    {},
	"qwerty":  {},
	"test":    {},
	"testing": {},
	"unknown": {},
}

var blockedPublicContactEmailDomains = map[string]struct{}{
	"example.com": {},
	"example.net": {},
	"example.org": {},
	"localhost":   {},
	"test.com":    {},
}

func normalizePublicContactToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))

	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}

		return -1
	}, value)
}

func isPlausiblePublicContactName(fullName string) bool {
	nameLen := utf8.RuneCountInString(fullName)
	if nameLen < publicContactMinNameRunes || nameLen > publicContactMaxNameRunes {
		return false
	}

	letterCount := 0

	for _, r := range fullName {
		if unicode.IsLetter(r) {
			letterCount++
		}
	}

	if letterCount < 2 {
		return false
	}

	normalized := normalizePublicContactToken(fullName)
	if _, blocked := blockedPublicContactNameTokens[normalized]; blocked {
		return false
	}

	return true
}

func isPlausiblePublicContactEmail(email string) bool {
	if len(email) == 0 || len(email) > publicContactMaxEmailLen {
		return false
	}

	parsed, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}

	if parsed.Address != email {
		return false
	}

	_, domain, hasAt := strings.Cut(email, "@")
	if !hasAt || !strings.Contains(domain, ".") {
		return false
	}

	normalizedDomain := strings.ToLower(strings.TrimSpace(domain))
	if _, blocked := blockedPublicContactEmailDomains[normalizedDomain]; blocked {
		return false
	}

	if strings.HasSuffix(normalizedDomain, ".invalid") || strings.HasSuffix(normalizedDomain, ".local") || strings.HasSuffix(normalizedDomain, ".test") {
		return false
	}

	return true
}

func rejectPublicContactSubmission(c flamego.Context, reason string, fullName string, email string) {
	logger.Warn(
		"Rejected public contact submission",
		"reason",
		reason,
		"full_name",
		fullName,
		"email",
		email,
		"ip",
		clientIP(c),
		"user_agent",
		c.Request().UserAgent(),
	)

	c.Redirect("/contact", http.StatusSeeOther)
}

// PublicContactForm renders the public /contact form.
func PublicContactForm(_ flamego.Context, t template.Template, data template.Data) {
	data["HideNav"] = true
	setPublicSiteTitle(data)

	t.HTML(http.StatusOK, "contact_public_form")
}

// SubmitPublicContact logs submission metadata and renders the contact page.
func SubmitPublicContact(c flamego.Context, t template.Template, data template.Data) {
	data["HideNav"] = true
	setPublicSiteTitle(data)

	if err := c.Request().ParseForm(); err != nil {
		rejectPublicContactSubmission(c, "invalid_form", "", "")

		return
	}

	fullName := strings.TrimSpace(c.Request().Form.Get("full_name"))
	email := strings.TrimSpace(c.Request().Form.Get("email"))

	if !isPlausiblePublicContactName(fullName) {
		rejectPublicContactSubmission(c, "invalid_name", fullName, email)

		return
	}

	if !isPlausiblePublicContactEmail(email) {
		rejectPublicContactSubmission(c, "invalid_email", fullName, email)

		return
	}

	stdLogger.Printf(
		"public contact submission full_name=%q email=%q ip=%q user_agent=%q",
		fullName,
		email,
		clientIP(c),
		c.Request().UserAgent(),
	)

	note, err := getContactPageNoteFn(c.Request().Context())
	if err != nil {
		logger.Error("Error fetching contact page note", "error", err)

		data["Error"] = "Failed to load contact page"

		t.HTML(http.StatusInternalServerError, "contact_public_form")

		return
	}

	data["Note"] = note

	t.HTML(http.StatusOK, "contact_public_view")
}
