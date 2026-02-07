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
- `AUTH_USERNAME` - Username for login
- `AUTH_PASSWORD_HASH` - Bcrypt hash for login password
- `CARDDAV_URL` - URL of the CardDAV server
- `CARDDAV_USERNAME` - Username for CardDAV authentication
- `CARDDAV_PASSWORD` - Password for CardDAV authentication
- `WEBDAV_ZK_PATH` - WebDAV URL to the Zettelkasten index `.org` file
- `WEBDAV_INV_PATH` - WebDAV base URL for inventory files
- `WEBDAV_FILES_PATH` - WebDAV base URL for file browsing
- `WEBDAV_TODO_PATH` - WebDAV URL to the todo `.org` file
- `WEBDAV_USERNAME` - Username for WebDAV authentication
- `WEBDAV_PASSWORD` - Password for WebDAV authentication
- `OLLAMA_URL` - Base URL for the Ollama server
- `OLLAMA_MODEL` - Ollama model name for AI summaries
