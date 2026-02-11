/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package whatsapp

import (
	"regexp"
	"strings"
)

var nonDigitRegex = regexp.MustCompile(`[^\d]`)

// NormalizePhone normalizes a phone number for comparison.
// Removes all non-digit characters.
func NormalizePhone(phone string) string {
	return nonDigitRegex.ReplaceAllString(phone, "")
}

// JIDToPhone extracts the phone number from a WhatsApp JID.
// JID format: 1234567890@s.whatsapp.net
func JIDToPhone(jid string) string {
	parts := strings.Split(jid, "@")
	if len(parts) > 0 {
		return parts[0]
	}

	return jid
}

// PhoneMatches checks if two phone numbers match.
// Handles different country code formats by checking suffix matching.
func PhoneMatches(phone1, phone2 string) bool {
	n1 := NormalizePhone(phone1)
	n2 := NormalizePhone(phone2)

	if n1 == "" || n2 == "" {
		return false
	}

	// Check exact match first
	if n1 == n2 {
		return true
	}

	// Check if one is a suffix of the other (handles country code differences)
	// E.g., "1234567890" matches "11234567890" (US country code)
	minLen := len(n1)
	if len(n2) < minLen {
		minLen = len(n2)
	}

	// Ensure the suffix is at least 7 digits to avoid false positives
	if minLen >= 7 {
		if strings.HasSuffix(n1, n2) || strings.HasSuffix(n2, n1) {
			return true
		}
	}

	return false
}
