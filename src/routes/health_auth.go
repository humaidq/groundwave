/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
)

const healthBreakGlassSessionKey = "health_break_glass_at"
const healthBreakGlassWindow = 30 * time.Minute

// BreakGlassForm renders the reauthentication page for health data access.
func BreakGlassForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["HeaderOnly"] = true
	data["Username"] = getSessionUsername(s)
	data["Next"] = sanitizeNextPath(c.Query("next"))

	t.HTML(http.StatusOK, "break_glass")
}

// BreakGlass handles reauthentication POST for health data access.
func BreakGlass(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["HeaderOnly"] = true

	username := c.Request().FormValue("username")
	password := c.Request().FormValue("password")
	data["Username"] = username

	next := sanitizeNextPath(c.Request().FormValue("next"))
	data["Next"] = next

	if msg, ok := validateCredentials(username, password); !ok {
		data["Error"] = msg
		t.HTML(http.StatusUnauthorized, "break_glass")
		return
	}

	s.Set(healthBreakGlassSessionKey, time.Now().Unix())

	if next == "" {
		next = "/health"
	}

	c.Redirect(next, http.StatusSeeOther)
}

// RequireHealthReauth enforces a timed reauthentication for health routes.
func RequireHealthReauth(s session.Session, c flamego.Context) {
	path := c.Request().URL.Path
	if !strings.HasPrefix(path, "/health") {
		c.Next()
		return
	}
	if strings.HasPrefix(path, "/health/break-glass") {
		c.Next()
		return
	}

	if isHealthBreakGlassValid(s, time.Now()) {
		c.Next()
		return
	}

	next := sanitizeNextPath(c.Request().URL.RequestURI())
	redirectURL := "/health/break-glass?next=" + url.QueryEscape(next)
	c.Redirect(redirectURL, http.StatusSeeOther)
}

func isHealthBreakGlassValid(s session.Session, now time.Time) bool {
	stamp, ok := getHealthBreakGlassTime(s)
	if !ok {
		return false
	}
	return now.Sub(stamp) <= healthBreakGlassWindow
}

func getHealthBreakGlassTime(s session.Session) (time.Time, bool) {
	val := s.Get(healthBreakGlassSessionKey)
	if val == nil {
		return time.Time{}, false
	}

	switch v := val.(type) {
	case int64:
		return time.Unix(v, 0), true
	case int:
		return time.Unix(int64(v), 0), true
	case float64:
		return time.Unix(int64(v), 0), true
	case time.Time:
		return v, true
	case *time.Time:
		if v == nil {
			return time.Time{}, false
		}
		return *v, true
	default:
		log.Printf("Unexpected break glass timestamp type: %T", val)
		return time.Time{}, false
	}
}

func getSessionUsername(s session.Session) string {
	if val := s.Get("username"); val != nil {
		if username, ok := val.(string); ok && username != "" {
			return username
		}
	}
	return os.Getenv("AUTH_USERNAME")
}

func sanitizeNextPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/health"
	}
	if !strings.HasPrefix(raw, "/") {
		return "/health"
	}
	if strings.HasPrefix(raw, "//") {
		return "/health"
	}
	if strings.Contains(raw, "://") {
		return "/health"
	}
	if strings.Contains(raw, "\n") || strings.Contains(raw, "\r") {
		return "/health"
	}
	return raw
}
