/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

var (
	publicFilesStatFn       = db.StatPublicFile
	publicFilesFetchFn      = db.FetchPublicFile
	publicFilesOpenStreamFn = db.OpenPublicFileStream
)

const maxPublicFilesPathDecodePasses = 8

// PublicFilesView renders a public file viewer from WEBDAV_PUBLIC_PATH.
func PublicFilesView(c flamego.Context, t template.Template, data template.Data) {
	relPath, ok := sanitizePublicFilesPath(c.Param("path"))
	if !ok {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	data["HideNav"] = true
	setPublicSiteTitle(data)
	data["CurrentPath"] = relPath
	data["CurrentPathDisplay"] = formatFilesPathDisplay(relPath)
	data["FileName"] = path.Base(relPath)
	data["FileURL"] = ""
	data["DownloadURL"] = publicFilesRawURL(relPath)
	data["FileSize"] = int64(0)
	data["FileModTime"] = time.Time{}

	ctx := c.Request().Context()

	fileEntry, err := publicFilesStatFn(ctx, relPath)
	if err != nil {
		logger.Error("Error loading public file metadata", "path", relPath, "error", err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	if fileEntry.IsDir {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	data["FileName"] = fileEntry.Name
	data["FileSize"] = fileEntry.Size
	data["FileModTime"] = fileEntry.ModTime

	viewerType := filesViewerType(fileEntry.Name)

	var (
		fileData       []byte
		fileDataLoaded bool
	)

	if viewerType == "unknown" {
		fileData, _, err = publicFilesFetchFn(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching public file for preview", "path", relPath, "error", err)

			data["PreviewError"] = "Preview unavailable"
		} else {
			fileDataLoaded = true
			viewerType = filesViewerTypeWithContentFallback(fileEntry.Name, fileData)
		}
	}

	if filesViewerTypeIsEditable(viewerType) && !fileDataLoaded {
		fileData, _, err = publicFilesFetchFn(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching public file for preview", "path", relPath, "error", err)

			data["PreviewError"] = "Preview unavailable"
			viewerType = "unknown"
		} else {
			fileDataLoaded = true
		}
	}

	data["ViewerType"] = viewerType
	if isPublicPreviewViewerType(viewerType) {
		data["FileURL"] = publicFilesPreviewURL(relPath)
	}

	if filesViewerTypeIsEditable(viewerType) && fileDataLoaded {
		data["FileText"] = string(fileData)
	}

	t.HTML(http.StatusOK, "files_public_view")
}

// PublicFilesPreview serves safe inline previews for supported media types.
func PublicFilesPreview(c flamego.Context) {
	relPath, ok := sanitizePublicFilesPath(c.Param("path"))
	if !ok {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	ctx := c.Request().Context()

	fileEntry, err := publicFilesStatFn(ctx, relPath)
	if err != nil {
		logger.Error("Error loading public preview metadata", "path", relPath, "error", err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	if fileEntry.IsDir {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	viewerType := filesViewerType(fileEntry.Name)
	if !isPublicPreviewViewerType(viewerType) {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	rangeHeader := strings.TrimSpace(c.Request().Header.Get("Range"))
	ifRangeHeader := strings.TrimSpace(c.Request().Header.Get("If-Range"))

	fileStream, err := publicFilesOpenStreamFn(ctx, relPath, rangeHeader, ifRangeHeader)
	if err != nil {
		logger.Error("Error fetching public preview file", "path", relPath, "error", err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	defer func() {
		if err := fileStream.Reader.Close(); err != nil {
			logger.Error("Error closing public preview file stream", "path", relPath, "error", err)
		}
	}()

	filename := sanitizeFilenameForHeader(path.Base(relPath))

	headers := c.ResponseWriter().Header()
	headers.Set("Content-Type", normalizePublicPreviewContentType(fileStream.ContentType, viewerType))
	headers.Set("Content-Disposition", "inline; filename=\""+filename+"\"")
	headers.Set("X-Content-Type-Options", "nosniff")
	applyWebDAVStreamHeaders(headers, fileStream)

	statusCode := fileStream.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	c.ResponseWriter().WriteHeader(statusCode)

	if _, err := io.Copy(c.ResponseWriter(), fileStream.Reader); err != nil {
		logger.Error("Error writing public preview response", "path", relPath, "error", err)
	}
}

// PublicFilesRaw proxies file bytes from WEBDAV_PUBLIC_PATH.
func PublicFilesRaw(c flamego.Context) {
	relPath, ok := sanitizePublicFilesPath(c.Param("path"))
	if !ok {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	ctx := c.Request().Context()

	fileEntry, err := publicFilesStatFn(ctx, relPath)
	if err != nil {
		logger.Error("Error loading public file metadata", "path", relPath, "error", err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	if fileEntry.IsDir {
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	rangeHeader := strings.TrimSpace(c.Request().Header.Get("Range"))
	ifRangeHeader := strings.TrimSpace(c.Request().Header.Get("If-Range"))

	fileStream, err := publicFilesOpenStreamFn(ctx, relPath, rangeHeader, ifRangeHeader)
	if err != nil {
		logger.Error("Error fetching public file", "path", relPath, "error", err)
		c.ResponseWriter().WriteHeader(http.StatusNotFound)

		return
	}

	defer func() {
		if err := fileStream.Reader.Close(); err != nil {
			logger.Error("Error closing public file stream", "path", relPath, "error", err)
		}
	}()

	filename := sanitizeFilenameForHeader(path.Base(relPath))

	headers := c.ResponseWriter().Header()
	headers.Set("Content-Type", "application/octet-stream")
	headers.Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	headers.Set("X-Content-Type-Options", "nosniff")
	applyWebDAVStreamHeaders(headers, fileStream)

	statusCode := fileStream.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	c.ResponseWriter().WriteHeader(statusCode)

	if _, err := io.Copy(c.ResponseWriter(), fileStream.Reader); err != nil {
		logger.Error("Error writing public file response", "path", relPath, "error", err)
	}
}
func sanitizePublicFilesPath(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	decoded, ok := decodePublicFilesPath(raw)
	if !ok {
		return "", false
	}

	if strings.Contains(decoded, "\\") {
		return "", false
	}

	if strings.IndexFunc(decoded, isPublicFilesPathControlRune) >= 0 {
		return "", false
	}

	if strings.HasPrefix(decoded, "/") || strings.HasSuffix(decoded, "/") {
		return "", false
	}

	if strings.Contains(decoded, "//") {
		return "", false
	}

	segments := strings.Split(decoded, "/")
	cleaned := make([]string, 0, len(segments))

	for _, segment := range segments {
		if segment == "" {
			return "", false
		}

		if segment == "" || segment == "." || segment == ".." {
			return "", false
		}

		if strings.HasPrefix(segment, ".") {
			return "", false
		}

		if strings.IndexFunc(segment, isPublicFilesPathControlRune) >= 0 {
			return "", false
		}

		cleaned = append(cleaned, segment)
	}

	if len(cleaned) == 0 {
		return "", false
	}

	return strings.Join(cleaned, "/"), true
}

func decodePublicFilesPath(raw string) (string, bool) {
	decoded := raw

	// Decode repeatedly to collapse nested encodings such as %252f.
	for i := 0; i < maxPublicFilesPathDecodePasses && strings.Contains(decoded, "%"); i++ {
		next, err := url.PathUnescape(decoded)
		if err != nil {
			return "", false
		}

		if next == decoded {
			break
		}

		decoded = next
	}

	// Any remaining percent is rejected to avoid ambiguous path handling.
	if strings.Contains(decoded, "%") {
		return "", false
	}

	return decoded, true
}

func isPublicFilesPathControlRune(r rune) bool {
	return r < 0x20 || r == 0x7f
}

func publicFilesRawURL(relPath string) string {
	if relPath == "" {
		return "/f/raw"
	}

	segments := strings.Split(relPath, "/")
	escaped := make([]string, 0, len(segments))

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		escaped = append(escaped, url.PathEscape(segment))
	}

	return "/f/raw/" + strings.Join(escaped, "/")
}

func publicFilesPreviewURL(relPath string) string {
	if relPath == "" {
		return "/f/preview"
	}

	segments := strings.Split(relPath, "/")
	escaped := make([]string, 0, len(segments))

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		escaped = append(escaped, url.PathEscape(segment))
	}

	return "/f/preview/" + strings.Join(escaped, "/")
}

func isPublicPreviewViewerType(viewerType string) bool {
	switch viewerType {
	case "pdf", "image", "video", "audio":
		return true
	default:
		return false
	}
}

func normalizePublicPreviewContentType(contentType string, viewerType string) string {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	switch viewerType {
	case "pdf":
		return "application/pdf"
	case "image":
		if strings.HasPrefix(contentType, "image/") {
			return contentType
		}

		return "application/octet-stream"
	case "video":
		if strings.HasPrefix(contentType, "video/") {
			return contentType
		}

		return "application/octet-stream"
	case "audio":
		if strings.HasPrefix(contentType, "audio/") {
			return contentType
		}

		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}
