package server

import (
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // 注册 WebP 解码器
)

// 文件管理根目录，默认为可执行文件同目录下的 data
var dataDir string

func InitDataDir(dir string) {
	dataDir = dir
	os.MkdirAll(dataDir, 0755)
}

// thumbCacheValid 检查缩略图缓存是否有效（缓存文件比源文件新）
func thumbCacheValid(srcPath string) bool {
	var thumbPath string
	if isImageExt(srcPath) {
		thumbPath = getThumbCachePath(srcPath)
	} else if isVideoExt(srcPath) {
		thumbPath = getVideoThumbCachePath(srcPath)
	} else {
		return false
	}
	if thumbInfo, err := os.Stat(thumbPath); err == nil {
		if srcInfo, err := os.Stat(srcPath); err == nil {
			return thumbInfo.ModTime().After(srcInfo.ModTime())
		}
	}
	return false
}

// generateThumbForFile 为单个文件生成缩略图（图片或视频），并打印日志
func generateThumbForFile(absPath string) {
	name := filepath.Base(absPath)
	if isImageExt(name) {
		if _, err := generateThumbnail(absPath); err != nil {
			log.Printf("[缩略图] 生成图片缩略图失败 %s: %v", name, err)
		} else {
			log.Printf("[缩略图] 已生成图片缩略图: %s", name)
		}
	} else if isVideoExt(name) {
		if _, err := generateVideoThumbnail(absPath); err != nil {
			log.Printf("[缩略图] 生成视频缩略图失败 %s: %v", name, err)
		} else {
			log.Printf("[缩略图] 已生成视频缩略图: %s", name)
		}
	}
}

// GenerateAllThumbnails 扫描 dataDir 下所有图片和视频，生成缩略图（已缓存则跳过）
func GenerateAllThumbnails() {
	log.Println("[缩略图] 开始扫描并生成所有媒体缩略图...")
	generated := 0
	skipped := 0
	filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// 跳过缩略图缓存目录和数据库
		if strings.Contains(path, ".thumbs") || strings.Contains(path, "gnas.db") {
			return nil
		}
		name := d.Name()
		if isImageExt(name) || isVideoExt(name) {
			if thumbCacheValid(path) {
				skipped++
				return nil
			}
			generateThumbForFile(path)
			generated++
		}
		return nil
	})
	log.Printf("[缩略图] 扫描完成，新生成 %d 个，已缓存跳过 %d 个", generated, skipped)
}

func safePath(name string) (string, error) {
	p := filepath.Join(dataDir, name)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	dataAbs, _ := filepath.Abs(dataDir)
	rel, err := filepath.Rel(dataAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法路径")
	}
	return abs, nil
}

func safeFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("文件名不能为空")
	}
	if name != filepath.Base(name) || name == "." || name == ".." {
		return "", fmt.Errorf("非法文件名")
	}
	return name, nil
}

// FileInfo 文件信息
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// HandleFileList 浏览文件列表
func HandleFileList(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "/"
	}

	absPath, err := safePath(dir)
	if err != nil {
		writeError(w, "非法路径")
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeOK(w, []FileInfo{})
			return
		}
		writeError(w, "读取目录失败")
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		// 跳过隐藏目录、缩略图缓存、数据库文件
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "gnas.db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		relPath := filepath.Join(dir, entry.Name())
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.ToSlash(relPath),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	// 目录排前面，然后按名称排序
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	writeOK(w, files)
}

// HandleFileUpload 上传文件
func HandleFileUpload(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("path")
	if dir == "" {
		dir = "/"
	}

	absDir, err := safePath(dir)
	if err != nil {
		writeError(w, "非法路径")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, "解析上传数据失败")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, "获取上传文件失败")
		return
	}
	defer file.Close()

	fileName, err := safeFileName(header.Filename)
	if err != nil {
		writeError(w, "非法文件名")
		return
	}
	dstPath := filepath.Join(absDir, fileName)
	dst, err := os.Create(dstPath)
	if err != nil {
		writeError(w, "创建文件失败")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, "写入文件失败")
		return
	}

	// 上传完成后，异步生成缩略图
	go generateThumbForFile(dstPath)

	writeOK(w, nil)
}

// mimeTypeByExt 根据扩展名返回 MIME 类型
// formatContentDisposition 生成支持中文文件名的 Content-Disposition header (RFC 5987)
func formatContentDisposition(disposition, filename string) string {
	encoded := url.PathEscape(filename)
	// filename 用 ASCII 兼容的占位，filename* 用 UTF-8 编码
	asciiName := toASCII(filename)
	// 转义双引号防止 header 注入
	asciiName = strings.ReplaceAll(asciiName, `"`, `\"`)
	return fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", disposition, asciiName, encoded)
}

// toASCII 将非 ASCII 字符替换为 _，确保 filename 字段兼容旧浏览器
func toASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func mimeTypeByExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogv":
		return "video/ogg"
	case ".mov":
		return "video/quicktime"
	case ".mp3":
		return "audio/mpeg"
	case ".ogg":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	case ".aac":
		return "audio/aac"
	case ".m4a":
		return "audio/mp4"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	default:
		return "application/octet-stream"
	}
}

// isImageExt 判断是否为图片扩展名
func isImageExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// isVideoExt 判断是否为视频扩展名
func isVideoExt(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp4", ".webm", ".ogv", ".mov":
		return true
	}
	return false
}

// MediaItem 媒体文件信息
type MediaItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Type    string    `json:"type"` // "image" 或 "video"
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

// HandleGalleryList 获取所有图片和视频
func HandleGalleryList(w http.ResponseWriter, r *http.Request) {
	var items []MediaItem
	filepath.WalkDir(dataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// 跳过缩略图缓存目录
		if strings.Contains(path, ".thumbs") {
			return nil
		}
		name := d.Name()
		relPath, err := filepath.Rel(dataDir, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if isImageExt(name) {
			items = append(items, MediaItem{
				Name:    name,
				Path:    filepath.ToSlash(relPath),
				Type:    "image",
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		} else if isVideoExt(name) {
			items = append(items, MediaItem{
				Name:    name,
				Path:    filepath.ToSlash(relPath),
				Type:    "video",
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}
		return nil
	})

	// 按修改时间倒序（最新的在前）
	sort.Slice(items, func(i, j int) bool {
		return items[i].ModTime.After(items[j].ModTime)
	})

	if items == nil {
		items = []MediaItem{}
	}
	writeOK(w, items)
}

// getThumbCachePath 获取缩略图缓存路径
func getThumbCachePath(absPath string) string {
	thumbDir := filepath.Join(dataDir, ".thumbs")
	// 缩略图命名：sl_原文件名.扩展名
	name := filepath.Base(absPath)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return filepath.Join(thumbDir, "sl_"+base+ext)
}

// getVideoThumbCachePath 获取视频缩略图缓存路径（固定输出 jpg）
func getVideoThumbCachePath(absPath string) string {
	thumbDir := filepath.Join(dataDir, ".thumbs")
	name := filepath.Base(absPath)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return filepath.Join(thumbDir, "sl_"+base+".jpg")
}

// generateVideoThumbnail 使用 ffmpeg 从视频中截取第一帧作为缩略图
func generateVideoThumbnail(srcPath string) (string, error) {
	thumbPath := getVideoThumbCachePath(srcPath)
	thumbDir := filepath.Dir(thumbPath)
	os.MkdirAll(thumbDir, 0755)

	// 检查缓存是否有效
	if thumbInfo, err := os.Stat(thumbPath); err == nil {
		if srcInfo, err := os.Stat(srcPath); err == nil {
			if thumbInfo.ModTime().After(srcInfo.ModTime()) {
				return thumbPath, nil
			}
		}
	}

	// 使用 ffmpeg 截取第 1 秒的画面，宽度缩放到 300px
	cmd := exec.Command("ffmpeg",
		"-i", srcPath,
		"-ss", "00:00:01",
		"-vframes", "1",
		"-vf", "scale=300:-1",
		"-y",
		thumbPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg 生成视频缩略图失败: %v, output: %s", err, string(output))
	}

	return thumbPath, nil
}

// generateThumbnail 生成缩略图，最大宽度 300px，返回缓存文件路径
func generateThumbnail(srcPath string) (string, error) {
	thumbPath := getThumbCachePath(srcPath)
	thumbDir := filepath.Dir(thumbPath)
	os.MkdirAll(thumbDir, 0755)

	// 检查缓存是否有效（缓存文件修改时间晚于原文件）
	if thumbInfo, err := os.Stat(thumbPath); err == nil {
		if srcInfo, err := os.Stat(srcPath); err == nil {
			if thumbInfo.ModTime().After(srcInfo.ModTime()) {
				return thumbPath, nil
			}
		}
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	if width <= 300 {
		// 原图小于 300px，直接复制作为缩略图
		f.Seek(0, 0)
		out, err := os.Create(thumbPath)
		if err != nil {
			return "", err
		}
		defer out.Close()
		io.Copy(out, f)
		return thumbPath, nil
	}

	// 按比例缩放，最大宽度 300px
	ratio := float64(300) / float64(width)
	height := int(float64(bounds.Dy()) * ratio)

	dst := image.NewRGBA(image.Rect(0, 0, 300, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	out, err := os.Create(thumbPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	ext := strings.ToLower(filepath.Ext(srcPath))
	switch ext {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(out, dst, &jpeg.Options{Quality: 80})
	case ".png":
		err = png.Encode(out, dst)
	case ".gif":
		err = gif.Encode(out, dst, nil)
	default:
		err = jpeg.Encode(out, dst, &jpeg.Options{Quality: 80})
	}
	if err != nil {
		return "", err
	}
	return thumbPath, nil
}

// HandleFileDownload 下载/预览文件
func HandleFileDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, "缺少路径参数")
		return
	}

	absPath, err := safePath(path)
	if err != nil {
		writeError(w, "非法路径")
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		writeError(w, "文件不存在")
		return
	}

	// 根据 disposition 参数决定是下载还是预览
	disposition := r.URL.Query().Get("disposition")
	if disposition == "inline" {
		// 预览模式：设置正确的 Content-Type 和 inline
		mime := mimeTypeByExt(filepath.Base(absPath))
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Disposition", formatContentDisposition("inline", filepath.Base(absPath)))
		f, err := os.Open(absPath)
		if err != nil {
			writeError(w, "打开文件失败")
			return
		}
		defer f.Close()
		http.ServeContent(w, r, filepath.Base(absPath), info.ModTime(), f)
		return
	}

	// 下载模式：使用自定义 header 确保中文文件名正确
	name := filepath.Base(absPath)
	mime := mimeTypeByExt(name)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", formatContentDisposition("attachment", name))
	f, err := os.Open(absPath)
	if err != nil {
		writeError(w, "打开文件失败")
		return
	}
	defer f.Close()
	http.ServeContent(w, r, name, info.ModTime(), f)
}

// HandleFileThumbnail 获取缩略图
func HandleFileThumbnail(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, "缺少路径参数")
		return
	}

	absPath, err := safePath(path)
	if err != nil {
		writeError(w, "非法路径")
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		writeError(w, "文件不存在")
		return
	}

	// 根据文件类型选择生成方式
	if isVideoExt(absPath) {
		thumbPath, err := generateVideoThumbnail(absPath)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		thumbInfo, err := os.Stat(thumbPath)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", thumbInfo.Size()))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		f, err := os.Open(thumbPath)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer f.Close()
		io.Copy(w, f)
		return
	}

	// 非图片文件返回 404
	if !isImageExt(absPath) {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	thumbPath, err := generateThumbnail(absPath)
	if err != nil {
		// 生成失败则回退到原图
		w.Header().Set("Content-Type", mimeTypeByExt(filepath.Base(absPath)))
		f, err := os.Open(absPath)
		if err != nil {
			writeError(w, "打开文件失败")
			return
		}
		defer f.Close()
		io.Copy(w, f)
		return
	}

	thumbInfo, err := os.Stat(thumbPath)
	if err != nil {
		writeError(w, "读取缩略图失败")
		return
	}

	w.Header().Set("Content-Type", mimeTypeByExt(filepath.Base(thumbPath)))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", thumbInfo.Size()))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	f, err := os.Open(thumbPath)
	if err != nil {
		writeError(w, "打开文件失败")
		return
	}
	defer f.Close()
	io.Copy(w, f)
}

// HandleFileDelete 删除文件或目录
func HandleFileDelete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}

	absPath, err := safePath(data.Path)
	if err != nil {
		writeError(w, "非法路径")
		return
	}

	if absPath == dataDir {
		writeError(w, "不能删除根目录")
		return
	}

	// 清理缩略图缓存
	cleanThumbCache(absPath)

	if err := os.RemoveAll(absPath); err != nil {
		writeError(w, "删除失败")
		return
	}

	writeOK(w, nil)
}

// cleanThumbCache 清理文件对应的缩略图缓存
func cleanThumbCache(absPath string) {
	if isImageExt(absPath) {
		thumbPath := getThumbCachePath(absPath)
		os.Remove(thumbPath)
	} else if isVideoExt(absPath) {
		thumbPath := getVideoThumbCachePath(absPath)
		os.Remove(thumbPath)
	}
}

// HandleFileBatchDelete 批量删除文件或目录
func HandleFileBatchDelete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}

	if len(data.Paths) == 0 {
		writeError(w, "未选择文件")
		return
	}

	var failed []string
	for _, p := range data.Paths {
		absPath, err := safePath(p)
		if err != nil {
			failed = append(failed, p)
			continue
		}
		if absPath == dataDir {
			failed = append(failed, p)
			continue
		}
		cleanThumbCache(absPath)
		if err := os.RemoveAll(absPath); err != nil {
			failed = append(failed, p)
		}
	}

	if len(failed) > 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"code":    1,
			"message": fmt.Sprintf("部分文件删除失败: %d 个", len(failed)),
			"data":    failed,
		})
		return
	}

	writeOK(w, nil)
}

// HandleFileMkdir 新建文件夹
func HandleFileMkdir(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}

	absPath, err := safePath(data.Path)
	if err != nil {
		writeError(w, "非法路径")
		return
	}

	if err := os.MkdirAll(absPath, 0755); err != nil {
		writeError(w, "创建目录失败")
		return
	}

	writeOK(w, nil)
}

// HandleFileRename 重命名
func HandleFileRename(w http.ResponseWriter, r *http.Request) {
	var data struct {
		OldPath string `json:"oldPath"`
		NewName string `json:"newName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}

	absOld, err := safePath(data.OldPath)
	if err != nil {
		writeError(w, "非法路径")
		return
	}

	newName, err := safeFileName(data.NewName)
	if err != nil {
		writeError(w, "非法文件名")
		return
	}
	parent := filepath.Dir(absOld)
	absNew := filepath.Join(parent, newName)

	// 清理旧缩略图缓存
	cleanThumbCache(absOld)

	if err := os.Rename(absOld, absNew); err != nil {
		writeError(w, "重命名失败")
		return
	}

	// 重命名后重新生成缩略图
	go generateThumbForFile(absNew)

	writeOK(w, nil)
}
