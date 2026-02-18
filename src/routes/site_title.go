/*
 * Copyright 2026 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"os"
	"strings"

	"github.com/flamego/template"
)

const (
	defaultSiteTitle      = "Groundwave"
	publicSiteTitleEnvVar = "PUBLIC_SITE_TITLE"
	proofOfWorkPageTitle  = "Verifying Browser"
)

func setPublicSiteTitle(data template.Data) {
	title := strings.TrimSpace(os.Getenv(publicSiteTitleEnvVar))
	if title == "" {
		title = defaultSiteTitle
	}

	data["PageTitle"] = title
}

func setProofOfWorkPageTitle(data template.Data) {
	data["PageTitle"] = proofOfWorkPageTitle
}
