<!--
SPDX-FileCopyrightText: 2025 Humaid Alqasimi
SPDX-License-Identifier: Apache-2.0
-->

# AGENTS.md

This file guides coding agents working in this repository.
Follow these conventions when editing any file in this repo.

See FEATURES.md file for the big picture.

## Repository Context

- Project name: Groundwave (personal CRM + ham radio logging)
- Language: Go 1.24+ (module in `src/`)
- Web framework: Flamego
- Database: PostgreSQL (pgx + pgxpool)
- Migrations: Goose (embedded in binary)
- Templates/static: Embedded in binary

## Build / Run Commands

Prefer Nix-based commands when available.

- Enter dev shell (recommended):
  - `nix develop`
- Run the app (auto-migrations):
  - `nix run`
- Build package (use these flags to avoid cache timeouts):
  - `nix build --option substituters '' --option builders ''`
- Local build (only if needed):
  - `cd src && go build -o ../bin/groundwave`
- Local run (from Go module):
  - `cd src && go run .`
- Start web server (built binary):
  - `./groundwave start --database-url "postgres://user:pass@localhost:5432/groundwave"`

## Test Commands

Tests live in the Go module (`src/`).

- Run all tests:
  - `cd src && go test ./...`
- Run a single package:
  - `cd src && go test ./utils`
- Run a single test:
  - `cd src && go test ./utils -run TestCreateGridMap`

Notes:
- `TestCreateGridMap*` writes files in the repo root and cleans them up.
- Provide write permissions to the working directory when running tests.

## Lint / Format

There is no repo-wide lint config (no golangci-lint config in root).
Use standard Go tools.

- Format all Go files:
  - `cd src && gofmt -w $(rg --files -g "*.go")`
- Basic static checks (optional):
  - `cd src && go vet ./...`

## Migrations

Migrations live in `src/db/migrations/` and are embedded in `src/db/schema.go`.

- Create migration (development only):
  - `./groundwave migrate create add_new_feature`
- Run migrations manually:
  - `./groundwave migrate up --database-url "postgres://..."`

Caveat:
- SQL blocks using dollar-quoted strings (`DO $$ ... $$`) must be wrapped with:
  - `-- +goose StatementBegin`
  - `-- +goose StatementEnd`

## Code Style Guidelines (Go)

### Formatting

- Run `gofmt` before committing.
- Use tabs for indentation (default Go formatting).
- Keep line lengths reasonable; wrap long literals.

### Imports

- Group imports as:
  1. Standard library
  2. Third-party dependencies
  3. Local module (`github.com/humaidq/groundwave/...`)
- Separate groups with blank lines.
- Use import aliases only when needed (e.g., `flamegoTemplate`).

### Packages and Naming

- Package names are short and lower-case (`cmd`, `db`, `routes`, `utils`).
- Exported identifiers use `PascalCase`.
- Unexported identifiers use `camelCase`.
- Avoid one-letter names except for small scopes (`i`, `err`).
- Boolean variables read as predicates (`isService`, `hasCardDAV`).

### Types and Structs

- Use typed structs for form input and database operations.
- Use pointers for optional fields to distinguish empty vs missing values.
- Keep struct field ordering consistent with DB schema or form structure.

### Error Handling

- Wrap errors with `%w` using `fmt.Errorf`.
- Return early on errors; avoid deep nesting.
- Log non-fatal failures and continue when safe (see routes handlers).
- Avoid `panic` except for truly unrecoverable boot failures.

### Logging

- Use `log.Printf`/`log.Println` for runtime logging.
- Log errors with enough context (IDs, input values).
- Avoid logging secrets (DB URLs, credentials).

### Context Usage

- Pass `context.Context` explicitly into DB calls.
- In handlers, prefer `c.Request().Context()`.
- In background tasks, use `context.Background()` or explicit timeouts.

### Database Access

- Use `db.GetPool()` to access `pgxpool`.
- All DB calls should accept `context.Context`.
- Use `gen_random_uuid()` in migrations for UUIDs.
- Keep migration files immutable once applied.

### HTTP Handlers (routes)

- Keep handlers thin; delegate to `db/` helpers.
- Parse forms with `c.Request().ParseForm()`.
- Use `template.Data` for view data and `template.Template` for rendering.
- On user-facing errors, set flash messages and redirect.

### Templates and Static Files

- Templates live in `src/templates/` and are embedded.
- Use `.html` templates and `template.Data` map keys.
- Static assets are embedded in `src/static/`.

### Testing Style

- Use `testing` package and table tests where helpful.
- Use `t.Fatalf` for fatal assertions.
- Clean up any files created by tests.

## No External Editor Rules Detected

- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md` were found.
