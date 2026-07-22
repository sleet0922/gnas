package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// HandleFileFlatten moves files from a directory tree into the data root.
// Existing names are preserved by adding a numeric suffix instead of overwriting.
func HandleFileFlatten(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path string `json:"path"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
			writeError(w, "invalid request")
			return
		}
	}
	if payload.Path == "" {
		payload.Path = "/"
	}

	source, err := safePath(payload.Path)
	if err != nil {
		writeError(w, "invalid path")
		return
	}
	sourceInfo, err := os.Stat(source)
	if err != nil || !sourceInfo.IsDir() {
		writeError(w, "source must be a directory")
		return
	}
	if sourceEntry, err := os.Lstat(source); err != nil || sourceEntry.Mode()&os.ModeSymlink != 0 {
		writeError(w, "symbolic link directories are not supported")
		return
	}

	rootAbs, _ := filepath.Abs(dataDir)
	sourceAbs, _ := filepath.Abs(source)
	rootRel, _ := relativePathWithin(rootAbs, sourceAbs)
	flattenRoot := rootRel == "."

	var files []string
	var dirs []string
	err = filepath.WalkDir(source, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == source {
			return nil
		}
		rel, err := filepath.Rel(source, current)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		if shouldSkipArchivePath(relSlash) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			dirs = append(dirs, current)
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		// Files already directly in the data root are not part of the flatten operation.
		if flattenRoot && !strings.Contains(rel, string(filepath.Separator)) {
			return nil
		}
		files = append(files, current)
		return nil
	})
	if err != nil {
		writeError(w, "failed to scan directory")
		return
	}

	moved := 0
	failed := make([]string, 0)
	for _, sourceFile := range files {
		name := filepath.Base(sourceFile)
		destination := uniqueFlattenDestination(rootAbs, name)
		cleanThumbCache(sourceFile)
		deleteQdrantVectorsForPath(sourceFile, false)
		if err := os.Rename(sourceFile, destination); err != nil {
			clearDeletedEmbeddingPath(sourceFile)
			failed = append(failed, sourceFile)
			continue
		}
		moved++
	}

	removedDirs := 0
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Remove(dirs[i]); err == nil {
			removedDirs++
		}
	}
	if !flattenRoot {
		if err := os.Remove(source); err == nil {
			removedDirs++
		}
	}
	if moved > 0 {
		go GenerateAllThumbnails()
	}

	result := map[string]interface{}{
		"moved":       moved,
		"failed":      failed,
		"removedDirs": removedDirs,
	}
	if len(failed) > 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code":    1,
			"message": fmt.Sprintf("moved %d files, %d failed", moved, len(failed)),
			"data":    result,
		})
		return
	}
	writeOK(w, result)
}

func uniqueFlattenDestination(root, name string) string {
	candidate := filepath.Join(root, name)
	if _, err := os.Lstat(candidate); os.IsNotExist(err) {
		return candidate
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate = filepath.Join(root, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
