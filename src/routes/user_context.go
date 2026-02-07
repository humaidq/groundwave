/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"
	"github.com/google/uuid"

	"github.com/humaidq/groundwave/db"
)

// UserContextInjector loads session user metadata into templates.
func UserContextInjector() flamego.Handler {
	return func(c flamego.Context, s session.Session, data template.Data) {
		authenticated, _ := s.Get("authenticated").(bool)
		data["IsAuthenticated"] = authenticated
		if !authenticated {
			return
		}

		isAdmin, err := resolveSessionIsAdmin(c.Request().Context(), s)
		if err != nil {
			log.Printf("Failed to resolve user admin state: %v", err)
			return
		}
		data["IsAdmin"] = isAdmin
	}
}

// RequireAdmin blocks access for non-admin users.
func RequireAdmin(s session.Session, c flamego.Context) {
	isAdmin, err := resolveSessionIsAdmin(c.Request().Context(), s)
	if err != nil || !isAdmin {
		SetErrorFlash(s, "Access restricted")
		c.Redirect("/inventory", http.StatusSeeOther)
		return
	}
	c.Next()
}

func resolveSessionIsAdmin(ctx context.Context, s session.Session) (bool, error) {
	user, err := resolveSessionUser(ctx, s)
	if err != nil {
		return false, err
	}
	return user.IsAdmin, nil
}

func resolveSessionUser(ctx context.Context, s session.Session) (*db.User, error) {
	userID, ok := getSessionUserID(s)
	if !ok {
		return nil, fmt.Errorf("session user missing")
	}

	isAdmin, hasAdmin := s.Get("user_is_admin").(bool)
	displayName, hasName := s.Get("user_display_name").(string)
	if hasAdmin && hasName {
		if parsedID, err := uuid.Parse(userID); err == nil {
			return &db.User{
				ID:          parsedID,
				DisplayName: displayName,
				IsAdmin:     isAdmin,
			}, nil
		}
	}

	user, err := db.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.DisplayName != "" {
		s.Set("user_display_name", user.DisplayName)
	}
	s.Set("user_is_admin", user.IsAdmin)
	return user, nil
}
