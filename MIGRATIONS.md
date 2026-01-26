<!--
SPDX-FileCopyrightText: 2025 Humaid Alqasimi
SPDX-License-Identifier: Apache-2.0
-->

# Database Migrations

This project uses [goose](https://github.com/pressly/goose) for database migrations with **embedded migrations** for production deployment.

## Key Features

✅ **Automatic migrations on startup** - The `start` command runs pending migrations automatically
✅ **Embedded in binary** - All migrations are compiled into the binary, no source files needed in production
✅ **Version controlled** - Each schema change is tracked as a separate migration file
✅ **Rollback support** - Can undo migrations if needed (via CLI)
✅ **Production ready** - Works in packaged deployments without access to source code

## How It Works in Production

When you deploy a new version of the application:

1. **Deploy the binary**: Copy just the `groundwave` binary to your server

2. **Start the application**: Migrations run automatically
   ```bash
   ./groundwave start --database-url "postgres://user:pass@host/db"
   ```

The application will:
- Connect to the database
- Check which migrations have been applied
- Run any pending migrations from the embedded files
- Start the web server

**No source code or migration files needed on the server!** Everything is embedded in the binary.

## Development Workflow

### Creating a New Migration

When you need to change the database schema:

```bash
# Create a new migration file
./groundwave migrate create add_user_preferences

# This creates: src/db/migrations/YYYYMMDDHHMMSS_add_user_preferences.sql
```

Edit the generated file:

```sql
-- +goose Up
ALTER TABLE contacts ADD COLUMN preferences JSONB;

-- +goose Down
ALTER TABLE contacts DROP COLUMN preferences;
```

### Testing Migrations Locally

```bash
# Set your DATABASE_URL
export DATABASE_URL="postgres://user:pass@localhost/mydb"

# Run migrations
./groundwave migrate up

# Check status
./groundwave migrate status

# Check version
./groundwave migrate version

# Rollback last migration (if needed)
./groundwave migrate down
```

### Deploying Changes

1. **Create migration** in development
2. **Test locally** with `migrate up`
3. **Commit** the migration file to git
4. **Deploy** to server
5. **Restart application**: Migrations run automatically on startup

## Migration Files Location

All migrations are stored in:
```
src/db/migrations/
├── 00001_initial_schema.sql
├── 00002_add_user_preferences.sql
└── ...
```

These files are embedded into the binary using Go's `//go:embed` directive in `src/db/schema.go`.

## CLI Commands Reference

### `migrate up`
Run all pending migrations
```bash
./groundwave migrate up --database-url "postgres://..."
```

### `migrate down`
Roll back the most recent migration
```bash
./groundwave migrate down --database-url "postgres://..."
```

### `migrate status`
Show which migrations have been applied
```bash
./groundwave migrate status --database-url "postgres://..."
```

### `migrate version`
Print the current database version
```bash
./groundwave migrate version --database-url "postgres://..."
```

### `migrate create <name>`
Create a new migration file (development only)
```bash
./groundwave migrate create add_new_feature
```
**Note**: This requires source code access and is for development only. Run from the `src/` directory.

## goose_db_version Table

Goose automatically creates a `goose_db_version` table to track which migrations have been applied:

```sql
SELECT * FROM goose_db_version;
```

This ensures migrations are only run once and in the correct order.

## Best Practices

1. **Never edit applied migrations** - Create a new migration instead
2. **Test rollbacks** - Make sure your `-- +goose Down` works
3. **Keep migrations small** - One logical change per migration
4. **Name descriptively** - Use clear names like `add_user_email_index`

## Troubleshooting

### "Migration already applied"
The migration has already run. Check with `migrate status`.

### "Failed to run migrations" in production
Check database connectivity and permissions. Review logs for specific error details.

### Manual migration needed
If you need to run SQL manually:
```bash
psql $DATABASE_URL -f manual_fix.sql
# Then mark as applied if needed
```

## Example: Adding a New Feature

```bash
# 1. Create migration
./groundwave migrate create add_contact_tags

# 2. Edit src/db/migrations/XXXXX_add_contact_tags.sql
#    Add your schema changes

# 3. Test locally
./groundwave migrate up

# 4. Update Go code to use new schema
#    Edit src/db/models.go, src/db/contacts.go, etc.

# 5. Test the application
./groundwave start

# 6. Deploy to production
#    Migrations will run automatically on startup!
```

## Automatic Startup Migrations

The `start` command automatically runs migrations via `db.SyncSchema()`:

```go
// In cmd/web.go
if err := db.SyncSchema(ctx); err != nil {
    return fmt.Errorf("failed to sync schema: %w", err)
}
```

This ensures your database is always up-to-date when the application starts.
