/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"
	"net/url"
	"strings"

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

	authenticated, ok := s.Get("authenticated").(bool)
	if ok && authenticated {
		c.Redirect(next, http.StatusSeeOther)
		return
	}

	data["HeaderOnly"] = true
	data["Next"] = next

	t.HTML(http.StatusOK, "login")
}

// Logout handles logout request
func Logout(s session.Session, c flamego.Context) {
	s.Delete("authenticated")
	s.Delete("user_id")
	s.Delete("user_display_name")
	s.Delete("user_is_admin")
	s.Delete("userID")
	s.Delete(sensitiveAccessSessionKey)
	s.Delete("private_mode")
	c.Redirect("/login")
}

// RequireAuth is a middleware that checks if user is authenticated
func RequireAuth(s session.Session, c flamego.Context) {
	authenticated, ok := s.Get("authenticated").(bool)
	if !ok || !authenticated {
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
