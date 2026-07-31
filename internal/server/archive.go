package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	maxImportArchiveSize   int64 = 100 << 30
	maxExpandedArchiveSize int64 = 64 << 30

	migrationArchiveDir        = "_gnas_migration"
	migrationManifestEntry     = migrationArchiveDir + "/manifest.json"
	migrationVectorsEntry      = migrationArchiveDir + "/vectors.ndjson"
	migrationDisplayPrefix     = migrationArchiveDir + "/display_thumbs/"
	migrationVectorThumbPrefix = migrationArchiveDir + "/vector_thumbs/"
	migrationFormatVersion     = 1
)

type migrationManifest struct {
	Version       int       `json:"version"`
	ExportedAt    time.Time `json:"exported_at"`
	DisplayThumbs int       `json:"display_thumbnails"`
	VectorThumbs  int       `json:"vector_thumbnails"`
	Vectors       int       `json:"vectors"`
}

type portableVectorRecord struct {
	Path   string    `json:"path"`
	Vector []float32 `json:"vector"`
}

type archiveMediaFile struct {
	absPath string
	relPath string
}

// HandleGalleryExport creates a portable ZIP containing user files, both
// thumbnail caches, and Qdrant vectors. Runtime dependencies and credentials
// are intentionally excluded.
func HandleGalleryExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", formatContentDisposition("attachment", "gnas-gallery-"+time.Now().Format("20060102-150405")+".zip"))
	w.Header().Set("Cache-Control", "no-store")

	started := time.Now()
	log.Printf("[导出] 开始生成完整迁移 ZIP...")
	tmp, err := createGalleryExportTemp()
	if err != nil {
		log.Printf("[导出] 无法创建临时文件: %v", err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	zw := zip.NewWriter(tmp)
	mediaFiles, err := writeUserFilesToArchive(zw)
	manifest := migrationManifest{Version: migrationFormatVersion, ExportedAt: time.Now().UTC()}
	if err == nil {
		manifest.DisplayThumbs, manifest.VectorThumbs, err = writeThumbnailCachesToArchive(zw, mediaFiles)
	}
	if err == nil {
		manifest.Vectors, err = writeVectorsToArchive(zw)
	}
	if err == nil {
		err = writeJSONArchiveEntry(zw, migrationManifestEntry, manifest)
	}
	if closeErr := zw.Close(); err == nil {
		err = closeErr
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		log.Printf("[导出] 生成迁移 ZIP 失败: %v", err)
		return
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		log.Printf("[导出] 无法读取迁移 ZIP: %v", err)
		return
	}
	log.Printf("[导出] 迁移 ZIP 已生成，大小 %d 字节，开始传输...", info.Size())
	archive, err := os.Open(tmpPath)
	if err != nil {
		log.Printf("[导出] 无法打开迁移 ZIP: %v", err)
		return
	}
	defer archive.Close()
	written, copyErr := io.Copy(w, archive)
	if copyErr != nil {
		log.Printf("[导出] 传输中断，临时文件将删除: %v", copyErr)
		return
	}
	log.Printf("[导出] 迁移 ZIP 传输完成: %d 字节 (生成及传输耗时: %s)", written, time.Since(started).Round(time.Second))
}

func createGalleryExportTemp() (*os.File, error) {
	// 优先使用数据目录下的内部存储目录，避免占用小型 tmpfs；
	// 开发或非特权环境下回退到系统临时目录。
	exportDir := filepath.Join(dataDir, internalStorageDir)
	if err := os.MkdirAll(exportDir, 0755); err == nil {
		if tmp, err := os.CreateTemp(exportDir, ".gnas-gallery-*.zip"); err == nil {
			return tmp, nil
		}
	}
	return os.CreateTemp("", ".gnas-gallery-*.zip")
}

// CleanupGalleryExportTemps removes incomplete archives left by a process
// crash or forced service restart. Completed and interrupted HTTP transfers
// are removed by HandleGalleryExport itself.
func CleanupGalleryExportTemps() {
	// 清理数据目录内部存储中的残留
	if matches, err := filepath.Glob(filepath.Join(dataDir, internalStorageDir, ".gnas-gallery-*.zip")); err == nil {
		for _, name := range matches {
			if err := os.Remove(name); err == nil {
				log.Printf("[导出] 已清理上次未完成的临时文件: %s", name)
			}
		}
	}
	// 同时清理系统临时目录下的残留
	if matches, err := filepath.Glob(filepath.Join(os.TempDir(), ".gnas-gallery-*.zip")); err == nil {
		for _, name := range matches {
			if err := os.Remove(name); err == nil {
				log.Printf("[导出] 已清理上次未完成的临时文件: %s", name)
			}
		}
	}
}

func writeUserFilesToArchive(zw *zip.Writer) ([]archiveMediaFile, error) {
	mediaFiles := make([]archiveMediaFile, 0)
	err := filepath.WalkDir(dataDir, func(filePath string, entry os.DirEntry, walkErr error) error {
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
		method := uint16(zip.Deflate)
		if isImageExt(filePath) || isVideoExt(filePath) {
			method = zip.Store
		}
		if err := writeFileToArchive(zw, filePath, rel, method); err != nil {
			return err
		}
		if isImageExt(filePath) || isVideoExt(filePath) {
			mediaFiles = append(mediaFiles, archiveMediaFile{absPath: filePath, relPath: rel})
		}
		return nil
	})
	return mediaFiles, err
}

func writeThumbnailCachesToArchive(zw *zip.Writer, mediaFiles []archiveMediaFile) (int, int, error) {
	displayCount := 0
	vectorCount := 0
	for _, media := range mediaFiles {
		displayPath := getVideoThumbCachePath(media.absPath)
		if isImageExt(media.absPath) {
			displayPath = getThumbCachePath(media.absPath)
		}
		if regularFileExists(displayPath) {
			if err := writeFileToArchive(zw, displayPath, migrationDisplayPrefix+media.relPath, zip.Store); err != nil {
				return displayCount, vectorCount, err
			}
			displayCount++
		}
		if isImageExt(media.absPath) {
			vectorPath := getVectorThumbCachePath(media.absPath)
			if regularFileExists(vectorPath) {
				if err := writeFileToArchive(zw, vectorPath, migrationVectorThumbPrefix+media.relPath, zip.Store); err != nil {
					return displayCount, vectorCount, err
				}
				vectorCount++
			}
		}
	}
	return displayCount, vectorCount, nil
}

func writeVectorsToArchive(zw *zip.Writer) (int, error) {
	out, err := createArchiveEntry(zw, migrationVectorsEntry, zip.Deflate)
	if err != nil {
		return 0, err
	}
	encoder := json.NewEncoder(out)
	count := 0
	var offset json.RawMessage
	client := &http.Client{Timeout: 30 * time.Second}
	for {
		payload := map[string]interface{}{
			"limit":        64,
			"with_vector":  true,
			"with_payload": true,
		}
		if len(offset) > 0 && string(offset) != "null" {
			payload["offset"] = offset
		}
		body, _ := json.Marshal(payload)
		resp, err := client.Post(fmt.Sprintf("%s/collections/%s/points/scroll", qdrantURL, collection), "application/json", bytes.NewReader(body))
		if err != nil {
			return count, fmt.Errorf("read vectors from Qdrant: %w", err)
		}
		if resp.StatusCode >= 300 {
			responseBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return count, fmt.Errorf("read vectors from Qdrant: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
		}
		var result struct {
			Result struct {
				Points         []qdrantPathPoint `json:"points"`
				NextPageOffset json.RawMessage   `json:"next_page_offset"`
			} `json:"result"`
		}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if err != nil {
			return count, fmt.Errorf("decode vectors from Qdrant: %w", err)
		}
		for _, point := range result.Result.Points {
			if len(point.Vector) != embeddingDimension || point.Payload.Path == "" || point.Payload.VectorSource != "thumbnail" {
				continue
			}
			rel, ok := portableRelativePath(point.Payload.Path)
			if !ok || !isImageExt(rel) {
				continue
			}
			if err := encoder.Encode(portableVectorRecord{Path: rel, Vector: point.Vector}); err != nil {
				return count, err
			}
			count++
		}
		if len(result.Result.NextPageOffset) == 0 || string(result.Result.NextPageOffset) == "null" || len(result.Result.Points) == 0 {
			break
		}
		offset = result.Result.NextPageOffset
	}
	return count, nil
}

// HandleGalleryImport accepts both legacy user-only ZIP files and portable
// migration ZIP files produced by HandleGalleryExport.
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
	manifest, migrationEntries, err := validateImportArchive(archive.File)
	if err != nil {
		writeError(w, err.Error())
		return
	}

	imported := 0
	for _, entry := range archive.File {
		if isMigrationEntry(entry.Name) {
			continue
		}
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
		if err := extractArchiveFile(entry, destination); err != nil {
			writeError(w, "failed to write imported file")
			return
		}
		imported++
	}

	restoredThumbs := 0
	for _, entry := range migrationEntries {
		if strings.HasPrefix(entry.Name, migrationDisplayPrefix) || strings.HasPrefix(entry.Name, migrationVectorThumbPrefix) {
			if err := restoreThumbnailEntry(entry); err != nil {
				writeErrorStatus(w, http.StatusInternalServerError, "failed to restore thumbnail cache: "+err.Error())
				return
			}
			restoredThumbs++
		}
	}
	if manifest != nil && restoredThumbs != manifest.DisplayThumbs+manifest.VectorThumbs {
		writeError(w, "migration archive thumbnail data is incomplete")
		return
	}

	restoredVectors := 0
	if manifest != nil && manifest.Vectors > 0 {
		vectorEntry := migrationEntries[migrationVectorsEntry]
		if vectorEntry == nil {
			writeError(w, "migration archive is missing vector data")
			return
		}
		initQdrantCollection()
		restoredVectors, err = restoreVectorsFromArchive(vectorEntry)
		if err != nil {
			writeErrorStatus(w, http.StatusInternalServerError, "failed to restore vectors: "+err.Error())
			return
		}
		if restoredVectors != manifest.Vectors {
			writeError(w, "migration archive vector data is incomplete")
			return
		}
	}

	go func() {
		GenerateAllThumbnails()
		EnqueueMissingImageEmbeddings()
	}()
	writeOK(w, map[string]int{
		"imported":   imported,
		"thumbnails": restoredThumbs,
		"vectors":    restoredVectors,
	})
}

func validateImportArchive(entries []*zip.File) (*migrationManifest, map[string]*zip.File, error) {
	var expanded int64
	migrationEntries := make(map[string]*zip.File)
	for _, entry := range entries {
		if err := validateArchiveEntry(entry); err != nil {
			return nil, nil, err
		}
		if entry.UncompressedSize64 > uint64(maxExpandedArchiveSize) || expanded > maxExpandedArchiveSize-int64(entry.UncompressedSize64) {
			return nil, nil, fmt.Errorf("ZIP contents are too large")
		}
		expanded += int64(entry.UncompressedSize64)
		if isMigrationEntry(entry.Name) {
			if _, exists := migrationEntries[entry.Name]; exists {
				return nil, nil, fmt.Errorf("ZIP contains duplicate migration data")
			}
			migrationEntries[entry.Name] = entry
		}
	}
	if len(migrationEntries) == 0 {
		return nil, migrationEntries, nil
	}
	manifestEntry := migrationEntries[migrationManifestEntry]
	if manifestEntry == nil {
		return nil, nil, fmt.Errorf("migration archive is missing its manifest")
	}
	src, err := manifestEntry.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read migration manifest")
	}
	defer src.Close()
	var manifest migrationManifest
	if err := json.NewDecoder(io.LimitReader(src, 1<<20)).Decode(&manifest); err != nil {
		return nil, nil, fmt.Errorf("invalid migration manifest")
	}
	if manifest.Version != migrationFormatVersion {
		return nil, nil, fmt.Errorf("unsupported migration format version: %d", manifest.Version)
	}
	if manifest.DisplayThumbs < 0 || manifest.VectorThumbs < 0 || manifest.Vectors < 0 {
		return nil, nil, fmt.Errorf("invalid migration manifest counts")
	}
	return &manifest, migrationEntries, nil
}

func restoreThumbnailEntry(entry *zip.File) error {
	prefix := migrationDisplayPrefix
	vectorThumbnail := false
	if strings.HasPrefix(entry.Name, migrationVectorThumbPrefix) {
		prefix = migrationVectorThumbPrefix
		vectorThumbnail = true
	}
	rel := strings.TrimPrefix(entry.Name, prefix)
	absPath, err := safePath(filepath.FromSlash(rel))
	if err != nil {
		return fmt.Errorf("invalid media path %q", rel)
	}
	info, err := os.Stat(absPath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("media file is missing: %s", rel)
	}
	var destination string
	if vectorThumbnail {
		if !isImageExt(absPath) {
			return fmt.Errorf("vector thumbnail does not reference an image: %s", rel)
		}
		destination = getVectorThumbCachePath(absPath)
	} else if isImageExt(absPath) {
		destination = getThumbCachePath(absPath)
	} else if isVideoExt(absPath) {
		destination = getVideoThumbCachePath(absPath)
	} else {
		return fmt.Errorf("display thumbnail does not reference media: %s", rel)
	}
	if err := extractArchiveFile(entry, destination); err != nil {
		return err
	}
	now := time.Now()
	return os.Chtimes(destination, now, now)
}

func restoreVectorsFromArchive(entry *zip.File) (int, error) {
	src, err := entry.Open()
	if err != nil {
		return 0, err
	}
	defer src.Close()
	decoder := json.NewDecoder(src)
	batch := make([]map[string]interface{}, 0, 32)
	count := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		body, err := json.Marshal(map[string]interface{}{"points": batch})
		if err != nil {
			return err
		}
		client := &http.Client{Timeout: 60 * time.Second}
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/collections/%s/points?wait=true", qdrantURL, collection), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			responseBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("Qdrant returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
		}
		batch = batch[:0]
		return nil
	}
	for {
		var record portableVectorRecord
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				break
			}
			return count, fmt.Errorf("invalid vector data: %w", err)
		}
		if len(record.Vector) != embeddingDimension {
			return count, fmt.Errorf("vector for %s has %d dimensions", record.Path, len(record.Vector))
		}
		absPath, err := safePath(filepath.FromSlash(record.Path))
		if err != nil || !isImageExt(absPath) {
			return count, fmt.Errorf("invalid vector media path: %s", record.Path)
		}
		if info, err := os.Stat(absPath); err != nil || !info.Mode().IsRegular() {
			return count, fmt.Errorf("vector media file is missing: %s", record.Path)
		}
		batch = append(batch, map[string]interface{}{
			"id":     uuid.NewMD5(uuid.NameSpaceURL, []byte(absPath)).String(),
			"vector": record.Vector,
			"payload": map[string]interface{}{
				"path":          absPath,
				"name":          filepath.Base(absPath),
				"vector_source": "thumbnail",
			},
		})
		count++
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return count, err
			}
		}
	}
	if err := flush(); err != nil {
		return count, err
	}
	return count, nil
}

func writeFileToArchive(zw *zip.Writer, sourcePath, archiveName string, method uint16) error {
	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()
	info, err := src.Stat()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archiveName
	header.Method = method
	out, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, src)
	return err
}

func createArchiveEntry(zw *zip.Writer, name string, method uint16) (io.Writer, error) {
	header := &zip.FileHeader{Name: name, Method: method}
	header.SetModTime(time.Now())
	return zw.CreateHeader(header)
}

func writeJSONArchiveEntry(zw *zip.Writer, name string, value interface{}) error {
	out, err := createArchiveEntry(zw, name, zip.Deflate)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(value)
}

func extractArchiveFile(entry *zip.File, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	src, err := entry.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, io.LimitReader(src, maxExpandedArchiveSize+1))
	closeErr := dst.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func regularFileExists(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsRegular()
}

func portableRelativePath(absPath string) (string, bool) {
	rel, err := filepath.Rel(dataDir, filepath.Clean(absPath))
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "../") || shouldSkipArchivePath(rel) {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(rel))); err != nil {
		return "", false
	}
	return rel, true
}

func shouldSkipArchivePath(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ".") || systemExcludes[part] || part == migrationArchiveDir {
			return true
		}
	}
	return false
}

func isMigrationEntry(name string) bool {
	clean := path.Clean(name)
	return clean == migrationArchiveDir || strings.HasPrefix(clean, migrationArchiveDir+"/")
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
	if entry.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("ZIP symlinks are not supported")
	}
	if isMigrationEntry(clean) {
		return validateMigrationEntry(clean, entry.FileInfo().IsDir())
	}
	if shouldSkipArchivePath(clean) {
		return fmt.Errorf("ZIP contains a protected file or directory")
	}
	return nil
}

func validateMigrationEntry(name string, isDir bool) error {
	if isDir {
		return fmt.Errorf("migration archive contains an unexpected directory entry")
	}
	if name == migrationManifestEntry || name == migrationVectorsEntry {
		return nil
	}
	for _, prefix := range []string{migrationDisplayPrefix, migrationVectorThumbPrefix} {
		if strings.HasPrefix(name, prefix) {
			rel := strings.TrimPrefix(name, prefix)
			if rel == "" || path.IsAbs(rel) || filepath.VolumeName(rel) != "" || path.Clean(rel) != rel || shouldSkipArchivePath(rel) {
				return fmt.Errorf("migration archive contains an invalid media path")
			}
			return nil
		}
	}
	return fmt.Errorf("migration archive contains an unknown entry")
}
