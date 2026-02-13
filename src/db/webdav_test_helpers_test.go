// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"encoding/xml"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type webdavTestServer struct {
	server      *httptest.Server
	rootDir     string
	invDir      string
	filesDir    string
	zkDir       string
	zKDailyDir  string
	inventoryID string
}

func newWebDAVTestServer(t *testing.T) *webdavTestServer {
	t.Helper()

	root := t.TempDir()
	invDir := filepath.Join(root, "inv")
	filesDir := filepath.Join(root, "files")
	zkDir := filepath.Join(root, "zk")
	zkDailyDir := filepath.Join(zkDir, "daily")

	for _, dir := range []string{invDir, filesDir, zkDir, zkDailyDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	inventoryID := "GW-00001"
	if err := os.MkdirAll(filepath.Join(invDir, inventoryID), 0o750); err != nil {
		t.Fatalf("failed to create inventory dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(invDir, inventoryID, "manual.pdf"), []byte("pdf"), 0o600); err != nil {
		t.Fatalf("failed to write inventory file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(filesDir, "readme.txt"), []byte("readme"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(filesDir, "uploads"), 0o750); err != nil {
		t.Fatalf("failed to create uploads dir: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(filesDir, "archive"), 0o750); err != nil {
		t.Fatalf("failed to create archive dir: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(filesDir, "private"), 0o750); err != nil {
		t.Fatalf("failed to create private dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(filesDir, "private", ".gw_btg"), []byte("restricted"), 0o600); err != nil {
		t.Fatalf("failed to write break-glass marker: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(filesDir, "admin"), 0o750); err != nil {
		t.Fatalf("failed to create admin dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(filesDir, "admin", ".gw_admin"), []byte("admin"), 0o600); err != nil {
		t.Fatalf("failed to write admin marker: %v", err)
	}

	if err := os.WriteFile(filepath.Join(filesDir, ".hidden"), []byte("hidden"), 0o600); err != nil {
		t.Fatalf("failed to write hidden file: %v", err)
	}

	indexContent := "#+TITLE: Index\n:PROPERTIES:\n:ID: 11111111-1111-1111-1111-111111111111\n:END:\n[[id:22222222-2222-2222-2222-222222222222][Note One]]"
	if err := os.WriteFile(filepath.Join(zkDir, "index.org"), []byte(indexContent), 0o600); err != nil {
		t.Fatalf("failed to write index org: %v", err)
	}

	homeContent := "#+TITLE: Home Index\n#+access: home\n:PROPERTIES:\n:ID: 66666666-6666-6666-6666-666666666666\n:END:\n[[id:22222222-2222-2222-2222-222222222222][Note One]]\n[[id:33333333-3333-3333-3333-333333333333][Note Two]]"
	if err := os.WriteFile(filepath.Join(zkDir, "home.org"), []byte(homeContent), 0o600); err != nil {
		t.Fatalf("failed to write home org: %v", err)
	}

	noteOneContent := "#+TITLE: Note One\n#+access: public\n:PROPERTIES:\n:ID: 22222222-2222-2222-2222-222222222222\n:END:\n[[id:33333333-3333-3333-3333-333333333333][Note Two]]\n[[https://groundwave.example.com/contact/44444444-4444-4444-4444-444444444444][Contact]]"
	if err := os.WriteFile(filepath.Join(zkDir, "20240101010101-note-one.org"), []byte(noteOneContent), 0o600); err != nil {
		t.Fatalf("failed to write note one: %v", err)
	}

	noteTwoContent := "#+TITLE: Note Two\n:PROPERTIES:\n:ID: 33333333-3333-3333-3333-333333333333\n:END:\nParagraph one.\n\nParagraph two."
	if err := os.WriteFile(filepath.Join(zkDir, "20240102020202-note-two.org"), []byte(noteTwoContent), 0o600); err != nil {
		t.Fatalf("failed to write note two: %v", err)
	}

	dailyContent := "#+TITLE: Daily\n\nParagraph one.\n\nMet [[https://groundwave.example.com/contact/44444444-4444-4444-4444-444444444444][Contact]].\n\nParagraph three."
	if err := os.WriteFile(filepath.Join(zkDailyDir, "2024-01-01.org"), []byte(dailyContent), 0o600); err != nil {
		t.Fatalf("failed to write daily note: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/inv/", &simpleWebDAVHandler{prefix: "/inv", rootDir: invDir})
	mux.Handle("/files/", &simpleWebDAVHandler{prefix: "/files", rootDir: filesDir})
	mux.Handle("/zk/", &simpleWebDAVHandler{prefix: "/zk", rootDir: zkDir})

	server := httptest.NewServer(mux)

	return &webdavTestServer{
		server:      server,
		rootDir:     root,
		invDir:      invDir,
		filesDir:    filesDir,
		zkDir:       zkDir,
		zKDailyDir:  zkDailyDir,
		inventoryID: inventoryID,
	}
}

func (s *webdavTestServer) close() {
	if s.server != nil {
		s.server.Close()
	}
}

type simpleWebDAVHandler struct {
	prefix  string
	rootDir string
}

func (h *simpleWebDAVHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, h.prefix)
	if relPath == "" {
		relPath = "/"
	}

	relPath = strings.TrimPrefix(relPath, "/")

	fsPath := h.rootDir
	if relPath != "" {
		fsPath = filepath.Join(h.rootDir, relPath)
	}

	switch r.Method {
	case "PROPFIND":
		h.handlePropFind(w, r, fsPath)
	case http.MethodGet:
		handleGetFile(w, r, fsPath)
	case http.MethodPut:
		h.handlePut(w, r, fsPath)
	case http.MethodDelete:
		h.handleDelete(w, r, fsPath)
	case "MOVE":
		h.handleMove(w, r, fsPath)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleGetFile(w http.ResponseWriter, r *http.Request, fsPath string) {
	if _, err := os.Stat(fsPath); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, fsPath)
}

func (h *simpleWebDAVHandler) handlePut(w http.ResponseWriter, r *http.Request, fsPath string) {
	if strings.HasSuffix(r.URL.Path, "/") {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	parent := filepath.Dir(fsPath)
	if _, err := os.Stat(parent); err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(fsPath, body, 0o600); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *simpleWebDAVHandler) handleDelete(w http.ResponseWriter, r *http.Request, fsPath string) {
	if _, err := os.Stat(fsPath); err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := os.RemoveAll(fsPath); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *simpleWebDAVHandler) handleMove(w http.ResponseWriter, r *http.Request, fsPath string) {
	destHeader := r.Header.Get("Destination")
	if destHeader == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	destPath, err := h.resolveDestinationPath(destHeader)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(fsPath); err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	overwrite := strings.ToUpper(strings.TrimSpace(r.Header.Get("Overwrite")))
	if overwrite == "" {
		overwrite = "T"
	}

	if overwrite == "F" {
		if _, err := os.Stat(destPath); err == nil {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
	}

	if err := h.ensureParentExists(destPath); err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if overwrite != "F" {
		_ = os.RemoveAll(destPath)
	}

	if err := os.Rename(fsPath, destPath); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *simpleWebDAVHandler) resolveDestinationPath(destHeader string) (string, error) {
	parsed, err := url.Parse(destHeader)
	if err != nil {
		return "", err
	}

	pathValue := parsed.Path
	if pathValue == "" {
		pathValue = destHeader
	}
	pathValue = strings.TrimSuffix(pathValue, "/")

	if !strings.HasPrefix(pathValue, h.prefix) {
		return "", fmt.Errorf("destination outside prefix")
	}

	relPath := strings.TrimPrefix(pathValue, h.prefix)
	relPath = strings.TrimPrefix(relPath, "/")
	if relPath == "" {
		return "", fmt.Errorf("empty destination path")
	}

	return filepath.Join(h.rootDir, relPath), nil
}

func (h *simpleWebDAVHandler) ensureParentExists(destPath string) error {
	parent := filepath.Dir(destPath)
	_, err := os.Stat(parent)
	return err
}

type davMultiStatus struct {
	XMLName  xml.Name      `xml:"DAV: multistatus"`
	Response []davResponse `xml:"response"`
}

type davResponse struct {
	Href     string      `xml:"href"`
	PropStat davPropStat `xml:"propstat"`
}

type davPropStat struct {
	Prop   davProp `xml:"prop"`
	Status string  `xml:"status"`
}

type davProp struct {
	ResourceType  davResourceType `xml:"resourcetype"`
	ContentLength *int64          `xml:"getcontentlength,omitempty"`
	LastModified  string          `xml:"getlastmodified,omitempty"`
	ContentType   string          `xml:"getcontenttype,omitempty"`
	ETag          string          `xml:"getetag,omitempty"`
}

type davResourceType struct {
	Collection *struct{} `xml:"collection,omitempty"`
}

func (h *simpleWebDAVHandler) handlePropFind(w http.ResponseWriter, r *http.Request, fsPath string) {
	info, err := os.Stat(fsPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	entries := []davResponse{}
	entries = append(entries, h.buildResponse(r.URL.Path, info))

	if info.IsDir() {
		items, err := os.ReadDir(fsPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, item := range items {
			itemPath := filepath.Join(fsPath, item.Name())

			itemInfo, err := os.Stat(itemPath)
			if err != nil {
				continue
			}

			href := strings.TrimSuffix(r.URL.Path, "/") + "/" + item.Name()
			if itemInfo.IsDir() {
				href += "/"
			}

			entries = append(entries, h.buildResponse(href, itemInfo))
		}
	}

	ms := davMultiStatus{Response: entries}

	output, err := xml.Marshal(ms)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(output)
}

func (h *simpleWebDAVHandler) buildResponse(href string, info os.FileInfo) davResponse {
	prop := davProp{
		LastModified: info.ModTime().UTC().Format(http.TimeFormat),
	}
	if info.IsDir() {
		prop.ResourceType = davResourceType{Collection: &struct{}{}}
	} else {
		size := info.Size()
		prop.ContentLength = &size
		prop.ResourceType = davResourceType{}
		prop.ContentType = mime.TypeByExtension(filepath.Ext(info.Name()))
		prop.ETag = fmt.Sprintf("\"%d\"", info.ModTime().UnixNano())
	}

	return davResponse{
		Href: href,
		PropStat: davPropStat{
			Prop:   prop,
			Status: "HTTP/1.1 200 OK",
		},
	}
}
