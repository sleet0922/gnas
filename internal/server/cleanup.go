package server

import (
	"crypto/sha1"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// StaleResourceGroup 描述一类无用资源的统计信息
type StaleResourceGroup struct {
	Count     int64    `json:"count"`
	SizeBytes int64    `json:"sizeBytes"`
	Files     []string `json:"files"` // 相对路径或文件名，最多展示 50 条
}

// StaleScanResult 扫描无用资源的结果
type StaleScanResult struct {
	Thumbnails       StaleResourceGroup `json:"thumbnails"`
	VectorThumbnails StaleResourceGroup `json:"vectorThumbnails"`
	Vectors          StaleResourceGroup `json:"vectors"`
}

// 清理结果（已删除的数量/释放空间）
type StaleCleanupResult struct {
	Thumbnails       StaleResourceGroup `json:"thumbnails"`
	VectorThumbnails StaleResourceGroup `json:"vectorThumbnails"`
	Vectors          StaleResourceGroup `json:"vectors"`
	TotalFreedBytes  int64              `json:"totalFreedBytes"`
}

const staleFileListLimit = 50

func truncateFileList(files []string) []string {
	if len(files) > staleFileListLimit {
		return files[:staleFileListLimit]
	}
	return files
}

// scanStaleThumbnails 扫描普通缩略图目录，找出原图已不存在的缩略图
func scanStaleThumbnails() StaleResourceGroup {
	thumbDir := filepath.Join(dataDir, internalStorageDir, thumbnailCacheDirName)
	group := StaleResourceGroup{Files: []string{}}
	entries, err := os.ReadDir(thumbDir)
	if err != nil {
		return group
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 仅处理 sl_ 前缀的缩略图
		if !strings.HasPrefix(name, "sl_") {
			continue
		}
		// 反推原文件名：去掉 sl_ 前缀
		originalName := strings.TrimPrefix(name, "sl_")
		// HEIC/HEIF 缩略图被转成 jpg，需要还原原扩展名再检查
		candidates := []string{originalName}
		ext := filepath.Ext(originalName)
		base := strings.TrimSuffix(originalName, ext)
		if strings.EqualFold(ext, ".jpg") {
			candidates = append(candidates, base+".heic", base+".heif", base+".HEIC", base+".HEIF")
		}
		// 检查原图是否存在于 dataDir 任意子目录
		found := false
		// 先检查常见位置：dataDir 根目录及子目录
		for _, candidate := range candidates {
			if mediaExistsInDataDir(candidate) {
				found = true
				break
			}
		}
		if found {
			continue
		}
		// 无用缩略图
		info, err := entry.Info()
		size := int64(0)
		if err == nil {
			size = info.Size()
		}
		group.Count++
		group.SizeBytes += size
		if len(group.Files) < staleFileListLimit {
			group.Files = append(group.Files, name)
		}
	}
	return group
}

// mediaExistsInDataDir 检查指定文件名是否存在于 dataDir 任意位置
func mediaExistsInDataDir(filename string) bool {
	found := false
	filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != dataDir && (strings.HasPrefix(name, ".") || systemExcludes[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(d.Name(), filename) {
			found = true
		}
		return nil
	})
	return found
}

// scanStaleVectorThumbnails 扫描向量缩略图目录，找出原图已不存在的缩略图
// 向量缩略图命名为 vl_{sha1前12位}.jpg，sha1 基于原图绝对路径计算
func scanStaleVectorThumbnails() StaleResourceGroup {
	thumbDir := filepath.Join(dataDir, internalStorageDir, vectorThumbnailCacheDirName)
	group := StaleResourceGroup{Files: []string{}}
	entries, err := os.ReadDir(thumbDir)
	if err != nil {
		return group
	}
	// 收集所有原图的向量缩略图 SHA1 前缀（有效的集合）
	validPrefixes := buildValidVectorThumbPrefixes()
	if validPrefixes == nil {
		validPrefixes = map[string]bool{}
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "vl_") {
			continue
		}
		// 提取 sha1 前 12 位：vl_xxxxxxxxxxxx.jpg
		prefix := strings.TrimPrefix(name, "vl_")
		prefix = strings.TrimSuffix(prefix, filepath.Ext(prefix))
		if len(prefix) < 12 {
			continue
		}
		if validPrefixes[prefix] {
			continue
		}
		// 无用向量缩略图
		info, err := entry.Info()
		size := int64(0)
		if err == nil {
			size = info.Size()
		}
		group.Count++
		group.SizeBytes += size
		if len(group.Files) < staleFileListLimit {
			group.Files = append(group.Files, name)
		}
	}
	return group
}

// buildValidVectorThumbPrefixes 遍历所有图片文件，计算其向量缩略图的有效 SHA1 前缀
func buildValidVectorThumbPrefixes() map[string]bool {
	prefixes := map[string]bool{}
	filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != dataDir && (strings.HasPrefix(name, ".") || systemExcludes[name]) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isImageExt(d.Name()) {
			return nil
		}
		cleanPath := filepath.Clean(path)
		sum := sha1.Sum([]byte(cleanPath))
		prefix := fmt.Sprintf("%x", sum[:12])
		prefixes[prefix] = true
		return nil
	})
	return prefixes
}

// scanStaleVectors 扫描 Qdrant 中原图已不存在的向量
func scanStaleVectors() StaleResourceGroup {
	group := StaleResourceGroup{Files: []string{}}
	points, err := getQdrantPointMetadata()
	if err != nil {
		log.Printf("[清理] 扫描 Qdrant 过期向量失败: %v", err)
		return group
	}
	for _, point := range points {
		if _, err := os.Stat(point.Path); os.IsNotExist(err) {
			group.Count++
			if len(group.Files) < staleFileListLimit {
				group.Files = append(group.Files, point.Path)
			}
		}
	}
	return group
}

// ScanStaleResources 扫描所有无用资源（不删除）
func ScanStaleResources() StaleScanResult {
	var result StaleScanResult
	// 并行扫描三类资源
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); result.Thumbnails = scanStaleThumbnails() }()
	go func() { defer wg.Done(); result.VectorThumbnails = scanStaleVectorThumbnails() }()
	go func() { defer wg.Done(); result.Vectors = scanStaleVectors() }()
	wg.Wait()
	return result
}

// CleanupStaleResources 扫描并删除所有无用资源
func CleanupStaleResources() StaleCleanupResult {
	scan := ScanStaleResources()
	result := StaleCleanupResult{
		Thumbnails:       StaleResourceGroup{Count: scan.Thumbnails.Count, SizeBytes: scan.Thumbnails.SizeBytes, Files: scan.Thumbnails.Files},
		VectorThumbnails: StaleResourceGroup{Count: scan.VectorThumbnails.Count, SizeBytes: scan.VectorThumbnails.SizeBytes, Files: scan.VectorThumbnails.Files},
		Vectors:          StaleResourceGroup{Count: scan.Vectors.Count, Files: scan.Vectors.Files},
	}

	// 删除无用普通缩略图
	if scan.Thumbnails.Count > 0 {
		thumbDir := filepath.Join(dataDir, internalStorageDir, thumbnailCacheDirName)
		for _, name := range collectAllStaleThumbFiles(thumbDir, "sl_") {
			full := filepath.Join(thumbDir, name)
			os.Remove(full)
		}
	}

	// 删除无用向量缩略图
	if scan.VectorThumbnails.Count > 0 {
		vectorThumbDir := filepath.Join(dataDir, internalStorageDir, vectorThumbnailCacheDirName)
		validPrefixes := buildValidVectorThumbPrefixes()
		entries, _ := os.ReadDir(vectorThumbDir)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasPrefix(name, "vl_") {
				continue
			}
			prefix := strings.TrimPrefix(name, "vl_")
			prefix = strings.TrimSuffix(prefix, filepath.Ext(prefix))
			if len(prefix) < 12 {
				continue
			}
			if !validPrefixes[prefix] {
				os.Remove(filepath.Join(vectorThumbDir, name))
			}
		}
	}

	// 删除无用 Qdrant 向量
	if scan.Vectors.Count > 0 {
		points, err := getQdrantPointMetadata()
		if err == nil {
			var staleIDs []string
			for _, point := range points {
				if _, err := os.Stat(point.Path); os.IsNotExist(err) {
					if point.ID != "" {
						staleIDs = append(staleIDs, point.ID)
					}
				}
			}
			for start := 0; start < len(staleIDs); start += 256 {
				end := min(start+256, len(staleIDs))
				if err := deleteQdrantPointIDs(staleIDs[start:end]); err != nil {
					log.Printf("[清理] 批量删除过期向量失败: %v", err)
					break
				}
			}
		}
	}

	result.TotalFreedBytes = result.Thumbnails.SizeBytes + result.VectorThumbnails.SizeBytes
	if result.Thumbnails.Count > 0 || result.VectorThumbnails.Count > 0 || result.Vectors.Count > 0 {
		log.Printf("[清理] 已清理无用资源：缩略图 %d 个（%s），向量缩略图 %d 个（%s），向量 %d 个",
			result.Thumbnails.Count, formatBytes(uint64(result.Thumbnails.SizeBytes)),
			result.VectorThumbnails.Count, formatBytes(uint64(result.VectorThumbnails.SizeBytes)),
			result.Vectors.Count)
	}
	return result
}

// collectAllStaleThumbFiles 收集指定目录下指定前缀的无用缩略图文件名（完整列表）
func collectAllStaleThumbFiles(thumbDir, prefix string) []string {
	entries, err := os.ReadDir(thumbDir)
	if err != nil {
		return nil
	}
	var stale []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		originalName := strings.TrimPrefix(name, prefix)
		candidates := []string{originalName}
		ext := filepath.Ext(originalName)
		base := strings.TrimSuffix(originalName, ext)
		if strings.EqualFold(ext, ".jpg") {
			candidates = append(candidates, base+".heic", base+".heif", base+".HEIC", base+".HEIF")
		}
		found := false
		for _, candidate := range candidates {
			if mediaExistsInDataDir(candidate) {
				found = true
				break
			}
		}
		if !found {
			stale = append(stale, name)
		}
	}
	return stale
}

// HandleStaleScan API: 扫描无用资源（只读）
func HandleStaleScan(w http.ResponseWriter, r *http.Request) {
	result := ScanStaleResources()
	writeOK(w, result)
}

// HandleStaleCleanup API: 扫描并清理无用资源
func HandleStaleCleanup(w http.ResponseWriter, r *http.Request) {
	result := CleanupStaleResources()
	writeOK(w, result)
}
