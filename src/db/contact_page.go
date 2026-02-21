/*
 * Copyright 2026 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/humaidq/groundwave/utils"
)

// ContactPageNote represents the public contact org page.
type ContactPageNote struct {
	Title    string
	HTMLBody template.HTML
}

// GetContactPageNote fetches and parses the contact org page from WebDAV.
func GetContactPageNote(ctx context.Context) (*ContactPageNote, error) {
	contactPagePath := os.Getenv("WEBDAV_CONTACT_PAGE")
	if contactPagePath == "" {
		return nil, ErrWebDAVContactPageNotConfigured
	}

	parsedURL, err := url.Parse(contactPagePath)
	if err != nil {
		return nil, fmt.Errorf("invalid WEBDAV_CONTACT_PAGE URL: %w", err)
	}

	if !strings.HasSuffix(parsedURL.Path, ".org") {
		return nil, ErrWebDAVContactPageMustBeOrgFile
	}

	username := os.Getenv("WEBDAV_USERNAME")
	password := os.Getenv("WEBDAV_PASSWORD")
	httpClient := newTodoHTTPClient(username, password)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, contactPagePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contact page file: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close contact page response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrFetchContactPageFileFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read contact page file content: %w", err)
	}

	content := string(body)

	html, err := utils.ParseOrgToHTML(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse org-mode content: %w", err)
	}

	return &ContactPageNote{
		Title:    utils.ExtractTitle(content),
		HTMLBody: template.HTML(html), //nolint:gosec // HTML comes from trusted org parser output.
	}, nil
}
