/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserInvite represents an invitation token for provisioning a user.
type UserInvite struct {
	ID          uuid.UUID  `db:"id"`
	Token       string     `db:"token"`
	DisplayName *string    `db:"display_name"`
	CreatedAt   time.Time  `db:"created_at"`
	UsedAt      *time.Time `db:"used_at"`
	CreatedBy   *uuid.UUID `db:"created_by"`
}

// CreateUserInvite creates a new invite token.
func CreateUserInvite(ctx context.Context, createdBy string, displayName string) (*UserInvite, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	var createdByID *uuid.UUID
	if strings.TrimSpace(createdBy) != "" {
		parsed, err := uuid.Parse(createdBy)
		if err != nil {
			return nil, fmt.Errorf("invalid creator ID")
		}
		createdByID = &parsed
	}

	var displayNamePtr *string
	if name := strings.TrimSpace(displayName); name != "" {
		displayNamePtr = &name
	}

	var invite UserInvite
	query := `
		INSERT INTO user_invites (token, display_name, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, token, display_name, created_at, used_at, created_by
	`
	if err := pool.QueryRow(ctx, query, token, displayNamePtr, createdByID).Scan(
		&invite.ID,
		&invite.Token,
		&invite.DisplayName,
		&invite.CreatedAt,
		&invite.UsedAt,
		&invite.CreatedBy,
	); err != nil {
		return nil, fmt.Errorf("failed to create invite: %w", err)
	}

	return &invite, nil
}

// ListPendingUserInvites returns all unused invites.
func ListPendingUserInvites(ctx context.Context) ([]UserInvite, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, token, display_name, created_at, used_at, created_by
		FROM user_invites
		WHERE used_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list invites: %w", err)
	}
	defer rows.Close()

	var invites []UserInvite
	for rows.Next() {
		var invite UserInvite
		if err := rows.Scan(
			&invite.ID,
			&invite.Token,
			&invite.DisplayName,
			&invite.CreatedAt,
			&invite.UsedAt,
			&invite.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan invite: %w", err)
		}
		invites = append(invites, invite)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating invites: %w", err)
	}

	return invites, nil
}

// GetUserInviteByToken returns an invite by its token.
func GetUserInviteByToken(ctx context.Context, token string) (*UserInvite, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var invite UserInvite
	query := `
		SELECT id, token, display_name, created_at, used_at, created_by
		FROM user_invites
		WHERE token = $1
	`
	err := pool.QueryRow(ctx, query, token).Scan(
		&invite.ID,
		&invite.Token,
		&invite.DisplayName,
		&invite.CreatedAt,
		&invite.UsedAt,
		&invite.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return &invite, nil
}

// GetUserInviteByID returns an invite by its ID.
func GetUserInviteByID(ctx context.Context, id string) (*UserInvite, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var invite UserInvite
	query := `
		SELECT id, token, display_name, created_at, used_at, created_by
		FROM user_invites
		WHERE id = $1
	`
	err := pool.QueryRow(ctx, query, id).Scan(
		&invite.ID,
		&invite.Token,
		&invite.DisplayName,
		&invite.CreatedAt,
		&invite.UsedAt,
		&invite.CreatedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return &invite, nil
}

// MarkUserInviteUsed marks an invite as used.
func MarkUserInviteUsed(ctx context.Context, id string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	command, err := pool.Exec(ctx, `UPDATE user_invites SET used_at = NOW() WHERE id = $1 AND used_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("failed to mark invite used: %w", err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("invite not found")
	}

	return nil
}

// DeleteUserInvite removes an invite by ID.
func DeleteUserInvite(ctx context.Context, id string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	command, err := pool.Exec(ctx, `DELETE FROM user_invites WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete invite: %w", err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("invite not found")
	}

	return nil
}

func generateInviteToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to generate invite token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
