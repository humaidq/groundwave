// SPDX-FileCopyrightText: 2026 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"testing"

	"github.com/flamego/template"
)

func TestSetPublicSiteTitleUsesEnvironmentValue(t *testing.T) {
	t.Setenv(publicSiteTitleEnvVar, "  Public Logbook  ")

	data := template.Data{}
	setPublicSiteTitle(data)

	title, _ := data["PageTitle"].(string)
	if title != "Public Logbook" {
		t.Fatalf("expected public site title from environment, got %q", title)
	}
}

func TestSetPublicSiteTitleFallsBackToDefault(t *testing.T) {
	t.Setenv(publicSiteTitleEnvVar, "   ")

	data := template.Data{}
	setPublicSiteTitle(data)

	title, _ := data["PageTitle"].(string)
	if title != defaultSiteTitle {
		t.Fatalf("expected default site title %q, got %q", defaultSiteTitle, title)
	}
}

func TestSetProofOfWorkPageTitle(t *testing.T) {
	t.Parallel()

	data := template.Data{}
	setProofOfWorkPageTitle(data)

	title, _ := data["PageTitle"].(string)
	if title != proofOfWorkPageTitle {
		t.Fatalf("expected proof-of-work title %q, got %q", proofOfWorkPageTitle, title)
	}
}
