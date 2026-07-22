package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestGalleryExportAndImportRestoresPortableCachesAndVectors(t *testing.T) {
	oldDataDir := dataDir
	oldQdrantURL := qdrantURL
	defer func() {
		dataDir = oldDataDir
		qdrantURL = oldQdrantURL
	}()

	sourceDir := t.TempDir()
	dataDir = sourceDir
	photoPath := filepath.Join(sourceDir, "photos", "one.jpg")
	if err := os.MkdirAll(filepath.Dir(photoPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(photoPath, []byte("original-image"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "gnas.db"), []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}
	for cachePath, content := range map[string]string{
		getThumbCachePath(photoPath):       "display-thumbnail",
		getVectorThumbCachePath(photoPath): "vector-thumbnail",
	} {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cachePath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	vector := make([]float32, embeddingDimension)
	vector[0] = 0.25
	var mu sync.Mutex
	var restoredPoints []struct {
		ID      string    `json:"id"`
		Vector  []float32 `json:"vector"`
		Payload struct {
			Path         string `json:"path"`
			Name         string `json:"name"`
			VectorSource string `json:"vector_source"`
		} `json:"payload"`
	}
	qdrant := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/"+collection:
			_, _ = w.Write([]byte(`{"result":{"config":{"params":{"vectors":{"size":2048}}}}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/points/scroll"):
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"result": map[string]interface{}{
					"points": []map[string]interface{}{
						{
							"id":     "source-id",
							"vector": vector,
							"payload": map[string]interface{}{
								"path":          photoPath,
								"vector_source": "thumbnail",
							},
						},
					},
					"next_page_offset": nil,
				},
			})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/points"):
			var payload struct {
				Points []struct {
					ID      string    `json:"id"`
					Vector  []float32 `json:"vector"`
					Payload struct {
						Path         string `json:"path"`
						Name         string `json:"name"`
						VectorSource string `json:"vector_source"`
					} `json:"payload"`
				} `json:"points"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			restoredPoints = append(restoredPoints, payload.Points...)
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			http.Error(w, "unexpected Qdrant request", http.StatusNotFound)
		}
	}))
	defer qdrant.Close()
	qdrantURL = qdrant.URL

	exportResponse := httptest.NewRecorder()
	HandleGalleryExport(exportResponse, httptest.NewRequest(http.MethodGet, "/api/gallery/export", nil))
	if exportResponse.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", exportResponse.Code, exportResponse.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(exportResponse.Body.Bytes()), int64(exportResponse.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	wantEntries := map[string]bool{
		"photos/one.jpg": false,
		migrationDisplayPrefix + "photos/one.jpg":     false,
		migrationVectorThumbPrefix + "photos/one.jpg": false,
		migrationVectorsEntry:                         false,
		migrationManifestEntry:                        false,
	}
	for _, entry := range archive.File {
		if _, wanted := wantEntries[entry.Name]; wanted {
			wantEntries[entry.Name] = true
		}
		if entry.Name == "gnas.db" {
			t.Fatal("export included protected database")
		}
	}
	for name, found := range wantEntries {
		if !found {
			t.Errorf("export is missing %s", name)
		}
	}

	destinationDir := t.TempDir()
	dataDir = destinationDir
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "migration.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(exportResponse.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/gallery/import", &body)
	request.Header.Set("Content-Type", mw.FormDataContentType())
	importResponse := httptest.NewRecorder()
	HandleGalleryImport(importResponse, request)
	if importResponse.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", importResponse.Code, importResponse.Body.String())
	}

	restoredPhoto := filepath.Join(destinationDir, "photos", "one.jpg")
	assertFileContent(t, restoredPhoto, "original-image")
	assertFileContent(t, getThumbCachePath(restoredPhoto), "display-thumbnail")
	assertFileContent(t, getVectorThumbCachePath(restoredPhoto), "vector-thumbnail")
	mu.Lock()
	defer mu.Unlock()
	if len(restoredPoints) != 1 {
		t.Fatalf("restored vector points = %d, want 1", len(restoredPoints))
	}
	point := restoredPoints[0]
	if point.Payload.Path != restoredPhoto || point.Payload.Name != "one.jpg" || point.Payload.VectorSource != "thumbnail" {
		t.Fatalf("restored vector payload = %+v", point.Payload)
	}
	if len(point.Vector) != embeddingDimension || point.Vector[0] != vector[0] {
		t.Fatalf("restored vector was changed: dimensions=%d first=%v", len(point.Vector), point.Vector[0])
	}
}

func TestGalleryImportSupportsLegacyArchive(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

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
	response := httptest.NewRecorder()
	HandleGalleryImport(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", response.Code, response.Body.String())
	}
	assertFileContent(t, filepath.Join(dataDir, "restored", "photo.txt"), "restored")
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

func assertFileContent(t *testing.T, name, want string) {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != want {
		t.Fatalf("%s content = %q, want %q", name, content, want)
	}
}
