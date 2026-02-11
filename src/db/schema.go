/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"

	// Register pgx with database/sql for goose migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// GetEmbeddedMigrations returns the embedded migrations filesystem for use by CLI commands
func GetEmbeddedMigrations() embed.FS {
	return embedMigrations
}

// SyncSchema runs database migrations using goose
func SyncSchema(ctx context.Context) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	// Get the original DATABASE_URL (preserves Unix sockets, complex connection strings)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return ErrDatabaseURLEnvVarNotSet
	}

	// Open a database/sql connection for goose
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("Failed to close migration connection", "error", err)
		}
	}()

	// Set goose to use embedded migrations
	goose.SetBaseFS(embedMigrations)

	// Run migrations
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// After migrations complete successfully, sync reference ranges
	if err := SyncReferenceRanges(ctx); err != nil {
		return fmt.Errorf("failed to sync reference ranges: %w", err)
	}

	return nil
}
