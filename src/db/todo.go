/*
 * Copyright 2025 Humaid Alqasimi
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
	"time"

	"github.com/humaidq/groundwave/utils"
)

// TodoNote represents a single org-mode todo page.
type TodoNote struct {
	Title    string
	HTMLBody template.HTML
}

func newTodoHTTPClient(username, password string) *http.Client {
	transport := http.DefaultTransport
	if username != "" && password != "" {
		transport = &basicAuthTransport{
			Username: username,
			Password: password,
			Base:     http.DefaultTransport,
		}
	}

	return &http.Client{
		Timeout:   3 * time.Second,
		Transport: transport,
	}
}

// GetTodoNote fetches and parses the todo org-mode file from WebDAV.
func GetTodoNote(ctx context.Context) (*TodoNote, error) {
	todoPath := os.Getenv("WEBDAV_TODO_PATH")
	if todoPath == "" {
		return nil, ErrWebDAVTodoPathNotConfigured
	}

	parsedURL, err := url.Parse(todoPath)
	if err != nil {
		return nil, fmt.Errorf("invalid WEBDAV_TODO_PATH URL: %w", err)
	}

	if !strings.HasSuffix(parsedURL.Path, ".org") {
		return nil, ErrWebDAVTodoPathMustBeOrgFile
	}

	username := os.Getenv("WEBDAV_USERNAME")
	password := os.Getenv("WEBDAV_PASSWORD")
	httpClient := newTodoHTTPClient(username, password)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, todoPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch todo file: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close todo response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrFetchTodoFileFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read todo file content: %w", err)
	}

	content := string(body)

	html, err := utils.ParseOrgToHTML(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse org-mode content: %w", err)
	}

	return &TodoNote{
		Title:    utils.ExtractTitle(content),
		HTMLBody: template.HTML(html), //nolint:gosec // HTML comes from trusted org parser output.
	}, nil
}
