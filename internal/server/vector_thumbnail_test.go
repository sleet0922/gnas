package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVectorThumbCachePathUsesFullSourcePath(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	first := getVectorThumbCachePath(filepath.Join(dataDir, "first", "photo.jpg"))
	second := getVectorThumbCachePath(filepath.Join(dataDir, "second", "photo.jpg"))
	if first == second {
		t.Fatalf("vector thumbnail paths collided: %s", first)
	}
	wantDir := filepath.Join(dataDir, internalStorageDir, vectorThumbnailCacheDirName)
	if filepath.Dir(first) != wantDir || filepath.Ext(first) != ".jpg" {
		t.Fatalf("unexpected vector thumbnail path: %s", first)
	}
}

func TestCleanThumbCacheRemovesDisplayAndVectorThumbnails(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	source := filepath.Join(dataDir, "photo.jpg")
	if err := os.WriteFile(source, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	cachePaths := []string{getThumbCachePath(source), getVectorThumbCachePath(source)}
	for _, cachePath := range cachePaths {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cachePath, []byte("cache"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cleanThumbCache(source)
	for _, cachePath := range cachePaths {
		if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
			t.Fatalf("cache still exists: %s, err=%v", cachePath, err)
		}
	}
}
