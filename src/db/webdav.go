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
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-webdav"
)

// WebDAVConfig holds unified WebDAV configuration
type WebDAVConfig struct {
	Username  string // Shared: WEBDAV_USERNAME
	Password  string // Shared: WEBDAV_PASSWORD
	ZKPath    string // WEBDAV_ZK_PATH (for Zettelkasten)
	InvPath   string // WEBDAV_INV_PATH (for Inventory)
	FilesPath string // WEBDAV_FILES_PATH (for Files)
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

// WebDAVEntry represents a file or directory in WebDAV
type WebDAVEntry struct {
	Name        string
	Path        string
	Size        int64
	ModTime     time.Time
	IsDir       bool
	IsAdminOnly bool
}

// GetWebDAVConfig loads WebDAV configuration from environment
func GetWebDAVConfig() (*WebDAVConfig, error) {
	username := os.Getenv("WEBDAV_USERNAME")
	password := os.Getenv("WEBDAV_PASSWORD")
	zkPath := os.Getenv("WEBDAV_ZK_PATH")
	invPath := os.Getenv("WEBDAV_INV_PATH")
	filesPath := os.Getenv("WEBDAV_FILES_PATH")

	// Username and password are optional (no auth if not provided)
	// At least one path must be configured for this to be useful
	if zkPath == "" && invPath == "" && filesPath == "" {
		return nil, ErrNoWebDAVPathsConfigured
	}

	return &WebDAVConfig{
		Username:  username,
		Password:  password,
		ZKPath:    zkPath,
		InvPath:   invPath,
		FilesPath: filesPath,
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
		Timeout:   3 * time.Second, // Fast timeout for local/same-network WebDAV
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
		return nil, ErrWebDAVInventoryPathNotConfigured
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

	files := make([]WebDAVFile, 0, len(fileInfos))

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
		return nil, "", ErrWebDAVInventoryPathNotConfigured
	}

	// Construct file URL: WEBDAV_INV_PATH + "/" + inventoryID + "/" + filename
	fileURL := strings.TrimSuffix(config.InvPath, "/") + "/" + inventoryID + "/" + filename

	httpClient := newWebDAVHTTPClient(config)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch file: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close WebDAV inventory response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: HTTP %d", ErrFetchFileFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file content: %w", err)
	}

	contentType := inferContentType(filename)

	return body, contentType, nil
}

// ListFilesEntries lists files and directories in the WebDAV files directory.
func ListFilesEntries(ctx context.Context, dirPath string) ([]WebDAVEntry, error) {
	config, err := GetWebDAVConfig()
	if err != nil {
		return nil, err
	}

	if config.FilesPath == "" {
		return nil, ErrWebDAVFilesPathNotConfigured
	}

	client, err := newFilesWebDAVClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	basePath := filesBasePath(config)
	if basePath == "" {
		return nil, ErrWebDAVFilesPathNotConfigured
	}

	target := filesReadDirTarget(basePath, dirPath)

	fileInfos, err := client.ReadDir(ctx, target, false)
	if err != nil {
		return nil, fmt.Errorf("failed to list WebDAV directory: %w", err)
	}

	entries := make([]WebDAVEntry, 0, len(fileInfos))
	for _, info := range fileInfos {
		if isListingSelf(info.Path, dirPath, basePath) {
			continue
		}

		name := extractFilename(info.Path)
		if name == "" {
			continue
		}

		if strings.HasPrefix(name, ".") {
			continue
		}

		entry := WebDAVEntry{
			Name:    name,
			Path:    path.Join(dirPath, name),
			Size:    info.Size,
			ModTime: info.ModTime,
			IsDir:   info.IsDir,
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}

		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries, nil
}

// FetchFilesFile downloads a file from WebDAV files directory.
func FetchFilesFile(ctx context.Context, filePath string) ([]byte, string, error) {
	config, err := GetWebDAVConfig()
	if err != nil {
		return nil, "", err
	}

	if config.FilesPath == "" {
		return nil, "", ErrWebDAVFilesPathNotConfigured
	}

	client, err := newFilesWebDAVClient(config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	reader, err := client.Open(ctx, filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch WebDAV file: %w", err)
	}

	defer func() {
		if err := reader.Close(); err != nil {
			logger.Warn("Failed to close WebDAV file reader", "error", err)
		}
	}()

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file content: %w", err)
	}

	contentType := inferContentType(filePath)

	return body, contentType, nil
}

// UploadFilesFile uploads a file into the WebDAV files directory.
// The file is written to a temporary path and moved into place to avoid partial data.
func UploadFilesFile(ctx context.Context, filePath string, reader io.ReadSeeker, expectedSize int64) (int64, error) {
	config, err := GetWebDAVConfig()
	if err != nil {
		return 0, err
	}

	if config.FilesPath == "" {
		return 0, ErrWebDAVFilesPathNotConfigured
	}

	client, err := newFilesWebDAVClient(config)
	if err != nil {
		return 0, fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	exists, err := filesEntryExists(ctx, client, filePath)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, ErrWebDAVFilesEntryExists
	}

	dirPath := path.Dir(filePath)
	if dirPath == "." {
		dirPath = ""
	}

	if dirPath != "" {
		info, err := client.Stat(ctx, dirPath)
		if err != nil {
			if isWebDAVNotFound(err) {
				return 0, ErrWebDAVFilesEntryNotFound
			}
			return 0, fmt.Errorf("failed to verify upload directory: %w", err)
		}
		if !info.IsDir {
			return 0, fmt.Errorf("upload path is not a directory")
		}
	}

	filename := path.Base(filePath)
	tempName := fmt.Sprintf(".gw_upload_%d_%s", time.Now().UnixNano(), filename)
	tempPath := tempName
	if dirPath != "" {
		tempPath = path.Join(dirPath, tempName)
	}

	size, err := filesUploadSize(reader, expectedSize)
	if err != nil {
		return 0, err
	}

	if err := putFilesEntry(ctx, config, tempPath, reader, size); err != nil {
		cleanupFilesEntry(ctx, client, tempPath)
		if exists, existsErr := filesEntryExists(ctx, client, filePath); existsErr == nil && exists {
			return 0, ErrWebDAVFilesEntryExists
		}
		return 0, err
	}

	info, err := client.Stat(ctx, tempPath)
	if err != nil {
		cleanupFilesEntry(ctx, client, tempPath)
		return 0, fmt.Errorf("failed to verify uploaded file: %w", err)
	}
	if info.Size != size {
		cleanupFilesEntry(ctx, client, tempPath)
		return 0, fmt.Errorf("uploaded size mismatch: expected %d bytes, got %d", size, info.Size)
	}

	if err := client.Move(ctx, tempPath, filePath, &webdav.MoveOptions{NoOverwrite: true}); err != nil {
		cleanupFilesEntry(ctx, client, tempPath)
		if exists, existsErr := filesEntryExists(ctx, client, filePath); existsErr == nil && exists {
			return 0, ErrWebDAVFilesEntryExists
		}
		return 0, fmt.Errorf("failed to finalize upload: %w", err)
	}

	return size, nil
}

// MoveFilesEntry moves or renames a file or directory within the files WebDAV path.
func MoveFilesEntry(ctx context.Context, sourcePath string, destPath string) error {
	config, err := GetWebDAVConfig()
	if err != nil {
		return err
	}

	if config.FilesPath == "" {
		return ErrWebDAVFilesPathNotConfigured
	}

	client, err := newFilesWebDAVClient(config)
	if err != nil {
		return fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	exists, err := filesEntryExists(ctx, client, sourcePath)
	if err != nil {
		return err
	}
	if !exists {
		return ErrWebDAVFilesEntryNotFound
	}

	destExists, err := filesEntryExists(ctx, client, destPath)
	if err != nil {
		return err
	}
	if destExists {
		return ErrWebDAVFilesEntryExists
	}

	if err := client.Move(ctx, sourcePath, destPath, &webdav.MoveOptions{NoOverwrite: true}); err != nil {
		if exists, existsErr := filesEntryExists(ctx, client, destPath); existsErr == nil && exists {
			return ErrWebDAVFilesEntryExists
		}
		return fmt.Errorf("failed to move WebDAV entry: %w", err)
	}

	return nil
}

// DeleteFilesEntry deletes a file or directory in the files WebDAV path.
func DeleteFilesEntry(ctx context.Context, entryPath string) error {
	config, err := GetWebDAVConfig()
	if err != nil {
		return err
	}

	if config.FilesPath == "" {
		return ErrWebDAVFilesPathNotConfigured
	}

	client, err := newFilesWebDAVClient(config)
	if err != nil {
		return fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	exists, err := filesEntryExists(ctx, client, entryPath)
	if err != nil {
		return err
	}
	if !exists {
		return ErrWebDAVFilesEntryNotFound
	}

	if err := client.RemoveAll(ctx, entryPath); err != nil {
		return fmt.Errorf("failed to delete WebDAV entry: %w", err)
	}

	return nil
}

// IsFilesPathRestricted reports whether a directory path is restricted by a break-glass marker.
func IsFilesPathRestricted(ctx context.Context, dirPath string) (bool, error) {
	config, err := GetWebDAVConfig()
	if err != nil {
		return false, err
	}

	if config.FilesPath == "" {
		return false, ErrWebDAVFilesPathNotConfigured
	}

	client, err := newFilesWebDAVClient(config)
	if err != nil {
		return false, fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	basePath := filesBasePath(config)
	if basePath == "" {
		return false, ErrWebDAVFilesPathNotConfigured
	}

	return isFilesPathMarked(ctx, client, basePath, dirPath, ".gw_btg")
}

// IsFilesPathAdminOnly reports whether a directory path is restricted to admins.
func IsFilesPathAdminOnly(ctx context.Context, dirPath string) (bool, error) {
	config, err := GetWebDAVConfig()
	if err != nil {
		return false, err
	}

	if config.FilesPath == "" {
		return false, ErrWebDAVFilesPathNotConfigured
	}

	client, err := newFilesWebDAVClient(config)
	if err != nil {
		return false, fmt.Errorf("failed to create WebDAV client: %w", err)
	}

	basePath := filesBasePath(config)
	if basePath == "" {
		return false, ErrWebDAVFilesPathNotConfigured
	}

	return isFilesPathMarked(ctx, client, basePath, dirPath, ".gw_admin")
}

func newFilesWebDAVClient(config *WebDAVConfig) (*webdav.Client, error) {
	basePath := strings.TrimRight(config.FilesPath, "/")
	if basePath == "" {
		return nil, ErrWebDAVFilesPathNotConfigured
	}

	endpoint := basePath + "/"
	httpClient := newWebDAVHTTPClient(config)

	client, err := webdav.NewClient(httpClient, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create files webdav client: %w", err)
	}

	return client, nil
}

func isFilesPathMarked(ctx context.Context, client *webdav.Client, basePath string, dirPath string, marker string) (bool, error) {
	restricted, err := dirHasMarker(ctx, client, basePath, "", marker)
	if err != nil {
		return false, err
	}

	if restricted {
		return true, nil
	}

	if dirPath == "" {
		return false, nil
	}

	segments := strings.Split(dirPath, "/")
	current := ""

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if current == "" {
			current = segment
		} else {
			current = current + "/" + segment
		}

		restricted, err := dirHasMarker(ctx, client, basePath, current, marker)
		if err != nil {
			return false, err
		}

		if restricted {
			return true, nil
		}
	}

	return false, nil
}

func dirHasMarker(ctx context.Context, client *webdav.Client, basePath string, dirPath string, marker string) (bool, error) {
	target := filesReadDirTarget(basePath, dirPath)

	fileInfos, err := client.ReadDir(ctx, target, false)
	if err != nil {
		return false, fmt.Errorf("failed to read webdav directory %q: %w", target, err)
	}

	for _, info := range fileInfos {
		name := extractFilename(info.Path)
		if name == marker {
			return true, nil
		}
	}

	return false, nil
}

func filesUploadSize(reader io.ReadSeeker, expectedSize int64) (int64, error) {
	if reader == nil {
		return 0, fmt.Errorf("upload reader is nil")
	}

	end, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		if expectedSize >= 0 {
			end = expectedSize
		} else {
			return 0, fmt.Errorf("failed to determine upload size: %w", err)
		}
	}

	if expectedSize >= 0 && end != expectedSize {
		return 0, fmt.Errorf("upload size mismatch: expected %d bytes, got %d", expectedSize, end)
	}

	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to rewind upload: %w", err)
	}

	return end, nil
}

func putFilesEntry(ctx context.Context, config *WebDAVConfig, entryPath string, reader io.Reader, size int64) error {
	if size < 0 {
		return fmt.Errorf("upload size unknown")
	}

	entryURL, err := filesEntryURL(config, entryPath)
	if err != nil {
		return fmt.Errorf("failed to build upload URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, entryURL, reader)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")

	httpClient := newWebDAVHTTPClient(config)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close WebDAV upload response body", "error", err)
		}
	}()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("failed to upload file: HTTP %d", resp.StatusCode)
	}

	return nil
}

func filesEntryURL(config *WebDAVConfig, entryPath string) (string, error) {
	basePath := strings.TrimRight(config.FilesPath, "/") + "/"
	parsed, err := url.Parse(basePath)
	if err != nil {
		return "", err
	}

	if entryPath == "" {
		return parsed.String(), nil
	}

	cleaned := strings.TrimPrefix(entryPath, "/")
	parsed.Path = path.Join(strings.TrimSuffix(parsed.Path, "/"), cleaned)
	if !strings.HasPrefix(parsed.Path, "/") {
		parsed.Path = "/" + parsed.Path
	}

	return parsed.String(), nil
}

func filesEntryExists(ctx context.Context, client *webdav.Client, entryPath string) (bool, error) {
	_, err := client.Stat(ctx, entryPath)
	if err == nil {
		return true, nil
	}

	if isWebDAVNotFound(err) {
		return false, nil
	}

	return false, fmt.Errorf("failed to stat WebDAV entry: %w", err)
}

func cleanupFilesEntry(ctx context.Context, client *webdav.Client, entryPath string) {
	if entryPath == "" {
		return
	}

	if err := client.RemoveAll(ctx, entryPath); err != nil {
		if isWebDAVNotFound(err) {
			return
		}
		logger.Warn("Failed to clean up WebDAV temp entry", "path", entryPath, "error", err)
	}
}

func isWebDAVNotFound(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "404") || strings.Contains(message, "not found")
}

// Helper functions

func extractFilename(path string) string {
	trimmed := strings.TrimSuffix(path, "/")

	parts := strings.Split(strings.TrimPrefix(trimmed, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	return parts[len(parts)-1]
}

func isListingSelf(entryPath string, dirPath string, basePath string) bool {
	entry := normalizeFilesPathForCompare(entryPath, basePath)

	dir := normalizeFilesPathForCompare(dirPath, "")
	if entry == dir {
		return true
	}

	if dir == "" {
		entryRoot := normalizeFilesPathForCompare(entryPath, "")
		baseRoot := normalizeFilesPathForCompare(basePath, "")

		return entryRoot == baseRoot
	}

	return false
}

func normalizeFilesPathForCompare(value string, basePath string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	if decoded, err := url.PathUnescape(trimmed); err == nil {
		trimmed = decoded
	}

	if basePath != "" {
		base := basePath
		if decoded, err := url.PathUnescape(base); err == nil {
			base = decoded
		}

		if trimmed != base {
			original := trimmed

			trimmed = strings.TrimPrefix(trimmed, base)
			if trimmed == original {
				altBase := strings.TrimSuffix(base, "/")
				if altBase != "" && altBase != "/" {
					trimmed = strings.TrimPrefix(trimmed, altBase)
				}
			}
		}
	}

	trimmed = strings.TrimPrefix(trimmed, "/")

	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "." {
		trimmed = ""
	}

	return trimmed
}

func filesBasePath(config *WebDAVConfig) string {
	parsed, err := url.Parse(config.FilesPath)
	if err != nil {
		return ""
	}

	basePath := parsed.Path
	if basePath == "" {
		basePath = "/"
	}

	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}

	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}

	return basePath
}

func filesReadDirTarget(basePath string, dirPath string) string {
	trimmed := strings.TrimSpace(dirPath)
	if trimmed == "" || trimmed == "." || trimmed == "/" {
		return basePath
	}

	trimmed = strings.TrimPrefix(trimmed, "/")

	trimmed = strings.TrimSuffix(trimmed, "/")
	if trimmed == "" {
		return basePath
	}

	root := strings.TrimSuffix(basePath, "/")

	target := path.Join(root, trimmed)
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}

	if !strings.HasSuffix(target, "/") {
		target += "/"
	}

	return target
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
