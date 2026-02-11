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

var (
	// ErrInviteNotExpired is returned when regeneration is requested before expiry.
	ErrInviteNotExpired = errors.New("invite is not expired")
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
		return nil, ErrDatabaseConnectionNotInitialized
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	var createdByID *uuid.UUID

	if strings.TrimSpace(createdBy) != "" {
		parsed, err := uuid.Parse(createdBy)
		if err != nil {
			return nil, ErrInvalidCreatorID
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

// ListPendingUserInvites returns all unused, unexpired invites.
func ListPendingUserInvites(ctx context.Context) ([]UserInvite, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT id, token, display_name, created_at, used_at, created_by
		FROM user_invites
		WHERE used_at IS NULL
		  AND created_at >= NOW() - INTERVAL '24 hours'
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

// ListExpiredUserInvites returns all unused invites older than 24 hours.
func ListExpiredUserInvites(ctx context.Context) ([]UserInvite, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT id, token, display_name, created_at, used_at, created_by
		FROM user_invites
		WHERE used_at IS NULL
		  AND created_at < NOW() - INTERVAL '24 hours'
		ORDER BY created_at DESC
	`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list expired invites: %w", err)
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
			return nil, fmt.Errorf("failed to scan expired invite: %w", err)
		}

		invites = append(invites, invite)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating expired invites: %w", err)
	}

	return invites, nil
}

// GetUserInviteByToken returns an active invite by its token.
func GetUserInviteByToken(ctx context.Context, token string) (*UserInvite, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	var invite UserInvite

	query := `
		SELECT id, token, display_name, created_at, used_at, created_by
		FROM user_invites
		WHERE token = $1
		  AND used_at IS NULL
		  AND created_at >= NOW() - INTERVAL '24 hours'
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
			return nil, nil //nolint:nilnil // Missing or expired token should return no invite without error.
		}

		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return &invite, nil
}

// GetUserInviteByID returns an active invite by its ID.
func GetUserInviteByID(ctx context.Context, id string) (*UserInvite, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	var invite UserInvite

	query := `
		SELECT id, token, display_name, created_at, used_at, created_by
		FROM user_invites
		WHERE id = $1
		  AND used_at IS NULL
		  AND created_at >= NOW() - INTERVAL '24 hours'
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
			return nil, nil //nolint:nilnil // Missing or expired invite ID should return no invite without error.
		}

		return nil, fmt.Errorf("failed to get invite: %w", err)
	}

	return &invite, nil
}

// MarkUserInviteUsed marks an active invite as used.
func MarkUserInviteUsed(ctx context.Context, id string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	command, err := pool.Exec(ctx, `
		UPDATE user_invites
		SET used_at = NOW()
		WHERE id = $1
		  AND used_at IS NULL
		  AND created_at >= NOW() - INTERVAL '24 hours'
	`, id)
	if err != nil {
		return fmt.Errorf("failed to mark invite used: %w", err)
	}

	if command.RowsAffected() == 0 {
		return ErrInviteNotFound
	}

	return nil
}

// RegenerateExpiredUserInvite rotates an expired invite token and resets its TTL.
func RegenerateExpiredUserInvite(ctx context.Context, id string) (*UserInvite, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	var invite UserInvite

	query := `
		UPDATE user_invites
		SET token = $1,
		    created_at = NOW()
		WHERE id = $2
		  AND used_at IS NULL
		  AND created_at < NOW() - INTERVAL '24 hours'
		RETURNING id, token, display_name, created_at, used_at, created_by
	`

	err = pool.QueryRow(ctx, query, token, id).Scan(
		&invite.ID,
		&invite.Token,
		&invite.DisplayName,
		&invite.CreatedAt,
		&invite.UsedAt,
		&invite.CreatedBy,
	)
	if err == nil {
		return &invite, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to regenerate invite: %w", err)
	}

	var isExpired bool

	err = pool.QueryRow(ctx, `
		SELECT created_at < NOW() - INTERVAL '24 hours'
		FROM user_invites
		WHERE id = $1
		  AND used_at IS NULL
	`, id).Scan(&isExpired)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInviteNotFound
		}

		return nil, fmt.Errorf("failed to load invite status: %w", err)
	}

	if !isExpired {
		return nil, ErrInviteNotExpired
	}

	return nil, ErrInviteNotFound
}

// DeleteUserInvite removes an invite by ID.
func DeleteUserInvite(ctx context.Context, id string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	command, err := pool.Exec(ctx, `DELETE FROM user_invites WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete invite: %w", err)
	}

	if command.RowsAffected() == 0 {
		return ErrInviteNotFound
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
