// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"strings"
	"testing"

	"net/netip"
)

func TestParseASNSet(t *testing.T) {
	t.Parallel()

	set, err := ParseASNSet("13335, 15169, 13335, 0")
	if err != nil {
		t.Fatalf("ParseASNSet returned error: %v", err)
	}

	if len(set) != 3 {
		t.Fatalf("unexpected ASN set size: got %d, want %d", len(set), 3)
	}

	if _, ok := set[13335]; !ok {
		t.Fatal("expected ASN 13335 in set")
	}

	if _, ok := set[15169]; !ok {
		t.Fatal("expected ASN 15169 in set")
	}

	if _, ok := set[0]; !ok {
		t.Fatal("expected ASN 0 in set")
	}
}

func TestParseASNSetRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	if _, err := ParseASNSet("13335,not-a-number"); err == nil {
		t.Fatal("expected ParseASNSet to reject invalid ASN")
	}
}

func TestParseCountryCodeSet(t *testing.T) {
	t.Parallel()

	set, err := ParseCountryCodeSet("cn, RU, us")
	if err != nil {
		t.Fatalf("ParseCountryCodeSet returned error: %v", err)
	}

	if len(set) != 3 {
		t.Fatalf("unexpected country set size: got %d, want %d", len(set), 3)
	}

	if _, ok := set["CN"]; !ok {
		t.Fatal("expected country CN in set")
	}

	if _, ok := set["RU"]; !ok {
		t.Fatal("expected country RU in set")
	}

	if _, ok := set["US"]; !ok {
		t.Fatal("expected country US in set")
	}
}

func TestParseCountryCodeSetRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	if _, err := ParseCountryCodeSet("CN,XYZ"); err == nil {
		t.Fatal("expected ParseCountryCodeSet to reject invalid country code")
	}
}

func TestParseIPASNTSVAndLookup(t *testing.T) {
	t.Parallel()

	data := strings.Join([]string{
		"1.1.1.0\t1.1.1.255\t13335\tUS\tCLOUDFLARE",
		"8.8.8.0\t8.8.8.255\t15169\tUS\tGOOGLE",
		"2001:db8::\t2001:db8::ffff\t64500\tZZ\tDOC",
	}, "\n")

	table, err := parseIPASNTSV(strings.NewReader(data))
	if err != nil {
		t.Fatalf("parseIPASNTSV returned error: %v", err)
	}

	tests := []struct {
		name    string
		ip      string
		asn     uint32
		country string
		ok      bool
	}{
		{name: "IPv4 in Cloudflare range", ip: "1.1.1.1", asn: 13335, country: "US", ok: true},
		{name: "IPv4 in Google range", ip: "8.8.8.8", asn: 15169, country: "US", ok: true},
		{name: "IPv6 in documentation range", ip: "2001:db8::1", asn: 64500, country: "ZZ", ok: true},
		{name: "IPv4 not found", ip: "9.9.9.9", asn: 0, country: "", ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			addr, parseErr := netip.ParseAddr(tc.ip)
			if parseErr != nil {
				t.Fatalf("failed to parse IP %q: %v", tc.ip, parseErr)
			}

			gotASN, gotCountry, gotOK := table.lookup(addr)
			if gotOK != tc.ok {
				t.Fatalf("unexpected lookup status for %q: got %v, want %v", tc.ip, gotOK, tc.ok)
			}

			if gotASN != tc.asn {
				t.Fatalf("unexpected ASN for %q: got %d, want %d", tc.ip, gotASN, tc.asn)
			}

			if gotCountry != tc.country {
				t.Fatalf("unexpected country for %q: got %q, want %q", tc.ip, gotCountry, tc.country)
			}
		})
	}
}
