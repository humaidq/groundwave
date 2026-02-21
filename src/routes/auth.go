/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
)

// LoginForm renders the login page
func LoginForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	next := sanitizeNextPath(c.Query("next"))
	if strings.TrimSpace(c.Query("next")) == "" {
		next = "/"
	}

	if isSessionAuthenticated(s, time.Now()) {
		c.Redirect(next, http.StatusSeeOther)
		return
	}

	data["HeaderOnly"] = true
	data["Next"] = next

	t.HTML(http.StatusOK, "login")
}

// Logout handles logout request
func Logout(s session.Session, c flamego.Context) {
	clearAuthenticatedSession(s)
	c.Redirect("/login")
}

// RequireAuth is a middleware that checks if user is authenticated
func RequireAuth(s session.Session, c flamego.Context) {
	if !isSessionAuthenticated(s, time.Now()) {
		next := sanitizeNextPath(c.Request().Header.Get("Referer"))
		if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodHead {
			next = sanitizeNextPath(c.Request().URL.RequestURI())
		}

		redirectURL := "/login?next=" + url.QueryEscape(next)
		logAccessDenied(c, s, "not_authenticated", http.StatusFound, "/login", "next", next)
		c.Redirect(redirectURL)

		return
	}

	c.Next()
}
