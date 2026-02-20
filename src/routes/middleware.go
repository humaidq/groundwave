/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"

	"github.com/flamego/csrf"
	"github.com/flamego/flamego"
	"github.com/flamego/template"
)

// CSRFInjector automatically injects CSRF token into template data for all routes
func CSRFInjector() flamego.Handler {
	return func(x csrf.CSRF, data template.Data) {
		data["csrf_token"] = x.Token()
	}
}

// NoCacheHeaders disables caching for all page responses and blocks indexing.
func NoCacheHeaders() flamego.Handler {
	return func(c flamego.Context) {
		header := c.ResponseWriter().Header()
		header.Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet")

		if c.Request().Method == http.MethodGet || c.Request().Method == http.MethodHead {
			header.Set("Cache-Control", "no-store, max-age=0")
			header.Set("Pragma", "no-cache")
			header.Set("Expires", "0")
		}

		c.Next()
	}
}
