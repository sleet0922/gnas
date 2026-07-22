package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jeessy2/gnas/internal/db"
)

const (
	ollamaURL  = "http://127.0.0.1:11434/api/embeddings"
	qdrantURL  = "http://127.0.0.1:6333"
	collection = "gnas_photos"
	// modelName 不再使用 Ollama，将直接调用 Python 脚本
)

const embeddingQueueSize = 8

var (
	embeddingSlots        = make(chan struct{}, 1)
	embeddingQueue        = make(chan string, embeddingQueueSize)
	embeddingPending      sync.Map
	deletedEmbeddingPaths sync.Map
	embeddingOnce         sync.Once
)

func startEmbeddingWorker() {
	embeddingOnce.Do(func() {
		go func() {
			for path := range embeddingQueue {
				generateImageEmbedding(path)
				embeddingPending.Delete(path)
			}
		}()
	})
}

func enqueueImageEmbedding(path string) {
	clearDeletedEmbeddingPath(path)
	if enabled, err := db.GetSetting("ai_enabled"); err != nil || enabled != "true" {
		return
	}
	if _, loaded := embeddingPending.LoadOrStore(path, struct{}{}); loaded {
		return
	}
	startEmbeddingWorker()
	embeddingQueue <- path
}

func markDeletedEmbeddingPath(path string) {
	deletedEmbeddingPaths.Store(filepath.Clean(path), struct{}{})
}

func clearDeletedEmbeddingPath(path string) {
	path = filepath.Clean(path)
	deletedEmbeddingPaths.Range(func(key, _ interface{}) bool {
		deletedPath := key.(string)
		if path == deletedPath || strings.HasPrefix(path, deletedPath+string(filepath.Separator)) {
			deletedEmbeddingPaths.Delete(key)
		}
		return true
	})
}

func isDeletedEmbeddingPath(path string) bool {
	path = filepath.Clean(path)
	deleted := false
	deletedEmbeddingPaths.Range(func(key, _ interface{}) bool {
		deletedPath := key.(string)
		if path == deletedPath || strings.HasPrefix(path, deletedPath+string(filepath.Separator)) {
			deleted = true
			return false
		}
		return true
	})
	return deleted
}

// initQdrantCollection 初始化 Qdrant 集合
func initQdrantCollection() {
	// 检查 collection 是否存在
	resp, err := http.Get(fmt.Sprintf("%s/collections/%s", qdrantURL, collection))
	if err == nil && resp.StatusCode == 200 {
		resp.Body.Close()
		return // 已存在
	}

	// Qwen3-VL-Embedding 输出 2048 维向量
	dim := 2048

	payload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     dim,
			"distance": "Cosine",
		},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/collections/%s", qdrantURL, collection), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("[Qdrant] 创建集合失败: %v", err)
		return
	}
	defer res.Body.Close()
	log.Printf("[Qdrant] 创建集合 %s 成功 (维度: %d)", collection, dim)
}

// getEmbeddingFromPython 调用本地 FastAPI 服务生成向量
func getEmbeddingFromPython(imagePath string, textQuery string) ([]float32, error) {
	embeddingSlots <- struct{}{}
	defer func() { <-embeddingSlots }()
	return getEmbeddingFromPythonUnbounded(imagePath, textQuery)
}

func getEmbeddingFromPythonUnbounded(imagePath string, textQuery string) ([]float32, error) {
	payload := map[string]interface{}{}
	if imagePath != "" {
		payload["image_path"] = imagePath
	}
	if textQuery != "" {
		payload["text"] = textQuery
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request body failed: %v", err)
	}

	resp, err := http.Post("http://127.0.0.1:8000/embed", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("请求向量服务失败: %v (请确保 FastAPI 服务已启动)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("向量服务返回错误 (状态码 %d): %s", resp.StatusCode, string(respBody))
	}

	var response struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("解析向量服务响应失败: %v", err)
	}

	return response.Embedding, nil
}

// generateImageEmbedding 为图片生成向量并存入 Qdrant
func generateImageEmbedding(absPath string) {
	if isDeletedEmbeddingPath(absPath) {
		return
	}
	if _, err := os.Stat(absPath); err != nil {
		return
	}
	filename := filepath.Base(absPath)
	vec, err := getEmbeddingFromPython(absPath, "")
	if err != nil {
		log.Printf("[向量] 获取嵌入失败 %s: %v", filename, err)
		return
	}
	if isDeletedEmbeddingPath(absPath) {
		return
	}
	if _, err := os.Stat(absPath); err != nil {
		return
	}

	// 存入 Qdrant
	id := uuid.NewMD5(uuid.NameSpaceURL, []byte(absPath)).String() // 基于路径生成稳定的 UUID

	payload := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":     id,
				"vector": vec,
				"payload": map[string]interface{}{
					"path": absPath,
					"name": filename,
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/collections/%s/points?wait=true", qdrantURL, collection), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Qdrant] 存储向量失败 %s: %v", filename, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		log.Printf("[Qdrant] 存储向量返回错误: %s", string(b))
	} else {
		log.Printf("[向量] 已成功为照片生成并保存向量: %s", filename)
	}
}

type qdrantPathPoint struct {
	ID      string `json:"id"`
	Payload struct {
		Path string `json:"path"`
	} `json:"payload"`
}

func qdrantPointIDsForPath(absPath string, recursive bool) ([]string, error) {
	target := filepath.Clean(absPath)
	var ids []string
	var offset json.RawMessage

	for {
		payload := map[string]interface{}{
			"limit":        256,
			"with_vector":  false,
			"with_payload": true,
		}
		if len(offset) > 0 && string(offset) != "null" {
			payload["offset"] = offset
		}
		body, _ := json.Marshal(payload)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(fmt.Sprintf("%s/collections/%s/points/scroll", qdrantURL, collection), "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, err
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
			return nil, err
		}
		for _, point := range result.Result.Points {
			pointPath := filepath.Clean(point.Payload.Path)
			matches := pointPath == target
			if recursive {
				matches = matches || strings.HasPrefix(pointPath, target+string(filepath.Separator))
			}
			if matches && point.ID != "" {
				ids = append(ids, point.ID)
			}
		}
		if len(result.Result.NextPageOffset) == 0 || string(result.Result.NextPageOffset) == "null" || len(result.Result.Points) == 0 {
			break
		}
		offset = result.Result.NextPageOffset
	}
	return ids, nil
}

func deleteQdrantPointIDs(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	body, _ := json.Marshal(map[string]interface{}{"points": ids})
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/collections/%s/points?wait=true", qdrantURL, collection), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Qdrant returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func deleteQdrantVectorsForPath(absPath string, recursive bool) {
	markDeletedEmbeddingPath(absPath)

	var ids []string
	var err error
	if recursive {
		ids, err = qdrantPointIDsForPath(absPath, true)
	} else {
		ids = []string{uuid.NewMD5(uuid.NameSpaceURL, []byte(absPath)).String()}
	}
	if err != nil {
		log.Printf("[Qdrant] 获取待删除向量失败 %s: %v", absPath, err)
		return
	}
	if err := deleteQdrantPointIDs(ids); err != nil {
		log.Printf("[Qdrant] 删除向量失败 %s: %v", absPath, err)
		return
	}
	if len(ids) > 0 {
		log.Printf("[Qdrant] 已删除 %d 个媒体向量: %s", len(ids), absPath)
	}
}

// HandleSearchPhotos API: 搜索照片
func HandleSearchPhotos(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, "搜索内容不能为空")
		return
	}

	// 1. 获取搜索词向量
	vec, err := getEmbeddingFromPython("", query)
	if err != nil {
		log.Printf("[搜索] 获取搜索词向量失败: %v", err)
		writeError(w, "无法生成搜索向量")
		return
	}

	// 2. 在 Qdrant 中搜索
	payload := map[string]interface{}{
		"vector":       vec,
		"limit":        20,
		"with_payload": true,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(fmt.Sprintf("%s/collections/%s/points/search", qdrantURL, collection), "application/json", bytes.NewBuffer(body))
	if err != nil {
		writeError(w, "搜索引擎请求失败")
		return
	}
	defer resp.Body.Close()

	var result struct {
		Result []struct {
			Score   float32 `json:"score"`
			Payload struct {
				Path string `json:"path"`
				Name string `json:"name"`
			} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		writeError(w, "搜索引擎响应解析失败")
		return
	}

	// 提取结果并返回
	var searchItems []MediaItem
	for _, res := range result.Result {
		absPath := res.Payload.Path
		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}
		relPath, err := filepath.Rel(dataDir, absPath)
		if err != nil {
			continue
		}

		mediaType := "image"
		if isVideoExt(info.Name()) {
			mediaType = "video"
		}

		searchItems = append(searchItems, MediaItem{
			Name:    info.Name(),
			Path:    filepath.ToSlash(relPath),
			Type:    mediaType,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	writeOK(w, searchItems)
}

type qdrantPoint struct {
	Id      string    `json:"id"`
	Vector  []float32 `json:"vector"`
	Payload struct {
		Path string `json:"path"`
		Name string `json:"name"`
	} `json:"payload"`
}

func getAllQdrantPoints() ([]qdrantPoint, error) {
	payload := map[string]interface{}{
		"limit":        10000,
		"with_vector":  true,
		"with_payload": true,
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(fmt.Sprintf("%s/collections/%s/points/scroll", qdrantURL, collection), "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Result struct {
			Points []qdrantPoint `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Result.Points, nil
}

type DuplicateGroup struct {
	Similarity float32     `json:"similarity"`
	Items      []MediaItem `json:"items"`
}

// HandleGalleryDuplicates API: 获取相似/重复图片分组
func HandleGalleryDuplicates(w http.ResponseWriter, r *http.Request) {
	aiEnabled, err := db.GetSetting("ai_enabled")
	if err != nil || aiEnabled != "true" {
		writeError(w, "AI功能未启用，无法进行图片查重")
		return
	}

	points, err := getAllQdrantPoints()
	if err != nil {
		writeError(w, "获取向量数据失败")
		return
	}

	n := len(points)
	if n < 2 {
		writeOK(w, []DuplicateGroup{})
		return
	}

	// 1. 计算所有两两相似度
	type Pair struct {
		i, j  int
		score float32
	}
	var pairs []Pair

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			// Dot product (since unit vectors, dot product = cosine similarity)
			score := float32(0)
			v1 := points[i].Vector
			v2 := points[j].Vector
			if len(v1) == len(v2) && len(v1) > 0 {
				for k := 0; k < len(v1); k++ {
					score += v1[k] * v2[k]
				}
			}

			// Threshold for duplicate detection is 0.85
			if score >= 0.85 {
				pairs = append(pairs, Pair{i: i, j: j, score: score})
			}
		}
	}

	// 2. 按相似度从高到低排序
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})

	// 3. 并查集 (DSU) 分组
	parent := make([]int, n)
	maxScore := make([]float32, n)
	for i := 0; i < n; i++ {
		parent[i] = i
		maxScore[i] = 0.0
	}

	var find func(int) int
	find = func(i int) int {
		if parent[i] == i {
			return i
		}
		parent[i] = find(parent[i])
		return parent[i]
	}

	union := func(i, j int, score float32) {
		rootI := find(i)
		rootJ := find(j)
		if rootI != rootJ {
			parent[rootI] = rootJ
			if score > maxScore[rootJ] {
				maxScore[rootJ] = score
			}
			if maxScore[rootI] > maxScore[rootJ] {
				maxScore[rootJ] = maxScore[rootI]
			}
		} else {
			if score > maxScore[rootJ] {
				maxScore[rootJ] = score
			}
		}
	}

	for _, pair := range pairs {
		union(pair.i, pair.j, pair.score)
	}

	// 4. 收集分组结果
	groupsMap := make(map[int][]int)
	for i := 0; i < n; i++ {
		root := find(i)
		groupsMap[root] = append(groupsMap[root], i)
	}

	var groups []DuplicateGroup
	for root, indices := range groupsMap {
		// 只有包含 2 个及以上元素的分组才算有重复图片
		if len(indices) < 2 {
			continue
		}

		var items []MediaItem
		for _, idx := range indices {
			p := points[idx]
			absPath := p.Payload.Path
			info, err := os.Stat(absPath)
			if err != nil {
				continue
			}
			relPath, err := filepath.Rel(dataDir, absPath)
			if err != nil {
				continue
			}
			mediaType := "image"
			if isVideoExt(info.Name()) {
				mediaType = "video"
			}
			items = append(items, MediaItem{
				Name:    info.Name(),
				Path:    filepath.ToSlash(relPath),
				Type:    mediaType,
				Size:    info.Size(),
				ModTime: info.ModTime(),
			})
		}

		if len(items) >= 2 {
			groups = append(groups, DuplicateGroup{
				Similarity: maxScore[root],
				Items:      items,
			})
		}
	}

	// 5. 将分组按照相似度从高到低排序
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Similarity > groups[j].Similarity
	})

	writeOK(w, groups)
}
