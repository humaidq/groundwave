/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

// FilesList renders the WebDAV files listing page.
func FilesList(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	relPath, ok := sanitizeFilesPath(c.Query("path"))
	if !ok {
		SetErrorFlash(s, "Invalid path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	setFilesBaseData(data, relPath)

	fileName := path.Base(relPath)
	fileURL := "/files/file?path=" + url.QueryEscape(relPath)
	data["FileName"] = fileName
	data["FileURL"] = fileURL
	data["DownloadURL"] = fileURL + "&download=1"
	data["FileSize"] = int64(0)
	data["FileModTime"] = time.Time{}

	ctx := c.Request().Context()

	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil {
		logger.Error("Error resolving admin state", "error", err)

		isAdmin = false
	}

	data["IsAdmin"] = isAdmin

	adminOnly, err := db.IsFilesPathAdminOnly(ctx, relPath)
	if err != nil {
		logger.Error("Error checking WebDAV admin restriction", "path", relPath, "error", err)

		data["Error"] = "Failed to load files. Please check your WEBDAV_FILES_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables."
		data["Entries"] = []db.WebDAVEntry{}

		t.HTML(http.StatusOK, "files")

		return
	}

	if adminOnly && !isAdmin {
		SetErrorFlash(s, "Access restricted")

		if relPath == "" {
			c.Redirect("/inventory", http.StatusSeeOther)
			return
		}

		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	data["IsAdminOnly"] = adminOnly

	restricted, err := db.IsFilesPathRestricted(ctx, relPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", relPath, "error", err)

		data["Error"] = "Failed to load files. Please check your WEBDAV_FILES_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables."
		data["Entries"] = []db.WebDAVEntry{}

		t.HTML(http.StatusOK, "files")

		return
	}

	if adminOnly {
		restricted = true
	}

	data["IsRestricted"] = restricted
	if restricted {
		data["PageRequiresSensitiveAccess"] = true
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	entries, err := db.ListFilesEntries(ctx, relPath)
	if err != nil {
		logger.Error("Error listing WebDAV files", "path", relPath, "error", err)

		data["Error"] = "Failed to load files. Please check your WEBDAV_FILES_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables."
		entries = []db.WebDAVEntry{}
	}

	if !isAdmin {
		filtered := make([]db.WebDAVEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir {
				entryAdminOnly, err := db.IsFilesPathAdminOnly(ctx, entry.Path)
				if err != nil {
					logger.Error("Error checking admin-only marker", "path", entry.Path, "error", err)
					continue
				}

				if entryAdminOnly {
					continue
				}
			}

			filtered = append(filtered, entry)
		}

		entries = filtered
	} else {
		for i := range entries {
			if !entries[i].IsDir {
				continue
			}

			entryAdminOnly, err := db.IsFilesPathAdminOnly(ctx, entries[i].Path)
			if err != nil {
				logger.Error("Error checking admin-only marker", "path", entries[i].Path, "error", err)
				continue
			}

			entries[i].IsAdminOnly = entryAdminOnly
		}
	}

	data["Entries"] = entries
	data["EntriesCount"] = len(entries)

	t.HTML(http.StatusOK, "files")
}

// FilesView renders a file viewer page for a WebDAV file.
func FilesView(c flamego.Context, s session.Session, t template.Template, data template.Data) {
	relPath, ok := sanitizeFilesPath(c.Query("path"))
	if !ok || relPath == "" {
		SetErrorFlash(s, "Invalid file path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	setFilesBaseData(data, relPath)

	fileName := path.Base(relPath)
	fileURL := "/files/file?path=" + url.QueryEscape(relPath)
	data["FileName"] = fileName
	data["FileURL"] = fileURL
	data["DownloadURL"] = fileURL + "&download=1"
	data["FileSize"] = int64(0)
	data["FileModTime"] = time.Time{}

	ctx := c.Request().Context()

	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil {
		logger.Error("Error resolving admin state", "error", err)

		isAdmin = false
	}

	data["IsAdmin"] = isAdmin

	dirPath := path.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	}

	adminOnly, err := db.IsFilesPathAdminOnly(ctx, dirPath)
	if err != nil {
		logger.Error("Error checking WebDAV admin restriction", "path", relPath, "error", err)
		SetErrorFlash(s, "Failed to load file")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	if adminOnly && !isAdmin {
		SetErrorFlash(s, "Access restricted")

		if dirPath == "" {
			c.Redirect("/inventory", http.StatusSeeOther)
			return
		}

		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	data["IsAdminOnly"] = adminOnly

	restricted, err := db.IsFilesPathRestricted(ctx, dirPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", relPath, "error", err)
		SetErrorFlash(s, "Failed to load file")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	if adminOnly {
		restricted = true
	}

	data["IsRestricted"] = restricted
	if restricted {
		data["PageRequiresSensitiveAccess"] = true
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	entries, err := db.ListFilesEntries(ctx, dirPath)
	if err != nil {
		logger.Error("Error listing WebDAV files", "path", dirPath, "error", err)

		data["Error"] = "Failed to load file. Please check your WEBDAV_FILES_PATH, WEBDAV_USERNAME, and WEBDAV_PASSWORD environment variables."
		data["Entries"] = []db.WebDAVEntry{}

		t.HTML(http.StatusOK, "files_view")

		return
	}

	var fileEntry *db.WebDAVEntry

	for i := range entries {
		if entries[i].Name == fileName {
			fileEntry = &entries[i]
			break
		}
	}

	if fileEntry == nil {
		SetErrorFlash(s, "File not found")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	if fileEntry.IsDir {
		c.Redirect(filesRedirectPath(relPath), http.StatusSeeOther)
		return
	}

	data["FileName"] = fileEntry.Name
	data["FileSize"] = fileEntry.Size
	data["FileModTime"] = fileEntry.ModTime

	viewerType := filesViewerType(fileEntry.Name)

	data["ViewerType"] = viewerType
	if viewerType == "text" || viewerType == "markdown" {
		fileData, _, err := db.FetchFilesFile(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching WebDAV file for preview", "path", relPath, "error", err)

			data["PreviewError"] = "Preview unavailable"
			data["ViewerType"] = "unknown"
		} else {
			data["FileText"] = string(fileData)
		}
	}

	t.HTML(http.StatusOK, "files_view")
}

// DownloadFilesFile proxies a file download from WebDAV files directory.
func DownloadFilesFile(c flamego.Context, s session.Session) {
	relPath, ok := sanitizeFilesPath(c.Query("path"))
	if !ok || relPath == "" {
		SetErrorFlash(s, "Invalid file path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	dirPath := path.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	}

	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil {
		logger.Error("Error resolving admin state", "error", err)

		isAdmin = false
	}

	adminOnly, err := db.IsFilesPathAdminOnly(ctx, dirPath)
	if err != nil {
		logger.Error("Error checking WebDAV admin restriction", "path", relPath, "error", err)
		SetErrorFlash(s, "Failed to load file")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	if adminOnly && !isAdmin {
		SetErrorFlash(s, "Access restricted")

		if dirPath == "" {
			c.Redirect("/inventory", http.StatusSeeOther)
			return
		}

		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	restricted, err := db.IsFilesPathRestricted(ctx, dirPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", relPath, "error", err)
		SetErrorFlash(s, "Failed to load file")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	if adminOnly {
		restricted = true
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	fileData, contentType, err := db.FetchFilesFile(ctx, relPath)
	if err != nil {
		logger.Error("Error fetching WebDAV file", "path", relPath, "error", err)
		SetErrorFlash(s, "File not found")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	filename := sanitizeFilenameForHeader(path.Base(relPath))

	contentDisposition := "inline"
	if isDownloadRequested(c.Query("download")) {
		contentDisposition = "attachment"
	}

	c.ResponseWriter().Header().Set("Content-Type", contentType)
	c.ResponseWriter().Header().Set("Content-Disposition", contentDisposition+"; filename=\""+filename+"\"")
	c.ResponseWriter().Header().Set("Content-Length", strconv.Itoa(len(fileData)))

	c.ResponseWriter().WriteHeader(http.StatusOK)

	if _, err := c.ResponseWriter().Write(fileData); err != nil {
		logger.Error("Error writing file response", "path", relPath, "error", err)
	}
}

func setFilesBaseData(data template.Data, relPath string) {
	data["IsFiles"] = true
	data["CurrentPath"] = relPath
	data["CurrentPathDisplay"] = formatFilesPathDisplay(relPath)
	data["HasParent"] = relPath != ""
	data["ParentPath"] = parentFilesPath(relPath)
	data["Breadcrumbs"] = buildFilesBreadcrumbs(relPath)
}

func buildFilesBreadcrumbs(relPath string) []BreadcrumbItem {
	if relPath == "" {
		return []BreadcrumbItem{{Name: "Files", URL: "/files", IsCurrent: true}}
	}

	segments := strings.Split(relPath, "/")
	crumbs := make([]BreadcrumbItem, 0, len(segments)+1)
	crumbs = append(crumbs, BreadcrumbItem{Name: "Files", URL: "/files", IsCurrent: false})

	current := ""

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		if current == "" {
			current = segment
		} else {
			current = current + "/" + segment
		}

		isCurrent := i == len(segments)-1

		urlPath := ""
		if !isCurrent {
			urlPath = "/files?path=" + url.QueryEscape(current)
		}

		crumbs = append(crumbs, BreadcrumbItem{Name: segment, URL: urlPath, IsCurrent: isCurrent})
	}

	return crumbs
}

func formatFilesPathDisplay(relPath string) string {
	if relPath == "" {
		return "/"
	}

	return "/" + relPath
}

func parentFilesPath(relPath string) string {
	if relPath == "" {
		return ""
	}

	parent := path.Dir(relPath)
	if parent == "." {
		return ""
	}

	return parent
}

func filesRedirectPath(relPath string) string {
	if relPath == "" {
		return "/files"
	}

	return "/files?path=" + url.QueryEscape(relPath)
}

func sanitizeFilesPath(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", true
	}

	if strings.Contains(raw, "\\") {
		return "", false
	}

	raw = strings.TrimPrefix(raw, "/")

	segments := strings.Split(raw, "/")

	cleaned := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if segment == "." || segment == ".." {
			return "", false
		}

		if strings.HasPrefix(segment, ".") {
			return "", false
		}

		cleaned = append(cleaned, segment)
	}

	return strings.Join(cleaned, "/"), true
}

func filesViewerType(filename string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(filename), "."))
	if ext == "" {
		return "unknown"
	}

	switch ext {
	case "pdf":
		return "pdf"
	case "jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "tif", "tiff":
		return "image"
	case "mp4", "mov", "mkv", "avi", "webm":
		return "video"
	case "mp3", "wav", "ogg", "m4a", "flac", "aac", "opus":
		return "audio"
	case "md", "markdown", "mdown", "mkd", "mkdn":
		return "markdown"
	case "txt", "log", "csv", "json", "xml", "yaml", "yml", "toml", "ini", "conf", "cfg":
		return "text"
	default:
		return "unknown"
	}
}

func isDownloadRequested(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes"
}
