# GNAS 后端 API 文档

> 适用于 Flutter Android 端开发  
> 基础地址：`http://<host>:8080`  
> 数据存储根目录：`/var/lib/gnas/`

---

## 目录

1. [通用说明](#1-通用说明)
2. [认证](#2-认证)
3. [登录 / 登出 / 修改密码](#3-登录--登出--修改密码)
4. [系统状态](#4-系统状态)
5. [系统信息](#5-系统信息)
6. [日志](#7-日志)
7. [文件管理](#8-文件管理)
8. [相册（Gallery）](#9-相册)
9. [附录](#10-附录)

---

## 1. 通用说明

### 1.1 请求/响应格式

所有 API 请求体使用 `application/json`（文件上传除外）。

**成功响应格式：**
```json
{
  "code": 0,
  "data": { ... }
}
```

**失败响应格式：**
```json
{
  "code": 1,
  "message": "错误描述"
}
```

**HTTP 状态码说明：**

| 状态码 | 含义 | 触发条件 |
|--------|------|----------|
| 200 | 成功 | 正常处理完毕，业务成功/失败由 `code` 字段区分 |
| 400 | 请求错误 | 参数缺失、格式错误、用户名密码错误等业务异常 |
| 401 | 未登录 | 缺少 token 或 token 过期/无效 |
| 403 | 禁止访问 | 公网访问被禁用（`notAllowWanAccess=true` 且非内网 IP） |
| 404 | 资源不存在 | 请求的文件等资源不存在 |

### 1.2 数据存储

- 文件存储根目录：`/var/lib/gnas/`
- SQLite 数据库：`/var/lib/gnas/gnas.db`
- 缩略图缓存目录：`/var/lib/gnas/.thumbs/`
- 缩略图文件名格式：`sl_原文件名.扩展名`（如 `sl_photo.jpg`）

---

## 2. 认证

### 2.1 认证方式

使用 **JWT（JSON Web Token）** 进行认证。

- **算法**：HS256
- **有效期**：30 天
- **Payload 字段**：
  - `username`: 用户名
  - `exp`: 过期时间
  - `iat`: 签发时间

### 2.2 携带 Token 方式

**方式一：Authorization Header（推荐用于 AJAX/代码请求）**
```
Authorization: Bearer <token>
```

**方式二：URL Query 参数（用于图片/文件等直接 URL 访问）**
```
/api/files/thumb?path=photo.jpg&token=<token>
/api/files/download?path=file.pdf&token=<token>
```

---

## 3. 登录 / 登出 / 修改密码

### 3.1 登录检测（GET）

获取登录状态和系统初始化信息。

```
GET /api/login
```

**请求头：** 无（不需要认证）

**响应示例：**
```json
{
  "code": 0,
  "data": {
    "needSetup": false
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| needSetup | boolean | 是否需要初始化（始终返回 false） |

> **首次启动行为：** 后端自动创建默认用户 `root`，默认密码 `root`，同时默认开启 `notAllowWanAccess=true`。

---

### 3.2 登录（POST）

```
POST /api/login
Content-Type: application/json
```

**请求体：**
```json
{
  "username": "root",
  "password": "root"
}
```

**成功响应：**
```json
{
  "code": 0,
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs..."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| token | string | JWT 令牌，有效期 30 天 |

**失败响应：**
```json
{
  "code": 1,
  "message": "用户名或密码错误"
}
```

> **登录失败锁定：** 连续 5 次登录失败后锁定，返回 `"登录失败次数过多，请稍后再试"`。使用 `clearToken()`（客户端清除 token）配合重新登录可重置计数器（实际计数器位于服务端内存，重启服务亦可重置）。

---

### 3.3 登出

```
POST /api/logout
Authorization: Bearer <token>
```

**请求体：** 无（body 为空）

**响应：**
```json
{
  "code": 0,
  "data": null
}
```

> 登出是客户端的操作（删除本地 token），服务端无实际状态清理。

---

### 3.4 修改密码

```
POST /api/change-password
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体：**
```json
{
  "oldPassword": "root",
  "newPassword": "newpass123"
}
```

**参数说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| oldPassword | string | 是 | 当前密码 |
| newPassword | string | 是 | 新密码，至少 4 个字符 |

**成功响应：**
```json
{
  "code": 0,
  "data": null
}
```

**失败响应：**
```json
{
  "code": 1,
  "message": "旧密码错误"
}
```

> **注意：** 修改密码后，当前使用的 JWT token 仍然有效（不会立即过期）。客户端应该重新登录获取新 token。

---

## 4. 系统状态

### 4.1 获取系统状态

```
GET /api/status
Authorization: Bearer <token>
```

**请求参数：** 无

**响应：**
```json
{
  "code": 0,
  "data": {
    "version": "1.0.0",
    "username": "root"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| version | string | 后端版本号 |
| username | string | 当前登录用户名 |

> **用途：** 客户端可用此接口检查 token 是否有效，同时获取用户名。

---

## 5. 系统信息

### 5.1 获取系统硬件/进程信息

```
GET /api/system
Authorization: Bearer <token>
```

**请求参数：** 无

**响应：**
```json
{
  "code": 0,
  "data": {
    "os": "linux",
    "arch": "amd64",
    "cpuCores": 4,
    "cpuUsage": 23.5,
    "procCPU": 1.2,
    "memoryTotal": 8388608,
    "memoryUsed": 4194304,
    "memoryFree": 4194304,
    "procMem": 524288,
    "procMemSys": 1048576,
    "diskTotal": 1073741824,
    "diskUsed": 536870912,
    "diskFree": 536870912,
    "uptime": 3600.5,
    "dbSize": 4096,
    "dbSizeString": "4.0 KB"
  }
}
```

**字段说明：**

| 字段 | 类型 | 单位 | 说明 |
|------|------|------|------|
| os | string | - | 操作系统（如 `linux`） |
| arch | string | - | CPU 架构（如 `amd64`） |
| cpuCores | int | - | CPU 逻辑核心数 |
| cpuUsage | float | % | **系统** CPU 使用率（0~100） |
| procCPU | float | % | **进程** CPU 使用率（0~100*核数） |
| memoryTotal | uint64 | bytes | 系统总物理内存 |
| memoryUsed | uint64 | bytes | 系统已用内存 |
| memoryFree | uint64 | bytes | 系统可用内存 |
| procMem | uint64 | bytes | 本进程堆内存使用量（Go Alloc） |
| procMemSys | uint64 | bytes | 本进程从系统获取的总内存（Go Sys） |
| diskTotal | uint64 | bytes | 数据目录所在磁盘总容量 |
| diskUsed | uint64 | bytes | 数据目录所在磁盘已用容量 |
| diskFree | uint64 | bytes | 数据目录所在磁盘可用容量 |
| uptime | float | 秒 | 服务启动至今的时长 |
| dbSize | int64 | bytes | SQLite 数据库文件大小 |
| dbSizeString | string | - | 数据库文件大小（格式化显示，如 `4.0 KB`） |

> **注意：** CPU 使用率是瞬时采样值，需要在间隔后再次调用才能计算使用率。首次调用返回 0。

---



## 7. 日志

### 7.1 获取日志

```
GET /api/logs
Authorization: Bearer <token>
```

**请求参数：** 无

**响应：**
```json
{
  "code": 0,
  "data": [
    "2026/06/10 12:00:00 NAS 服务启动，监听 :8080",
    "2026/06/10 12:00:05 域名更新成功: example.com -> 1.2.3.4"
  ]
}
```

> **说明：** 日志数据存储在服务端内存中，最多保留 200 条，服务重启后清空。

---

### 7.2 清除日志

```
POST /api/logs/clear
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体：** 无

**响应：**
```json
{
  "code": 0,
  "data": null
}
```

---

## 8. 文件管理

### 8.1 通用说明

- **安全机制：** 所有文件路径通过 `safePath()` 函数校验，防止路径穿越攻击（`../../etc/passwd`）
- **路径格式：** 路径使用 `/` 分隔符，如 `/photos/vacation/photo.jpg`
- **根目录：** `/` 对应数据存储根目录（`/var/lib/gnas/`）
- **文件隐藏规则：** 以 `.` 开头的文件和目录不会被列出（包括 `.thumbs` 缩略图缓存目录）
- **数据库文件：** 以 `gnas.db` 开头的文件不会被列出

---

### 8.2 文件列表

```
GET /api/files?path=/
Authorization: Bearer <token>
```

**查询参数：**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| path | string | 否 | `/` | 目录路径 |

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "name": "photos",
      "path": "/photos",
      "isDir": true,
      "size": 4096,
      "modTime": "2026-06-10T12:00:00Z"
    },
    {
      "name": "readme.txt",
      "path": "/readme.txt",
      "isDir": false,
      "size": 1024,
      "modTime": "2026-06-09T10:00:00Z"
    }
  ]
}
```

**FileInfo 字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 文件名 |
| path | string | 相对路径（用于后续 API 调用） |
| isDir | boolean | 是否为目录 |
| size | int64 | 文件大小（字节） |
| modTime | string | 修改时间（RFC3339 格式） |

> **排序规则：** 目录排前面，目录内按文件名不区分大小写排序。

---

### 8.3 上传文件

```
POST /api/files/upload?path=/
Authorization: Bearer <token>
Content-Type: multipart/form-data
```

**查询参数：**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| path | string | 否 | `/` | 上传到的目录路径 |

**表单字段：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 上传的文件，字段名必须为 `file` |

**响应：**
```json
{
  "code": 0,
  "data": null
}
```

> **限制：** 单次上传文件大小限制为 32MB（`32 << 20`），如果需要上传更大文件，客户端需要分片上传。

---

### 8.4 下载文件

```
GET /api/files/download?path=/photo.jpg&token=<token>
```

> 此接口支持通过 URL Query 参数传递 token，便于浏览器直接访问。

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 文件路径 |
| disposition | string | 否 | 预览模式：`inline`；不传则为附件下载模式 |

**请求头（使用 Authorization header 时）：**
```
Authorization: Bearer <token>
```

**响应：**

- **下载模式（不传 disposition）：**
  - `Content-Type`: 根据文件类型自动识别
  - `Content-Disposition`: `attachment; filename="..."; filename*=UTF-8''...`
  - `Content-Length`: 文件大小
  - 响应体：文件二进制内容

- **预览模式（`disposition=inline`）：**
  - `Content-Type`: 根据文件类型自动识别
  - `Content-Disposition`: `inline; filename="..."; filename*=UTF-8''...`
  - `Content-Length`: 文件大小
  - 响应体：文件二进制内容

> **注意：** 对于图片等可直接预览的文件，使用 `inline` 模式会直接在浏览器中显示。

**Flutter 示例：**
```dart
// 下载文件
var response = await http.get(
  Uri.parse('$baseUrl/api/files/download?path=/photo.jpg&token=$token'),
);
// 保存 response.bodyBytes 到本地文件

// 预览图片
var response = await http.get(
  Uri.parse('$baseUrl/api/files/download?path=/photo.jpg&disposition=inline&token=$token'),
);
// 使用 Image.memory(response.bodyBytes) 显示
```

---

### 8.5 获取缩略图

```
GET /api/files/thumb?path=/photo.jpg&token=<token>
```

> 此接口支持通过 URL Query 参数传递 token，便于 `<img>` 标签直接使用。

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 图片文件路径 |

**响应：**
- `Content-Type`: 图片 MIME 类型
- `Cache-Control`: `public, max-age=86400`（缓存 24 小时）
- 响应体：缩略图二进制内容

> **缩略图生成规则：**
> 1. **缓存目录：** `/var/lib/gnas/.thumbs/`
> 2. **缓存文件名：** `sl_原文件名.扩展名`（如 `sl_beach.jpg`）
> 3. **生成算法：** Catmull-Rom 缩放，最大宽度 **300px**，按比例缩放
> 4. **输出格式：** JPEG（quality=80）、PNG、GIF 保持原格式
> 5. **缓存策略：** 缓存文件修改时间晚于原文件则直接返回缓存，否则重新生成
> 6. **原图小于 300px 时：** 直接复制原图作为缩略图
> 7. **非图片文件：** 返回 404

**Flutter 示例：**
```dart
// 方式一：直接使用 URL（推荐）
Image.network('$baseUrl/api/files/thumb?path=/photo.jpg&token=$token');

// 方式二：手动请求
var response = await http.get(
  Uri.parse('$baseUrl/api/files/thumb?path=/photo.jpg&token=$token'),
);
if (response.statusCode == 200) {
  // 使用 response.bodyBytes 显示缩略图
}
```

---

### 8.6 新建文件夹

```
POST /api/files/mkdir
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体：**
```json
{
  "path": "/新建文件夹"
}
```

**参数说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 要创建的目录路径（支持多级，如 `/a/b/c`） |

**响应：**
```json
{
  "code": 0,
  "data": null
}
```

---

### 8.7 重命名

```
POST /api/files/rename
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体：**
```json
{
  "oldPath": "/old_name.txt",
  "newName": "new_name.txt"
}
```

**参数说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| oldPath | string | 是 | 原文件/目录路径 |
| newName | string | 是 | 新文件名（仅文件名，不是完整路径） |

> **注意：** `newName` 只是文件名，不能包含路径。重命名后缩略图缓存会被清理。

**响应：**
```json
{
  "code": 0,
  "data": null
}
```

---

### 8.8 删除文件/目录

```
POST /api/files/delete
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体：**
```json
{
  "path": "/unwanted_file.txt"
}
```

**参数说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 要删除的文件或目录路径 |

> **注意：** 删除目录会递归删除所有内容。删除图片时会自动清理对应的缩略图缓存。

**响应：**
```json
{
  "code": 0,
  "data": null
}
```

---

### 8.9 批量删除

```
POST /api/files/batch-delete
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体：**
```json
{
  "paths": ["/file1.txt", "/file2.txt", "/dir1"]
}
```

**参数说明：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| paths | string[] | 是 | 要删除的文件或目录路径数组 |

**成功响应：**
```json
{
  "code": 0,
  "data": null
}
```

**部分失败响应：**
```json
{
  "code": 1,
  "message": "部分文件删除失败: 2 个",
  "data": ["/file1.txt", "/dir1"]
}
```

> **注意：** `data` 字段中会包含删除失败的路径列表。删除图片时会自动清理对应的缩略图缓存。

---

## 9. 相册

### 9.1 获取所有媒体文件

```
GET /api/gallery
Authorization: Bearer <token>
```

**请求参数：** 无

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "name": "beach.jpg",
      "path": "/photos/beach.jpg",
      "type": "image",
      "size": 2048576,
      "modTime": "2026-06-10T12:00:00Z"
    },
    {
      "name": "vacation.mp4",
      "path": "/videos/vacation.mp4",
      "type": "video",
      "size": 52428800,
      "modTime": "2026-06-09T10:00:00Z"
    }
  ]
}
```

**MediaItem 字段说明：**

| 字段 | 类型 | 说明 |
|------|------|------|
| name | string | 文件名 |
| path | string | 相对路径（可用于文件下载接口获取原图） |
| type | string | 媒体类型：`image` 或 `video` |
| size | int64 | 文件大小（字节） |
| modTime | string | 修改时间（RFC3339 格式） |

> **媒体识别规则：**
> - 图片扩展名：`.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.bmp`
> - 视频扩展名：`.mp4`, `.webm`, `.ogv`, `.mov`
>
> **排序规则：** 按修改时间倒序（最新的在前）
>
> 缩略图缓存目录 `.thumbs` 下的文件不会被包含。

**Flutter 端资源获取建议：**

| 场景 | 接口 | 说明 |
|------|------|------|
| 图片缩略图 | `GET /api/files/thumb?path=...&token=...` | 快速加载，带宽友好 |
| 图片原图 | `GET /api/files/download?path=...&disposition=inline&token=...` | 完整分辨率查看 |
| 视频预览 | `GET /api/files/download?path=...&disposition=inline&token=...` | 流式播放视频 |

---

## 10. 附录

### 10.1 完整路由表

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | `/api/login` | ❌ | 登录检测 |
| POST | `/api/login` | ❌ | 用户登录 |
| POST | `/api/logout` | ✅ | 登出 |
| POST | `/api/change-password` | ✅ | 修改密码 |
| GET | `/api/status` | ✅ | 系统状态（获取登录用户名） |
| GET | `/api/system` | ✅ | 系统信息（CPU/内存/磁盘） |
| GET | `/api/config` | ✅ | 获取所有配置 |
| POST | `/api/config/save` | ✅ | 保存所有配置 |
| GET | `/api/logs` | ✅ | 获取日志 |
| POST | `/api/logs/clear` | ✅ | 清空日志 |
| POST | `/api/webhook/test` | ✅ | 测试 Webhook |
| GET | `/api/files` | ✅ | 文件列表（支持 query `path`） |
| POST | `/api/files/upload` | ✅ | 上传文件（支持 query `path`） |
| GET | `/api/files/download` | ✅ | 下载/预览文件 |
| GET | `/api/files/thumb` | ✅ | 获取缩略图 |
| POST | `/api/files/delete` | ✅ | 删除文件/目录 |
| POST | `/api/files/batch-delete` | ✅ | 批量删除 |
| POST | `/api/files/mkdir` | ✅ | 新建文件夹 |
| POST | `/api/files/rename` | ✅ | 重命名 |
| GET | `/api/gallery` | ✅ | 获取所有媒体文件 |

### 10.2 认证方式一览

| 接口类型 | 认证方式 |
|----------|----------|
| 普通 API (json) | `Authorization: Bearer <token>` |
| 文件上传 | `Authorization: Bearer <token>` |
| 图片缩略图 URL | query param `?token=<token>` |
| 文件下载 URL | query param `?token=<token>` |

### 10.3 JWT Token 说明

- **加密算法：** HS256
- **有效期：** 30 天
- **Payload 示例：**
  ```json
  {
    "username": "root",
    "exp": 1786400000,
    "iat": 1686400000
  }
  ```
- **密钥：** 服务启动时随机生成，重启后所有旧 token 失效

### 10.4 错误码汇总

| HTTP 状态码 | 业务 code | 含义 |
|-------------|-----------|------|
| 200 | 0 | 操作成功 |
| 200 | 1 | 部分失败（如批量删除） |
| 400 | 1 | 请求参数错误 / 业务逻辑错误 |
| 401 | 1 | token 缺失或无效 |
| 403 | 1 | 禁止公网访问 |
| 404 | - | 资源不存在（直接返回 404，无 JSON body） |

### 10.5 目录结构（服务端）

```
/var/lib/gnas/
├── .thumbs/              # 缩略图缓存目录
│   ├── sl_photo.jpg
│   └── sl_image.png
├── gnas.db               # SQLite 数据库文件
├── gnas.db-wal           # SQLite WAL 日志
├── gnas.db-shm           # SQLite 共享内存
├── photos/               # 用户上传的文件
│   ├── beach.jpg
│   └── vacation.mp4
└── documents/
    └── readme.txt
```