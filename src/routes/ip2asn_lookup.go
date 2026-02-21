/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"
)

var (
	errIPASNMissingColumns = errors.New("ip2asn TSV row has fewer than 3 columns")
	errIPASNFamilyMismatch = errors.New("ip2asn TSV row has mismatched IP families")
	errIPASNRangeOrder     = errors.New("ip2asn TSV row start IP exceeds end IP")
	errInvalidCountryCode  = errors.New("invalid country code")
)

type ipASNRangeV4 struct {
	start   uint32
	end     uint32
	asn     uint32
	country string
}

type ipASNRangeV6 struct {
	start   [16]byte
	end     [16]byte
	asn     uint32
	country string
}

type ipASNTable struct {
	v4 []ipASNRangeV4
	v6 []ipASNRangeV6
}

// NewEmbeddedIPASNResolver builds an in-memory ASN resolver from embedded TSV data.
func NewEmbeddedIPASNResolver() (ClientASNResolver, error) {
	file, err := embeddedIPASNData.Open(embeddedIPASNFilename)
	if err != nil {
		return nil, fmt.Errorf("open embedded ip2asn file: %w", err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Error("Failed to close embedded ip2asn file", "error", closeErr)
		}
	}()

	table, err := parseIPASNTSV(file)
	if err != nil {
		return nil, fmt.Errorf("parse embedded ip2asn file: %w", err)
	}

	return func(request *http.Request) (uint32, string, bool) {
		addr, ok := clientIPAddr(request)
		if !ok {
			return 0, "", false
		}

		return table.lookup(addr)
	}, nil
}

// ParseASNSet parses a comma-separated ASN list into a set.
func ParseASNSet(raw string) (map[uint32]struct{}, error) {
	set := make(map[uint32]struct{})

	for _, item := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}

		value, err := strconv.ParseUint(trimmed, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parse ASN %q: %w", trimmed, err)
		}

		set[uint32(value)] = struct{}{}
	}

	return set, nil
}

// ParseCountryCodeSet parses a comma-separated country code list into a set.
func ParseCountryCodeSet(raw string) (map[string]struct{}, error) {
	set := make(map[string]struct{})

	for _, item := range strings.Split(raw, ",") {
		code := normalizeCountryCode(item)
		if code == "" {
			continue
		}

		if len(code) != 2 {
			return nil, fmt.Errorf("%w: %q", errInvalidCountryCode, strings.TrimSpace(item))
		}

		for _, ch := range code {
			if ch < 'A' || ch > 'Z' {
				return nil, fmt.Errorf("%w: %q", errInvalidCountryCode, strings.TrimSpace(item))
			}
		}

		set[code] = struct{}{}
	}

	return set, nil
}

func parseIPASNTSV(reader io.Reader) (*ipASNTable, error) {
	table := &ipASNTable{}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)

	lineNumber := 0

	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 5)
		if len(fields) < 3 {
			return nil, fmt.Errorf("%w at line %d", errIPASNMissingColumns, lineNumber)
		}

		startAddr, err := netip.ParseAddr(strings.TrimSpace(fields[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: parse start IP %q: %w", lineNumber, fields[0], err)
		}

		endAddr, err := netip.ParseAddr(strings.TrimSpace(fields[1]))
		if err != nil {
			return nil, fmt.Errorf("line %d: parse end IP %q: %w", lineNumber, fields[1], err)
		}

		asnValue, err := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("line %d: parse ASN %q: %w", lineNumber, fields[2], err)
		}

		countryCode := ""
		if len(fields) >= 4 {
			countryCode = normalizeCountryCode(fields[3])
		}

		startAddr = startAddr.Unmap()
		endAddr = endAddr.Unmap()

		if startAddr.Is4() != endAddr.Is4() {
			return nil, fmt.Errorf("%w at line %d (%q, %q)", errIPASNFamilyMismatch, lineNumber, fields[0], fields[1])
		}

		if startAddr.Compare(endAddr) > 0 {
			return nil, fmt.Errorf("%w at line %d (%q, %q)", errIPASNRangeOrder, lineNumber, fields[0], fields[1])
		}

		if startAddr.Is4() {
			startBytes := startAddr.As4()
			endBytes := endAddr.As4()

			table.v4 = append(table.v4, ipASNRangeV4{
				start:   bytesToUint32(startBytes),
				end:     bytesToUint32(endBytes),
				asn:     uint32(asnValue),
				country: countryCode,
			})

			continue
		}

		table.v6 = append(table.v6, ipASNRangeV6{
			start:   startAddr.As16(),
			end:     endAddr.As16(),
			asn:     uint32(asnValue),
			country: countryCode,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ip2asn TSV: %w", err)
	}

	sort.Slice(table.v4, func(i, j int) bool {
		return table.v4[i].start < table.v4[j].start
	})

	sort.Slice(table.v6, func(i, j int) bool {
		return compareBytes16(table.v6[i].start, table.v6[j].start) < 0
	})

	return table, nil
}

func (table *ipASNTable) lookup(addr netip.Addr) (uint32, string, bool) {
	if !addr.IsValid() {
		return 0, "", false
	}

	addr = addr.Unmap()

	if addr.Is4() {
		value := bytesToUint32(addr.As4())

		index := sort.Search(len(table.v4), func(i int) bool {
			return table.v4[i].start > value
		}) - 1

		if index < 0 {
			return 0, "", false
		}

		if value > table.v4[index].end {
			return 0, "", false
		}

		return table.v4[index].asn, table.v4[index].country, true
	}

	if !addr.Is6() {
		return 0, "", false
	}

	value := addr.As16()

	index := sort.Search(len(table.v6), func(i int) bool {
		return compareBytes16(table.v6[i].start, value) > 0
	}) - 1

	if index < 0 {
		return 0, "", false
	}

	if compareBytes16(value, table.v6[index].end) > 0 {
		return 0, "", false
	}

	return table.v6[index].asn, table.v6[index].country, true
}

func normalizeCountryCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func clientIPAddr(request *http.Request) (netip.Addr, bool) {
	if request == nil {
		return netip.Addr{}, false
	}

	rawIP := strings.TrimSpace(clientIPFromHTTPRequest(request))
	if rawIP == "" {
		return netip.Addr{}, false
	}

	if addr, err := netip.ParseAddr(rawIP); err == nil {
		return addr, true
	}

	if host, _, err := net.SplitHostPort(rawIP); err == nil {
		if addr, parseErr := netip.ParseAddr(strings.TrimSpace(host)); parseErr == nil {
			return addr, true
		}
	}

	return netip.Addr{}, false
}

func clientIPFromHTTPRequest(request *http.Request) string {
	if request == nil {
		return ""
	}

	forwardedFor := strings.TrimSpace(request.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		if idx := strings.Index(forwardedFor, ","); idx != -1 {
			forwardedFor = forwardedFor[:idx]
		}

		if ip := strings.TrimSpace(forwardedFor); ip != "" {
			return ip
		}
	}

	realIP := strings.TrimSpace(request.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}

	if host, _, err := net.SplitHostPort(strings.TrimSpace(request.RemoteAddr)); err == nil {
		return host
	}

	return strings.TrimSpace(request.RemoteAddr)
}

func bytesToUint32(value [4]byte) uint32 {
	return uint32(value[0])<<24 |
		uint32(value[1])<<16 |
		uint32(value[2])<<8 |
		uint32(value[3])
}

func compareBytes16(left, right [16]byte) int {
	return bytes.Compare(left[:], right[:])
}
