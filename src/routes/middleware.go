/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
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
