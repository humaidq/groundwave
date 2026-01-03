/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/humaidq/groundwave/db"
	"github.com/pressly/goose/v3"
	"github.com/urfave/cli/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var CmdMigrate = &cli.Command{
	Name:  "migrate",
	Usage: "Database migration commands",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "database-url",
			Sources: cli.EnvVars("DATABASE_URL"),
			Usage:   "PostgreSQL connection string (e.g., postgres://user:pass@localhost/dbname)",
		},
	},
	Commands: []*cli.Command{
		{
			Name:   "up",
			Usage:  "Run all pending migrations",
			Action: migrateUp,
		},
		{
			Name:   "down",
			Usage:  "Roll back the last migration",
			Action: migrateDown,
		},
		{
			Name:   "status",
			Usage:  "Show migration status",
			Action: migrateStatus,
		},
		{
			Name:  "create",
			Usage: "Create a new migration file <name>",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:  "sql",
					Usage: "Create a SQL migration (default)",
					Value: true,
				},
			},
			Action: migrateCreate,
		},
		{
			Name:   "version",
			Usage:  "Print the current version of the database",
			Action: migrateVersion,
		},
	},
}

func getDB(cmd *cli.Command) (*sql.DB, error) {
	databaseURL := cmd.String("database-url")
	if databaseURL == "" {
		return nil, fmt.Errorf("database-url is required (set via --database-url or DATABASE_URL env var)")
	}

	// Set DATABASE_URL for db package
	os.Setenv("DATABASE_URL", databaseURL)

	// Open a database/sql connection for goose
	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set goose to use embedded migrations from db package
	goose.SetBaseFS(db.GetEmbeddedMigrations())

	// Set dialect
	if err := goose.SetDialect("postgres"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to set dialect: %w", err)
	}

	return sqlDB, nil
}

func migrateUp(ctx context.Context, cmd *cli.Command) error {
	db, err := getDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println("Migrations completed successfully")
	return nil
}

func migrateDown(ctx context.Context, cmd *cli.Command) error {
	db, err := getDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Down(db, "migrations"); err != nil {
		return fmt.Errorf("failed to roll back migration: %w", err)
	}

	fmt.Println("Migration rolled back successfully")
	return nil
}

func migrateStatus(ctx context.Context, cmd *cli.Command) error {
	db, err := getDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.Status(db, "migrations"); err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	return nil
}

func migrateVersion(ctx context.Context, cmd *cli.Command) error {
	db, err := getDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	version, err := goose.GetDBVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get database version: %w", err)
	}

	fmt.Printf("Database version: %d\n", version)
	return nil
}

func migrateCreate(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < 1 {
		return fmt.Errorf("migration name is required")
	}
	name := args.First()

	// Note: This command is for development only and requires source code access
	// Create migration in the migrations directory (filesystem path, not embedded)
	migrationsDir := "db/migrations"
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Don't use embedded FS for create - we need to write to actual filesystem
	if err := goose.Create(nil, migrationsDir, name, "sql"); err != nil {
		return fmt.Errorf("failed to create migration: %w", err)
	}

	fmt.Printf("Created new migration in %s/\n", migrationsDir)
	return nil
}
