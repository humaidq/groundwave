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
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-webdav"
	"github.com/humaidq/groundwave/utils"
)

// ZettelkastenConfig holds the zettelkasten configuration
type ZettelkastenConfig struct {
	BaseURL   string // WebDAV directory URL (e.g., https://webdav.example.com/org/)
	Username  string
	Password  string
	IndexFile string // Filename of the starting note
}

// ZKNote represents a zettelkasten note
type ZKNote struct {
	ID       string
	Title    string
	Filename string
	IsPublic bool
	HTMLBody template.HTML
}

// ZKNoteSummary represents a lightweight note listing
// for selection in the zettelkasten chat UI.
type ZKNoteSummary struct {
	ID       string
	Title    string
	IsPublic bool
}

// ZKChatNote represents a note with raw org content
// for use in chat prompts.
type ZKChatNote struct {
	ID      string
	Title   string
	Content string
}

// In-memory cache for ID to filename mappings
var (
	idToFilenameCache = make(map[string]string)
	cacheMutex        sync.RWMutex

	noteLinkPattern  = regexp.MustCompile(`<a([^>]*?)href="(/note/([a-f0-9\-]+))"([^>]*)>`)
	classAttrPattern = regexp.MustCompile(`\sclass="([^"]*)"`)
)

// GetZKConfig loads zettelkasten configuration from environment variables
func GetZKConfig() (*ZettelkastenConfig, error) {
	zkPath := os.Getenv("WEBDAV_ZK_PATH")
	username := os.Getenv("WEBDAV_USERNAME")
	password := os.Getenv("WEBDAV_PASSWORD")

	if zkPath == "" {
		return nil, fmt.Errorf("WEBDAV_ZK_PATH not configured")
	}

	// Parse WEBDAV_ZK_PATH to extract base URL and index filename
	// Example: https://webdav.example.com/org/abc-index.org
	//   -> baseURL: https://webdav.example.com/org/
	//   -> indexFile: abc-index.org

	parsedURL, err := url.Parse(zkPath)
	if err != nil {
		return nil, fmt.Errorf("invalid WEBDAV_ZK_PATH URL: %w", err)
	}

	// Extract the directory path and filename
	pathParts := strings.Split(strings.TrimPrefix(parsedURL.Path, "/"), "/")
	if len(pathParts) == 0 {
		return nil, fmt.Errorf("WEBDAV_ZK_PATH must include a filename")
	}

	indexFile := pathParts[len(pathParts)-1]
	if !strings.HasSuffix(indexFile, ".org") {
		return nil, fmt.Errorf("WEBDAV_ZK_PATH must point to a .org file")
	}

	// Reconstruct base URL (everything except the filename)
	basePathParts := pathParts[:len(pathParts)-1]
	basePath := "/" + strings.Join(basePathParts, "/")
	if basePath != "/" {
		basePath += "/"
	}

	baseURL := fmt.Sprintf("%s://%s%s", parsedURL.Scheme, parsedURL.Host, basePath)

	return &ZettelkastenConfig{
		BaseURL:   baseURL,
		Username:  username,
		Password:  password,
		IndexFile: indexFile,
	}, nil
}

func getZKDailyBaseURL(config *ZettelkastenConfig) string {
	return strings.TrimSuffix(config.BaseURL, "/") + "/daily/"
}

// newZKHTTPClient creates an HTTP client for WebDAV operations
func newZKHTTPClient(config *ZettelkastenConfig) *http.Client {
	transport := http.DefaultTransport

	// Add basic auth if credentials are provided
	if config.Username != "" && config.Password != "" {
		transport = &basicAuthTransport{
			Username: config.Username,
			Password: config.Password,
			Base:     http.DefaultTransport,
		}
	}

	return &http.Client{
		Timeout:   3 * time.Second, // Fast timeout for local/same-network WebDAV
		Transport: transport,
	}
}

// FetchOrgFile fetches a single .org file from WebDAV
func FetchOrgFile(ctx context.Context, filename string) (string, error) {
	config, err := GetZKConfig()
	if err != nil {
		return "", err
	}

	httpClient := newZKHTTPClient(config)

	// Construct full URL
	fileURL := config.BaseURL + filename

	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file %s: %w", filename, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close zettelkasten response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch file %s: HTTP %d", filename, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(body), nil
}

// FetchDailyOrgFile fetches a daily journal org file from WebDAV.
func FetchDailyOrgFile(ctx context.Context, filename string) (string, error) {
	config, err := GetZKConfig()
	if err != nil {
		return "", err
	}

	httpClient := newZKHTTPClient(config)
	dailyBaseURL := getZKDailyBaseURL(config)

	fileURL := dailyBaseURL + filename

	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file %s: %w", filename, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close zettelkasten daily response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch file %s: HTTP %d", filename, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(body), nil
}

// ListOrgFiles lists all .org files in the WebDAV directory
func ListOrgFiles(ctx context.Context) ([]string, error) {
	config, err := GetZKConfig()
	if err != nil {
		return nil, err
	}

	httpClient := newZKHTTPClient(config)

	// Create WebDAV client
	client, err := webdav.NewClient(httpClient, config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	baseURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebDAV base URL: %w", err)
	}
	basePath := strings.TrimRight(baseURL.Path, "/") + "/"

	// List files in directory (use "." for current directory relative to BaseURL)
	fileInfos, err := client.ReadDir(ctx, basePath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}

	logger.Info("Found WebDAV directory items", "count", len(fileInfos), "base_url", config.BaseURL)

	var orgFiles []string
	for _, info := range fileInfos {
		if !info.IsDir && strings.HasSuffix(info.Path, ".org") {
			// Extract just the filename from the path
			parts := strings.Split(strings.TrimPrefix(info.Path, "/"), "/")
			filename := parts[len(parts)-1]
			orgFiles = append(orgFiles, filename)
		}
	}

	return orgFiles, nil
}

// ListDailyOrgFiles lists .org files in the WebDAV daily journal directory.
func ListDailyOrgFiles(ctx context.Context) ([]string, error) {
	config, err := GetZKConfig()
	if err != nil {
		return nil, err
	}

	httpClient := newZKHTTPClient(config)
	dailyBaseURL := getZKDailyBaseURL(config)

	client, err := webdav.NewClient(httpClient, dailyBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	dailyURL, err := url.Parse(dailyBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebDAV daily URL: %w", err)
	}
	dailyPath := strings.TrimRight(dailyURL.Path, "/") + "/"

	fileInfos, err := client.ReadDir(ctx, dailyPath, false)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list daily directory: %w", err)
	}

	logger.Info("Found WebDAV daily directory items", "count", len(fileInfos), "base_url", dailyBaseURL)

	var orgFiles []string
	for _, info := range fileInfos {
		if !info.IsDir && strings.HasSuffix(info.Path, ".org") {
			parts := strings.Split(strings.TrimPrefix(info.Path, "/"), "/")
			filename := parts[len(parts)-1]
			orgFiles = append(orgFiles, filename)
		}
	}

	return orgFiles, nil
}

// FindFileByID resolves a note ID to its filename using caching
func FindFileByID(ctx context.Context, id string) (string, error) {
	// Validate UUID format for security
	if err := utils.ValidateUUID(id); err != nil {
		return "", err
	}

	// Check cache first
	cacheMutex.RLock()
	filename, exists := idToFilenameCache[id]
	cacheMutex.RUnlock()

	if exists {
		return filename, nil
	}

	// Cache miss - scan all .org files
	logger.Info("Cache miss for ID, scanning WebDAV directory", "id", id)

	files, err := ListOrgFiles(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list org files: %w", err)
	}

	// Scan each file to extract ID and build cache
	for _, file := range files {
		content, err := FetchOrgFile(ctx, file)
		if err != nil {
			logger.Warn("Skipping unreadable file", "file", file, "error", err)
			continue
		}

		fileID, err := utils.ExtractIDProperty(content)
		if err != nil {
			// File doesn't have an ID property, skip it
			continue
		}

		// Cache the mapping
		cacheMutex.Lock()
		idToFilenameCache[fileID] = file
		cacheMutex.Unlock()

		// Check if this is the file we're looking for
		if fileID == id {
			return file, nil
		}
	}

	return "", fmt.Errorf("note with ID %s not found (scanned %d files)", id, len(files))
}

// ListZKNotes returns all zettelkasten notes with IDs and titles.
func ListZKNotes(ctx context.Context) ([]ZKNoteSummary, error) {
	files, err := ListOrgFiles(ctx)
	if err != nil {
		return nil, err
	}

	notes := make([]ZKNoteSummary, 0, len(files))
	for _, file := range files {
		content, err := FetchOrgFile(ctx, file)
		if err != nil {
			logger.Warn("Skipping unreadable file", "file", file, "error", err)
			continue
		}

		id, err := utils.ExtractIDProperty(content)
		if err != nil {
			continue
		}

		notes = append(notes, ZKNoteSummary{
			ID:       id,
			Title:    utils.ExtractTitle(content),
			IsPublic: utils.IsPublicAccess(content),
		})
	}

	sort.Slice(notes, func(i, j int) bool {
		return strings.ToLower(notes[i].Title) < strings.ToLower(notes[j].Title)
	})

	return notes, nil
}

// GetZKNoteForChat fetches raw org content for chat prompts.
func GetZKNoteForChat(ctx context.Context, id string) (*ZKChatNote, error) {
	filename, err := FindFileByID(ctx, id)
	if err != nil {
		return nil, err
	}

	content, err := FetchOrgFile(ctx, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch note: %w", err)
	}

	return &ZKChatNote{
		ID:      id,
		Title:   utils.ExtractTitle(content),
		Content: content,
	}, nil
}

// GetZKNoteLinks returns cached forward links for a note.
func GetZKNoteLinks(id string) []string {
	return GetForwardLinksFromCache(id)
}

func annotateRestrictedNoteLinks(htmlContent string) string {
	if GetLastCacheBuildTime().IsZero() {
		return htmlContent
	}

	return noteLinkPattern.ReplaceAllStringFunc(htmlContent, func(link string) string {
		matches := noteLinkPattern.FindStringSubmatch(link)
		if len(matches) < 4 {
			return link
		}
		noteID := matches[3]
		if IsPublicNoteFromCache(noteID) {
			return link
		}

		if classAttrPattern.MatchString(link) {
			return classAttrPattern.ReplaceAllStringFunc(link, func(classAttr string) string {
				classMatches := classAttrPattern.FindStringSubmatch(classAttr)
				if len(classMatches) < 2 {
					return classAttr
				}
				for _, existing := range strings.Fields(classMatches[1]) {
					if existing == "restricted-link" {
						return classAttr
					}
				}
				return fmt.Sprintf(` class="%s restricted-link"`, classMatches[1])
			})
		}

		return strings.Replace(link, "<a", `<a class="restricted-link"`, 1)
	})
}

// GetNoteByID fetches and parses a note by its ID
func GetNoteByID(ctx context.Context, id string) (*ZKNote, error) {
	return GetNoteByIDWithBasePath(ctx, id, "/zk")
}

// GetNoteByIDWithBasePath fetches and parses a note by its ID using a base path for id links.
func GetNoteByIDWithBasePath(ctx context.Context, id string, basePath string) (*ZKNote, error) {
	filename, err := FindFileByID(ctx, id)
	if err != nil {
		return nil, err
	}

	content, err := FetchOrgFile(ctx, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch note: %w", err)
	}

	html, err := utils.ParseOrgToHTMLWithBasePath(content, basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse org-mode content: %w", err)
	}

	trimmedBase := strings.TrimRight(strings.TrimSpace(basePath), "/")
	if trimmedBase == "/note" {
		html = annotateRestrictedNoteLinks(html)
	}

	title := utils.ExtractTitle(content)
	isPublic := utils.IsPublicAccess(content)

	return &ZKNote{
		ID:       id,
		Title:    title,
		Filename: filename,
		IsPublic: isPublic,
		HTMLBody: template.HTML(html),
	}, nil
}

// GetIndexNote fetches and parses the index/starting note
func GetIndexNote(ctx context.Context) (*ZKNote, error) {
	config, err := GetZKConfig()
	if err != nil {
		return nil, err
	}

	content, err := FetchOrgFile(ctx, config.IndexFile)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch index note: %w", err)
	}

	html, err := utils.ParseOrgToHTMLWithBasePath(content, "/zk")
	if err != nil {
		return nil, fmt.Errorf("failed to parse org-mode content: %w", err)
	}

	title := utils.ExtractTitle(content)
	id, _ := utils.ExtractIDProperty(content)
	isPublic := utils.IsPublicAccess(content)

	return &ZKNote{
		ID:       id,
		Title:    title,
		Filename: config.IndexFile,
		IsPublic: isPublic,
		HTMLBody: template.HTML(html),
	}, nil
}
