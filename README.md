# GNAS (Go Private NAS)

GNAS 是一个轻量级、安全且易于部署的个人私有 NAS（网络附加存储）系统。它由 Go 编写的后端服务、基于 Vue 3 + Vuetify 3 的 Web 管理端以及基于 Flutter 的 Android 移动客户端组成。

---

## 项目特性

### 📂 文件管理
- **全面操作**：支持文件及文件夹的浏览、新建、重命名、上传、下载与删除操作。
- **批量处理**：支持多选文件进行批量删除，提高管理效率。
- **元数据定制**：支持为文件添加显示别名、描述信息和自定义标签（Tags），便于检索。
- **快捷收藏**：支持一键收藏重要文件。

### 🖼️ 智能相册 (Gallery)
- **多媒体支持**：自动扫描和展示图片与视频文件，支持常见媒体格式（含 WebP / MP4 等）。
- **缩略图缓存**：后端集成自动缩略图生成器（视频依赖 `ffmpeg`），显著降低前端加载带宽并提升渲染速度。
- **图片懒加载**：前端采用懒加载技术，保证数千张照片的流畅滑动体验。

### 🖥️ 系统状态与占用监控
- **运行指标**：实时监控主机 OS、CPU 架构、运行时间（Uptime）以及 SQLite 数据库占用空间。
- **系统占用**：可视化展示系统整体 CPU 使用率、物理内存及磁盘的使用情况。
- **进程监控**：展示 GNAS 自身进程的 CPU 与堆内存消耗，便于资源审计。

### 🪵 运行日志仪表盘
- **内存日志**：后端通过内存缓冲区收集系统日志，在 Web 控制端和 App 端提供实时的日志流显示。
- **一键清除**：支持清空日志缓冲区，利于日志审计。

### 🔒 安全防护机制
- **JWT 身份认证**：API 访问受 JSON Web Token (JWT) 保护，支持强密码验证与动态 Token 颁发。
- **路径穿越保护**：严格的文件路径安全校验，防止恶意路径穿越（`../`）攻击。
- **禁止公网访问（WAN Block）**：提供基于 IP 的访问控制，开启后仅限局域网/内网 IP 访问，防止外网未授权嗅探。

---

## 技术栈

### 后端 (Go)
- **核心框架**：原生 `net/http` 服务端，无笨重依赖。
- **数据库**：`modernc.org/sqlite`（纯 Go 实现的 SQLite 驱动，**无需 CGO**，极易跨平台编译）。
- **认证安全**：`github.com/golang-jwt/jwt/v5` 进行会话管理，`golang.org/x/crypto/bcrypt` 密码哈希。
- **媒体处理**：`golang.org/x/image` 执行图片缩略图无损缩放。

### 网页前端 (Vue)
- **框架**：Vue 3 (Composition API) + TypeScript
- **构建工具**：Vite
- **UI 框架**：Vuetify 3 (Material Design 风格)
- **路由管理**：Vue Router

### 移动客户端 (Flutter)
- **跨平台框架**：Flutter (Dart)
- **UI 风格**：Material Design 3
- **本地存储**：`shared_preferences` 存储连接配置与 Token。

---

## 项目目录结构

```text
gnas/
├── main.go               # 后端服务入口及命令行参数解析
├── Makefile              # 快速运行、打包与部署指令
├── go.mod                # Go 依赖配置
├── go.sum                # Go 依赖版本校验
├── internal/             # 后端业务逻辑包
│   ├── db/               # 数据库初始化、SQLite Schema 迁移及通用查询
│   └── server/           # HTTP 路由注册、中间件及 API 处理器（认证、文件、系统、缩略图等）
├── web/                  # 前端 Web 客户端
│   ├── src/
│   │   ├── pages/        # Vue 页面组件（文件管理、相册、系统占用、系统日志等）
│   │   ├── router/       # Vue 路由配置
│   │   └── App.vue       # 主页面布局框架
│   └── package.json      # 前端依赖配置
└── gnas_app/             # 移动客户端 (Flutter)
    ├── lib/
    │   ├── models/       # 数据模型 (系统状态、文件信息等)
    │   ├── pages/        # Flutter 界面 (登录、仪表盘、文件列表、相册、设置、日志等)
    │   ├── services/     # API 客户端封装
    │   └── main.dart     # Flutter 应用程序入口
    └── pubspec.yaml      # Flutter 依赖配置
```

---

## 快速开始

### 1. 编译前端网页
```bash
cd web
npm install
npm run build
```
编译产物将输出在 `web/dist` 目录中。后端 Go 服务在构建时会通过 `go:embed` 将该目录静态文件打包到单一二进制文件中。

### 2. 编译并启动后端服务
在项目根目录下：
```bash
# 自动整理 Go 依赖
go mod tidy

# 本地直接运行（默认监听 :8080，数据存放在 ./data 目录）
go run main.go -l :8080 -data ./data
```

### 3. 跨平台打包 (例如 Linux x64)
```bash
# 静态编译为 Linux 二进制文件（无需任何运行时依赖）
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -o gnas main.go
```

### 4. 移动客户端运行
确保已配置 Flutter 环境，进入 `gnas_app` 目录：
```bash
cd gnas_app
flutter pub get
flutter run
```
在 App 登录界面输入 GNAS 服务端的地址（如 `http://192.168.1.100:8080`）及账户信息（默认账号：`root`，密码：`root`）即可建立连接。

---

## 命令行参数说明

GNAS 后端支持以下命令行参数：
- `-l` : 绑定监听的地址与端口（默认 `":8080"`）。
- `-data` : 数据和 SQLite 数据库的存储路径（默认 `"/var/lib/gnas"`）。