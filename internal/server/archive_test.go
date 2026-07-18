package server

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGalleryExportAndImport(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()
	if err := os.MkdirAll(filepath.Join(dataDir, "photos"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "photos", "one.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "gnas.db"), []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	HandleGalleryExport(response, httptest.NewRequest(http.MethodGet, "/api/gallery/export", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", response.Code, response.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 { // directory and file
		t.Fatalf("export entries = %d, want 2", len(archive.File))
	}
	for _, entry := range archive.File {
		if entry.Name == "gnas.db" {
			t.Fatal("export included protected database")
		}
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "backup.zip")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(part)
	entry, err := zw.Create("restored/photo.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("restored"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/gallery/import", &body)
	request.Header.Set("Content-Type", mw.FormDataContentType())
	response = httptest.NewRecorder()
	HandleGalleryImport(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", response.Code, response.Body.String())
	}
	content, err := os.ReadFile(filepath.Join(dataDir, "restored", "photo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "restored" {
		t.Fatalf("imported content = %q", content)
	}
}

func TestGalleryImportRejectsTraversal(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "bad.zip")
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(part)
	if _, err := zw.Create("../escape.txt"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/gallery/import", &body)
	request.Header.Set("Content-Type", mw.FormDataContentType())
	response := httptest.NewRecorder()
	HandleGalleryImport(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("import status = %d, want 400", response.Code)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dataDir), "escape.txt")); err == nil {
		t.Fatal("traversal file was created")
	}
}
