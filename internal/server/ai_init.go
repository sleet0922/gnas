package server

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	_ "embed"

	"github.com/jeessy2/gnas/internal/db"
)

//go:embed embed_server.py
var embedServerCode string

// CheckAndInstallAI 检测并安装 Ollama 和 Qdrant
func CheckAndInstallAI() {
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
	aptCmd := exec.Command("apt-get", "install", "-y", "python3-venv", "python3-pip", "python3")
	if output, err := aptCmd.CombinedOutput(); err != nil {
		log.Printf("[AI 初始化] apt install 失败: %v, output: %s", err, string(output))
		// 继续执行，可能系统已存在
	}

	envDir := filepath.Join(dataDir, "qwen3_env")
	modelDir := filepath.Join(dataDir, "qwen3_vl_ov")

	// 2. 创建虚拟环境
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		log.Println("[AI 初始化] 正在创建 Python 虚拟环境...")
		venvCmd := exec.Command("python3", "-m", "venv", envDir)
		if output, err := venvCmd.CombinedOutput(); err != nil {
			log.Printf("[AI 初始化] 创建虚拟环境失败: %v, output: %s", err, string(output))
			return
		}
	}

	pythonPath := filepath.Join(envDir, "bin", "python")
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
	tmpDir := filepath.Join(dataDir, "tmp")
	os.MkdirAll(tmpDir, 0755)

	installCmd := exec.Command(pipPath, "install", "--no-cache-dir", "torch", "torchvision", "transformers", "pillow", "fastapi", "uvicorn", "modelscope", "qwen-vl-utils")
	installCmd.Env = append(os.Environ(), "TMPDIR="+tmpDir)
	if output, err := installCmd.CombinedOutput(); err != nil {
		log.Printf("[AI 初始化] 安装模型依赖警告 (尝试继续): %v, output: %s", err, string(output))
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
	serverCmd.Env = append(os.Environ(), "MODELSCOPE_CACHE="+filepath.Join(dataDir, "modelscope_cache"))
	
	// 重定向输出到日志文件
	logFile, err := os.OpenFile(filepath.Join(dataDir, "embed_server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
		wgetCmd := exec.Command("wget", "https://github.com/qdrant/qdrant/releases/download/v1.18.2/qdrant_1.18.2-1_amd64.deb", "-O", "/tmp/qdrant.deb")
		if output, err := wgetCmd.CombinedOutput(); err != nil {
			log.Printf("[AI 初始化] 下载 Qdrant 失败: %v, output: %s", err, string(output))
			return
		}
		// dpkg 安装
		dpkgCmd := exec.Command("dpkg", "-i", "/tmp/qdrant.deb")
		if output, err := dpkgCmd.CombinedOutput(); err != nil {
			log.Printf("[AI 初始化] 安装 Qdrant 失败: %v, output: %s", err, string(output))
			return
		}
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
		for i := 0; i < 10; i++ {
			time.Sleep(2 * time.Second)
			resp, err := client.Get("http://127.0.0.1:6333/")
			if err == nil {
				resp.Body.Close()
				break
			}
		}
		initQdrantCollection()
	} else {
		log.Printf("[AI 初始化] 当前系统为 %s，请手动安装 Qdrant (推荐使用 Docker: docker run -p 6333:6333 qdrant/qdrant)", runtime.GOOS)
	}
}
