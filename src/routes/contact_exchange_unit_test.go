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
