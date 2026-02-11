// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"testing"

	"github.com/emersion/go-vcard"
)

func TestNormalizeCardDAVPhotoVariants(t *testing.T) {
	t.Parallel()

	if normalizeCardDAVPhoto(nil) != "" {
		t.Fatalf("expected empty photo for nil field")
	}

	if normalizeCardDAVPhoto(&vcard.Field{Value: " "}) != "" {
		t.Fatalf("expected empty photo for blank value")
	}

	url := "https://example.com/photo.jpg"
	if normalizeCardDAVPhoto(&vcard.Field{Value: url}) != url {
		t.Fatalf("expected URL to be preserved")
	}

	dataURL := "data:image/png;base64,abc"
	if normalizeCardDAVPhoto(&vcard.Field{Value: dataURL}) != dataURL {
		t.Fatalf("expected data URL to be preserved")
	}
}

func TestParseDateStringInvalid(t *testing.T) {
	t.Parallel()

	if _, err := parseDateString("not-a-date"); err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestNormalizeCardDAVEmailMailto(t *testing.T) {
	t.Parallel()

	if got := normalizeCardDAVEmail("mailto:User@Example.com"); got != "user@example.com" {
		t.Fatalf("expected normalized mailto email, got %q", got)
	}
}

func TestSelectPrimaryEmailFallbacks(t *testing.T) {
	t.Parallel()

	if got := selectPrimaryEmail("missing@example.com", []string{"first@example.com"}); got != "first@example.com" {
		t.Fatalf("expected fallback to first email, got %q", got)
	}

	if got := selectPrimaryEmail("", []string{}); got != "" {
		t.Fatalf("expected empty primary email, got %q", got)
	}
}

func TestSelectPrimaryPhoneFallbacks(t *testing.T) {
	t.Parallel()

	if got := selectPrimaryPhone("999", []string{"123"}); got != "123" {
		t.Fatalf("expected fallback to first phone, got %q", got)
	}

	if got := selectPrimaryPhone("", []string{}); got != "" {
		t.Fatalf("expected empty primary phone, got %q", got)
	}
}

func TestSelectPrimaryPhoneByDigits(t *testing.T) {
	t.Parallel()

	inserted := []string{"+1 (555) 111-2222", "+971 50 123 4567"}
	if got := selectPrimaryPhoneByDigits("15551112222", inserted); got != "+1 (555) 111-2222" {
		t.Fatalf("expected formatted primary phone match, got %q", got)
	}

	if got := selectPrimaryPhoneByDigits("", inserted); got != "+1 (555) 111-2222" {
		t.Fatalf("expected fallback to first inserted phone, got %q", got)
	}
}

func TestNormalizePhoneDigits(t *testing.T) {
	t.Parallel()

	if got := normalizePhoneDigits("+1 (555) 111-2222"); got != "15551112222" {
		t.Fatalf("expected digits-only phone, got %q", got)
	}
}

func TestMediaTypeFromPhotoParams(t *testing.T) {
	t.Parallel()

	if mediaTypeFromPhotoParams(vcard.Params{vcard.ParamType: []string{"image/png"}}) != "image/png" {
		t.Fatalf("expected image/png media type")
	}

	if mediaTypeFromPhotoParams(vcard.Params{vcard.ParamType: []string{"GIF"}}) != "image/gif" {
		t.Fatalf("expected gif media type")
	}

	if mediaTypeFromPhotoParams(vcard.Params{vcard.ParamType: []string{"unknown"}}) != "" {
		t.Fatalf("expected empty media type")
	}
}
