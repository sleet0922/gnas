package server

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jeessy2/gnas/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// MemoryLogs 内存日志
type MemoryLogs struct {
	MaxNum int
	Logs   []string
	mu     sync.Mutex
}

func (m *MemoryLogs) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Logs = append(m.Logs, string(p))
	if len(m.Logs) > m.MaxNum {
		m.Logs = m.Logs[len(m.Logs)-m.MaxNum:]
	}
	return len(p), nil
}

var mlogs = &MemoryLogs{MaxNum: 200}

func init() {
	log.SetOutput(io.MultiWriter(mlogs, os.Stdout))
}

// GetLogs 获取日志
func GetLogs() []string {
	mlogs.mu.Lock()
	defer mlogs.mu.Unlock()
	result := make([]string, len(mlogs.Logs))
	copy(result, mlogs.Logs)
	return result
}

// ClearLogs 清除日志
func ClearLogs() {
	mlogs.mu.Lock()
	defer mlogs.mu.Unlock()
	mlogs.Logs = mlogs.Logs[:0]
}

var startTime = time.Now()

// Version 版本号，由 main.go 注入
var Version = "DEV"

// 登录限制器：按 IP+用户名+时间窗口
type loginAttempt struct {
	failures    int
	lockedUntil time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]*loginAttempt // key: IP+username
}

var ld = &loginLimiter{attempts: make(map[string]*loginAttempt)}

// getClientIP 从请求中提取客户端 IP
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// JSON 响应辅助
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"code": 0,
		"data": data,
	})
}

func writeError(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]interface{}{
		"code":    1,
		"message": msg,
	})
}

func writeErrorStatus(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{
		"code":    1,
		"message": msg,
	})
}

// hashPassword 使用 bcrypt 哈希密码
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// checkPassword 校验密码
func checkPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

func ensureDefaultUser() {
	hasUser, err := db.HasAnyUser()
	if err != nil || hasUser {
		return
	}
	hashedPwd, err := hashPassword("root")
	if err != nil {
		return
	}
	db.CreateUser("root", hashedPwd)
}

// HandleLogin 登录
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	ensureDefaultUser()

	if r.Method == http.MethodGet {
		writeOK(w, map[string]interface{}{
			"needSetup": false,
		})
		return
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}

	if data.Username == "" || data.Password == "" {
		writeError(w, "必须输入用户名和密码")
		return
	}

	// 按 IP+用户名 检查登录锁定状态
	limiterKey := getClientIP(r) + ":" + data.Username

	ld.mu.Lock()
	attempt := ld.attempts[limiterKey]
	if attempt == nil {
		attempt = &loginAttempt{}
		ld.attempts[limiterKey] = attempt
	}
	if !attempt.lockedUntil.IsZero() && time.Now().Before(attempt.lockedUntil) {
		ld.mu.Unlock()
		writeError(w, "登录失败次数过多，请 15 分钟后再试")
		return
	}
	// 锁定过期后自动重置计数
	if !attempt.lockedUntil.IsZero() && !time.Now().Before(attempt.lockedUntil) {
		attempt.failures = 0
		attempt.lockedUntil = time.Time{}
	}
	ld.mu.Unlock()

	// 从 SQLite 验证
	user, err := db.GetUser(data.Username)
	if err != nil || user == nil {
		ld.mu.Lock()
		attempt.failures++
		if attempt.failures >= 5 {
			attempt.lockedUntil = time.Now().Add(15 * time.Minute)
		}
		ld.mu.Unlock()
		writeError(w, "用户名或密码错误")
		return
	}

	if checkPassword(user.Password, data.Password) {
		ld.mu.Lock()
		attempt.failures = 0
		attempt.lockedUntil = time.Time{}
		ld.mu.Unlock()

		token, err := issueToken(user.Username)
		if err != nil {
			writeError(w, "生成 token 失败")
			return
		}

		// 检测是否使用默认密码 (root/root)
		mustChange := data.Username == "root" && data.Password == "root"
		writeOK(w, map[string]interface{}{
			"success":              true,
			"token":                token,
			"must_change_password": mustChange,
		})
		return
	}

	ld.mu.Lock()
	attempt.failures++
	if attempt.failures >= 5 {
		attempt.lockedUntil = time.Now().Add(15 * time.Minute)
	}
	ld.mu.Unlock()
	writeError(w, "用户名或密码错误")
}

// HandleLogout 登出
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	if token := tokenFromRequest(r); token != "" {
		RevokeToken(token)
	}
	writeOK(w, nil)
}

// HandleChangePassword 修改密码
func HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	var data struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "请求格式错误")
		return
	}

	if data.OldPassword == "" || data.NewPassword == "" {
		writeError(w, "必须输入旧密码和新密码")
		return
	}

	if len(data.NewPassword) < 8 {
		writeError(w, "新密码至少 8 个字符")
		return
	}

	username := CurrentUsername(r)
	if username == "" {
		writeErrorStatus(w, http.StatusUnauthorized, "token 缺失或无效")
		return
	}

	// 验证旧密码
	user, err := db.GetUser(username)
	if err != nil || user == nil {
		writeError(w, "用户不存在")
		return
	}

	if !checkPassword(user.Password, data.OldPassword) {
		writeError(w, "旧密码错误")
		return
	}

	hashedPwd, err := hashPassword(data.NewPassword)
	if err != nil {
		writeError(w, "密码加密失败")
		return
	}

	if err := db.UpdateUserPassword(username, hashedPwd); err != nil {
		writeError(w, "修改密码失败")
		return
	}

	writeOK(w, nil)
}

// HandleGetLogs 获取日志
func HandleGetLogs(w http.ResponseWriter, r *http.Request) {
	writeOK(w, GetLogs())
}

// HandleClearLogs 清除日志
func HandleClearLogs(w http.ResponseWriter, r *http.Request) {
	ClearLogs()
	writeOK(w, nil)
}

// HandleStatus 获取系统状态
func HandleStatus(w http.ResponseWriter, r *http.Request) {
	var username string
	rows, err := db.GetDB().Query("SELECT username FROM users LIMIT 1")
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			rows.Scan(&username)
		}
	}
	writeOK(w, map[string]interface{}{
		"version":  Version,
		"username": username,
	})
}
