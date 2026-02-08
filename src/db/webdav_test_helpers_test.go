// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"encoding/xml"
	"fmt"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	inventoryID := "GW-00001"
	if err := os.MkdirAll(filepath.Join(invDir, inventoryID), 0o755); err != nil {
		t.Fatalf("failed to create inventory dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(invDir, inventoryID, "manual.pdf"), []byte("pdf"), 0o644); err != nil {
		t.Fatalf("failed to write inventory file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(filesDir, "readme.txt"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(filesDir, "private"), 0o755); err != nil {
		t.Fatalf("failed to create private dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "private", ".gw_btg"), []byte("restricted"), 0o644); err != nil {
		t.Fatalf("failed to write break-glass marker: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(filesDir, "admin"), 0o755); err != nil {
		t.Fatalf("failed to create admin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "admin", ".gw_admin"), []byte("admin"), 0o644); err != nil {
		t.Fatalf("failed to write admin marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, ".hidden"), []byte("hidden"), 0o644); err != nil {
		t.Fatalf("failed to write hidden file: %v", err)
	}

	indexContent := "#+TITLE: Index\n:PROPERTIES:\n:ID: 11111111-1111-1111-1111-111111111111\n:END:\n[[id:22222222-2222-2222-2222-222222222222][Note One]]"
	if err := os.WriteFile(filepath.Join(zkDir, "index.org"), []byte(indexContent), 0o644); err != nil {
		t.Fatalf("failed to write index org: %v", err)
	}
	noteOneContent := "#+TITLE: Note One\n#+access: public\n:PROPERTIES:\n:ID: 22222222-2222-2222-2222-222222222222\n:END:\n[[id:33333333-3333-3333-3333-333333333333][Note Two]]\n[[https://groundwave.example.com/contact/44444444-4444-4444-4444-444444444444][Contact]]"
	if err := os.WriteFile(filepath.Join(zkDir, "20240101010101-note-one.org"), []byte(noteOneContent), 0o644); err != nil {
		t.Fatalf("failed to write note one: %v", err)
	}
	noteTwoContent := "#+TITLE: Note Two\n:PROPERTIES:\n:ID: 33333333-3333-3333-3333-333333333333\n:END:\nParagraph one.\n\nParagraph two."
	if err := os.WriteFile(filepath.Join(zkDir, "20240102020202-note-two.org"), []byte(noteTwoContent), 0o644); err != nil {
		t.Fatalf("failed to write note two: %v", err)
	}
	dailyContent := "#+TITLE: Daily\n\nParagraph one.\n\nParagraph two.\n\nParagraph three."
	if err := os.WriteFile(filepath.Join(zkDailyDir, "2024-01-01.org"), []byte(dailyContent), 0o644); err != nil {
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
	entries = append(entries, h.buildResponse(r.URL.Path, fsPath, info))

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
			entries = append(entries, h.buildResponse(href, itemPath, itemInfo))
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

func (h *simpleWebDAVHandler) buildResponse(href string, fsPath string, info os.FileInfo) davResponse {
	prop := davProp{
		LastModified: info.ModTime().UTC().Format(time.RFC1123),
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
