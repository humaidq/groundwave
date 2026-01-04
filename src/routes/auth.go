/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"
	"os"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"golang.org/x/crypto/bcrypt"
)

// LoginForm renders the login page
func LoginForm(c flamego.Context, t template.Template, data template.Data) {
	t.HTML(http.StatusOK, "login")
}

// Login handles login POST request
func Login(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	username := c.Request().FormValue("username")
	password := c.Request().FormValue("password")

	// Get credentials from environment variables
	envUsername := os.Getenv("AUTH_USERNAME")
	envPasswordHash := os.Getenv("AUTH_PASSWORD_HASH")

	// Check if authentication is configured
	if envUsername == "" || envPasswordHash == "" {
		log.Println("Warning: AUTH_USERNAME or AUTH_PASSWORD_HASH not set")
		data["Error"] = "Authentication not configured"
		t.HTML(http.StatusUnauthorized, "login")
		return
	}

	// Verify username
	if username != envUsername {
		data["Error"] = "Invalid username or password"
		t.HTML(http.StatusUnauthorized, "login")
		return
	}

	// Verify password against bcrypt hash
	err := bcrypt.CompareHashAndPassword([]byte(envPasswordHash), []byte(password))
	if err != nil {
		data["Error"] = "Invalid username or password"
		t.HTML(http.StatusUnauthorized, "login")
		return
	}

	// Set session
	s.Set("authenticated", true)
	s.Set("username", username)

	// Redirect to home page
	c.Redirect("/")
}

// Logout handles logout request
func Logout(s session.Session, c flamego.Context) {
	s.Delete("authenticated")
	s.Delete("username")
	c.Redirect("/login")
}

// RequireAuth is a middleware that checks if user is authenticated
func RequireAuth(s session.Session, c flamego.Context) {
	authenticated, ok := s.Get("authenticated").(bool)
	if !ok || !authenticated {
		c.Redirect("/login")
		return
	}
	c.Next()
}
