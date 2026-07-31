package main

import (
	"crypto/tls"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jeessy2/gnas/internal/db"
	"github.com/jeessy2/gnas/internal/server"
)

//go:embed all:web/dist
var webDist embed.FS

const (
	listenAddr = ":8082"
	dataPath   = "/var/lib/gnas"
)

var version = "DEV"

func main() {
	// 初始化数据目录
	dataDir := filepath.Clean(dataPath)
	os.MkdirAll(dataDir, 0755)
	server.InitDataDir(dataDir)
	server.Version = version
	server.CleanupGalleryExportTemps()

	// 检查并安装 ffmpeg
	server.CheckAndInstallFFmpeg()

	// 初始化 SQLite 数据库
	dbPath := filepath.Join(dataDir, "gnas.db")
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 初始化 JWT 密钥（从 DB 加载或生成新密钥）
	server.InitAuth()

	// 检查并安装 AI 依赖 (Ollama, Qdrant)
	server.CheckAndInstallAI()

	// 启动 HTTP 服务
	if err := startHTTPServer(); err != nil {
		log.Fatalf("启动 HTTP 服务失败: %v", err)
	}

	// 启动后异步扫描生成所有媒体缩略图
	go server.GenerateAllThumbnails()

	// 启动回收站自动清理定时任务
	server.StartRecycleBinCleanup()

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("服务正在关闭...")
}

func startHTTPServer() error {
	mux := http.NewServeMux()
	publicAPI := func(h http.HandlerFunc) http.HandlerFunc {
		return server.WithAccessControl(h)
	}
	protectedAPI := func(h http.HandlerFunc) http.HandlerFunc {
		return server.WithAccessControl(server.RequireAuth(h))
	}

	// API 路由
	mux.HandleFunc("/api/login", publicAPI(server.HandleLogin))
	mux.HandleFunc("/api/logout", protectedAPI(server.HandleLogout))
	mux.HandleFunc("/api/change-password", protectedAPI(server.HandleChangePassword))
	mux.HandleFunc("/api/status", protectedAPI(server.HandleStatus))
	mux.HandleFunc("/api/logs", protectedAPI(server.HandleGetLogs))
	mux.HandleFunc("/api/logs/clear", protectedAPI(server.HandleClearLogs))

	// 文件管理 API
	mux.HandleFunc("/api/files", protectedAPI(server.HandleFileList))
	mux.HandleFunc("/api/files/upload", protectedAPI(server.HandleFileUpload))
	mux.HandleFunc("/api/files/download", protectedAPI(server.HandleFileDownload))
	mux.HandleFunc("/api/files/thumb", protectedAPI(server.HandleFileThumbnail))
	mux.HandleFunc("/api/gallery", protectedAPI(server.HandleGalleryList))
	mux.HandleFunc("/api/gallery/export", protectedAPI(server.HandleGalleryExport))
	mux.HandleFunc("/api/gallery/import", protectedAPI(server.HandleGalleryImport))
	mux.HandleFunc("/api/files/delete", protectedAPI(server.HandleFileDelete))
	mux.HandleFunc("/api/files/batch-delete", protectedAPI(server.HandleFileBatchDelete))
	mux.HandleFunc("/api/files/mkdir", protectedAPI(server.HandleFileMkdir))
	mux.HandleFunc("/api/files/rename", protectedAPI(server.HandleFileRename))
	mux.HandleFunc("/api/files/flatten", protectedAPI(server.HandleFileFlatten))
	mux.HandleFunc("/api/search", protectedAPI(server.HandleSearchPhotos))
	mux.HandleFunc("/api/gallery/duplicates", protectedAPI(server.HandleGalleryDuplicates))

	// 回收站 API
	mux.HandleFunc("/api/recycle-bin", protectedAPI(server.HandleRecycleBinList))
	mux.HandleFunc("/api/recycle-bin/thumb", protectedAPI(server.HandleRecycleBinThumb))
	mux.HandleFunc("/api/recycle-bin/restore", protectedAPI(server.HandleRecycleBinRestore))
	mux.HandleFunc("/api/recycle-bin/delete", protectedAPI(server.HandleRecycleBinDelete))
	mux.HandleFunc("/api/recycle-bin/clear", protectedAPI(server.HandleRecycleBinClear))

	// 系统信息与设置 API
	mux.HandleFunc("/api/system", protectedAPI(server.HandleSystemInfo))
	mux.HandleFunc("/api/system/stale-scan", protectedAPI(server.HandleStaleScan))
	mux.HandleFunc("/api/system/stale-cleanup", protectedAPI(server.HandleStaleCleanup))
	mux.HandleFunc("/api/settings", protectedAPI(server.HandleGetSettings))
	mux.HandleFunc("/api/settings/update", protectedAPI(server.HandleUpdateSettings))

	// 静态文件服务
	distFS, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Printf("未找到前端静态文件: %v, 仅提供 API 服务", err)
	} else {
		fileServer := http.FileServer(http.FS(distFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path != "/" && path != "" {
				f, err := distFS.Open(path[1:])
				if err != nil {
					r.URL.Path = "/"
				} else {
					f.Close()
				}
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	tlsSettings, err := server.GetTLSSettings()
	if err != nil {
		return fmt.Errorf("读取 TLS 配置失败: %w", err)
	}
	if err := server.ValidateTLSSettings(tlsSettings); err != nil {
		return err
	}

	var tlsConfig *tls.Config
	if tlsSettings.Enabled {
		certificate, err := tls.LoadX509KeyPair(tlsSettings.CertFile, tlsSettings.KeyFile)
		if err != nil {
			return fmt.Errorf("加载 TLS 证书失败: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		}
	}

	// 解析监听地址
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("监听端口异常: %w", err)
	}

	httpServer := &http.Server{Handler: mux, TLSConfig: tlsConfig}
	if tlsSettings.Enabled {
		log.Printf("NAS HTTPS 服务启动，监听 %s", listenAddr)
	} else {
		log.Printf("NAS HTTP 服务启动，监听 %s", listenAddr)
	}
	go func() {
		var serveErr error
		if tlsSettings.Enabled {
			serveErr = httpServer.ServeTLS(l, "", "")
		} else {
			serveErr = httpServer.Serve(l)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常: %v", serveErr)
		}
	}()

	return nil
}
