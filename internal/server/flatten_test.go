package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileFlattenMovesNestedFilesAndAvoidsCollisions(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	paths := map[string]string{
		"root.txt":             "root",
		"first/photo.jpg":      "first",
		"second/photo.jpg":     "second",
		"second/deep/document": "deep",
	}
	for name, content := range paths {
		filePath := filepath.Join(dataDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/files/flatten", strings.NewReader(`{"path":"/"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	HandleFileFlatten(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			Moved int `json:"moved"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 || result.Data.Moved != 3 {
		t.Fatalf("result = %+v", result)
	}
	for _, name := range []string{"photo.jpg", "photo (1).jpg", "document"} {
		if _, err := os.Stat(filepath.Join(dataDir, name)); err != nil {
			t.Fatalf("missing flattened file %q: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dataDir, "first")); !os.IsNotExist(err) {
		t.Fatalf("first directory still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "second")); !os.IsNotExist(err) {
		t.Fatalf("second directory still exists: %v", err)
	}
}
