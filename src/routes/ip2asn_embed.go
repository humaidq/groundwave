/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import "embed"

const embeddedIPASNFilename = "ip2asn-combined.tsv"

//go:embed ip2asn-combined.tsv
var embeddedIPASNData embed.FS
