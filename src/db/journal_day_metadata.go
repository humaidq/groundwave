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

// ListJournalDayLocations fetches all locations for a journal day.
func ListJournalDayLocations(ctx context.Context, day time.Time) ([]JournalDayLocation, error) {
	if pool == nil {
		return nil, ErrDatabaseConnectionNotInitialized
	}

	query := `
		SELECT id, day, location_lat, location_lon, created_at, updated_at
		FROM journal_day_metadata
		WHERE day = $1
		ORDER BY created_at ASC
	`

	rows, err := pool.Query(ctx, query, day)
	if err != nil {
		return nil, fmt.Errorf("failed to query journal day locations: %w", err)
	}
	defer rows.Close()

	var locations []JournalDayLocation

	for rows.Next() {
		var location JournalDayLocation

		err := rows.Scan(
			&location.ID,
			&location.Day,
			&location.LocationLat,
			&location.LocationLon,
			&location.CreatedAt,
			&location.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan journal day location: %w", err)
		}

		locations = append(locations, location)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating journal day locations: %w", err)
	}

	return locations, nil
}

// CreateJournalDayLocation creates a location entry for a journal day.
func CreateJournalDayLocation(ctx context.Context, day time.Time, lat float64, lon float64) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	query := `
		INSERT INTO journal_day_metadata (day, location_lat, location_lon)
		VALUES ($1, $2, $3)
	`

	_, err := pool.Exec(ctx, query, day, lat, lon)
	if err != nil {
		return fmt.Errorf("failed to create journal day location: %w", err)
	}

	return nil
}

// DeleteJournalDayLocation deletes a location entry by ID.
func DeleteJournalDayLocation(ctx context.Context, locationID uuid.UUID) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	query := `DELETE FROM journal_day_metadata WHERE id = $1`

	result, err := pool.Exec(ctx, query, locationID)
	if err != nil {
		return fmt.Errorf("failed to delete journal day location: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrLocationNotFound, locationID)
	}

	return nil
}
