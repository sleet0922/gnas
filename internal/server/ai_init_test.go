package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateAIStorageDirs(t *testing.T) {
	legacyNames := []string{"modelscope_cache", "qwen3_env", "qwen3_vl_ov", "tmp", "embed_server.log"}

	for _, prefix := range []string{"", "."} {
		t.Run(prefix+"legacy", func(t *testing.T) {
			oldDataDir := dataDir
			dataDir = t.TempDir()
			defer func() { dataDir = oldDataDir }()

			for _, name := range legacyNames[:4] {
				if err := os.MkdirAll(filepath.Join(dataDir, prefix+name), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dataDir, prefix+legacyNames[4]), []byte("log"), 0644); err != nil {
				t.Fatal(err)
			}

			migrateAIStorageDirs()
			for _, name := range legacyNames {
				if _, err := os.Lstat(filepath.Join(dataDir, internalStorageDir, name)); err != nil {
					t.Fatalf("missing migrated path %s: %v", name, err)
				}
				if _, err := os.Lstat(filepath.Join(dataDir, prefix+name)); !os.IsNotExist(err) {
					t.Fatalf("legacy path still exists %s: %v", prefix+name, err)
				}
			}
		})
	}
}

func TestMigrateAIStorageDirsLeavesExistingTarget(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	target := filepath.Join(dataDir, internalStorageDir, modelDirName)
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "marker"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dataDir, ".qwen3_vl_ov")
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}

	migrateAIStorageDirs()
	content, err := os.ReadFile(filepath.Join(target, "marker"))
	if err != nil || string(content) != "new" {
		t.Fatalf("existing target was changed: content=%q err=%v", content, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy directory should remain when target exists: %v", err)
	}
}

func TestMigrateAIStorageDirsCreatesContainerWithoutLegacyData(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	migrateAIStorageDirs()
	if info, err := os.Stat(filepath.Join(dataDir, internalStorageDir)); err != nil || !info.IsDir() {
		t.Fatalf("missing internal storage container: info=%v err=%v", info, err)
	}
}

func TestMigrateAIStorageDirsMovesThumbnailCache(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	legacy := filepath.Join(dataDir, ".thumbs")
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "sl_photo.jpg"), []byte("thumb"), 0644); err != nil {
		t.Fatal(err)
	}

	migrateAIStorageDirs()
	target := filepath.Join(dataDir, internalStorageDir, thumbnailCacheDirName, "sl_photo.jpg")
	if content, err := os.ReadFile(target); err != nil || string(content) != "thumb" {
		t.Fatalf("thumbnail cache was not migrated: content=%q err=%v", content, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy thumbnail cache still exists: %v", err)
	}
}

func TestRepairVirtualEnvPaths(t *testing.T) {
	oldDataDir := dataDir
	dataDir = t.TempDir()
	defer func() { dataDir = oldDataDir }()

	envPath := filepath.Join(dataDir, internalStorageDir, envDirName)
	if err := os.MkdirAll(filepath.Join(envPath, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(dataDir, envDirName)
	script := "#!" + filepath.Join(legacyPath, "bin", "python3") + "\nprint('ok')\n"
	scriptPath := filepath.Join(envPath, "bin", "pip")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	repairVirtualEnvPaths(envPath)
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "#!" + filepath.Join(envPath, "bin", "python3")
	if string(content)[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("shebang was not repaired: %q", content)
	}
}
