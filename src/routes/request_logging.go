/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"

	"github.com/humaidq/groundwave/logging"
)

var requestLogger = logging.Logger(logging.SourceWebRequest)

// RequestLogger logs request metadata and timing for each HTTP request.
func RequestLogger(c flamego.Context, s session.Session) {
	start := time.Now()

	c.Next()

	status := c.ResponseWriter().Status()
	if status == 0 {
		status = http.StatusOK
	}

	fields := []interface{}{
		"event", "request",
		"status", status,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	fields = append(fields, baseRequestFields(c, s)...)

	requestLogger.Info("request", fields...)
}

func logAccessDenied(c flamego.Context, s session.Session, reason string, status int, redirect string, extra ...interface{}) {
	fields := []interface{}{
		"event", "access_denied",
		"reason", reason,
		"status", status,
	}
	if redirect != "" {
		fields = append(fields, "redirect", redirect)
	}

	fields = append(fields, baseRequestFields(c, s)...)
	fields = append(fields, extra...)

	requestLogger.Warn("access denied", fields...)
}

func logBreakGlassView(c flamego.Context, s session.Session, next string) {
	fields := []interface{}{
		"event", "break_glass_view",
	}
	if next != "" {
		fields = append(fields, "next", next)
	}

	fields = append(fields, baseRequestFields(c, s)...)

	requestLogger.Info("break glass view", fields...)
}

func baseRequestFields(c flamego.Context, s session.Session) []interface{} {
	authenticated, userID := sessionAuthInfo(s)

	fields := []interface{}{
		"method", c.Request().Method,
		"path", c.Request().URL.Path,
		"ip", clientIP(c),
		"user_agent", c.Request().UserAgent(),
		"authenticated", authenticated,
	}
	if userID != "" {
		fields = append(fields, "user_id", userID)
	}

	return fields
}

func sessionAuthInfo(s session.Session) (bool, string) {
	authenticated, ok := s.Get("authenticated").(bool)
	if !ok || !authenticated {
		return false, ""
	}

	userID, _ := getSessionUserID(s)

	return true, userID
}

func clientIP(c flamego.Context) string {
	forwardedFor := c.Request().Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		if idx := strings.Index(forwardedFor, ","); idx != -1 {
			forwardedFor = forwardedFor[:idx]
		}

		if ip := strings.TrimSpace(forwardedFor); ip != "" {
			return ip
		}
	}

	return c.RemoteAddr()
}
