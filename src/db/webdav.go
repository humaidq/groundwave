/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package db

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-webdav"
)

// WebDAVConfig holds unified WebDAV configuration
type WebDAVConfig struct {
	Username string // Shared: WEBDAV_USERNAME
	Password string // Shared: WEBDAV_PASSWORD
	ZKPath   string // WEBDAV_ZK_PATH (for Zettelkasten)
	InvPath  string // WEBDAV_INV_PATH (for Inventory)
}

// WebDAVFile represents a file in WebDAV
type WebDAVFile struct {
	Name        string
	Path        string
	Size        int64
	ModTime     time.Time
	IsDir       bool
	ContentType string // Inferred from extension
}

// GetWebDAVConfig loads WebDAV configuration from environment
func GetWebDAVConfig() (*WebDAVConfig, error) {
	username := os.Getenv("WEBDAV_USERNAME")
	password := os.Getenv("WEBDAV_PASSWORD")
	zkPath := os.Getenv("WEBDAV_ZK_PATH")
	invPath := os.Getenv("WEBDAV_INV_PATH")

	// Username and password are optional (no auth if not provided)
	// At least one path must be configured for this to be useful
	if zkPath == "" && invPath == "" {
		return nil, fmt.Errorf("neither WEBDAV_ZK_PATH nor WEBDAV_INV_PATH configured")
	}

	return &WebDAVConfig{
		Username: username,
		Password: password,
		ZKPath:   zkPath,
		InvPath:  invPath,
	}, nil
}

// newWebDAVHTTPClient creates an HTTP client for WebDAV operations
func newWebDAVHTTPClient(config *WebDAVConfig) *http.Client {
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
		Timeout:   30 * time.Second,
		Transport: transport,
	}
}

// ListInventoryFiles lists files in the WebDAV inventory directory for a specific item
// Returns empty slice if directory doesn't exist (graceful degradation)
func ListInventoryFiles(ctx context.Context, inventoryID string) ([]WebDAVFile, error) {
	config, err := GetWebDAVConfig()
	if err != nil {
		return nil, err
	}

	if config.InvPath == "" {
		return nil, fmt.Errorf("WEBDAV_INV_PATH not configured")
	}

	// Construct directory path: WEBDAV_INV_PATH + "/" + inventoryID
	dirPath := strings.TrimSuffix(config.InvPath, "/") + "/" + inventoryID

	httpClient := newWebDAVHTTPClient(config)
	client, err := webdav.NewClient(httpClient, dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	// List files in the directory
	fileInfos, err := client.ReadDir(ctx, ".", false)
	if err != nil {
		// Directory doesn't exist - return empty list (graceful degradation)
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return []WebDAVFile{}, nil
		}
		return nil, fmt.Errorf("failed to list WebDAV directory: %w", err)
	}

	var files []WebDAVFile
	for _, info := range fileInfos {
		if info.IsDir {
			continue // Skip subdirectories
		}

		file := WebDAVFile{
			Name:        extractFilename(info.Path),
			Path:        info.Path,
			Size:        info.Size,
			ModTime:     info.ModTime,
			IsDir:       info.IsDir,
			ContentType: inferContentType(info.Path),
		}
		files = append(files, file)
	}

	return files, nil
}

// FetchInventoryFile downloads a file from WebDAV inventory directory
func FetchInventoryFile(ctx context.Context, inventoryID string, filename string) ([]byte, string, error) {
	config, err := GetWebDAVConfig()
	if err != nil {
		return nil, "", err
	}

	if config.InvPath == "" {
		return nil, "", fmt.Errorf("WEBDAV_INV_PATH not configured")
	}

	// Construct file URL: WEBDAV_INV_PATH + "/" + inventoryID + "/" + filename
	fileURL := strings.TrimSuffix(config.InvPath, "/") + "/" + inventoryID + "/" + filename

	httpClient := newWebDAVHTTPClient(config)

	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("failed to fetch file: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file content: %w", err)
	}

	contentType := inferContentType(filename)
	return body, contentType, nil
}

// Helper functions

func extractFilename(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	return parts[len(parts)-1]
}

func inferContentType(filename string) string {
	// Find last dot for extension
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 || lastDot == len(filename)-1 {
		return "application/octet-stream"
	}

	ext := strings.ToLower(filename[lastDot+1:])

	contentTypes := map[string]string{
		"pdf":  "application/pdf",
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"png":  "image/png",
		"gif":  "image/gif",
		"webp": "image/webp",
		"svg":  "image/svg+xml",
		"txt":  "text/plain",
		"md":   "text/markdown",
		"html": "text/html",
		"htm":  "text/html",
		"css":  "text/css",
		"js":   "application/javascript",
		"csv":  "text/csv",
		"json": "application/json",
		"xml":  "application/xml",
		"zip":  "application/zip",
		"tar":  "application/x-tar",
		"gz":   "application/gzip",
		"7z":   "application/x-7z-compressed",
		"rar":  "application/vnd.rar",
		"doc":  "application/msword",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xls":  "application/vnd.ms-excel",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"ppt":  "application/vnd.ms-powerpoint",
		"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"odt":  "application/vnd.oasis.opendocument.text",
		"ods":  "application/vnd.oasis.opendocument.spreadsheet",
		"odp":  "application/vnd.oasis.opendocument.presentation",
		"mp3":  "audio/mpeg",
		"wav":  "audio/wav",
		"ogg":  "audio/ogg",
		"mp4":  "video/mp4",
		"avi":  "video/x-msvideo",
		"mkv":  "video/x-matroska",
		"webm": "video/webm",
	}

	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}
