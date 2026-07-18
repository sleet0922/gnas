package server

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxImportArchiveSize   int64 = 100 << 30
	maxExpandedArchiveSize int64 = 64 << 30
)

// HandleGalleryExport creates a ZIP containing user-managed files below dataDir.
// Internal state such as the database, thumbnail cache, and AI runtime is omitted.
func HandleGalleryExport(w http.ResponseWriter, r *http.Request) {
	tmp, err := os.CreateTemp("", ".gnas-gallery-*.zip")
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, "failed to create export")
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	zw := zip.NewWriter(tmp)
	err = filepath.WalkDir(dataDir, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == dataDir {
			return nil
		}
		rel, err := filepath.Rel(dataDir, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipArchivePath(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive contains symlink: %s", rel)
		}
		if entry.IsDir() {
			_, err = zw.Create(rel + "/")
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("archive contains unsupported file: %s", rel)
		}
		src, err := os.Open(filePath)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			src.Close()
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		out, err := zw.CreateHeader(header)
		if err == nil {
			_, err = io.Copy(out, src)
		}
		src.Close()
		return err
	})
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, "failed to create export")
		return
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, "failed to read export")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", formatContentDisposition("attachment", "gnas-gallery-"+time.Now().Format("20060102-150405")+".zip"))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	archive, err := os.Open(tmpPath)
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, "failed to read export")
		return
	}
	defer archive.Close()
	_, _ = io.Copy(w, archive)
}

// HandleGalleryImport accepts a ZIP and restores its user files below dataDir.
func HandleGalleryImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportArchiveSize)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeError(w, "failed to parse import file")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, "a ZIP file is required")
		return
	}
	defer file.Close()
	if header.Size > maxImportArchiveSize {
		writeError(w, "import file is too large")
		return
	}

	tmp, err := os.CreateTemp("", ".gnas-import-*.zip")
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, "failed to create temporary file")
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	written, err := io.Copy(tmp, io.LimitReader(file, maxImportArchiveSize+1))
	if err != nil {
		tmp.Close()
		writeError(w, "failed to save import file")
		return
	}
	if written > maxImportArchiveSize {
		tmp.Close()
		writeError(w, "import file is too large")
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, "failed to save import file")
		return
	}

	archive, err := zip.OpenReader(tmpPath)
	if err != nil {
		writeError(w, "invalid ZIP file")
		return
	}
	defer archive.Close()
	var expanded int64
	for _, entry := range archive.File {
		if err := validateArchiveEntry(entry); err != nil {
			writeError(w, err.Error())
			return
		}
		if entry.UncompressedSize64 > uint64(maxExpandedArchiveSize) || expanded > maxExpandedArchiveSize-int64(entry.UncompressedSize64) {
			writeError(w, "ZIP contents are too large")
			return
		}
		expanded += int64(entry.UncompressedSize64)
	}

	imported := 0
	for _, entry := range archive.File {
		destination, err := safePath(filepath.FromSlash(entry.Name))
		if err != nil {
			writeError(w, "ZIP contains an invalid path")
			return
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0755); err != nil {
				writeError(w, "failed to create import directory")
				return
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			writeError(w, "failed to create import directory")
			return
		}
		src, err := entry.Open()
		if err != nil {
			writeError(w, "failed to read ZIP entry")
			return
		}
		dst, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err == nil {
			_, err = io.Copy(dst, io.LimitReader(src, maxExpandedArchiveSize+1))
		}
		src.Close()
		if dst != nil {
			dst.Close()
		}
		if err != nil {
			writeError(w, "failed to write imported file")
			return
		}
		imported++
	}

	go GenerateAllThumbnails()
	writeOK(w, map[string]int{"imported": imported})
}

func shouldSkipArchivePath(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ".") || systemExcludes[part] {
			return true
		}
	}
	return false
}

func validateArchiveEntry(entry *zip.File) error {
	name := entry.Name
	if name == "" || strings.Contains(name, "\\") || path.IsAbs(name) || filepath.VolumeName(name) != "" {
		return fmt.Errorf("ZIP contains an invalid path")
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("ZIP contains an invalid path")
	}
	if shouldSkipArchivePath(clean) {
		return fmt.Errorf("ZIP contains a protected file or directory")
	}
	if entry.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("ZIP symlinks are not supported")
	}
	return nil
}
