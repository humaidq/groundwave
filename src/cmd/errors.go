/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package cmd

import "errors"

var (
	errDatabaseURLRequired   = errors.New("database-url is required (set via --database-url or DATABASE_URL env var)")
	errMigrationNameRequired = errors.New("migration name is required")
	errCSRFSecretRequired    = errors.New("CSRF_SECRET is required")
	errInvalidRuntimeEnv     = errors.New(runtimeEnvVar + " must be one of: development, dev, production, prod")
)
