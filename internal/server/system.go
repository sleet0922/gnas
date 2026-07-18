package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jeessy2/gnas/internal/db"
)

// SystemInfo 系统信息
type SystemInfo struct {
	AI           AIStatus `json:"ai"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	CPUCores     int      `json:"cpuCores"`
	MemoryTotal  uint64   `json:"memoryTotal"`
	MemoryUsed   uint64   `json:"memoryUsed"`
	MemoryFree   uint64   `json:"memoryFree"`
	DiskTotal    uint64   `json:"diskTotal"`
	DiskUsed     uint64   `json:"diskUsed"`
	DiskFree     uint64   `json:"diskFree"`
	Uptime       float64  `json:"uptime"`       // 秒
	ProcMem      uint64   `json:"procMem"`      // 进程内存（Alloc）
	ProcMemSys   uint64   `json:"procMemSys"`   // 进程从系统获取的内存
	CPUUsage     float64  `json:"cpuUsage"`     // 系统CPU使用率 0-100
	ProcCPU      float64  `json:"procCPU"`      // 进程CPU使用率 0-100
	DBSize       int64    `json:"dbSize"`       // 数据库文件大小
	DBSizeString string   `json:"dbSizeString"` // 数据库文件大小（格式化）
}

type AIStatus struct {
	Enabled bool            `json:"enabled"`
	Model   AIServiceStatus `json:"model"`
	Qdrant  AIServiceStatus `json:"qdrant"`
}

type AIServiceStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Version string `json:"version,omitempty"`
	Device  string `json:"device,omitempty"`
}

var cpuStatsLock sync.Mutex
var lastCPUTime cpuTime
var lastProcCPUTime procCPUTime
var lastCPUReadTime time.Time

type cpuTime struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

type procCPUTime struct {
	utime     uint64
	stime     uint64
	cutime    uint64
	cstime    uint64
	startTime uint64
}

func readCPUTime() (cpuTime, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuTime{}, err
	}
	var ct cpuTime
	_, err = fmt.Sscanf(string(data), "cpu %d %d %d %d %d %d %d %d",
		&ct.user, &ct.nice, &ct.system, &ct.idle, &ct.iowait, &ct.irq, &ct.softirq, &ct.steal)
	return ct, err
}

func readProcCPUTime() (procCPUTime, error) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return procCPUTime{}, err
	}
	var pid int
	var comm string
	var state rune
	var ppid, pgrp, session, tty_nr, tpgid int
	var flags uint
	var minflt, cminflt, majflt, cmajflt uint64
	var utime, stime, cutime, cstime uint64
	var priority, nice int
	var numThreads int
	var itrealvalue int64
	var starttime uint64
	_, err = fmt.Sscanf(string(data),
		"%d %s %c %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d",
		&pid, &comm, &state, &ppid, &pgrp, &session, &tty_nr, &tpgid, &flags,
		&minflt, &cminflt, &majflt, &cmajflt,
		&utime, &stime, &cutime, &cstime,
		&priority, &nice, &numThreads, &itrealvalue, &starttime)
	return procCPUTime{utime: utime, stime: stime, cutime: cutime, cstime: cstime, startTime: starttime}, err
}

func getClockTick() uint64 {
	return 100 // Linux 默认 100Hz
}

// HandleSystemInfo 获取系统信息
func HandleSystemInfo(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	info := SystemInfo{
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		CPUCores:    runtime.NumCPU(),
		MemoryTotal: getTotalMemory(),
		MemoryUsed:  getUsedMemory(),
		MemoryFree:  getFreeMemory(),
		Uptime:      time.Since(startTime).Seconds(),
		ProcMem:     m.Alloc,
		ProcMemSys:  m.Sys,
	}
	aiEnabled, settingErr := db.GetSetting("ai_enabled")
	info.AI = probeAIStatus(settingErr == nil && aiEnabled == "true")

	// 磁盘信息
	s := &syscallStat{}
	if err := s.get(dataDir); err == nil {
		info.DiskTotal = s.total
		info.DiskUsed = s.used
		info.DiskFree = s.free
	}

	// 数据库文件大小
	dbPath := filepath.Join(dataDir, "gnas.db")
	if fi, err := os.Stat(dbPath); err == nil {
		info.DBSize = fi.Size()
		info.DBSizeString = formatBytes(uint64(fi.Size()))
	}

	// CPU 使用率
	cpuStatsLock.Lock()
	defer cpuStatsLock.Unlock()

	now := time.Now()
	currentCPUTime, err1 := readCPUTime()
	currentProcCPUTime, err2 := readProcCPUTime()

	if err1 == nil && err2 == nil && !lastCPUReadTime.IsZero() {
		// 系统 CPU 使用率
		totalDiff := (currentCPUTime.user - lastCPUTime.user) +
			(currentCPUTime.nice - lastCPUTime.nice) +
			(currentCPUTime.system - lastCPUTime.system) +
			(currentCPUTime.idle - lastCPUTime.idle) +
			(currentCPUTime.iowait - lastCPUTime.iowait) +
			(currentCPUTime.irq - lastCPUTime.irq) +
			(currentCPUTime.softirq - lastCPUTime.softirq) +
			(currentCPUTime.steal - lastCPUTime.steal)
		busyDiff := totalDiff - (currentCPUTime.idle - lastCPUTime.idle) - (currentCPUTime.iowait - lastCPUTime.iowait)
		if totalDiff > 0 {
			info.CPUUsage = float64(busyDiff) / float64(totalDiff) * 100
		}

		// 进程 CPU 使用率
		procDiff := (currentProcCPUTime.utime - lastProcCPUTime.utime) +
			(currentProcCPUTime.stime - lastProcCPUTime.stime)
		elapsedSec := now.Sub(lastCPUReadTime).Seconds()
		if elapsedSec > 0 {
			info.ProcCPU = float64(procDiff) / (elapsedSec * float64(getClockTick())) * 100
			if info.ProcCPU < 0 {
				info.ProcCPU = 0
			}
			if info.ProcCPU > 100*float64(runtime.NumCPU()) {
				info.ProcCPU = 100 * float64(runtime.NumCPU())
			}
		}
	}

	lastCPUTime = currentCPUTime
	lastProcCPUTime = currentProcCPUTime
	lastCPUReadTime = now

	writeOK(w, info)
}

func probeAIStatus(enabled bool) AIStatus {
	status := AIStatus{Enabled: enabled}
	if !enabled {
		status.Model.Status = "disabled"
		status.Model.Message = "AI 未启用"
		status.Qdrant.Status = "disabled"
		status.Qdrant.Message = "AI 未启用"
		return status
	}

	client := &http.Client{Timeout: 2 * time.Second}
	status.Model = probeModelStatus(client)
	status.Qdrant = probeQdrantStatus(client)
	return status
}

func probeModelStatus(client *http.Client) AIServiceStatus {
	result := AIServiceStatus{Status: "unavailable", Message: "模型服务不可用"}
	resp, err := client.Get("http://127.0.0.1:8000/health")
	if err != nil {
		if embedServerRunning() {
			result.Status = "loading"
			result.Message = "模型正在加载"
		}
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		result.Status = "loading"
		result.Message = "模型正在加载"
		return result
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Message = fmt.Sprintf("模型服务返回 HTTP %d", resp.StatusCode)
		return result
	}
	var payload struct {
		Status string `json:"status"`
		Device string `json:"device"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		result.Message = "模型健康检查响应无效"
		return result
	}
	result.Status = "ready"
	result.Message = "模型已加载"
	result.Device = payload.Device
	return result
}

func probeQdrantStatus(client *http.Client) AIServiceStatus {
	result := AIServiceStatus{Status: "unavailable", Message: "Qdrant 不可用"}
	resp, err := client.Get("http://127.0.0.1:6333/")
	if err != nil {
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Message = fmt.Sprintf("Qdrant 返回 HTTP %d", resp.StatusCode)
		return result
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		result.Message = "Qdrant 健康检查响应无效"
		return result
	}
	result.Status = "ready"
	result.Message = "Qdrant 已就绪"
	result.Version = payload.Version
	return result
}

func embedServerRunning() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var pid int
		if _, err := fmt.Sscan(entry.Name(), &pid); err != nil {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err == nil && containsEmbedServer(string(cmdline)) {
			return true
		}
	}
	return false
}

func containsEmbedServer(cmdline string) bool {
	for _, part := range strings.Split(cmdline, "\x00") {
		if filepath.Base(part) == "embed_server.py" {
			return true
		}
	}
	return false
}

func formatBytes(bytes uint64) string {
	if bytes == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	size := float64(bytes)
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}

// HandleGetSettings 获取系统设置
func HandleGetSettings(w http.ResponseWriter, r *http.Request) {
	aiEnabled, err := db.GetSetting("ai_enabled")
	if err != nil {
		writeError(w, "获取配置失败")
		return
	}

	enabled := false
	if aiEnabled == "true" {
		enabled = true
	}

	writeOK(w, map[string]interface{}{
		"ai_enabled": enabled,
	})
}

// HandleUpdateSettings 更新系统设置
func HandleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		AIEnabled bool `json:"ai_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, "解析请求参数失败")
		return
	}

	val := "false"
	if payload.AIEnabled {
		val = "true"
	}

	// 保存设置
	if err := db.SetSetting("ai_enabled", val); err != nil {
		writeError(w, "保存配置失败")
		return
	}

	// 动态启动/关闭 AI 后台服务
	if payload.AIEnabled {
		log.Println("[设置] AI 功能被启用，触发依赖检测与服务启动...")
		CheckAndInstallAI()
	} else {
		log.Println("[设置] AI 功能被禁用，正在停止后台模型服务以释放内存...")
		go func() {
			// 关闭 Python 向量服务器
			exec.Command("pkill", "-f", "embed_server.py").Run()
			// 关闭 Qdrant
			exec.Command("pkill", "qdrant").Run()
		}()
	}

	writeOK(w, nil)
}
