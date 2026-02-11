// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

var testSchemaName string

func TestMain(m *testing.M) {
	ctx := context.Background()

	baseDatabaseURL := os.Getenv("DATABASE_URL")
	if baseDatabaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	if err := ensureDatabaseExists(ctx, baseDatabaseURL); err != nil {
		fmt.Fprintln(os.Stderr, "failed to ensure database exists:", err)
		os.Exit(1)
	}

	testSchemaName = fmt.Sprintf("test_%d_%d", time.Now().UnixNano(), os.Getpid())

	if err := createTestSchema(ctx, baseDatabaseURL, testSchemaName); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create test schema:", err)
		os.Exit(1)
	}

	if err := initTestPool(ctx, baseDatabaseURL, testSchemaName); err != nil {
		fmt.Fprintln(os.Stderr, "failed to init test pool:", err)
		os.Exit(1)
	}

	searchPathURL, err := withSearchPath(baseDatabaseURL, testSchemaName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to build search_path url:", err)
		os.Exit(1)
	}

	if err := syncSchemaForTest(ctx, searchPathURL); err != nil {
		fmt.Fprintln(os.Stderr, "failed to sync schema:", err)
		os.Exit(1)
	}

	code := m.Run()

	Close()

	if err := dropTestSchema(ctx, baseDatabaseURL, testSchemaName); err != nil {
		fmt.Fprintln(os.Stderr, "failed to drop test schema:", err)
	}

	if code != 0 {
		os.Exit(code)
	}

	os.Exit(0)
}

func initTestPool(ctx context.Context, databaseURL string, schemaName string) error {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database url: %w", err)
	}

	if config.ConnConfig.RuntimeParams == nil {
		config.ConnConfig.RuntimeParams = map[string]string{}
	}

	config.ConnConfig.RuntimeParams["search_path"] = schemaName + ",public"

	config.MaxConns = 5
	config.MinConns = 1

	pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create test pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

func syncSchemaForTest(ctx context.Context, databaseURL string) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			logger.Warn("Failed to close migration connection", "error", err)
		}
	}()

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err := SyncReferenceRanges(ctx); err != nil {
		return fmt.Errorf("failed to sync reference ranges: %w", err)
	}

	return nil
}

func withSearchPath(databaseURL string, schemaName string) (string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse database url: %w", err)
	}

	query := parsed.Query()
	query.Set("options", fmt.Sprintf("-c search_path=%s,public", schemaName))
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func createTestSchema(ctx context.Context, databaseURL string, schemaName string) error {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database url: %w", err)
	}

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	defer func() {
		if err := conn.Close(ctx); err != nil {
			logger.Warn("Failed to close schema connection", "error", err)
		}
	}()

	query := "CREATE SCHEMA " + pgx.Identifier{schemaName}.Sanitize()
	if _, err := conn.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func dropTestSchema(ctx context.Context, databaseURL string, schemaName string) error {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database url: %w", err)
	}

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	defer func() {
		if err := conn.Close(ctx); err != nil {
			logger.Warn("Failed to close schema connection", "error", err)
		}
	}()

	query := fmt.Sprintf("DROP SCHEMA %s CASCADE", pgx.Identifier{schemaName}.Sanitize())
	if _, err := conn.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to drop schema: %w", err)
	}

	return nil
}

func resetDatabase(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	if err := truncateSchema(ctx); err != nil {
		t.Fatalf("failed to truncate schema: %v", err)
	}

	resetZettelkastenCaches()
}

func truncateSchema(ctx context.Context) error {
	if pool == nil {
		return ErrDatabaseConnectionNotInitialized
	}

	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = $1`, testSchemaName)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var targets []string

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return fmt.Errorf("failed to scan table: %w", err)
		}

		if table == "goose_db_version" {
			continue
		}

		targets = append(targets, pgx.Identifier{testSchemaName, table}.Sanitize())
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating tables: %w", err)
	}

	if len(targets) == 0 {
		return nil
	}

	query := fmt.Sprintf("TRUNCATE %s RESTART IDENTITY CASCADE", strings.Join(targets, ", "))
	if _, err := pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to truncate tables: %w", err)
	}

	return nil
}

func resetZettelkastenCaches() {
	cacheMutex.Lock()

	idToFilenameCache = make(map[string]string)

	cacheMutex.Unlock()

	backlinkMutex.Lock()

	backlinkCache = make(map[string][]string)
	forwardLinkCache = make(map[string][]string)
	publicNoteCache = make(map[string]bool)
	lastCacheBuild = time.Time{}

	backlinkMutex.Unlock()

	journalMutex.Lock()

	journalCache = make(map[string]JournalEntry)
	lastJournalBuild = time.Time{}

	journalMutex.Unlock()

	zkNoteMutex.Lock()

	zkNoteCache = make(map[string][]ZKTimelineNote)
	lastZKNoteBuild = time.Time{}

	zkNoteMutex.Unlock()
}
