/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"fmt"
	"net/http"
	"net/url"
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
	next := sanitizeNextPath(c.Query("next"))
	logBreakGlassView(c, s, next)

	data["HeaderOnly"] = true
	data["DisplayName"] = getSessionDisplayName(s)
	data["Next"] = next
	data["BreakGlassBase"] = c.Request().URL.Path

	t.HTML(http.StatusOK, "break_glass")
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

	redirectToBreakGlass(c, s)
}

// RequireSensitiveAccess redirects to break glass when locked.
func RequireSensitiveAccess(s session.Session, c flamego.Context) {
	if HasSensitiveAccess(s, time.Now()) {
		c.Next()
		return
	}
	redirectToBreakGlass(c, s)
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
		logger.Warn("Unexpected break glass timestamp type", "type", fmt.Sprintf("%T", val))
		return time.Time{}, false
	}
}

func redirectToBreakGlass(c flamego.Context, s session.Session) {
	request := c.Request()
	if request.Method == http.MethodGet || request.Method == http.MethodHead {
		next := sanitizeNextPath(request.URL.RequestURI())
		redirectURL := "/break-glass?next=" + url.QueryEscape(next)
		logAccessDenied(c, s, "sensitive_access_locked", http.StatusSeeOther, "/break-glass", "next", next)
		c.Redirect(redirectURL, http.StatusSeeOther)
		return
	}

	next := sanitizeNextPath(request.Header.Get("Referer"))
	redirectURL := "/break-glass?next=" + url.QueryEscape(next)
	logAccessDenied(c, s, "sensitive_access_locked", http.StatusSeeOther, "/break-glass", "next", next)
	c.Redirect(redirectURL, http.StatusSeeOther)
}

func getSessionDisplayName(s session.Session) string {
	if val := s.Get("user_display_name"); val != nil {
		if displayName, ok := val.(string); ok && displayName != "" {
			return displayName
		}
	}
	return "Admin"
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
