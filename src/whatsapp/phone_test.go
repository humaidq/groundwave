// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package whatsapp

import "testing"

func TestNormalizePhone(t *testing.T) {
	t.Parallel()

	if got := NormalizePhone("+1 (650) 555-0123"); got != "16505550123" {
		t.Fatalf("NormalizePhone returned %q", got)
	}

	if got := NormalizePhone("abc"); got != "" {
		t.Fatalf("expected empty normalized phone, got %q", got)
	}
}

func TestJIDToPhone(t *testing.T) {
	t.Parallel()

	if got := JIDToPhone("1234567890@s.whatsapp.net"); got != "1234567890" {
		t.Fatalf("unexpected JID phone: %q", got)
	}

	if got := JIDToPhone("no-at-sign"); got != "no-at-sign" {
		t.Fatalf("unexpected fallback JID value: %q", got)
	}
}

func TestPhoneMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "exact match", a: "+1 650-555-0123", b: "16505550123", want: true},
		{name: "suffix match with country code", a: "6505550123", b: "16505550123", want: true},
		{name: "different numbers", a: "6505550123", b: "442079460958", want: false},
		{name: "too short suffix not allowed", a: "123456", b: "00123456", want: false},
		{name: "empty first", a: "", b: "123", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := PhoneMatches(tt.a, tt.b); got != tt.want {
				t.Fatalf("PhoneMatches(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
