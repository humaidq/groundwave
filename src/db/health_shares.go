/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// HealthProfileShare represents a shared profile entry.
type HealthProfileShare struct {
	UserID    uuid.UUID  `db:"user_id"`
	ProfileID uuid.UUID  `db:"profile_id"`
	CreatedAt time.Time  `db:"created_at"`
	CreatedBy *uuid.UUID `db:"created_by"`
}

// ListHealthProfilesForUser returns profiles shared with a user.
func ListHealthProfilesForUser(ctx context.Context, userID string) ([]HealthProfileSummary, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT s.id, s.name, s.date_of_birth, s.gender, s.description, s.is_primary,
		       s.created_at, s.updated_at, s.followup_count, s.last_followup_date
		FROM health_profiles_summary s
		INNER JOIN health_profile_shares h ON h.profile_id = s.id
		WHERE h.user_id = $1
		ORDER BY s.name ASC
	`

	rows, err := pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shared profiles: %w", err)
	}
	defer rows.Close()

	var profiles []HealthProfileSummary
	for rows.Next() {
		var profile HealthProfileSummary
		err := rows.Scan(
			&profile.ID, &profile.Name, &profile.DateOfBirth, &profile.Gender, &profile.Description,
			&profile.IsPrimary,
			&profile.CreatedAt, &profile.UpdatedAt,
			&profile.FollowupCount, &profile.LastFollowupDate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan profile: %w", err)
		}
		profiles = append(profiles, profile)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating profiles: %w", err)
	}

	return profiles, nil
}

// UserHasHealthProfileAccess checks whether a user can access a profile.
func UserHasHealthProfileAccess(ctx context.Context, userID, profileID string) (bool, error) {
	if pool == nil {
		return false, fmt.Errorf("database connection not initialized")
	}

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM health_profile_shares WHERE user_id = $1 AND profile_id = $2)`,
		userID, profileID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check profile share: %w", err)
	}

	return exists, nil
}

// UserHasHealthFollowupAccess checks whether a user can access a follow-up.
func UserHasHealthFollowupAccess(ctx context.Context, userID, followupID string) (bool, error) {
	if pool == nil {
		return false, fmt.Errorf("database connection not initialized")
	}

	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM health_followups f
			INNER JOIN health_profile_shares s ON s.profile_id = f.profile_id
			WHERE f.id = $1 AND s.user_id = $2
		)
	`
	if err := pool.QueryRow(ctx, query, followupID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check follow-up share: %w", err)
	}

	return exists, nil
}

// ListHealthProfileShares returns all profile shares.
func ListHealthProfileShares(ctx context.Context) ([]HealthProfileShare, error) {
	if pool == nil {
		return nil, fmt.Errorf("database connection not initialized")
	}

	query := `
		SELECT user_id, profile_id, created_at, created_by
		FROM health_profile_shares
		ORDER BY created_at DESC
	`
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list profile shares: %w", err)
	}
	defer rows.Close()

	var shares []HealthProfileShare
	for rows.Next() {
		var share HealthProfileShare
		if err := rows.Scan(&share.UserID, &share.ProfileID, &share.CreatedAt, &share.CreatedBy); err != nil {
			return nil, fmt.Errorf("failed to scan profile share: %w", err)
		}
		shares = append(shares, share)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating profile shares: %w", err)
	}

	return shares, nil
}

// SetHealthProfileShares replaces the profile shares for a user.
func SetHealthProfileShares(ctx context.Context, userID string, profileIDs []string, createdBy string) error {
	if pool == nil {
		return fmt.Errorf("database connection not initialized")
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID")
	}

	var createdByUUID *uuid.UUID
	if createdBy != "" {
		parsed, err := uuid.Parse(createdBy)
		if err != nil {
			return fmt.Errorf("invalid creator ID")
		}
		createdByUUID = &parsed
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM health_profile_shares WHERE user_id = $1`, userUUID); err != nil {
		return fmt.Errorf("failed to clear existing shares: %w", err)
	}

	for _, profileID := range profileIDs {
		profileUUID, err := uuid.Parse(profileID)
		if err != nil {
			return fmt.Errorf("invalid profile ID")
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO health_profile_shares (user_id, profile_id, created_by) VALUES ($1, $2, $3)`,
			userUUID, profileUUID, createdByUUID,
		); err != nil {
			return fmt.Errorf("failed to add profile share: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit profile shares: %w", err)
	}

	return nil
}
