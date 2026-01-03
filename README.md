# Groundwave

## Description

Personal CRM with Amateur Radio Logging - A contact relationship management system with QSO logging features.

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
  - Example: `postgres://groundwave:password@localhost:5432/groundwave`

### Optional - CardDAV Integration

To enable CardDAV contact synchronization, set the following environment variables:

- `CARDDAV_URL` - URL of the CardDAV server
  - Example: `https://carddav.example.com`
- `CARDDAV_USERNAME` - Username for CardDAV authentication
- `CARDDAV_PASSWORD` - Password for CardDAV authentication

When these variables are configured, you can:
- Link existing contacts to CardDAV contacts by UUID
- Create new contacts linked to CardDAV contacts
- View live CardDAV contact data alongside local contact data

