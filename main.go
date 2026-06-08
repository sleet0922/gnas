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
	"strconv"
	"syscall"
	"time"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/dns"
	"github.com/jeessy2/ddns-go/v6/util"
	"github.com/jeessy2/gnas/internal/ddns"
	"github.com/jeessy2/gnas/internal/server"
)

//go:embed all:web/dist
var webDist embed.FS

var (
	listenAddr   = flag.String("l", ":8080", "监听地址")
	configFile   = flag.String("c", "", "配置文件路径")
	ddnsInterval = flag.Int("f", 300, "DDNS 更新间隔(秒)")
	cacheTimes   = flag.Int("cacheTimes", 5, "缓存次数")
	skipVerify   = flag.Bool("skipVerify", false, "跳过证书验证")
	customDNS    = flag.String("dns", "", "自定义 DNS 服务器")
)

var version = "DEV"

func main() {
	flag.Parse()

	// 设置配置文件路径
	if *configFile != "" {
		absPath, _ := filepath.Abs(*configFile)
		os.Setenv(util.ConfigFilePathENV, absPath)
	}
	os.Setenv(util.IPCacheTimesENV, strconv.Itoa(*cacheTimes))

	if *skipVerify {
		util.SetInsecureSkipVerify()
	}
	if *customDNS != "" {
		util.SetDNS(*customDNS)
	}

	// 初始化 DDNS 配置
	conf, _ := config.GetConfigCached()
	conf.CompatibleConfig()
	util.InitLogLang(conf.Lang)
	util.InitBackupDNS(*customDNS, conf.Lang)

	// 等待网络
	util.WaitInternet(dns.Addresses)

	// 启动 DDNS 定时任务
	ddnsSvc := ddns.New(time.Duration(*ddnsInterval) * time.Second)
	ddnsSvc.Start()
	defer ddnsSvc.Stop()

	// 启动 HTTP 服务
	if err := startHTTPServer(); err != nil {
		log.Fatalf("启动 HTTP 服务失败: %v", err)
	}

	// 等待退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("服务正在关闭...")
}

func startHTTPServer() error {
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/login", server.HandleLogin)
	mux.HandleFunc("/api/logout", server.AuthMiddleware(server.HandleLogout))
	mux.HandleFunc("/api/status", server.AuthMiddleware(server.HandleStatus))
	mux.HandleFunc("/api/config", server.AuthMiddleware(server.HandleGetConfig))
	mux.HandleFunc("/api/config/save", server.AuthMiddleware(server.HandleSaveConfig))
	mux.HandleFunc("/api/logs", server.AuthMiddleware(server.HandleGetLogs))
	mux.HandleFunc("/api/logs/clear", server.AuthMiddleware(server.HandleClearLogs))
	mux.HandleFunc("/api/webhook/test", server.AuthMiddleware(server.HandleWebhookTest))

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
