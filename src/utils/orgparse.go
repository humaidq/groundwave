/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package utils

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/niklasfasching/go-org/org"
	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var newOrgConfig = org.New

var parseOrg = func(config *org.Configuration, reader io.Reader) *org.Document {
	return config.Parse(reader, "")
}

var newHTMLWriter = org.NewHTMLWriter

var writeOrg = func(doc *org.Document, writer *org.HTMLWriter) (string, error) {
	return doc.Write(writer)
}

var parseHTMLFragment = nethtml.ParseFragment

var renderHTML = nethtml.Render

// ParseOrgToHTML converts org-mode content to HTML
func ParseOrgToHTML(content string) (string, error) {
	return ParseOrgToHTMLWithBasePath(content, "/zk")
}

// ParseOrgToHTMLWithBasePath converts org-mode content to HTML using a base path for id links.
func ParseOrgToHTMLWithBasePath(content string, basePath string) (string, error) {
	config := newOrgConfig()

	config.DefaultSettings["TODO"] = "TODO PROJ STRT WAIT HOLD | DONE KILL"

	trimmedBase := strings.TrimRight(strings.TrimSpace(basePath), "/")
	if trimmedBase == "" {
		trimmedBase = "/zk"
	}

	// Custom link resolver for org-roam ID links
	config.ResolveLink = func(protocol string, description []org.Node, link string) org.Node {
		if protocol == "id" {
			cleanLink := strings.TrimPrefix(link, "id:")
			return org.RegularLink{
				Protocol:    "",
				Description: description,
				URL:         fmt.Sprintf("%s/%s", trimmedBase, cleanLink),
			}
		}
		return org.RegularLink{
			Protocol:    protocol,
			Description: description,
			URL:         link,
		}
	}

	// Parse the org-mode content
	doc := parseOrg(config, strings.NewReader(content))
	if doc.Error != nil {
		return "", fmt.Errorf("failed to parse org-mode content: %w", doc.Error)
	}

	// Render to HTML
	writer := newHTMLWriter()
	writer.HighlightCodeBlock = func(source, lang string, inline bool, params map[string]string) string {
		if inline {
			return `<code class="inline-code">` + html.EscapeString(source) + `</code>`
		}
		return `<pre><code class="code-block">` + html.EscapeString(source) + `</code></pre>`
	}

	renderedHTML, err := writeOrg(doc, writer)
	if err != nil {
		return "", fmt.Errorf("failed to render HTML: %w", err)
	}

	annotatedHTML, err := addExternalLinkPrefix(renderedHTML)
	if err != nil {
		return "", fmt.Errorf("failed to annotate external links: %w", err)
	}

	return annotatedHTML, nil
}

var internalLinkPrefixes = []string{"/zk", "/note", "/groundwave"}

func addExternalLinkPrefix(htmlBody string) (string, error) {
	if strings.TrimSpace(htmlBody) == "" {
		return htmlBody, nil
	}

	container := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := parseHTMLFragment(strings.NewReader(htmlBody), container)
	if err != nil {
		return "", err
	}

	for _, node := range nodes {
		container.AppendChild(node)
	}

	annotateExternalLinks(container)

	var buffer bytes.Buffer
	for child := container.FirstChild; child != nil; child = child.NextSibling {
		if err := renderHTML(&buffer, child); err != nil {
			return "", err
		}
	}

	return buffer.String(), nil
}

func annotateExternalLinks(node *nethtml.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.ElementNode && child.Data == "a" {
			href := ""
			for _, attr := range child.Attr {
				if attr.Key == "href" {
					href = attr.Val
					break
				}
			}

			if isExternalLink(href) && !linkHasPrefix(child) {
				prefixNode := &nethtml.Node{Type: nethtml.TextNode, Data: "🗗 "}
				if child.FirstChild != nil {
					child.InsertBefore(prefixNode, child.FirstChild)
				} else {
					child.AppendChild(prefixNode)
				}
			}
		}

		annotateExternalLinks(child)
	}
}

func linkHasPrefix(link *nethtml.Node) bool {
	if link.FirstChild == nil || link.FirstChild.Type != nethtml.TextNode {
		return false
	}

	return strings.HasPrefix(link.FirstChild.Data, "🗗")
}

func isExternalLink(href string) bool {
	href = strings.TrimSpace(href)
	if href == "" {
		return false
	}

	if strings.HasPrefix(href, "#") {
		return false
	}

	for _, prefix := range internalLinkPrefixes {
		if strings.HasPrefix(href, prefix) {
			return false
		}
	}

	if isGroundwaveBaseURLLink(href) {
		return false
	}

	return true
}

func isGroundwaveBaseURLLink(href string) bool {
	baseURL := strings.TrimSpace(os.Getenv("GROUNDWAVE_BASE_URL"))
	if baseURL == "" {
		return false
	}

	trimmedBase := strings.TrimRight(baseURL, "/")
	if trimmedBase == "" {
		return false
	}

	if strings.HasPrefix(href, trimmedBase) {
		return true
	}

	parsedBase, ok := parseAbsoluteURL(trimmedBase)
	if !ok {
		return false
	}

	parsedHref, ok := parseAbsoluteURL(href)
	if !ok {
		return false
	}

	if !strings.EqualFold(parsedBase.Host, parsedHref.Host) {
		return false
	}

	basePath := strings.TrimRight(parsedBase.Path, "/")
	if basePath == "" || basePath == "/" {
		return true
	}

	return parsedHref.Path == basePath || strings.HasPrefix(parsedHref.Path, basePath+"/")
}

func parseAbsoluteURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		parsed, err = url.Parse("https://" + raw)
	}
	if err != nil || parsed.Host == "" {
		return nil, false
	}

	return parsed, true
}

// IsPublicAccess checks for #+access: public in org content.
func IsPublicAccess(content string) bool {
	re := regexp.MustCompile(`(?im)^\s*#\+access:\s*public\s*$`)
	return re.MatchString(content)
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

// ExtractDateDirective extracts a date from a #+DATE: directive.
func ExtractDateDirective(content string) (time.Time, bool) {
	re := regexp.MustCompile(`(?im)^\s*#\+DATE:\s*<?(\d{4}-\d{2}-\d{2})`)
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return time.Time{}, false
	}

	parsed, err := time.Parse("2006-01-02", matches[1])
	if err != nil {
		return time.Time{}, false
	}

	return parsed, true
}
