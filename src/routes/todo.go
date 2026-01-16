/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"log"
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

// Todo renders the todo org-mode page.
func Todo(c flamego.Context, t template.Template, data template.Data) {
	ctx := c.Request().Context()

	note, err := db.GetTodoNote(ctx)
	if err != nil {
		log.Printf("Error fetching todo note: %v", err)
		data["Error"] = "Failed to load todo list. Please check your WEBDAV_TODO_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables."
	} else {
		data["Note"] = note
	}

	data["IsTodo"] = true
	data["PageTitle"] = "Todo"

	t.HTML(http.StatusOK, "todo")
}
