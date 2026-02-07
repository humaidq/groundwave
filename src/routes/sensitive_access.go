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

const sensitiveAccessSessionKey = "sensitive_access_at"
const sensitiveAccessWindow = 30 * time.Minute

// BreakGlassForm renders the reauthentication page for sensitive data access.
func BreakGlassForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	data["HeaderOnly"] = true
	data["Username"] = getSessionUsername(s)
	data["Next"] = sanitizeNextPath(c.Query("next"))

	t.HTML(http.StatusOK, "break_glass")
}

// BreakGlass handles reauthentication POST for sensitive data access.
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

	s.Set(sensitiveAccessSessionKey, time.Now().Unix())

	if next == "" {
		next = "/contacts"
	}

	c.Redirect(next, http.StatusSeeOther)
}

// RequireSensitiveAccessForHealth enforces sensitive access for health routes.
func RequireSensitiveAccessForHealth(s session.Session, c flamego.Context) {
	path := c.Request().URL.Path
	if !strings.HasPrefix(path, "/health") {
		c.Next()
		return
	}
	if strings.HasPrefix(path, "/health/break-glass") {
		c.Next()
		return
	}

	if HasSensitiveAccess(s, time.Now()) {
		c.Next()
		return
	}

	redirectToBreakGlass(c)
}

// RequireSensitiveAccess redirects to break glass when locked.
func RequireSensitiveAccess(s session.Session, c flamego.Context) {
	if HasSensitiveAccess(s, time.Now()) {
		c.Next()
		return
	}
	redirectToBreakGlass(c)
}

// LockSensitiveAccess clears sensitive access for this session.
func LockSensitiveAccess(s session.Session, c flamego.Context) {
	s.Delete(sensitiveAccessSessionKey)

	referer := c.Request().Header.Get("Referer")
	if referer == "" {
		referer = "/contacts"
	}
	c.Redirect(referer, http.StatusSeeOther)
}

// HasSensitiveAccess returns true if the session is within the unlock window.
func HasSensitiveAccess(s session.Session, now time.Time) bool {
	stamp, ok := getSensitiveAccessTime(s)
	if !ok {
		return false
	}
	return now.Sub(stamp) <= sensitiveAccessWindow
}

func getSensitiveAccessTime(s session.Session) (time.Time, bool) {
	val := s.Get(sensitiveAccessSessionKey)
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

func redirectToBreakGlass(c flamego.Context) {
	request := c.Request()
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		next := sanitizeNextPath(request.URL.RequestURI())
		redirectURL := "/break-glass?next=" + url.QueryEscape(next)
		c.Redirect(redirectURL, http.StatusSeeOther)
		return
	}

	next := sanitizeNextPath(request.Header.Get("Referer"))
	redirectURL := "/break-glass?next=" + url.QueryEscape(next)
	c.Redirect(redirectURL, http.StatusSeeOther)
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
		return "/contacts"
	}
	if strings.Contains(raw, "\n") || strings.Contains(raw, "\r") {
		return "/contacts"
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "/contacts"
		}
		path := parsed.EscapedPath()
		if path == "" {
			path = "/"
		}
		if strings.HasPrefix(path, "//") {
			return "/contacts"
		}
		if parsed.RawQuery != "" {
			return path + "?" + parsed.RawQuery
		}
		return path
	}
	if !strings.HasPrefix(raw, "/") {
		return "/contacts"
	}
	if strings.HasPrefix(raw, "//") {
		return "/contacts"
	}
	return raw
}
