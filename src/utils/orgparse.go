/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package utils

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/niklasfasching/go-org/org"
)

// ParseOrgToHTML converts org-mode content to HTML
func ParseOrgToHTML(content string) (string, error) {
	config := org.New()

	config.DefaultSettings["TODO"] = "TODO PROJ STRT WAIT HOLD | DONE KILL"

	// Custom link resolver for org-roam ID links
	config.ResolveLink = func(protocol string, description []org.Node, link string) org.Node {
		// Handle id: protocol links (org-roam)
		if protocol == "id" {
			// Strip "id:" prefix if it's included in the link parameter
			// (some org parsers include the protocol in the link)
			cleanLink := strings.TrimPrefix(link, "id:")

			// Convert [[id:uuid][Title]] to /zk/uuid
			return org.RegularLink{
				Protocol:    "",
				Description: description,
				URL:         fmt.Sprintf("/zk/%s", cleanLink),
			}
		}
		// Return nil for default handling of other protocols
		return nil
	}

	// Parse the org-mode content
	doc := config.Parse(strings.NewReader(content), "")
	if doc.Error != nil {
		return "", fmt.Errorf("failed to parse org-mode content: %w", doc.Error)
	}

	// Render to HTML
	writer := org.NewHTMLWriter()
	writer.HighlightCodeBlock = func(source, lang string, inline bool, params map[string]string) string {
		// Simple code block rendering without syntax highlighting
		if inline {
			return `<code class="inline-code">` + html.EscapeString(source) + `</code>`
		}
		return `<pre><code class="code-block">` + html.EscapeString(source) + `</code></pre>`
	}

	renderedHTML, err := doc.Write(writer)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	return renderedHTML, nil
}

// ExtractIDProperty extracts the :ID: property from org-mode content
// Org-roam files typically have a properties block like:
//
//	:PROPERTIES:
//	:ID:       075915aa-f7b9-499c-9858-8167d6b1e11b
//	:END:
func ExtractIDProperty(content string) (string, error) {
	// Match :ID: followed by whitespace and a UUID
	re := regexp.MustCompile(`(?i):ID:\s+([a-f0-9\-]+)`)
	matches := re.FindStringSubmatch(content)

	if len(matches) < 2 {
		return "", fmt.Errorf("no ID property found in content")
	}

	return strings.TrimSpace(matches[1]), nil
}

// ExtractTitle extracts the title from org-mode content
// Tries #+TITLE: first, then falls back to the first headline
func ExtractTitle(content string) string {
	// Try to find #+TITLE: directive (case-insensitive)
	reTitleDirective := regexp.MustCompile(`(?i)^\s*#\+TITLE:\s+(.+)$`)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if matches := reTitleDirective.FindStringSubmatch(line); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	// Fallback: Find first headline (starts with one or more *)
	reHeadline := regexp.MustCompile(`(?m)^\*+\s+(.+)$`)
	if matches := reHeadline.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// If no title or headline found, return default
	return "Untitled Note"
}

// ValidateUUID checks if a string is a valid UUID format
// This prevents directory traversal attacks and malformed input
func ValidateUUID(id string) error {
	// UUIDs can be lowercase hex digits and hyphens
	// Example: 075915aa-f7b9-499c-9858-8167d6b1e11b
	re := regexp.MustCompile(`^[a-f0-9\-]+$`)

	if !re.MatchString(id) {
		return fmt.Errorf("invalid UUID format: %s", id)
	}

	if len(id) < 10 || len(id) > 100 {
		return fmt.Errorf("UUID length out of bounds: %s", id)
	}

	return nil
}
