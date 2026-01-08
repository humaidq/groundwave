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

The following environment variables are required or optional for running the application:

### Required

- `DATABASE_URL` - PostgreSQL connection string
  - Format: `postgres://user:password@host:port/dbname`
  - Example: `postgres:///groundwave` (Unix socket)

### Optional - CardDAV Integration

To enable CardDAV contact synchronization, set the following environment variables:

- `CARDDAV_URL` - URL of the CardDAV server
- `CARDDAV_USERNAME` - Username for CardDAV authentication
- `CARDDAV_PASSWORD` - Password for CardDAV authentication
