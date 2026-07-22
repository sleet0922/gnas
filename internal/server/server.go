package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
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

// 登录检测
type loginDetect struct {
	failedTimes uint32
	mu          sync.Mutex
}

var ld = &loginDetect{}

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

	ld.mu.Lock()
	if ld.failedTimes >= 5 {
		ld.failedTimes++
		ld.mu.Unlock()
		writeError(w, "登录失败次数过多，请稍后再试")
		return
	}
	ld.mu.Unlock()

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

	// 从 SQLite 验证
	user, err := db.GetUser(data.Username)
	if err != nil || user == nil {
		ld.mu.Lock()
		ld.failedTimes++
		ld.mu.Unlock()
		writeError(w, "用户名或密码错误")
		return
	}

	if checkPassword(user.Password, data.Password) {
		ld.mu.Lock()
		ld.failedTimes = 0
		ld.mu.Unlock()

		token, err := issueToken(user.Username)
		if err != nil {
			writeError(w, "生成 token 失败")
			return
		}

		writeOK(w, map[string]interface{}{
			"success": true,
			"token":   token,
		})
		return
	}

	ld.mu.Lock()
	ld.failedTimes++
	ld.mu.Unlock()
	writeError(w, "用户名或密码错误")
}

// HandleLogout 登出
func HandleLogout(w http.ResponseWriter, r *http.Request) {
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

	if len(data.NewPassword) < 4 {
		writeError(w, "新密码至少 4 个字符")
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
	if err == nil && rows.Next() {
		rows.Scan(&username)
		rows.Close()
	}
	writeOK(w, map[string]interface{}{
		"version":  "1.0.0",
		"username": username,
	})
}
