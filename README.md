# Groundwave

## Description

A personal database. Vibe-coded mess. Contains contacts (CardDAV),
QSL logs (ADIF), Zettelkasten (org-roam over WebDAV).

## Usage

This project includes a Nix development shell, which pulls in the required
version of Go. It also includes the application as a Nix package.

To run the application:

```
nix run
```

To load development shell:

```
nix develop
```

## Environment Variables

The following environment variables are used by the application:

- `DATABASE_URL` - PostgreSQL connection string
  - Format: `postgres://user:password@host:port/dbname`
  - Example: `postgres:///groundwave` (Unix socket)
- `CSRF_SECRET` - CSRF signing secret
- `GROUNDWAVE_ENV` - Runtime mode (`development` or `production`)
  - Default: `development`
  - In `production`, auth session cookies are marked `Secure`
  - This value is also propagated to Flamego's runtime environment
- `POW_DIFFICULTY` - Proof-of-work difficulty (leading zero bits) for new sessions
  - Default: `20`
  - Valid range: `8` to `28` (values are clamped into this range)
- `AUTH_USERNAME` - Username for login
- `AUTH_PASSWORD_HASH` - Bcrypt hash for login password
- `CARDDAV_URL` - URL of the CardDAV server
- `CARDDAV_USERNAME` - Username for CardDAV authentication
- `CARDDAV_PASSWORD` - Password for CardDAV authentication
- `WEBDAV_ZK_PATH` - WebDAV URL to the Zettelkasten index `.org` file
- `WEBDAV_HOME_PATH` - WebDAV URL to the Home Wiki index `.org` file
- `WEBDAV_INV_PATH` - WebDAV base URL for inventory files
- `WEBDAV_FILES_PATH` - WebDAV base URL for file browsing
- `WEBDAV_TODO_PATH` - WebDAV URL to the todo `.org` file
- `WEBDAV_USERNAME` - Username for WebDAV authentication
- `WEBDAV_PASSWORD` - Password for WebDAV authentication
- `OLLAMA_URL` - Base URL for the Ollama server
- `OLLAMA_MODEL` - Ollama model name for AI summaries
- `QRZ_API_KEY` - QRZ Logbook API key(s) for importing latest QSOs
  - Comma separated for multiple logbooks (example: `QRZ_API_KEY=apikey1,apikey2`)
- `QRZ_XML_USERNAME` - QRZ username for XML callsign lookups
  - Optional fallback: `QRZ_USERNAME`
- `QRZ_XML_PASSWORD` - QRZ password for XML callsign lookups
  - Optional fallback: `QRZ_PASSWORD`
- `QRZ_XML_AGENT` - Optional QRZ XML API agent string shown to QRZ
  - Default: `Groundwave/1.0 (+https://huma.id)`
- `QRZ_USERAGENT` - User-Agent header for QRZ Logbook sync requests (required)
