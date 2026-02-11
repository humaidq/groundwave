/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
)

// LoginForm renders the login page
func LoginForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	authenticated, ok := s.Get("authenticated").(bool)
	if ok && authenticated {
		c.Redirect("/", http.StatusSeeOther)
		return
	}

	data["HeaderOnly"] = true

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
		logAccessDenied(c, s, "not_authenticated", http.StatusFound, "/login")
		c.Redirect("/login")

		return
	}

	c.Next()
}
