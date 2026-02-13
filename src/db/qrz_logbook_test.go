// SPDX-License-Identifier: Apache-2.0
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"errors"
	"testing"
)

func TestQRZLogbookUserAgentHeader(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    string
		wantErr bool
	}{
		{
			name: "uses custom user agent from environment",
			env:  "Groundwave-Test/2.0",
			want: "Groundwave-Test/2.0",
		},
		{
			name:    "fails when empty",
			env:     "",
			wantErr: true,
		},
		{
			name:    "fails when whitespace",
			env:     "   ",
			wantErr: true,
		},
		{
			name: "trims surrounding whitespace",
			env:  " Groundwave-Custom/1.5 ",
			want: "Groundwave-Custom/1.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(qrzLogbookUserAgentEnvVar, tt.env)

			got, err := qrzLogbookUserAgentHeader()
			if tt.wantErr {
				if !errors.Is(err, ErrQRZUserAgentNotConfigured) {
					t.Fatalf("qrzLogbookUserAgentHeader() error = %v, want %v", err, ErrQRZUserAgentNotConfigured)
				}

				return
			}

			if err != nil {
				t.Fatalf("qrzLogbookUserAgentHeader() error = %v", err)
			}

			if got != tt.want {
				t.Fatalf("qrzLogbookUserAgentHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}
