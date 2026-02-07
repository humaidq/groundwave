/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateUserInput defines data for creating a user.
type CreateUserInput struct {
	ID          *uuid.UUID
	DisplayName string
	IsAdmin     bool
}

// CountUsers returns the number of users.
func CountUsers(ctx context.Context) (int, error) {
	if pool == nil {
		return 0, fmt.Errorf("database connection not initialized")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// CreateUser creates a user record.
func CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}
	if input.DisplayName == "" {
		return nil, fmt.Errorf("display name is required")
	}

	var user User
	query := `
		INSERT INTO users (id, display_name, is_admin)
		VALUES (COALESCE($1, gen_random_uuid()), $2, $3)
		RETURNING id, display_name, is_admin, created_at, updated_at
	`

	if err := pool.QueryRow(ctx, query, input.ID, input.DisplayName, input.IsAdmin).Scan(
		&user.ID,
		&user.DisplayName,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &user, nil
}

// DeleteUser removes a user by ID.
func DeleteUser(ctx context.Context, userID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	command, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetFirstUser returns the first created user.
func GetFirstUser(ctx context.Context) (*User, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var user User
	query := `
		SELECT id, display_name, is_admin, created_at, updated_at
		FROM users
		ORDER BY created_at ASC
		LIMIT 1
	`
	if err := pool.QueryRow(ctx, query).Scan(
		&user.ID,
		&user.DisplayName,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get first user: %w", err)
	}

	return &user, nil
}

// GetUserByID returns a user by ID.
func GetUserByID(ctx context.Context, id string) (*User, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	var user User
	query := `
		SELECT id, display_name, is_admin, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	if err := pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.DisplayName,
		&user.IsAdmin,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// ListUsers returns all users.
func ListUsers(ctx context.Context) ([]User, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, display_name, is_admin, created_at, updated_at
		FROM users
		ORDER BY created_at ASC
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.ID,
			&user.DisplayName,
			&user.IsAdmin,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// GetUserByWebAuthnID resolves a user by WebAuthn user handle bytes.
func GetUserByWebAuthnID(ctx context.Context, userHandle []byte) (*User, error) {
	userID, err := uuid.FromBytes(userHandle)
	if err != nil {
		return nil, fmt.Errorf("invalid user handle")
	}
	return GetUserByID(ctx, userID.String())
}

// ListUserPasskeys returns passkeys for a user.
func ListUserPasskeys(ctx context.Context, userID string) ([]UserPasskey, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT id, user_id, credential_id, credential_data, label, created_at, last_used_at
		FROM user_passkeys
		WHERE user_id = $1
		ORDER BY created_at ASC
	`
	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list passkeys: %w", err)
	}
	defer rows.Close()

	var passkeys []UserPasskey
	for rows.Next() {
		var passkey UserPasskey
		if err := rows.Scan(
			&passkey.ID,
			&passkey.UserID,
			&passkey.CredentialID,
			&passkey.CredentialData,
			&passkey.Label,
			&passkey.CreatedAt,
			&passkey.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan passkey: %w", err)
		}
		passkeys = append(passkeys, passkey)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating passkeys: %w", err)
	}

	return passkeys, nil
}

// CountUserPasskeys returns the number of passkeys for a user.
func CountUserPasskeys(ctx context.Context, userID string) (int, error) {
	if pool == nil {
		return 0, fmt.Errorf("database connection not initialized")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_passkeys WHERE user_id = $1`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count passkeys: %w", err)
	}
	return count, nil
}

// AddUserPasskey stores a new passkey for a user.
func AddUserPasskey(ctx context.Context, userID string, credential webauthn.Credential, label *string) (*UserPasskey, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	credentialData, err := encodeCredential(credential)
	if err != nil {
		return nil, err
	}

	var passkey UserPasskey
	query := `
		INSERT INTO user_passkeys (user_id, credential_id, credential_data, label, last_used_at)
		VALUES ($1, $2, $3, $4, NULL)
		RETURNING id, user_id, credential_id, credential_data, label, created_at, last_used_at
	`
	if err := pool.QueryRow(ctx, query, userID, credential.ID, credentialData, label).Scan(
		&passkey.ID,
		&passkey.UserID,
		&passkey.CredentialID,
		&passkey.CredentialData,
		&passkey.Label,
		&passkey.CreatedAt,
		&passkey.LastUsedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to store passkey: %w", err)
	}

	return &passkey, nil
}

// UpdateUserPasskeyCredential updates stored credential data and last used timestamp.
func UpdateUserPasskeyCredential(ctx context.Context, userID string, credential webauthn.Credential, lastUsed time.Time) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	credentialData, err := encodeCredential(credential)
	if err != nil {
		return err
	}

	command, err := pool.Exec(ctx, `
		UPDATE user_passkeys
		SET credential_data = $1, last_used_at = $2
		WHERE user_id = $3 AND credential_id = $4
	`, credentialData, lastUsed, userID, credential.ID)
	if err != nil {
		return fmt.Errorf("failed to update passkey: %w", err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("passkey not found")
	}

	return nil
}

// DeleteUserPasskey removes a passkey by ID.
func DeleteUserPasskey(ctx context.Context, userID string, passkeyID string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	command, err := pool.Exec(ctx, `DELETE FROM user_passkeys WHERE id = $1 AND user_id = $2`, passkeyID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete passkey: %w", err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("passkey not found")
	}

	return nil
}

// LoadUserCredentials loads WebAuthn credentials for a user.
func LoadUserCredentials(ctx context.Context, userID string) ([]webauthn.Credential, error) {
	passkeys, err := ListUserPasskeys(ctx, userID)
	if err != nil {
		return nil, err
	}

	credentials := make([]webauthn.Credential, 0, len(passkeys))
	for _, passkey := range passkeys {
		credential, err := decodeCredential(passkey.CredentialData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode passkey credential: %w", err)
		}
		credentials = append(credentials, credential)
	}

	return credentials, nil
}

func encodeCredential(credential webauthn.Credential) ([]byte, error) {
	data, err := json.Marshal(credential)
	if err != nil {
		return nil, fmt.Errorf("failed to encode credential: %w", err)
	}
	return data, nil
}

func decodeCredential(data []byte) (webauthn.Credential, error) {
	var credential webauthn.Credential
	if err := json.Unmarshal(data, &credential); err != nil {
		return webauthn.Credential{}, fmt.Errorf("failed to decode credential: %w", err)
	}
	return credential, nil
}
