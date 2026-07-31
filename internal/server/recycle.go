package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeessy2/gnas/internal/db"
)

const (
	recycleBinDirName = "recycle_bin"
	recycleRetention  = 7 * 24 * time.Hour
)

// recycleBinDir 返回回收站根目录
func recycleBinDir() string {
	return filepath.Join(dataDir, internalStorageDir, recycleBinDirName)
}

// recycleThumbDir 返回回收站缩略图目录
func recycleThumbDir() string {
	return filepath.Join(recycleBinDir(), "thumbs")
}

// moveToRecycleBin 将文件/目录移动到回收站，返回记录和错误。
// 调用者需确保 absPath 已通过 safePath 校验且不等于 dataDir。
func moveToRecycleBin(absPath string) (*db.RecycleItem, error) {
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("文件不存在")
	}

	info, _ := os.Stat(absPath)
	isDir := info.IsDir()
	name := filepath.Base(absPath)
	isVideo := isVideoExt(absPath)

	// 回收站存储路径：用时间戳避免重名
	ts := time.Now().Format("20060102_150405")
	storedName := fmt.Sprintf("%s_%s", ts, name)
	storedPath := filepath.Join(recycleBinDir(), storedName)

	// 确保回收站目录存在
	os.MkdirAll(recycleBinDir(), 0755)

	// 移动文件/目录到回收站
	if err := os.Rename(absPath, storedPath); err != nil {
		return nil, fmt.Errorf("移动到回收站失败: %w", err)
	}

	// 移动缩略图到回收站（用于回收站列表预览）
	thumbStoredPath := ""
	if !isDir {
		var srcThumb string
		if isImageExt(absPath) {
			srcThumb = getThumbCachePath(absPath)
		} else if isVideoExt(absPath) {
			srcThumb = getVideoThumbCachePath(absPath)
		}
		if srcThumb != "" {
			if _, err := os.Stat(srcThumb); err == nil {
				os.MkdirAll(recycleThumbDir(), 0755)
				thumbStoredName := fmt.Sprintf("%s_%s", ts, filepath.Base(srcThumb))
				thumbStoredPath = filepath.Join(recycleThumbDir(), thumbStoredName)
				os.Rename(srcThumb, thumbStoredPath)
			}
		}
	}

	// 删除向量（恢复时重新生成）
	deleteQdrantVectorsForPath(absPath, isDir)
	markDeletedEmbeddingPath(absPath)

	// 清理向量缩略图（向量缩略图不保留，恢复时重新生成）
	if !isDir && isImageExt(absPath) {
		os.Remove(getVectorThumbCachePath(absPath))
	}

	// 清理文件元数据
	db.DeleteFileMeta(absPath)

	item := &db.RecycleItem{
		OriginalPath:    absPath,
		StoredPath:      storedPath,
		ThumbStoredPath: thumbStoredPath,
		Name:            name,
		IsVideo:         isVideo,
		IsDir:           isDir,
		ExpireAt:        time.Now().Add(recycleRetention).Format("2006-01-02 15:04:05"),
	}

	id, err := db.AddRecycleItem(item)
	if err != nil {
		// DB 写入失败，尝试把文件移回原位
		os.Rename(storedPath, absPath)
		if thumbStoredPath != "" {
			os.Remove(thumbStoredPath)
		}
		return nil, fmt.Errorf("记录回收站信息失败: %w", err)
	}
	item.ID = id

	return item, nil
}

// restoreFromRecycleBin 从回收站恢复文件到原路径
func restoreFromRecycleBin(id int64) error {
	item, err := db.GetRecycleItem(id)
	if err != nil {
		return fmt.Errorf("获取回收站记录失败: %w", err)
	}
	if item == nil {
		return fmt.Errorf("回收站记录不存在")
	}

	// 确保原路径父目录存在
	os.MkdirAll(filepath.Dir(item.OriginalPath), 0755)

	// 如果原路径已被占用，加后缀
	destPath := item.OriginalPath
	if _, err := os.Stat(destPath); err == nil {
		ext := filepath.Ext(destPath)
		base := strings.TrimSuffix(destPath, ext)
		destPath = fmt.Sprintf("%s_restored_%d%s", base, time.Now().Unix(), ext)
	}

	// 移回文件
	if err := os.Rename(item.StoredPath, destPath); err != nil {
		return fmt.Errorf("恢复文件失败: %w", err)
	}

	// 恢复缩略图
	if item.ThumbStoredPath != "" {
		if _, err := os.Stat(item.ThumbStoredPath); err == nil {
			var destThumb string
			if item.IsVideo {
				destThumb = getVideoThumbCachePath(destPath)
			} else {
				destThumb = getThumbCachePath(destPath)
			}
			os.MkdirAll(filepath.Dir(destThumb), 0755)
			os.Rename(item.ThumbStoredPath, destThumb)
		}
	}

	// 删除回收站记录
	db.DeleteRecycleItem(id)

	// 异步重新生成向量
	if !item.IsDir && isImageExt(destPath) {
		EnqueueMissingImageEmbeddings()
	}

	return nil
}

// purgeFromRecycleBin 彻底删除回收站中的文件
func purgeFromRecycleBin(id int64) error {
	item, err := db.GetRecycleItem(id)
	if err != nil {
		return fmt.Errorf("获取回收站记录失败: %w", err)
	}
	if item == nil {
		return fmt.Errorf("回收站记录不存在")
	}

	// 删除回收站中的文件
	os.RemoveAll(item.StoredPath)
	if item.ThumbStoredPath != "" {
		os.Remove(item.ThumbStoredPath)
	}

	// 删除记录
	return db.DeleteRecycleItem(id)
}

// clearRecycleBin 清空回收站
func clearRecycleBin() (int, error) {
	items, err := db.ListRecycleItems()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		os.RemoveAll(item.StoredPath)
		if item.ThumbStoredPath != "" {
			os.Remove(item.ThumbStoredPath)
		}
		db.DeleteRecycleItem(item.ID)
		count++
	}
	return count, nil
}

// cleanupExpiredRecycleItems 清理过期的回收站项
func cleanupExpiredRecycleItems() int {
	items, err := db.ListExpiredRecycleItems()
	if err != nil {
		log.Printf("[回收站] 查询过期项失败: %v", err)
		return 0
	}
	count := 0
	for _, item := range items {
		os.RemoveAll(item.StoredPath)
		if item.ThumbStoredPath != "" {
			os.Remove(item.ThumbStoredPath)
		}
		db.DeleteRecycleItem(item.ID)
		count++
	}
	if count > 0 {
		log.Printf("[回收站] 自动清理过期项 %d 个", count)
	}
	return count
}

// StartRecycleBinCleanup 启动回收站自动清理定时任务
func StartRecycleBinCleanup() {
	// 启动时先清理一次
	cleanupExpiredRecycleItems()

	// 每 6 小时检查一次
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredRecycleItems()
		}
	}()
}

// --- HTTP Handlers ---

// HandleRecycleBinList 列出回收站内容
func HandleRecycleBinList(w http.ResponseWriter, r *http.Request) {
	items, err := db.ListRecycleItems()
	if err != nil {
		writeError(w, "获取回收站列表失败")
		return
	}
	// 只返回安全字段，不暴露服务器路径
	type recycleItemResp struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		IsVideo   bool   `json:"isVideo"`
		IsDir     bool   `json:"isDir"`
		HasThumb  bool   `json:"hasThumb"`
		DeletedAt string `json:"deletedAt"`
		ExpireAt  string `json:"expireAt"`
	}
	result := make([]recycleItemResp, 0, len(items))
	for _, item := range items {
		result = append(result, recycleItemResp{
			ID:        item.ID,
			Name:      item.Name,
			IsVideo:   item.IsVideo,
			IsDir:     item.IsDir,
			HasThumb:  item.ThumbStoredPath != "",
			DeletedAt: item.DeletedAt,
			ExpireAt:  item.ExpireAt,
		})
	}
	writeOK(w, result)
}

// HandleRecycleBinThumb 返回回收站缩略图
func HandleRecycleBinThumb(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	var id int64
	fmt.Sscanf(idStr, "%d", &id)
	item, err := db.GetRecycleItem(id)
	if err != nil || item == nil || item.ThumbStoredPath == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if _, err := os.Stat(item.ThumbStoredPath); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, item.ThumbStoredPath)
}

// HandleRecycleBinRestore 恢复文件
func HandleRecycleBinRestore(w http.ResponseWriter, r *http.Request) {
	var data struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}
	if len(data.IDs) == 0 {
		writeError(w, "未选择文件")
		return
	}
	restored := 0
	var lastErr error
	for _, id := range data.IDs {
		if err := restoreFromRecycleBin(id); err != nil {
			lastErr = err
		} else {
			restored++
		}
	}
	if restored == 0 {
		writeError(w, fmt.Sprintf("恢复失败: %v", lastErr))
		return
	}
	writeOK(w, map[string]interface{}{"restored": restored})
}

// HandleRecycleBinDelete 彻底删除
func HandleRecycleBinDelete(w http.ResponseWriter, r *http.Request) {
	var data struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}
	if len(data.IDs) == 0 {
		writeError(w, "未选择文件")
		return
	}
	deleted := 0
	for _, id := range data.IDs {
		if err := purgeFromRecycleBin(id); err == nil {
			deleted++
		}
	}
	writeOK(w, map[string]interface{}{"deleted": deleted})
}

// HandleRecycleBinClear 清空回收站
func HandleRecycleBinClear(w http.ResponseWriter, r *http.Request) {
	count, err := clearRecycleBin()
	if err != nil {
		writeError(w, "清空回收站失败")
		return
	}
	writeOK(w, map[string]interface{}{"cleared": count})
}
