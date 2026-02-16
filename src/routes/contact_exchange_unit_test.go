// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"bytes"
	"testing"

	"github.com/emersion/go-vcard"

	"github.com/humaidq/groundwave/db"
)

func testStringPointer(value string) *string {
	return &value
}

func TestContactExchangeGreetingName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		contact *db.ContactDetail
		want    string
	}{
		{
			name: "uses first word of first name and full last name",
			contact: &db.ContactDetail{Contact: db.Contact{
				NameGiven:   testStringPointer("John Alex"),
				NameFamily:  testStringPointer("St Smith"),
				NameDisplay: "John Alex St Smith",
			}},
			want: "John St Smith",
		},
		{
			name: "trims and collapses first name whitespace",
			contact: &db.ContactDetail{Contact: db.Contact{
				NameGiven:   testStringPointer("  John   Alex  "),
				NameFamily:  testStringPointer("  St Smith  "),
				NameDisplay: "John Alex St Smith",
			}},
			want: "John St Smith",
		},
		{
			name: "falls back to first name first word when no last name",
			contact: &db.ContactDetail{Contact: db.Contact{
				NameGiven:   testStringPointer("John Alex"),
				NameDisplay: "John Alex",
			}},
			want: "John",
		},
		{
			name: "uses full last name when first name missing",
			contact: &db.ContactDetail{Contact: db.Contact{
				NameFamily:  testStringPointer("St Smith"),
				NameDisplay: "St Smith",
			}},
			want: "St Smith",
		},
		{
			name: "falls back to display name when structured names missing",
			contact: &db.ContactDetail{Contact: db.Contact{
				NameDisplay: "VU2ABC",
			}},
			want: "VU2ABC",
		},
		{
			name:    "returns empty for nil contact",
			contact: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := contactExchangeGreetingName(tt.contact); got != tt.want {
				t.Fatalf("contactExchangeGreetingName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildContactVCardIncludesPhoto(t *testing.T) {
	t.Parallel()

	photoURL := "https://example.test/photo.jpg"
	contact := &db.ContactDetail{Contact: db.Contact{
		NameDisplay: "John Doe",
		PhotoURL:    &photoURL,
	}}

	vCardBytes, err := buildContactVCard(contact, "")
	if err != nil {
		t.Fatalf("buildContactVCard failed: %v", err)
	}

	card, err := vcard.NewDecoder(bytes.NewReader(vCardBytes)).Decode()
	if err != nil {
		t.Fatalf("failed to decode generated vcard: %v", err)
	}

	photoField := card.Get(vcard.FieldPhoto)
	if photoField == nil {
		t.Fatal("expected PHOTO field in generated vcard")
	}

	if photoField.Value != photoURL {
		t.Fatalf("expected PHOTO value %q, got %q", photoURL, photoField.Value)
	}

	if got := photoField.Params.Get(vcard.ParamValue); got != "uri" {
		t.Fatalf("expected PHOTO VALUE param to be uri, got %q", got)
	}
}

func TestBuildContactVCardIncludesAdditionalNote(t *testing.T) {
	t.Parallel()

	contact := &db.ContactDetail{Contact: db.Contact{NameDisplay: "John Doe"}}

	vCardBytes, err := buildContactVCard(contact, "  X University  ")
	if err != nil {
		t.Fatalf("buildContactVCard failed: %v", err)
	}

	card, err := vcard.NewDecoder(bytes.NewReader(vCardBytes)).Decode()
	if err != nil {
		t.Fatalf("failed to decode generated vcard: %v", err)
	}

	if got := card.Value(vcard.FieldNote); got != "X University" {
		t.Fatalf("expected NOTE field value %q, got %q", "X University", got)
	}
}

func TestBuildContactVCardIncludesContactURLs(t *testing.T) {
	t.Parallel()

	contact := &db.ContactDetail{
		Contact: db.Contact{NameDisplay: "John Doe"},
		URLs: []db.ContactURL{
			{URL: "https://example.test", URLType: db.URLWebsite},
			{URL: "https://github.com/johndoe", URLType: db.URLGitHub},
			{URL: "   ", URLType: db.URLOther},
		},
	}

	vCardBytes, err := buildContactVCard(contact, "")
	if err != nil {
		t.Fatalf("buildContactVCard failed: %v", err)
	}

	card, err := vcard.NewDecoder(bytes.NewReader(vCardBytes)).Decode()
	if err != nil {
		t.Fatalf("failed to decode generated vcard: %v", err)
	}

	urlFields := card[vcard.FieldURL]
	if len(urlFields) != 2 {
		t.Fatalf("expected 2 URL fields in generated vcard, got %d", len(urlFields))
	}

	if got := urlFields[0].Value; got != "https://example.test" {
		t.Fatalf("expected first URL value %q, got %q", "https://example.test", got)
	}

	if got := urlFields[0].Params.Get(vcard.ParamType); got != "website" {
		t.Fatalf("expected first URL TYPE %q, got %q", "website", got)
	}

	if got := urlFields[1].Value; got != "https://github.com/johndoe" {
		t.Fatalf("expected second URL value %q, got %q", "https://github.com/johndoe", got)
	}

	if got := urlFields[1].Params.Get(vcard.ParamType); got != "github" {
		t.Fatalf("expected second URL TYPE %q, got %q", "github", got)
	}
}
