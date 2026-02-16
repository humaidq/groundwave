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

// ContactExchangeLinkDefaultTTL is the default link lifetime for contact exchange URLs.
const ContactExchangeLinkDefaultTTL = time.Hour

// ContactExchangeLink represents a one-time public contact collection URL.
type ContactExchangeLink struct {
	ID             uuid.UUID  `db:"id"`
	ContactID      uuid.UUID  `db:"contact_id"`
	Token          string     `db:"token"`
	CollectPhone   bool       `db:"collect_phone"`
	CollectEmail   bool       `db:"collect_email"`
	AdditionalNote string     `db:"additional_note"`
	CreatedAt      time.Time  `db:"created_at"`
	ExpiresAt      time.Time  `db:"expires_at"`
	UsedAt         *time.Time `db:"used_at"`
}

// CreateContactExchangeLink creates a one-time contact exchange link.
// Any previously active links for the same contact are invalidated.
func CreateContactExchangeLink(ctx context.Context, contactID string, collectPhone bool, collectEmail bool, additionalNote string, ttl time.Duration) (*ContactExchangeLink, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	if !collectPhone && !collectEmail {
		return nil, ErrContactExchangeCollectFieldEmpty
	}

	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return nil, ErrContactNotFound
	}

	additionalNote = strings.TrimSpace(additionalNote)

	if ttl <= 0 {
		ttl = ContactExchangeLinkDefaultTTL
	}

	var contactExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contacts WHERE id = $1)`, contactID).Scan(&contactExists); err != nil {
		return nil, fmt.Errorf("failed to verify contact: %w", err)
	}

	if !contactExists {
		return nil, ErrContactNotFound
	}

	token, err := generateContactExchangeToken()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().UTC().Add(ttl)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start contact exchange transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			logger.Warn("Failed to rollback contact exchange link creation", "error", err)
		}
	}()

	if _, err := tx.Exec(ctx, `
		UPDATE contact_exchange_links
		SET used_at = NOW()
		WHERE contact_id = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
	`, contactID); err != nil {
		return nil, fmt.Errorf("failed to invalidate previous contact exchange links: %w", err)
	}

	var link ContactExchangeLink
	if err := tx.QueryRow(ctx, `
		INSERT INTO contact_exchange_links (
			contact_id, token, collect_phone, collect_email, additional_note, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, contact_id, token, collect_phone, collect_email, additional_note, created_at, expires_at, used_at
	`, contactID, token, collectPhone, collectEmail, additionalNote, expiresAt).Scan(
		&link.ID,
		&link.ContactID,
		&link.Token,
		&link.CollectPhone,
		&link.CollectEmail,
		&link.AdditionalNote,
		&link.CreatedAt,
		&link.ExpiresAt,
		&link.UsedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to insert contact exchange link: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit contact exchange link transaction: %w", err)
	}

	return &link, nil
}

// GetActiveContactExchangeLink returns the latest active link for a contact.
func GetActiveContactExchangeLink(ctx context.Context, contactID string) (*ContactExchangeLink, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return nil, nil //nolint:nilnil // Empty contact ID should behave as not found for lookups.
	}

	var link ContactExchangeLink

	err := pool.QueryRow(ctx, `
		SELECT id, contact_id, token, collect_phone, collect_email, additional_note, created_at, expires_at, used_at
		FROM contact_exchange_links
		WHERE contact_id = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`, contactID).Scan(
		&link.ID,
		&link.ContactID,
		&link.Token,
		&link.CollectPhone,
		&link.CollectEmail,
		&link.AdditionalNote,
		&link.CreatedAt,
		&link.ExpiresAt,
		&link.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // Missing active link should return nil without error.
		}

		return nil, fmt.Errorf("failed to fetch active contact exchange link: %w", err)
	}

	return &link, nil
}

// GetContactExchangeLinkByToken returns an active link by token.
func GetContactExchangeLinkByToken(ctx context.Context, token string) (*ContactExchangeLink, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil //nolint:nilnil // Empty token should behave as not found for lookups.
	}

	var link ContactExchangeLink

	err := pool.QueryRow(ctx, `
		SELECT id, contact_id, token, collect_phone, collect_email, additional_note, created_at, expires_at, used_at
		FROM contact_exchange_links
		WHERE token = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
		LIMIT 1
	`, token).Scan(
		&link.ID,
		&link.ContactID,
		&link.Token,
		&link.CollectPhone,
		&link.CollectEmail,
		&link.AdditionalNote,
		&link.CreatedAt,
		&link.ExpiresAt,
		&link.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // Missing or expired token should return nil without error.
		}

		return nil, fmt.Errorf("failed to fetch contact exchange link by token: %w", err)
	}

	return &link, nil
}

// GetContactExchangeLinkByTokenAllowUsed returns an unexpired link by token,
// even if it has already been consumed.
func GetContactExchangeLinkByTokenAllowUsed(ctx context.Context, token string) (*ContactExchangeLink, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil //nolint:nilnil // Empty token should behave as not found for lookups.
	}

	var link ContactExchangeLink

	err := pool.QueryRow(ctx, `
		SELECT id, contact_id, token, collect_phone, collect_email, additional_note, created_at, expires_at, used_at
		FROM contact_exchange_links
		WHERE token = $1
		  AND expires_at > NOW()
		LIMIT 1
	`, token).Scan(
		&link.ID,
		&link.ContactID,
		&link.Token,
		&link.CollectPhone,
		&link.CollectEmail,
		&link.AdditionalNote,
		&link.CreatedAt,
		&link.ExpiresAt,
		&link.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // Missing or expired token should return nil without error.
		}

		return nil, fmt.Errorf("failed to fetch contact exchange link by token including used links: %w", err)
	}

	return &link, nil
}

// MarkContactExchangeLinkUsed marks an active token as consumed.
func MarkContactExchangeLinkUsed(ctx context.Context, token string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return ErrContactExchangeLinkInvalid
	}

	command, err := pool.Exec(ctx, `
		UPDATE contact_exchange_links
		SET used_at = NOW()
		WHERE token = $1
		  AND used_at IS NULL
		  AND expires_at > NOW()
	`, token)
	if err != nil {
		return fmt.Errorf("failed to mark contact exchange link used: %w", err)
	}

	if command.RowsAffected() == 0 {
		return ErrContactExchangeLinkInvalid
	}

	return nil
}

// SetContactAsMe marks a contact as "Me" and clears the flag from all others.
func SetContactAsMe(ctx context.Context, contactID string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return ErrContactNotFound
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start me-contact transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			logger.Warn("Failed to rollback me-contact update", "error", err)
		}
	}()

	if _, err := tx.Exec(ctx, `UPDATE contacts SET is_me = false WHERE is_me = true AND id <> $1`, contactID); err != nil {
		return fmt.Errorf("failed to clear previous me-contact: %w", err)
	}

	command, err := tx.Exec(ctx, `UPDATE contacts SET is_me = true WHERE id = $1`, contactID)
	if err != nil {
		return fmt.Errorf("failed to set me-contact: %w", err)
	}

	if command.RowsAffected() == 0 {
		return ErrContactNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit me-contact transaction: %w", err)
	}

	return nil
}

// ClearContactAsMe unsets the "Me" marker for a contact.
func ClearContactAsMe(ctx context.Context, contactID string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	contactID = strings.TrimSpace(contactID)
	if contactID == "" {
		return ErrContactNotFound
	}

	command, err := pool.Exec(ctx, `UPDATE contacts SET is_me = false WHERE id = $1`, contactID)
	if err != nil {
		return fmt.Errorf("failed to clear me-contact: %w", err)
	}

	if command.RowsAffected() == 0 {
		return ErrContactNotFound
	}

	return nil
}

// GetMeContact returns the contact marked as "Me", if one exists.
func GetMeContact(ctx context.Context) (*ContactDetail, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	var contactID string

	err := pool.QueryRow(ctx, `
		SELECT id
		FROM contacts
		WHERE is_me = true
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1
	`).Scan(&contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // Missing me-contact should return nil without error.
		}

		return nil, fmt.Errorf("failed to fetch me-contact id: %w", err)
	}

	contact, err := GetContact(ctx, contactID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch me-contact details: %w", err)
	}

	return contact, nil
}

func generateContactExchangeToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to generate contact exchange token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
