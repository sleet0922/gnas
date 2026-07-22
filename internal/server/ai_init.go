package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "embed"

	"github.com/jeessy2/gnas/internal/db"
)

func runAICommand(timeout time.Duration, env []string, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if env != nil {
		cmd.Env = env
	}
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("command timed out after %s", timeout)
	}
	return output, err
}

//go:embed embed_server.py
var embedServerCode string

const (
	internalStorageDir = ".gnas"
	modelCacheDir      = "modelscope_cache"
	envDirName         = "qwen3_env"
	modelDirName       = "qwen3_vl_ov"
	tempDirName        = "tmp"
	logFileName        = "embed_server.log"
)

func migrateAIStorageDirs() {
	if err := os.MkdirAll(filepath.Join(dataDir, internalStorageDir), 0755); err != nil {
		log.Printf("[AI 初始化] 创建内部存储目录失败: %v", err)
		return
	}

	// Keep the migration local to dataDir so existing model caches and virtual
	// environments can be moved without copying or downloading them again.
	pairs := [][2]string{
		{thumbnailCacheDirName, ".thumbs"},
		{modelCacheDir, ".modelscope_cache"},
		{envDirName, ".qwen3_env"},
		{modelDirName, ".qwen3_vl_ov"},
		{tempDirName, ".tmp"},
		{logFileName, ".embed_server.log"},
	}
	for _, pair := range pairs {
		targetPath := filepath.Join(dataDir, internalStorageDir, pair[0])
		if _, err := os.Lstat(targetPath); err == nil {
			continue
		}
		legacyNames := []string{pair[1], strings.TrimPrefix(pair[1], ".")}
		for _, legacyName := range legacyNames {
			legacyPath := filepath.Join(dataDir, legacyName)
			if _, err := os.Lstat(legacyPath); err != nil {
				continue
			}
			if err := os.Rename(legacyPath, targetPath); err != nil {
				log.Printf("[AI 初始化] 内部存储迁移失败 %s -> %s: %v", legacyName, filepath.Join(internalStorageDir, pair[0]), err)
			} else {
				log.Printf("[AI 初始化] 已迁移内部存储: %s -> %s", legacyName, filepath.Join(internalStorageDir, pair[0]))
			}
			break
		}
	}
	repairVirtualEnvPaths(filepath.Join(dataDir, internalStorageDir, envDirName))
}

func aiStoragePath(name string) string {
	return filepath.Join(dataDir, internalStorageDir, name)
}

func repairVirtualEnvPaths(envPath string) {
	if _, err := os.Stat(envPath); err != nil {
		return
	}
	targetAbs, err := filepath.Abs(envPath)
	if err != nil {
		return
	}
	legacyPaths := []string{
		filepath.Join(dataDir, envDirName),
		filepath.Join(dataDir, "."+envDirName),
	}
	legacyAbs := make([]string, 0, len(legacyPaths)*2)
	for _, path := range legacyPaths {
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		legacyAbs = append(legacyAbs, abs, filepath.ToSlash(abs))
	}
	targetSlash := filepath.ToSlash(targetAbs)

	paths := []string{filepath.Join(envPath, "pyvenv.cfg")}
	binDir := filepath.Join(envPath, "bin")
	entries, err := os.ReadDir(binDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				paths = append(paths, filepath.Join(binDir, entry.Name()))
			}
		}
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(content, 0) >= 0 {
			continue
		}
		updated := string(content)
		for i := 0; i < len(legacyAbs); i += 2 {
			updated = strings.ReplaceAll(updated, legacyAbs[i], targetAbs)
			updated = strings.ReplaceAll(updated, legacyAbs[i+1], targetSlash)
		}
		if updated != string(content) {
			mode := os.FileMode(0644)
			if info, err := os.Stat(path); err == nil {
				mode = info.Mode().Perm()
			}
			if err := os.WriteFile(path, []byte(updated), mode); err != nil {
				log.Printf("[AI 初始化] 修正 Python 虚拟环境路径失败 %s: %v", path, err)
			}
		}
	}
}

// CheckAndInstallAI 检测并安装 Ollama 和 Qdrant
func CheckAndInstallAI() {
	migrateAIStorageDirs()
	go func() {
		aiEnabled, err := db.GetSetting("ai_enabled")
		if err != nil || aiEnabled != "true" {
			log.Println("[AI 初始化] AI 功能当前未启用，跳过后台服务初始化。")
			return
		}

		// 1. 检查并安装 Python 环境与多模态大模型
		checkAndInstallPythonVLM()

		// 2. 检查并安装 Qdrant
		checkAndInstallQdrant()
	}()
}

func checkAndInstallPythonVLM() {
	if runtime.GOOS != "linux" {
		log.Printf("[AI 初始化] 当前系统非 Linux (%s)，跳过自动安装 Python 依赖及模型。", runtime.GOOS)
		return
	}

	log.Println("[AI 初始化] 准备配置 Python 虚拟环境与多模态向量模型...")

	// 1. 安装系统级 python 依赖
	aptEnv := append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if output, err := runAICommand(5*time.Minute, aptEnv, "apt-get", "update", "-o", "Acquire::Retries=3"); err != nil {
		log.Printf("[AI 初始化] apt update 失败: %v, output: %s", err, string(output))
		return
	}
	if output, err := runAICommand(5*time.Minute, aptEnv, "apt-get", "install", "-y", "python3-venv", "python3-pip", "python3"); err != nil {
		log.Printf("[AI 初始化] apt install 失败: %v, output: %s", err, string(output))
		return
	}
	if _, err := exec.LookPath("python3"); err != nil {
		log.Printf("[AI 初始化] Python 安装后仍不可用: %v", err)
		return
	}

	envDir := aiStoragePath(envDirName)
	modelDir := aiStoragePath(modelDirName)

	// 2. 创建虚拟环境
	pythonPath := filepath.Join(envDir, "bin", "python")
	if _, err := os.Stat(pythonPath); err != nil {
		if removeErr := os.RemoveAll(envDir); removeErr != nil {
			log.Printf("[AI 初始化] 清理不完整 Python 虚拟环境失败: %v", removeErr)
			return
		}
		log.Println("[AI 初始化] 正在创建 Python 虚拟环境...")
		if output, err := runAICommand(5*time.Minute, aptEnv, "python3", "-m", "venv", envDir); err != nil {
			log.Printf("[AI 初始化] 创建虚拟环境失败: %v, output: %s", err, string(output))
			return
		}
	}

	pipPath := filepath.Join(envDir, "bin", "pip")

	// 3. 检查服务是否已经在运行
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8000/health")
	if err == nil && resp.StatusCode == 200 {
		resp.Body.Close()
		log.Println("[AI 初始化] 多模态向量服务已在运行中。")
		return
	}

	// 4. 安装必要的 Python 依赖包
	log.Println("[AI 初始化] 正在检测并安装多模态大模型所需依赖包（首次安装需要几分钟）...")
	tmpDir := aiStoragePath(tempDirName)
	os.MkdirAll(tmpDir, 0755)

	pipEnv := append(os.Environ(), "TMPDIR="+tmpDir, "PIP_NO_CACHE_DIR=1")
	if output, err := runAICommand(30*time.Minute, pipEnv, pipPath, "install", "--no-cache-dir", "torch", "torchvision", "transformers", "pillow", "fastapi", "uvicorn", "modelscope", "qwen-vl-utils"); err != nil {
		log.Printf("[AI 初始化] 安装模型依赖失败: %v, output: %s", err, string(output))
		return
	} else {
		log.Println("[AI 初始化] 模型依赖包配置完成！")
	}

	// 5. 写入 embed_server.py
	os.MkdirAll(modelDir, 0755)
	embedPath := filepath.Join(modelDir, "embed_server.py")
	if err := os.WriteFile(embedPath, []byte(embedServerCode), 0644); err != nil {
		log.Printf("[AI 初始化] 写入 embed_server.py 失败: %v", err)
		return
	}

	// 6. 后台启动 FastAPI 服务
	serverCmd := exec.Command(pythonPath, embedPath)
	serverCmd.Dir = modelDir
	serverCmd.Env = append(os.Environ(), "MODELSCOPE_CACHE="+aiStoragePath(modelCacheDir))

	// 重定向输出到日志文件
	logFile, err := os.OpenFile(aiStoragePath(logFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		serverCmd.Stdout = logFile
		serverCmd.Stderr = logFile
	}

	log.Println("[AI 初始化] 正在拉起后台多模态向量服务...")
	if err := serverCmd.Start(); err != nil {
		log.Printf("[AI 初始化] 启动多模态向量服务失败: %v", err)
		return
	}

	// 7. 等待加载成功 (可能需要从 ModelScope 下载 4GB 的模型，给足时间)
	log.Println("[AI 初始化] 正在下载并加载官方 Qwen3-VL-Embedding-2B 模型，首次拉起大约需要几分钟，请耐心等待...")
	success := false
	for i := 0; i < 450; i++ { // 最多等待 15 分钟 (450 * 2 秒)
		time.Sleep(2 * time.Second)
		resp, err := client.Get("http://127.0.0.1:8000/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				success = true
				break
			}
		}
	}

	if success {
		log.Println("[AI 初始化] 多模态向量服务启动成功！已在本地 8000 端口提供 embedding 提取服务。")
	} else {
		log.Println("[AI 初始化] 警告：多模态向量服务未能按时响应，请检查数据目录下的 embed_server.log 日志。")
	}
}

func checkAndInstallQdrant() {
	// 简单探测 6333 端口
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:6333/")
	if err == nil {
		resp.Body.Close()
		log.Println("[AI 初始化] 已检测到 Qdrant 运行在 6333 端口。")
		initQdrantCollection()
		return
	}

	log.Println("[AI 初始化] 未检测到运行中的 Qdrant，准备安装...")
	if runtime.GOOS == "linux" {
		log.Println("[AI 初始化] 开始下载并安装 Qdrant (deb)...")
		// 下载 deb
		qdrantDeb := "/tmp/qdrant.deb"
		os.Remove(qdrantDeb)
		if output, err := runAICommand(2*time.Minute, nil, "wget", "--inet4-only", "--timeout=30", "--tries=2", "--no-verbose", "https://github.com/qdrant/qdrant/releases/download/v1.18.2/qdrant_1.18.2-1_amd64.deb", "-O", qdrantDeb); err != nil {
			log.Printf("[AI 初始化] 下载 Qdrant 失败: %v, output: %s", err, string(output))
			os.Remove(qdrantDeb)
			return
		}
		if info, err := os.Stat(qdrantDeb); err != nil || info.Size() == 0 {
			log.Printf("[AI 初始化] 下载 Qdrant 完成但安装包为空: %v", err)
			os.Remove(qdrantDeb)
			return
		}
		// dpkg 安装
		if output, err := runAICommand(5*time.Minute, nil, "dpkg", "-i", qdrantDeb); err != nil {
			log.Printf("[AI 初始化] 安装 Qdrant 失败: %v, output: %s", err, string(output))
			return
		}
		os.Remove(qdrantDeb)
		log.Println("[AI 初始化] Qdrant 安装成功！")

		// 启动服务 (某些 deb 包可能没有 systemd，直接后台运行二进制文件)
		qdrantCmd := exec.Command("/usr/bin/qdrant")
		qdrantCmd.Dir = "/var/lib/qdrant" // Qdrant 需要一个工作目录
		err = qdrantCmd.Start()
		if err != nil {
			log.Printf("[AI 初始化] 启动 Qdrant 失败: %v", err)
		} else {
			// 将进程丢弃让其在后台独立运行，或保留引用
			go func() {
				qdrantCmd.Wait()
			}()
		}

		// 等待启动
		ready := false
		for i := 0; i < 10; i++ {
			time.Sleep(2 * time.Second)
			resp, err := client.Get("http://127.0.0.1:6333/")
			if err == nil {
				resp.Body.Close()
				ready = true
				break
			}
		}
		if !ready {
			log.Println("[AI 初始化] Qdrant 安装后未能在 20 秒内启动")
			return
		}
		initQdrantCollection()
	} else {
		log.Printf("[AI 初始化] 当前系统为 %s，请手动安装 Qdrant (推荐使用 Docker: docker run -p 6333:6333 qdrant/qdrant)", runtime.GOOS)
	}
}
