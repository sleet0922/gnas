package server

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const systemdUnitPath = "/etc/systemd/system/gnas.service"

// EnsureSystemdService installs a boot-time unit when the binary is launched
// directly on a systemd-based Linux host. Existing units are left untouched.
func EnsureSystemdService() {
	if runtime.GOOS != "linux" {
		return
	}
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		log.Printf("[systemd] 未检测到运行中的 systemd，跳过服务文件安装")
		return
	}
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		log.Printf("[systemd] 未找到 systemctl，跳过服务文件安装")
		return
	}
	if _, err := os.Stat(systemdUnitPath); err == nil {
		return
	} else if !os.IsNotExist(err) {
		log.Printf("[systemd] 检查服务文件失败: %v", err)
		return
	}

	executable, err := os.Executable()
	if err != nil {
		log.Printf("[systemd] 获取程序路径失败: %v", err)
		return
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		log.Printf("[systemd] 解析程序路径失败: %v", err)
		return
	}
	unit := fmt.Sprintf(`[Unit]
Description=GNAS Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s
WorkingDirectory=/var/lib/gnas
User=root
Restart=always
RestartSec=5
KillMode=control-group

[Install]
WantedBy=multi-user.target
`, systemdExecPath(executable))

	if err := os.WriteFile(systemdUnitPath, []byte(unit), 0644); err != nil {
		log.Printf("[systemd] 写入服务文件失败: %v", err)
		return
	}
	if output, err := exec.Command(systemctl, "daemon-reload").CombinedOutput(); err != nil {
		log.Printf("[systemd] daemon-reload 失败: %v, output: %s", err, strings.TrimSpace(string(output)))
		return
	}
	if output, err := exec.Command(systemctl, "enable", "gnas.service").CombinedOutput(); err != nil {
		log.Printf("[systemd] enable 失败: %v, output: %s", err, strings.TrimSpace(string(output)))
		return
	}
	log.Printf("[systemd] 已创建并启用 %s，下次启动将自动运行", systemdUnitPath)
}

func systemdExecPath(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
	}
	return path
}
