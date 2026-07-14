package main

import (
	"embed"
	"flag"
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

var (
	listenAddr   = flag.String("l", ":8080", "监听地址")
	dataDirFlag  = flag.String("data", "/var/lib/gnas", "数据目录")
)

var version = "DEV"

func main() {
	flag.Parse()

	// 初始化数据目录
	dataDir := *dataDirFlag
	os.MkdirAll(dataDir, 0755)
	server.InitDataDir(dataDir)

	// 初始化 SQLite 数据库
	dbPath := filepath.Join(dataDir, "gnas.db")
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 启动 HTTP 服务
	if err := startHTTPServer(); err != nil {
		log.Fatalf("启动 HTTP 服务失败: %v", err)
	}

	// 启动后异步扫描生成所有媒体缩略图
	go server.GenerateAllThumbnails()

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
	mux.HandleFunc("/api/files/delete", protectedAPI(server.HandleFileDelete))
	mux.HandleFunc("/api/files/batch-delete", protectedAPI(server.HandleFileBatchDelete))
	mux.HandleFunc("/api/files/mkdir", protectedAPI(server.HandleFileMkdir))
	mux.HandleFunc("/api/files/rename", protectedAPI(server.HandleFileRename))

	// 系统信息 API
	mux.HandleFunc("/api/system", protectedAPI(server.HandleSystemInfo))

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

	// 解析监听地址
	l, err := net.Listen("tcp", *listenAddr)
	if err != nil {
		return fmt.Errorf("监听端口异常: %w", err)
	}

	log.Printf("NAS 服务启动，监听 %s", *listenAddr)
	go func() {
		if err := http.Serve(l, mux); err != nil {
			log.Fatalf("HTTP 服务异常: %v", err)
		}
	}()

	return nil
}
