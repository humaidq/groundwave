/*
 * Copyright 2025 Humaid Alqasimi
 * SPDX-License-Identifier: Apache-2.0
 */
package routes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/flamego/template"

	"github.com/humaidq/groundwave/db"
)

const (
	filesUploadMaxBytes = 50 << 20

	filesLoadErrorMessage      = "Failed to load files"
	filesLoadFileErrorMessage  = "Failed to load file"
	filesFolderNotFoundMessage = "Folder not found"
	filesUploadErrorMessage    = "Failed to upload file"
)

var (
	filesIsPathAdminOnlyFn  = db.IsFilesPathAdminOnly
	filesIsPathRestrictedFn = db.IsFilesPathRestricted
	filesListEntriesFn      = db.ListFilesEntries
	filesOpenFileStreamFn   = db.OpenFilesFileStream
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

	adminOnly, err := filesIsPathAdminOnlyFn(ctx, relPath)
	if err != nil {
		if relPath != "" && errors.Is(err, db.ErrWebDAVFilesEntryNotFound) {
			SetErrorFlash(s, filesFolderNotFoundMessage)
			c.Redirect("/files", http.StatusSeeOther)

			return
		}

		logger.Error("Error checking WebDAV admin restriction", "path", relPath, "error", err)

		data["Error"] = filesLoadErrorMessage
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

	restricted, err := filesIsPathRestrictedFn(ctx, relPath)
	if err != nil {
		if relPath != "" && errors.Is(err, db.ErrWebDAVFilesEntryNotFound) {
			SetErrorFlash(s, filesFolderNotFoundMessage)
			c.Redirect("/files", http.StatusSeeOther)

			return
		}

		logger.Error("Error checking WebDAV restriction", "path", relPath, "error", err)

		data["Error"] = filesLoadErrorMessage
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

	entries, err := filesListEntriesFn(ctx, relPath)
	if err != nil {
		if relPath != "" && errors.Is(err, db.ErrWebDAVFilesEntryNotFound) {
			SetErrorFlash(s, filesFolderNotFoundMessage)
			c.Redirect("/files", http.StatusSeeOther)

			return
		}

		logger.Error("Error listing WebDAV files", "path", relPath, "error", err)

		data["Error"] = filesLoadErrorMessage
		entries = []db.WebDAVEntry{}
	}

	if !isAdmin {
		filtered := make([]db.WebDAVEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir {
				entryAdminOnly, err := filesIsPathAdminOnlyFn(ctx, entry.Path)
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

			entryAdminOnly, err := filesIsPathAdminOnlyFn(ctx, entries[i].Path)
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

	adminOnly, err := filesIsPathAdminOnlyFn(ctx, dirPath)
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

	restricted, err := filesIsPathRestrictedFn(ctx, dirPath)
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

	entries, err := filesListEntriesFn(ctx, dirPath)
	if err != nil {
		logger.Error("Error listing WebDAV files", "path", dirPath, "error", err)

		data["Error"] = filesLoadFileErrorMessage
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

	var (
		fileData       []byte
		fileDataLoaded bool
	)

	if viewerType == "unknown" {
		fileData, _, err = db.FetchFilesFile(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching WebDAV file for preview", "path", relPath, "error", err)

			data["PreviewError"] = "Preview unavailable"
		} else {
			fileDataLoaded = true
			viewerType = filesViewerTypeWithContentFallback(fileEntry.Name, fileData)
		}
	}

	if filesViewerTypeIsEditable(viewerType) && !fileDataLoaded {
		fileData, _, err = db.FetchFilesFile(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching WebDAV file for preview", "path", relPath, "error", err)

			data["PreviewError"] = "Preview unavailable"
			viewerType = "unknown"
		} else {
			fileDataLoaded = true
		}
	}

	data["ViewerType"] = viewerType
	data["CanEditFile"] = isAdmin && filesViewerTypeIsEditable(viewerType) && strings.TrimSpace(fileEntry.ETag) != ""

	if filesViewerTypeIsEditable(viewerType) && fileDataLoaded {
		data["FileText"] = string(fileData)
	}

	t.HTML(http.StatusOK, "files_view")
}

// FilesEditForm renders an editor for plaintext WebDAV files.
func FilesEditForm(c flamego.Context, s session.Session, t template.Template, data template.Data) {
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

	if !isAdmin {
		SetErrorFlash(s, "Access restricted")

		dirPath := path.Dir(relPath)
		if dirPath == "." {
			dirPath = ""
		}

		if dirPath == "" {
			c.Redirect("/inventory", http.StatusSeeOther)
			return
		}

		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	data["IsAdmin"] = true

	dirPath := path.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	}

	adminOnly, restricted, err := filesPathRestrictions(ctx, dirPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", relPath, "error", err)
		SetErrorFlash(s, "Failed to load file")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	data["IsAdminOnly"] = adminOnly
	data["IsRestricted"] = restricted

	if restricted {
		data["PageRequiresSensitiveAccess"] = true
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	entries, err := filesListEntriesFn(ctx, dirPath)
	if err != nil {
		logger.Error("Error listing WebDAV files", "path", dirPath, "error", err)
		SetErrorFlash(s, "Failed to load file")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

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

	viewerType := filesViewerType(fileEntry.Name)

	var (
		fileData       []byte
		fileDataLoaded bool
	)

	if viewerType == "unknown" {
		fileData, _, err = db.FetchFilesFile(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching WebDAV file for edit", "path", relPath, "error", err)
			SetErrorFlash(s, "Failed to load file")
			c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

			return
		}

		fileDataLoaded = true
		viewerType = filesViewerTypeWithContentFallback(fileEntry.Name, fileData)
	}

	if !filesViewerTypeIsEditable(viewerType) {
		SetErrorFlash(s, "Only plaintext files can be edited")
		c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

		return
	}

	fileETag, ok := sanitizeFilesETag(fileEntry.ETag)
	if !ok {
		SetErrorFlash(s, "Editing unavailable for this file")
		c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

		return
	}

	if !fileDataLoaded {
		fileData, _, err = db.FetchFilesFile(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching WebDAV file for edit", "path", relPath, "error", err)
			SetErrorFlash(s, "Failed to load file")
			c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

			return
		}
	}

	fileText, preserveTrailingNewline := maybeTrimFilesTrailingNewlineForEdit(
		string(fileData),
		isTruthyFormValue(c.Query("trim_eof_newline")),
	)

	data["ViewerType"] = viewerType
	data["FileName"] = fileEntry.Name
	data["FileSize"] = fileEntry.Size
	data["FileModTime"] = fileEntry.ModTime
	data["FileETag"] = fileETag
	data["FileText"] = fileText
	data["PreserveTrailingNewline"] = preserveTrailingNewline

	t.HTML(http.StatusOK, "files_edit")
}

// UpdateFilesFile saves edits to a plaintext WebDAV file.
func UpdateFilesFile(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing files edit form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relPath, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok || relPath == "" {
		SetErrorFlash(s, "Invalid file path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	fileETag, ok := sanitizeFilesETag(c.Request().FormValue("etag"))
	if !ok {
		SetErrorFlash(s, "Missing file version. Reload and try again")
		c.Redirect("/files/edit?path="+url.QueryEscape(relPath), http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	viewerType := filesViewerType(path.Base(relPath))
	if viewerType == "unknown" {
		fileData, _, err := db.FetchFilesFile(ctx, relPath)
		if err != nil {
			logger.Error("Error fetching WebDAV file for edit", "path", relPath, "error", err)
			SetErrorFlash(s, "Failed to load file")
			c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

			return
		}

		viewerType = filesViewerTypeWithContentFallback(path.Base(relPath), fileData)
	}

	if !filesViewerTypeIsEditable(viewerType) {
		SetErrorFlash(s, "Only plaintext files can be edited")
		c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

		return
	}

	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil {
		logger.Error("Error resolving admin state", "error", err)

		isAdmin = false
	}

	if !isAdmin {
		SetErrorFlash(s, "Access restricted")

		dirPath := path.Dir(relPath)
		if dirPath == "." {
			dirPath = ""
		}

		if dirPath == "" {
			c.Redirect("/inventory", http.StatusSeeOther)
			return
		}

		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	dirPath := path.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	}

	_, restricted, err := filesPathRestrictions(ctx, dirPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", dirPath, "error", err)
		SetErrorFlash(s, "Failed to save file")
		c.Redirect("/files/edit?path="+url.QueryEscape(relPath), http.StatusSeeOther)

		return
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	fileText := c.Request().FormValue("content")
	if isTruthyFormValue(c.Request().FormValue("preserve_trailing_newline")) && !strings.HasSuffix(fileText, "\n") {
		fileText += "\n"
	}

	err = db.UpdateFilesFile(ctx, relPath, []byte(fileText), fileETag)
	if err != nil {
		logger.Error("Error updating WebDAV file", "path", relPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryConflict):
			SetErrorFlash(s, "File changed since you opened it. Reload and try again")
			c.Redirect("/files/edit?path="+url.QueryEscape(relPath), http.StatusSeeOther)
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			SetErrorFlash(s, "File not found")
			c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)
		case errors.Is(err, db.ErrWebDAVFilesEntryIsDirectory):
			SetErrorFlash(s, "That path is a folder")
			c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)
		case errors.Is(err, db.ErrWebDAVFilesEntryETagRequired):
			SetErrorFlash(s, "Missing file version. Reload and try again")
			c.Redirect("/files/edit?path="+url.QueryEscape(relPath), http.StatusSeeOther)
		default:
			SetErrorFlash(s, "Failed to save file")
			c.Redirect("/files/edit?path="+url.QueryEscape(relPath), http.StatusSeeOther)
		}

		return
	}

	SetSuccessFlash(s, "File updated")
	c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)
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

	adminOnly, err := filesIsPathAdminOnlyFn(ctx, dirPath)
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

	restricted, err := filesIsPathRestrictedFn(ctx, dirPath)
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

	rangeHeader := strings.TrimSpace(c.Request().Header.Get("Range"))
	ifRangeHeader := strings.TrimSpace(c.Request().Header.Get("If-Range"))

	fileStream, err := filesOpenFileStreamFn(ctx, relPath, rangeHeader, ifRangeHeader)
	if err != nil {
		logger.Error("Error fetching WebDAV file", "path", relPath, "error", err)
		SetErrorFlash(s, "File not found")
		c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)

		return
	}

	defer func() {
		if err := fileStream.Reader.Close(); err != nil {
			logger.Error("Error closing WebDAV file stream", "path", relPath, "error", err)
		}
	}()

	filename := sanitizeFilenameForHeader(path.Base(relPath))

	downloadRequested := isDownloadRequested(c.Query("download"))

	contentDisposition := "inline"
	if downloadRequested {
		contentDisposition = "attachment"
	}

	responseContentType := fileResponseContentType(fileStream.ContentType, downloadRequested)

	headers := c.ResponseWriter().Header()
	headers.Set("Content-Type", responseContentType)
	headers.Set("Content-Disposition", contentDisposition+"; filename=\""+filename+"\"")
	applyWebDAVStreamHeaders(headers, fileStream)

	if downloadRequested {
		headers.Set("X-Content-Type-Options", "nosniff")
	}

	statusCode := fileStream.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	c.ResponseWriter().WriteHeader(statusCode)

	if _, err := io.Copy(c.ResponseWriter(), fileStream.Reader); err != nil {
		logger.Error("Error writing file response", "path", relPath, "error", err)
	}
}

// UploadFilesFile uploads a new file into the current files directory.
func UploadFilesFile(c flamego.Context, s session.Session) {
	if err := c.Request().ParseMultipartForm(filesUploadMaxBytes); err != nil {
		logger.Error("Error parsing files upload form", "error", err)
		SetErrorFlash(s, "Failed to parse upload form")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relDir, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok {
		SetErrorFlash(s, "Invalid upload path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	isAdmin, err := resolveSessionIsAdmin(ctx, s)
	if err != nil {
		logger.Error("Error resolving admin state", "error", err)

		isAdmin = false
	}

	adminOnly, restricted, err := filesPathRestrictions(ctx, relDir)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", relDir, "error", err)
		SetErrorFlash(s, filesUploadErrorMessage)
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	if adminOnly && !isAdmin {
		SetErrorFlash(s, "Access restricted")

		if relDir == "" {
			c.Redirect("/inventory", http.StatusSeeOther)
			return
		}

		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	file, header, err := c.Request().FormFile("upload_file")
	if err != nil {
		logger.Error("Error reading upload file", "error", err)
		SetErrorFlash(s, "No file uploaded or invalid file")
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			logger.Error("Error closing upload file", "error", err)
		}
	}()

	filename := strings.ReplaceAll(header.Filename, "\\", "/")
	filename = path.Base(filename)

	filename, ok = sanitizeFilesName(filename)
	if !ok {
		SetErrorFlash(s, "Invalid file name")
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	targetPath := filename
	if relDir != "" {
		targetPath = path.Join(relDir, filename)
	}

	if _, err := db.UploadFilesFile(ctx, targetPath, file, header.Size); err != nil {
		logger.Error("Error uploading WebDAV file", "path", targetPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryExists):
			SetErrorFlash(s, "A file with that name already exists")
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			SetErrorFlash(s, "Destination folder not found")
		default:
			SetErrorFlash(s, "Failed to upload file")
		}

		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Uploaded file")
	c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)
}

// CreateFilesTextFile creates a new plaintext file in the current files directory.
func CreateFilesTextFile(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing create-file form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relDir, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok {
		SetErrorFlash(s, "Invalid current path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	fileName, ok := sanitizeFilesName(c.Request().FormValue("file_name"))
	if !ok {
		SetErrorFlash(s, "Invalid file name")
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	if !isFilesPlaintextFilename(fileName) {
		SetErrorFlash(s, "Only plaintext files can be created here")
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	targetPath := fileName
	if relDir != "" {
		targetPath = path.Join(relDir, fileName)
	}

	ctx := c.Request().Context()

	_, restricted, err := filesPathRestrictions(ctx, relDir)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", relDir, "error", err)
		SetErrorFlash(s, "Failed to create file")
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	content := normalizeFilesPlaintextContentForCreate(c.Request().FormValue("content"))
	reader := strings.NewReader(content)

	if _, err := db.UploadFilesFile(ctx, targetPath, reader, int64(len(content))); err != nil {
		logger.Error("Error creating WebDAV text file", "path", targetPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryExists):
			SetErrorFlash(s, "A file with that name already exists")
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			SetErrorFlash(s, "Destination folder not found")
		default:
			SetErrorFlash(s, "Failed to create file")
		}

		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "File created")
	c.Redirect("/files/edit?path="+url.QueryEscape(targetPath)+"&trim_eof_newline=1", http.StatusSeeOther)
}

// CreateFilesDirectory creates a new directory in the current files directory.
func CreateFilesDirectory(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing create-directory form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relDir, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok {
		SetErrorFlash(s, "Invalid current path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	dirName, ok := sanitizeFilesName(c.Request().FormValue("dir_name"))
	if !ok {
		SetErrorFlash(s, "Invalid directory name")
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	targetPath := dirName
	if relDir != "" {
		targetPath = path.Join(relDir, dirName)
	}

	ctx := c.Request().Context()

	_, restricted, err := filesPathRestrictions(ctx, relDir)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", relDir, "error", err)
		SetErrorFlash(s, "Failed to create folder")
		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	if err := db.CreateFilesDirectory(ctx, targetPath); err != nil {
		logger.Error("Error creating WebDAV directory", "path", targetPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryExists):
			SetErrorFlash(s, "A folder with that name already exists")
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			SetErrorFlash(s, "Parent folder not found")
		default:
			SetErrorFlash(s, "Failed to create folder")
		}

		c.Redirect(filesRedirectPath(relDir), http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Folder created")
	c.Redirect(filesCreateDirectoryRedirectPath(relDir, c.Request().FormValue("redirect_to")), http.StatusSeeOther)
}

// RenameFilesEntry renames a file or folder within the same directory.
func RenameFilesEntry(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing rename form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relPath, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok || relPath == "" {
		SetErrorFlash(s, "Invalid path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	sourceEntry, entryFound, err := filesLookupEntry(ctx, relPath)
	if err != nil {
		logger.Error("Error loading WebDAV entry before rename", "path", relPath, "error", err)
		SetErrorFlash(s, "Failed to rename entry")
		c.Redirect(filesRedirectPath(parentFilesPath(relPath)), http.StatusSeeOther)

		return
	}

	entryIsDir := entryFound && sourceEntry.IsDir

	entryLabel := "file"
	if entryIsDir {
		entryLabel = "folder"
	}

	entryViewPath := "/files/view?path=" + url.QueryEscape(relPath)
	if entryIsDir {
		entryViewPath = filesRedirectPath(relPath)
	}

	newName, ok := sanitizeFilesName(c.Request().FormValue("new_name"))
	if !ok {
		if entryIsDir {
			SetErrorFlash(s, "Invalid folder name")
		} else {
			SetErrorFlash(s, "Invalid file name")
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	oldDir := parentFilesPath(relPath)

	destPath := newName
	if oldDir != "" {
		destPath = path.Join(oldDir, newName)
	}

	if destPath == relPath {
		if entryIsDir {
			SetWarningFlash(s, "Folder name unchanged")
		} else {
			SetWarningFlash(s, "File name unchanged")
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	restrictionPath := oldDir
	if entryIsDir {
		restrictionPath = relPath
	}

	_, restricted, err := filesPathRestrictions(ctx, restrictionPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", restrictionPath, "error", err)

		if entryIsDir {
			SetErrorFlash(s, "Failed to rename folder")
		} else {
			SetErrorFlash(s, "Failed to rename file")
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	if err := db.MoveFilesEntry(ctx, relPath, destPath); err != nil {
		logger.Error("Error renaming WebDAV entry", "path", relPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			if entryIsDir {
				SetErrorFlash(s, "Folder not found")
			} else {
				SetErrorFlash(s, "File not found")
			}
		case errors.Is(err, db.ErrWebDAVFilesEntryExists):
			if entryIsDir {
				SetErrorFlash(s, "A folder with that name already exists")
			} else {
				SetErrorFlash(s, "A file with that name already exists")
			}
		default:
			SetErrorFlash(s, "Failed to rename "+entryLabel)
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	if entryIsDir {
		SetSuccessFlash(s, "Folder renamed")
		c.Redirect(filesRedirectPath(destPath), http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "File renamed")
	c.Redirect("/files/view?path="+url.QueryEscape(destPath), http.StatusSeeOther)
}

// MoveFilesEntry moves a file or folder to a different directory.
func MoveFilesEntry(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing move form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relPath, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok || relPath == "" {
		SetErrorFlash(s, "Invalid path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	ctx := c.Request().Context()

	sourceEntry, entryFound, err := filesLookupEntry(ctx, relPath)
	if err != nil {
		logger.Error("Error loading WebDAV entry before move", "path", relPath, "error", err)
		SetErrorFlash(s, "Failed to move entry")
		c.Redirect(filesRedirectPath(parentFilesPath(relPath)), http.StatusSeeOther)

		return
	}

	entryIsDir := entryFound && sourceEntry.IsDir

	entryLabel := "file"
	if entryIsDir {
		entryLabel = "folder"
	}

	entryViewPath := "/files/view?path=" + url.QueryEscape(relPath)
	if entryIsDir {
		entryViewPath = filesRedirectPath(relPath)
	}

	targetDir, ok := sanitizeFilesPath(c.Request().FormValue("target_dir"))
	if !ok {
		SetErrorFlash(s, "Invalid destination path")
		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	sourceDir := parentFilesPath(relPath)

	fileName := path.Base(relPath)

	destPath := fileName
	if targetDir != "" {
		destPath = path.Join(targetDir, fileName)
	}

	if destPath == relPath {
		if entryIsDir {
			SetWarningFlash(s, "Folder already in that location")
		} else {
			SetWarningFlash(s, "File already in that folder")
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	sourceRestrictionPath := sourceDir
	if entryIsDir {
		sourceRestrictionPath = relPath
	}

	_, sourceRestricted, err := filesPathRestrictions(ctx, sourceRestrictionPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", sourceRestrictionPath, "error", err)

		if entryIsDir {
			SetErrorFlash(s, "Failed to move folder")
		} else {
			SetErrorFlash(s, "Failed to move file")
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	_, destRestricted, err := filesPathRestrictions(ctx, targetDir)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", targetDir, "error", err)

		if entryIsDir {
			SetErrorFlash(s, "Failed to move folder")
		} else {
			SetErrorFlash(s, "Failed to move file")
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	if (sourceRestricted || destRestricted) && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	if err := db.MoveFilesEntry(ctx, relPath, destPath); err != nil {
		logger.Error("Error moving WebDAV entry", "path", relPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			if entryIsDir {
				SetErrorFlash(s, "Folder not found")
			} else {
				SetErrorFlash(s, "File not found")
			}
		case errors.Is(err, db.ErrWebDAVFilesEntryExists):
			if entryIsDir {
				SetErrorFlash(s, "A folder with that name already exists")
			} else {
				SetErrorFlash(s, "A file with that name already exists")
			}
		default:
			SetErrorFlash(s, "Failed to move "+entryLabel)
		}

		c.Redirect(entryViewPath, http.StatusSeeOther)

		return
	}

	if entryIsDir {
		SetSuccessFlash(s, "Folder moved")
		c.Redirect(filesRedirectPath(destPath), http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "File moved")
	c.Redirect("/files/view?path="+url.QueryEscape(destPath), http.StatusSeeOther)
}

// DeleteFilesEntry deletes a file from the WebDAV files directory.
func DeleteFilesEntry(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing delete form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relPath, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok || relPath == "" {
		SetErrorFlash(s, "Invalid file path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	dirPath := path.Dir(relPath)
	if dirPath == "." {
		dirPath = ""
	}

	ctx := c.Request().Context()

	_, restricted, err := filesPathRestrictions(ctx, dirPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", dirPath, "error", err)
		SetErrorFlash(s, "Failed to delete file")
		c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

		return
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	if err := db.DeleteFilesFile(ctx, relPath); err != nil {
		logger.Error("Error deleting WebDAV file", "path", relPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			SetErrorFlash(s, "File not found")
		case errors.Is(err, db.ErrWebDAVFilesEntryIsDirectory):
			SetErrorFlash(s, "That path is a folder. Use Delete folder instead")
		default:
			SetErrorFlash(s, "Failed to delete file")
		}

		c.Redirect("/files/view?path="+url.QueryEscape(relPath), http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "File deleted")
	c.Redirect(filesRedirectPath(dirPath), http.StatusSeeOther)
}

// DeleteFilesDirectory deletes an empty directory from the WebDAV files directory.
func DeleteFilesDirectory(c flamego.Context, s session.Session) {
	if err := c.Request().ParseForm(); err != nil {
		logger.Error("Error parsing directory delete form", "error", err)
		SetErrorFlash(s, "Failed to parse form data")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	relPath, ok := sanitizeFilesPath(c.Request().FormValue("path"))
	if !ok || relPath == "" {
		SetErrorFlash(s, "Invalid folder path")
		c.Redirect("/files", http.StatusSeeOther)

		return
	}

	parentPath := path.Dir(relPath)
	if parentPath == "." {
		parentPath = ""
	}

	ctx := c.Request().Context()

	_, restricted, err := filesPathRestrictions(ctx, parentPath)
	if err != nil {
		logger.Error("Error checking WebDAV restriction", "path", parentPath, "error", err)
		SetErrorFlash(s, "Failed to delete folder")
		c.Redirect(filesRedirectPath(parentPath), http.StatusSeeOther)

		return
	}

	if restricted && !HasSensitiveAccess(s, time.Now()) {
		redirectToBreakGlass(c, s)
		return
	}

	if err := db.DeleteFilesDirectory(ctx, relPath); err != nil {
		logger.Error("Error deleting WebDAV folder", "path", relPath, "error", err)

		switch {
		case errors.Is(err, db.ErrWebDAVFilesEntryNotFound):
			SetErrorFlash(s, "Folder not found")
		case errors.Is(err, db.ErrWebDAVFilesDirectoryNotEmpty):
			SetErrorFlash(s, "Folder is not empty")
		case errors.Is(err, db.ErrWebDAVFilesEntryNotDirectory):
			SetErrorFlash(s, "That path is not a folder")
		default:
			SetErrorFlash(s, "Failed to delete folder")
		}

		c.Redirect(filesRedirectPath(parentPath), http.StatusSeeOther)

		return
	}

	SetSuccessFlash(s, "Folder deleted")
	c.Redirect(filesRedirectPath(parentPath), http.StatusSeeOther)
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

func filesLookupEntry(ctx context.Context, relPath string) (db.WebDAVEntry, bool, error) {
	dirPath := parentFilesPath(relPath)
	entryName := path.Base(relPath)

	entries, err := filesListEntriesFn(ctx, dirPath)
	if err != nil {
		return db.WebDAVEntry{}, false, fmt.Errorf("failed to list files entries: %w", err)
	}

	for _, entry := range entries {
		if entry.Name == entryName {
			return entry, true, nil
		}
	}

	return db.WebDAVEntry{}, false, nil
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

func sanitizeFilesName(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	if raw == "." || raw == ".." {
		return "", false
	}

	if strings.Contains(raw, "/") || strings.Contains(raw, "\\") {
		return "", false
	}

	if strings.HasPrefix(raw, ".") {
		return "", false
	}

	return raw, true
}

func sanitizeFilesRedirectTarget(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}

	if parsed.IsAbs() || parsed.Host != "" {
		return "", false
	}

	if parsed.Path != "/files" {
		return "", false
	}

	query := parsed.Query()
	if len(query) == 0 {
		return "/files", true
	}

	if len(query) != 1 {
		return "", false
	}

	relPath, ok := sanitizeFilesPath(query.Get("path"))
	if !ok {
		return "", false
	}

	return filesRedirectPath(relPath), true
}

func filesCreateDirectoryRedirectPath(relDir string, rawRedirectTarget string) string {
	if target, ok := sanitizeFilesRedirectTarget(rawRedirectTarget); ok {
		return target
	}

	return filesRedirectPath(relDir)
}

func filesPathRestrictions(ctx context.Context, dirPath string) (bool, bool, error) {
	adminOnly, err := filesIsPathAdminOnlyFn(ctx, dirPath)
	if err != nil {
		return false, false, fmt.Errorf("failed to check files admin-only restriction: %w", err)
	}

	restricted, err := filesIsPathRestrictedFn(ctx, dirPath)
	if err != nil {
		return false, false, fmt.Errorf("failed to check files break-glass restriction: %w", err)
	}

	if adminOnly {
		restricted = true
	}

	return adminOnly, restricted, nil
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

func filesViewerTypeWithContentFallback(filename string, content []byte) string {
	viewerType := filesViewerType(filename)
	if viewerType != "unknown" {
		return viewerType
	}

	if isFilesPlaintextContent(content) {
		return "text"
	}

	return "unknown"
}

func isFilesPlaintextContent(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	sample := content
	if len(sample) > 512 {
		sample = sample[:512]
	}

	contentType := strings.ToLower(strings.TrimSpace(http.DetectContentType(sample)))
	if strings.HasPrefix(contentType, "text/") {
		return true
	}

	switch contentType {
	case "application/json", "application/xml", "application/javascript":
		return true
	default:
		return false
	}
}

func filesViewerTypeIsEditable(viewerType string) bool {
	return viewerType == "text" || viewerType == "markdown"
}

func isFilesPlaintextFilename(filename string) bool {
	viewerType := filesViewerType(filename)
	if viewerType == "unknown" {
		return true
	}

	return filesViewerTypeIsEditable(viewerType)
}

func sanitizeFilesETag(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}

	if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return "", false
	}

	return value, true
}

func normalizeFilesPlaintextContentForCreate(raw string) string {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	if normalized != "" && !strings.HasSuffix(normalized, "\n") {
		normalized += "\n"
	}

	return normalized
}

func maybeTrimFilesTrailingNewlineForEdit(raw string, trim bool) (string, bool) {
	if !trim {
		return raw, false
	}

	if !strings.HasSuffix(raw, "\n") {
		return raw, false
	}

	return strings.TrimSuffix(raw, "\n"), true
}

func isDownloadRequested(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes"
}

func isPDFContentType(contentType string) bool {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	return contentType == "application/pdf"
}

func fileResponseContentType(contentType string, downloadRequested bool) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return "application/octet-stream"
	}

	if downloadRequested && isPDFContentType(contentType) {
		return "application/octet-stream"
	}

	return contentType
}

func applyWebDAVStreamHeaders(headers http.Header, stream db.WebDAVFileStream) {
	if stream.AcceptRanges != "" {
		headers.Set("Accept-Ranges", stream.AcceptRanges)
	}

	if stream.ContentRange != "" {
		headers.Set("Content-Range", stream.ContentRange)
	}

	if stream.ContentLength != "" {
		headers.Set("Content-Length", stream.ContentLength)
	}

	if stream.ETag != "" {
		headers.Set("ETag", stream.ETag)
	}

	if stream.LastModified != "" {
		headers.Set("Last-Modified", stream.LastModified)
	}
}
