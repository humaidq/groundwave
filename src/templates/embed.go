/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package templates

import "embed"

// Append "**/*" if you also have template files in subdirectories
//
//go:embed *.html
var Templates embed.FS
