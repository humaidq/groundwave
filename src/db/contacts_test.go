// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import "testing"

func TestNormalizeLinkedInURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{
			name:  "https with www",
			input: "https://www.linkedin.com/in/SomeUser/",
			want:  "https://linkedin.com/in/someuser",
			ok:    true,
		},
		{
			name:  "no scheme",
			input: "linkedin.com/in/another",
			want:  "https://linkedin.com/in/another",
			ok:    true,
		},
		{
			name:  "www without scheme",
			input: "  www.linkedin.com/in/UserName  ",
			want:  "https://linkedin.com/in/username",
			ok:    true,
		},
		{
			name:  "missing username",
			input: "https://linkedin.com/in/",
			want:  "",
			ok:    false,
		},
		{
			name:  "wrong path",
			input: "https://linkedin.com/company/acme",
			want:  "",
			ok:    false,
		},
		{
			name:  "wrong host",
			input: "https://notlinkedin.com/in/foo",
			want:  "",
			ok:    false,
		},
		{
			name:  "extra segments",
			input: "linkedin.com/in/foo/bar",
			want:  "https://linkedin.com/in/foo",
			ok:    true,
		},
		{
			name:  "empty",
			input: "",
			want:  "",
			ok:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := NormalizeLinkedInURL(tc.input)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v", tc.ok, ok)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
