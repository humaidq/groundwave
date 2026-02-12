// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/niklasfasching/go-org/org"
	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	errTestBoom        = errors.New("boom")
	errTestWriteFailed = errors.New("write failed")
	errTestParseFailed = errors.New("parse failed")
)

func TestParseOrgToHTML(t *testing.T) {
	content := "* Heading\nSome text"

	rendered, err := ParseOrgToHTML(content)
	if err != nil {
		t.Fatalf("ParseOrgToHTML failed: %v", err)
	}

	if !strings.Contains(rendered, "Heading") {
		t.Fatalf("expected heading in output, got %s", rendered)
	}
}

func TestParseOrgToHTMLWithBasePathResolvesIDLinks(t *testing.T) {
	t.Setenv("GROUNDWAVE_BASE_URL", "https://groundwave.example.com")

	content := "[[id:075915aa-f7b9-499c-9858-8167d6b1e11b][My Note]] [[https://example.com][Ext]]"

	rendered, err := ParseOrgToHTMLWithBasePath(content, "/notes/")
	if err != nil {
		t.Fatalf("ParseOrgToHTMLWithBasePath failed: %v", err)
	}

	if !strings.Contains(rendered, "href=\"/notes/075915aa-f7b9-499c-9858-8167d6b1e11b\"") {
		t.Fatalf("expected id link to use base path, got %s", rendered)
	}

	if !strings.Contains(rendered, "href=\"https://example.com\"") {
		t.Fatalf("expected external link to render, got %s", rendered)
	}

	if !strings.Contains(rendered, "target=\"_blank\"") {
		t.Fatalf("expected external links to open in new tab, got %s", rendered)
	}

	if !strings.Contains(rendered, "rel=\"noopener noreferrer\"") {
		t.Fatalf("expected external links to include noopener noreferrer, got %s", rendered)
	}

	if !strings.Contains(rendered, "🗗") {
		t.Fatalf("expected external link prefix to be added")
	}
}

func TestParseOrgToHTMLWithBasePathFallback(t *testing.T) {
	content := "[[id:075915aa-f7b9-499c-9858-8167d6b1e11b][My Note]]"

	rendered, err := ParseOrgToHTMLWithBasePath(content, " ")
	if err != nil {
		t.Fatalf("ParseOrgToHTMLWithBasePath failed: %v", err)
	}

	if !strings.Contains(rendered, "href=\"/zk/075915aa-f7b9-499c-9858-8167d6b1e11b\"") {
		t.Fatalf("expected default base path /zk, got %s", rendered)
	}
}

func TestParseOrgToHTMLHighlightCodeBlocks(t *testing.T) {
	content := strings.Join([]string{
		"Inline src_go{code} example",
		"#+BEGIN_SRC go",
		"fmt.Println(\"hi\")",
		"#+END_SRC",
	}, "\n")

	rendered, err := ParseOrgToHTMLWithBasePath(content, "/zk")
	if err != nil {
		t.Fatalf("ParseOrgToHTMLWithBasePath failed: %v", err)
	}

	if !strings.Contains(rendered, "inline-code") {
		t.Fatalf("expected inline-code in output, got %s", rendered)
	}

	if !strings.Contains(rendered, "code-block") {
		t.Fatalf("expected code-block in output, got %s", rendered)
	}
}

func TestAddExternalLinkPrefix(t *testing.T) {
	t.Setenv("GROUNDWAVE_BASE_URL", "https://groundwave.example.com")

	input := `<p><a href="https://example.com">Example</a> ` +
		`<a href="/zk/123">Internal</a> ` +
		`<a href="#section">Anchor</a> ` +
		`<a href="https://example.com/already">🗗 Already</a></p>`

	output, err := addExternalLinkPrefix(input)
	if err != nil {
		t.Fatalf("addExternalLinkPrefix failed: %v", err)
	}

	if !strings.Contains(output, ">🗗 Example</a>") {
		t.Fatalf("expected prefix inserted for external link, got %s", output)
	}

	if !strings.Contains(output, `href="https://example.com" target="_blank" rel="noopener noreferrer"`) {
		t.Fatalf("expected external links to include target and rel attributes, got %s", output)
	}

	if !strings.Contains(output, ">Internal</a>") {
		t.Fatalf("expected internal link to remain unprefixed, got %s", output)
	}

	if strings.Contains(output, `href="/zk/123" target="_blank"`) {
		t.Fatalf("expected internal links to keep original target behavior, got %s", output)
	}

	if !strings.Contains(output, ">Anchor</a>") {
		t.Fatalf("expected anchor link to remain unprefixed, got %s", output)
	}

	if strings.Contains(output, `href="#section" target="_blank"`) {
		t.Fatalf("expected anchor links to keep original target behavior, got %s", output)
	}

	if !strings.Contains(output, `href="https://example.com/already" target="_blank" rel="noopener noreferrer"`) {
		t.Fatalf("expected already-prefixed external links to include target and rel attributes, got %s", output)
	}

	if strings.Count(output, "🗗") != 2 {
		t.Fatalf("expected two external link prefixes, got %d", strings.Count(output, "🗗"))
	}
}

func TestAddExternalLinkPrefixSkipsBaseURL(t *testing.T) {
	t.Setenv("GROUNDWAVE_BASE_URL", "https://groundwave.example.com")

	input := `<p><a href="https://groundwave.example.com/zk/123">Internal</a> ` +
		`<a href="https://example.com">External</a></p>`

	output, err := addExternalLinkPrefix(input)
	if err != nil {
		t.Fatalf("addExternalLinkPrefix failed: %v", err)
	}

	if strings.Contains(output, ">🗗 Internal</a>") {
		t.Fatalf("expected base URL link to remain unprefixed, got %s", output)
	}

	if strings.Contains(output, `href="https://groundwave.example.com/zk/123" target="_blank"`) {
		t.Fatalf("expected base URL link to keep original target behavior, got %s", output)
	}

	if !strings.Contains(output, `href="https://example.com" target="_blank" rel="noopener noreferrer"`) {
		t.Fatalf("expected external links to include target and rel attributes, got %s", output)
	}

	if strings.Count(output, "🗗") != 1 {
		t.Fatalf("expected one external link prefix, got %d", strings.Count(output, "🗗"))
	}
}

func TestAddExternalLinkPrefixMergesRelTokens(t *testing.T) {
	t.Setenv("GROUNDWAVE_BASE_URL", "https://groundwave.example.com")

	input := `<p><a href="https://example.com" rel="nofollow noreferrer">Example</a></p>`

	output, err := addExternalLinkPrefix(input)
	if err != nil {
		t.Fatalf("addExternalLinkPrefix failed: %v", err)
	}

	if !strings.Contains(output, `target="_blank"`) {
		t.Fatalf("expected target attribute to be present, got %s", output)
	}

	if !strings.Contains(output, `rel="nofollow noreferrer noopener"`) {
		t.Fatalf("expected rel tokens to be merged without duplicates, got %s", output)
	}
}

func TestAddExternalLinkPrefixEmpty(t *testing.T) {
	output, err := addExternalLinkPrefix("   ")
	if err != nil {
		t.Fatalf("addExternalLinkPrefix failed: %v", err)
	}

	if output != "   " {
		t.Fatalf("expected whitespace to be preserved, got %q", output)
	}
}

func TestAddExternalLinkPrefixEmptyAnchor(t *testing.T) {
	t.Setenv("GROUNDWAVE_BASE_URL", "https://groundwave.example.com")

	input := `<p><a href="https://example.com/empty"></a></p>`

	output, err := addExternalLinkPrefix(input)
	if err != nil {
		t.Fatalf("addExternalLinkPrefix failed: %v", err)
	}

	if !strings.Contains(output, "🗗") {
		t.Fatalf("expected prefix for empty anchor, got %s", output)
	}
}

func TestLinkHasPrefixNonTextChild(t *testing.T) {
	container := &nethtml.Node{Type: nethtml.ElementNode, Data: "div", DataAtom: atom.Div}
	fragment := `<a href="https://example.com"><span>Text</span></a>`

	nodes, err := nethtml.ParseFragment(strings.NewReader(fragment), container)
	if err != nil {
		t.Fatalf("ParseFragment failed: %v", err)
	}

	if len(nodes) == 0 {
		t.Fatalf("expected nodes to be parsed")
	}

	if linkHasPrefix(nodes[0]) {
		t.Fatalf("expected linkHasPrefix to be false for non-text child")
	}
}

func TestIsExternalLink(t *testing.T) {
	t.Setenv("GROUNDWAVE_BASE_URL", "https://groundwave.example.com")

	cases := []struct {
		href     string
		expected bool
	}{
		{href: "", expected: false},
		{href: "#section", expected: false},
		{href: "/zk/123", expected: false},
		{href: "/home/123", expected: false},
		{href: "/note/123", expected: false},
		{href: "https://groundwave.example.com", expected: false},
		{href: "https://groundwave.example.com/zk/123", expected: false},
		{href: "https://example.com", expected: true},
	}

	for _, tc := range cases {
		if got := isExternalLink(tc.href); got != tc.expected {
			t.Fatalf("isExternalLink(%q) expected %v, got %v", tc.href, tc.expected, got)
		}
	}
}

func TestMergeLinkRelValues(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		expected string
	}{
		{name: "empty", existing: "", expected: "noopener noreferrer"},
		{name: "adds required", existing: "nofollow", expected: "nofollow noopener noreferrer"},
		{name: "keeps existing order", existing: "noreferrer nofollow", expected: "noreferrer nofollow noopener"},
		{name: "dedupes case insensitive", existing: "NOOPENER noreferrer", expected: "NOOPENER noreferrer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeLinkRelValues(tt.existing, externalLinkRelTokens...)
			if got != tt.expected {
				t.Fatalf("mergeLinkRelValues(%q) = %q, expected %q", tt.existing, got, tt.expected)
			}
		})
	}
}

func TestIsPublicAccess(t *testing.T) {
	content := "#+access: public\n#+TITLE: Note"
	if !IsPublicAccess(content) {
		t.Fatalf("expected public access to be detected")
	}

	content = "#+ACCESS: PUBLIC"
	if !IsPublicAccess(content) {
		t.Fatalf("expected case-insensitive public access to be detected")
	}

	content = "#+access: private"
	if IsPublicAccess(content) {
		t.Fatalf("expected private access to be false")
	}
}

func TestIsHomeAccess(t *testing.T) {
	content := "#+access: home\n#+TITLE: Note"
	if !IsHomeAccess(content) {
		t.Fatalf("expected home access to be detected")
	}

	content = "#+ACCESS: HOME"
	if !IsHomeAccess(content) {
		t.Fatalf("expected case-insensitive home access to be detected")
	}

	content = "#+access: public"
	if IsHomeAccess(content) {
		t.Fatalf("expected public access to not match home access")
	}
}

func TestExtractIDProperty(t *testing.T) {
	content := ":PROPERTIES:\n:ID:       075915aa-f7b9-499c-9858-8167d6b1e11b\n:END:"

	id, err := ExtractIDProperty(content)
	if err != nil {
		t.Fatalf("ExtractIDProperty failed: %v", err)
	}

	if id != "075915aa-f7b9-499c-9858-8167d6b1e11b" {
		t.Fatalf("expected id, got %q", id)
	}

	if _, err := ExtractIDProperty("no id here"); err == nil {
		t.Fatalf("expected error when id missing")
	}
}

func TestExtractTitle(t *testing.T) {
	content := "#+TITLE: My Note\n* Heading"
	if got := ExtractTitle(content); got != "My Note" {
		t.Fatalf("expected title from directive, got %q", got)
	}

	content = "* Heading Title\nSome text"
	if got := ExtractTitle(content); got != "Heading Title" {
		t.Fatalf("expected title from heading, got %q", got)
	}

	content = "No title here"
	if got := ExtractTitle(content); got != "Untitled Note" {
		t.Fatalf("expected default title, got %q", got)
	}
}

func TestValidateUUID(t *testing.T) {
	valid := "075915aa-f7b9-499c-9858-8167d6b1e11b"
	if err := ValidateUUID(valid); err != nil {
		t.Fatalf("expected valid uuid, got %v", err)
	}

	if err := ValidateUUID("TOO-SHORT"); err == nil {
		t.Fatalf("expected error for short uuid")
	}

	if err := ValidateUUID("INVALID_UUID!"); err == nil {
		t.Fatalf("expected error for invalid characters")
	}

	tooLong := strings.Repeat("a", 101)
	if err := ValidateUUID(tooLong); err == nil {
		t.Fatalf("expected error for long uuid")
	}
}

func TestExtractDateDirective(t *testing.T) {
	content := "#+DATE: <2024-01-02>\n#+TITLE: Note"

	date, ok := ExtractDateDirective(content)
	if !ok {
		t.Fatalf("expected date to be extracted")
	}

	expected := time.Date(2024, time.January, 2, 0, 0, 0, 0, time.UTC)
	if !date.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, date)
	}

	if _, ok := ExtractDateDirective("#+DATE: 2024-13-40"); ok {
		t.Fatalf("expected invalid date to return false")
	}

	if _, ok := ExtractDateDirective("#+TITLE: Missing date"); ok {
		t.Fatalf("expected missing date to return false")
	}

	content = "#+DATE: 2024-01-02"

	date, ok = ExtractDateDirective(content)
	if !ok {
		t.Fatalf("expected date without angle brackets to be extracted")
	}

	if !date.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, date)
	}
}

func TestParseOrgToHTMLWithBasePathParseError(t *testing.T) {
	origParseOrg := parseOrg
	parseOrg = func(_ *org.Configuration, _ io.Reader) *org.Document {
		return &org.Document{Error: errTestBoom}
	}

	defer func() {
		parseOrg = origParseOrg
	}()

	if _, err := ParseOrgToHTMLWithBasePath("content", "/zk"); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseOrgToHTMLWithBasePathWriteError(t *testing.T) {
	origWriteOrg := writeOrg
	writeOrg = func(_ *org.Document, _ *org.HTMLWriter) (string, error) {
		return "", errTestWriteFailed
	}

	defer func() {
		writeOrg = origWriteOrg
	}()

	if _, err := ParseOrgToHTMLWithBasePath("content", "/zk"); err == nil {
		t.Fatalf("expected write error")
	}
}

func TestParseOrgToHTMLWithBasePathAnnotateError(t *testing.T) {
	origParseFragment := parseHTMLFragment
	parseHTMLFragment = func(_ io.Reader, _ *nethtml.Node) ([]*nethtml.Node, error) {
		return nil, errTestParseFailed
	}

	defer func() {
		parseHTMLFragment = origParseFragment
	}()

	if _, err := ParseOrgToHTMLWithBasePath("content", "/zk"); err == nil {
		t.Fatalf("expected annotation error")
	}
}

func TestAddExternalLinkPrefixRenderError(t *testing.T) {
	origRender := renderHTML
	renderHTML = func(_ io.Writer, _ *nethtml.Node) error {
		return errTestRenderFailed
	}

	defer func() {
		renderHTML = origRender
	}()

	if _, err := addExternalLinkPrefix("<p>Hi</p>"); err == nil {
		t.Fatalf("expected render error")
	}
}
