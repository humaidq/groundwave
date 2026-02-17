/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import "strings"

type parsedSearchQuery struct {
	freeTerms     []string
	tagNames      []string
	tiers         []Tier
	callSignExact []string
	callSignLike  []string
	bands         []string
	categories    []string
	hasTargets    []string
	missing       []string
}

func parseSearchQuery(raw string) parsedSearchQuery {
	parsed := parsedSearchQuery{}

	for _, rawToken := range strings.Fields(strings.TrimSpace(raw)) {
		token := strings.TrimSpace(rawToken)
		if token == "" {
			continue
		}

		isNegated := strings.HasPrefix(token, "-")
		if isNegated {
			token = strings.TrimSpace(strings.TrimPrefix(token, "-"))
			if token == "" {
				continue
			}
		}

		key, value, hasOperator := strings.Cut(token, ":")
		if !hasOperator {
			parsed.freeTerms = append(parsed.freeTerms, token)
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))

		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			parsed.freeTerms = append(parsed.freeTerms, token)
			continue
		}

		switch key {
		case "tag":
			if isNegated {
				continue
			}

			parsed.tagNames = appendUniqueString(parsed.tagNames, strings.ToLower(value))
		case "tier":
			if isNegated {
				continue
			}

			tier := Tier(strings.ToUpper(value))
			switch tier {
			case TierA, TierB, TierC, TierD, TierE, TierF:
				parsed.tiers = appendUniqueTier(parsed.tiers, tier)
			}
		case "callsign":
			normalized := strings.ToUpper(strings.TrimSpace(value))
			if normalized == "" {
				continue
			}

			if strings.Contains(normalized, "*") {
				parsed.callSignLike = appendUniqueString(parsed.callSignLike, sqlLikePatternFromWildcard(normalized))
			} else {
				parsed.callSignExact = appendUniqueString(parsed.callSignExact, normalized)
			}
		case "band":
			if isNegated {
				continue
			}

			parsed.bands = appendUniqueString(parsed.bands, strings.ToLower(value))
		case "category", "type":
			if isNegated {
				continue
			}

			parsed.categories = appendUniqueString(parsed.categories, strings.ToLower(strings.Join(strings.Fields(value), " ")))
		case "has":
			target := strings.ToLower(value)
			if isNegated {
				parsed.missing = appendUniqueString(parsed.missing, target)
			} else {
				parsed.hasTargets = appendUniqueString(parsed.hasTargets, target)
			}
		default:
			parsed.freeTerms = append(parsed.freeTerms, token)
		}
	}

	return parsed
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}

func appendUniqueTier(values []Tier, value Tier) []Tier {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}

	return append(values, value)
}

func sqlLikePatternFromWildcard(value string) string {
	return strings.ReplaceAll(value, "*", "%")
}
