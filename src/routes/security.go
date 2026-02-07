/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

// SessionMetadataMiddleware captures and stores device and IP info in the session
func SessionMetadataMiddleware() flamego.Handler {
	return func(c flamego.Context, s session.Session) {
		deviceLabel := parseUserAgent(c.Request().Header.Get("User-Agent"))
		if val, ok := s.Get("device_label").(string); !ok || val == "" || val != deviceLabel {
			s.Set("device_label", deviceLabel)
		}

		ip := getClientIP(c.Request())
		if val, ok := s.Get("device_ip").(string); !ok || val == "" || val != ip {
			s.Set("device_ip", ip)
		}

		c.Next()
	}
}

// parseUserAgent creates a simple device label from User-Agent string
func parseUserAgent(ua string) string {
	if ua == "" {
		return "Unknown device"
	}

	ua = strings.ToLower(ua)
	os := "Unknown OS"
	browser := "Unknown browser"

	// Simple OS detection
	if strings.Contains(ua, "android") {
		os = "Android"
	} else if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios") {
		os = "iOS"
	} else if strings.Contains(ua, "windows") {
		os = "Windows"
	} else if strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os") {
		os = "macOS"
	} else if strings.Contains(ua, "linux") {
		os = "Linux"
	}

	// Simple browser detection
	if strings.Contains(ua, "edg/") {
		browser = "Edge"
	} else if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg/") {
		browser = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		browser = "Safari"
	}

	return os + " / " + browser
}

// getClientIP extracts the real client IP address
func getClientIP(r *flamego.Request) string {
	// Check X-Forwarded-For header (first entry is the client)
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first := xff
		if idx := strings.Index(xff, ","); idx != -1 {
			first = xff[:idx]
		}
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}

	// Check X-Real-IP header
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// formatDuration creates a human-readable duration like "in 5d 10m"
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if days > 0 {
		if hours > 0 {
			parts = append(parts, fmt.Sprintf("%dh", hours))
		} else if minutes > 0 {
			parts = append(parts, fmt.Sprintf("%dm", minutes))
		}
	} else if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
		if minutes > 0 {
			parts = append(parts, fmt.Sprintf("%dm", minutes))
		}
	} else {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	if len(parts) == 0 {
		return "in 0m"
	}
	if len(parts) == 1 {
		return "in " + parts[0]
	}
	return "in " + parts[0] + " " + parts[1]
}

// SessionInfo represents a session for the security page
type SessionInfo struct {
	ExpiresAt time.Time
	ExpiresIn string
	Device    string
	IP        string
	IsCurrent bool
}

// Security renders the security page listing valid authenticated sessions
func Security(c flamego.Context, s session.Session, store session.Store, t template.Template, data template.Data) {
	// Type assert to our Postgres store to access ListValidSessions
	postgresStore, ok := store.(*db.PostgresSessionStore)
	if !ok {
		data["Error"] = "Unable to access session information"
		t.HTML(http.StatusInternalServerError, "security")
		return
	}

	// Get current session ID
	currentSessionID := s.ID()

	// List all valid sessions
	sessions, err := postgresStore.ListValidSessions(c.Request().Context())
	if err != nil {
		data["Error"] = "Failed to load session information"
		t.HTML(http.StatusInternalServerError, "security")
		return
	}

	// Convert to view models and mark current session
	var sessionInfos []SessionInfo
	for _, sess := range sessions {
		isCurrent := sess.ID == currentSessionID
		expiresIn := formatDuration(time.Until(sess.ExpiresAt))

		sessionInfos = append(sessionInfos, SessionInfo{
			ExpiresAt: sess.ExpiresAt,
			ExpiresIn: expiresIn,
			Device:    sess.DeviceLabel,
			IP:        sess.DeviceIP,
			IsCurrent: isCurrent,
		})
	}

	data["Sessions"] = sessionInfos
	data["Breadcrumbs"] = []BreadcrumbItem{
		{Name: "Security", URL: "", IsCurrent: true},
	}
	data["PageTitle"] = "Security"

	t.HTML(http.StatusOK, "security")
}

// InvalidateOtherSessions logs out all other authenticated sessions.
func InvalidateOtherSessions(c flamego.Context, s session.Session, store session.Store) {
	postgresStore, ok := store.(*db.PostgresSessionStore)
	if !ok {
		SetErrorFlash(s, "Unable to access session information")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	deleted, err := postgresStore.InvalidateOtherSessions(c.Request().Context(), s.ID())
	if err != nil {
		SetErrorFlash(s, "Failed to invalidate other sessions")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	if deleted == 0 {
		SetInfoFlash(s, "No other sessions to invalidate")
		c.Redirect("/security", http.StatusSeeOther)
		return
	}

	SetSuccessFlash(s, fmt.Sprintf("Invalidated %d other session(s)", deleted))
	c.Redirect("/security", http.StatusSeeOther)
}
