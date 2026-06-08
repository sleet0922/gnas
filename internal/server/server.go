package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/util"
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

// cookie 管理
const cookieName = "nas_token"

var cookieInSystem = &http.Cookie{}
var startTime = time.Now()

// token 生成
func generateToken(username string) string {
	key := []byte(fmt.Sprintf("gnas-%d", time.Now().UnixNano()))
	h := hmac.New(sha256.New, key)
	h.Write([]byte(fmt.Sprintf("%s%d", username, time.Now().Unix())))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

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

// AuthMiddleware 认证中间件
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookieInWeb, err := r.Cookie(cookieName)
		if err != nil {
			writeErrorStatus(w, http.StatusUnauthorized, "未登录")
			return
		}

		conf, _ := config.GetConfigCached()
		if conf.NotAllowWanAccess {
			if !util.IsPrivateNetwork(r.RemoteAddr) {
				writeErrorStatus(w, http.StatusForbidden, "禁止公网访问")
				return
			}
		}

		if cookieInSystem.Value != "" &&
			cookieInSystem.Value == cookieInWeb.Value &&
			cookieInSystem.Expires.After(time.Now()) {
			next(w, r)
			return
		}

		writeErrorStatus(w, http.StatusUnauthorized, "登录已过期")
	}
}

// HandleLogin 登录
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 返回登录状态
		conf, _ := config.GetConfigCached()
		writeOK(w, map[string]interface{}{
			"needSetup": conf.Username == "" && conf.Password == "",
		})
		return
	}

	conf, _ := config.GetConfigCached()
	util.InitLogLang(conf.Lang)

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

	// 首次设置
	if conf.Username == "" && conf.Password == "" {
		if time.Since(startTime) > 30*time.Minute {
			writeError(w, "初始设置超时，请重启服务")
			return
		}
		conf.NotAllowWanAccess = true
		conf.Username = data.Username
		hashedPwd, err := conf.CheckPassword(data.Password)
		if err != nil {
			writeError(w, err.Error())
			return
		}
		conf.Password = hashedPwd
		conf.SaveConfig()
	}

	// 验证登录
	if data.Username == conf.Username && util.PasswordOK(conf.Password, data.Password) {
		ld.mu.Lock()
		ld.failedTimes = 0
		ld.mu.Unlock()

		timeoutDays := 1
		if conf.NotAllowWanAccess {
			timeoutDays = 30
		}

		cookieInSystem = &http.Cookie{
			Name:     cookieName,
			Value:    generateToken(data.Username),
			Path:     "/",
			Expires:  time.Now().AddDate(0, 0, timeoutDays),
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		}
		http.SetCookie(w, cookieInSystem)
		writeOK(w, map[string]interface{}{
			"token": cookieInSystem.Value,
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
	cookieInSystem = &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookieInSystem)
	writeOK(w, nil)
}

// HandleGetConfig 获取 DDNS 配置
func HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	conf, err := config.GetConfigCached()
	if err != nil {
		conf.NotAllowWanAccess = true
	}

	ipv4, ipv6, _ := config.GetNetInterface()

	// 隐藏敏感信息
	type dnsConfResp struct {
		Name             string   `json:"name"`
		DnsName          string   `json:"dnsName"`
		DnsID            string   `json:"dnsId"`
		DnsSecret        string   `json:"dnsSecret"`
		DnsExtParam      string   `json:"dnsExtParam"`
		TTL              string   `json:"ttl"`
		Ipv4Enable       bool     `json:"ipv4Enable"`
		Ipv4GetType      string   `json:"ipv4GetType"`
		Ipv4Url          string   `json:"ipv4Url"`
		Ipv4NetInterface string   `json:"ipv4NetInterface"`
		Ipv4Cmd          string   `json:"ipv4Cmd"`
		Ipv4Domains      string   `json:"ipv4Domains"`
		Ipv6Enable       bool     `json:"ipv6Enable"`
		Ipv6GetType      string   `json:"ipv6GetType"`
		Ipv6Url          string   `json:"ipv6Url"`
		Ipv6NetInterface string   `json:"ipv6NetInterface"`
		Ipv6Cmd          string   `json:"ipv6Cmd"`
		Ipv6Reg          string   `json:"ipv6Reg"`
		Ipv6Domains      string   `json:"ipv6Domains"`
		HttpInterface    string   `json:"httpInterface"`
	}

	var dnsConfArray []dnsConfResp
	for _, dc := range conf.DnsConf {
		idHide, secretHide := hideIDSecret(&dc)
		dnsConfArray = append(dnsConfArray, dnsConfResp{
			Name:             dc.Name,
			DnsName:          dc.DNS.Name,
			DnsID:            idHide,
			DnsSecret:        secretHide,
			DnsExtParam:      dc.DNS.ExtParam,
			TTL:              dc.TTL,
			Ipv4Enable:       dc.Ipv4.Enable,
			Ipv4GetType:      dc.Ipv4.GetType,
			Ipv4Url:          dc.Ipv4.URL,
			Ipv4NetInterface: dc.Ipv4.NetInterface,
			Ipv4Cmd:          dc.Ipv4.Cmd,
			Ipv4Domains:      strings.Join(dc.Ipv4.Domains, "\n"),
			Ipv6Enable:       dc.Ipv6.Enable,
			Ipv6GetType:      dc.Ipv6.GetType,
			Ipv6Url:          dc.Ipv6.URL,
			Ipv6NetInterface: dc.Ipv6.NetInterface,
			Ipv6Cmd:          dc.Ipv6.Cmd,
			Ipv6Reg:          dc.Ipv6.Ipv6Reg,
			Ipv6Domains:      strings.Join(dc.Ipv6.Domains, "\n"),
			HttpInterface:    dc.HttpInterface,
		})
	}

	type netIface struct {
		Name    string   `json:"name"`
		Address []string `json:"address"`
	}

	var ipv4Ifaces, ipv6Ifaces []netIface
	for _, iface := range ipv4 {
		ipv4Ifaces = append(ipv4Ifaces, netIface{Name: iface.Name, Address: iface.Address})
	}
	for _, iface := range ipv6 {
		ipv6Ifaces = append(ipv6Ifaces, netIface{Name: iface.Name, Address: iface.Address})
	}

	writeOK(w, map[string]interface{}{
		"dnsConf":           dnsConfArray,
		"notAllowWanAccess": conf.NotAllowWanAccess,
		"username":          conf.Username,
		"webhookUrl":        conf.WebhookURL,
		"webhookRequestBody": conf.WebhookRequestBody,
		"webhookHeaders":    conf.WebhookHeaders,
		"ipv4Interfaces":    ipv4Ifaces,
		"ipv6Interfaces":    ipv6Ifaces,
	})
}

const displayCount = 3

func hideIDSecret(conf *config.DnsConfig) (idHide, secretHide string) {
	if len(conf.DNS.ID) > displayCount && conf.DNS.Name != "callback" {
		idHide = conf.DNS.ID[:displayCount] + strings.Repeat("*", len(conf.DNS.ID)-displayCount)
	} else {
		idHide = conf.DNS.ID
	}
	if len(conf.DNS.Secret) > displayCount && conf.DNS.Name != "callback" {
		secretHide = conf.DNS.Secret[:displayCount] + strings.Repeat("*", len(conf.DNS.Secret)-displayCount)
	} else {
		secretHide = conf.DNS.Secret
	}
	return
}

// HandleSaveConfig 保存 DDNS 配置
func HandleSaveConfig(w http.ResponseWriter, r *http.Request) {
	conf, _ := config.GetConfigCached()

	var data struct {
		Username           string `json:"username"`
		Password           string `json:"password"`
		NotAllowWanAccess  bool   `json:"notAllowWanAccess"`
		WebhookURL         string `json:"webhookUrl"`
		WebhookRequestBody string `json:"webhookRequestBody"`
		WebhookHeaders     string `json:"webhookHeaders"`
		DnsConf            []struct {
			Name             string `json:"name"`
			DnsName          string `json:"dnsName"`
			DnsID            string `json:"dnsId"`
			DnsSecret        string `json:"dnsSecret"`
			DnsExtParam      string `json:"dnsExtParam"`
			TTL              string `json:"ttl"`
			Ipv4Enable       bool   `json:"ipv4Enable"`
			Ipv4GetType      string `json:"ipv4GetType"`
			Ipv4Url          string `json:"ipv4Url"`
			Ipv4NetInterface string `json:"ipv4NetInterface"`
			Ipv4Cmd          string `json:"ipv4Cmd"`
			Ipv4Domains      string `json:"ipv4Domains"`
			Ipv6Enable       bool   `json:"ipv6Enable"`
			Ipv6GetType      string `json:"ipv6GetType"`
			Ipv6Url          string `json:"ipv6Url"`
			Ipv6NetInterface string `json:"ipv6NetInterface"`
			Ipv6Cmd          string `json:"ipv6Cmd"`
			Ipv6Reg          string `json:"ipv6Reg"`
			Ipv6Domains      string `json:"ipv6Domains"`
			HttpInterface    string `json:"httpInterface"`
		} `json:"dnsConf"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "数据解析失败")
		return
	}

	conf.Username = strings.TrimSpace(data.Username)
	conf.NotAllowWanAccess = data.NotAllowWanAccess
	conf.WebhookURL = strings.TrimSpace(data.WebhookURL)
	conf.WebhookRequestBody = strings.TrimSpace(data.WebhookRequestBody)
	conf.WebhookHeaders = strings.TrimSpace(data.WebhookHeaders)

	if data.Password != "" {
		hashedPwd, err := conf.CheckPassword(data.Password)
		if err != nil {
			writeError(w, err.Error())
			return
		}
		conf.Password = hashedPwd
	}

	if conf.Username == "" || conf.Password == "" {
		writeError(w, "必须输入用户名和密码")
		return
	}

	var dnsConfArray []config.DnsConfig
	for k, v := range data.DnsConf {
		dnsConf := config.DnsConfig{
			Name: v.Name,
			TTL:  v.TTL,
		}
		dnsConf.DNS.Name = v.DnsName
		dnsConf.DNS.ID = strings.TrimSpace(v.DnsID)
		dnsConf.DNS.Secret = strings.TrimSpace(v.DnsSecret)
		dnsConf.DNS.ExtParam = strings.TrimSpace(v.DnsExtParam)

		if v.Ipv4Domains == "" && v.Ipv6Domains == "" {
			util.Log("第 %d 个配置未填写域名", k+1)
		}

		dnsConf.Ipv4.Enable = v.Ipv4Enable
		dnsConf.Ipv4.GetType = v.Ipv4GetType
		dnsConf.Ipv4.URL = strings.TrimSpace(v.Ipv4Url)
		dnsConf.Ipv4.NetInterface = v.Ipv4NetInterface
		dnsConf.Ipv4.Cmd = strings.TrimSpace(v.Ipv4Cmd)
		dnsConf.Ipv4.Domains = util.SplitLines(v.Ipv4Domains)

		dnsConf.Ipv6.Enable = v.Ipv6Enable
		dnsConf.Ipv6.GetType = v.Ipv6GetType
		dnsConf.Ipv6.URL = strings.TrimSpace(v.Ipv6Url)
		dnsConf.Ipv6.NetInterface = v.Ipv6NetInterface
		dnsConf.Ipv6.Cmd = strings.TrimSpace(v.Ipv6Cmd)
		dnsConf.Ipv6.Ipv6Reg = strings.TrimSpace(v.Ipv6Reg)
		dnsConf.Ipv6.Domains = util.SplitLines(v.Ipv6Domains)
		dnsConf.HttpInterface = strings.TrimSpace(v.HttpInterface)

		// 保留未修改的 ID/Secret
		if k < len(conf.DnsConf) {
			c := &conf.DnsConf[k]
			idHide, secretHide := hideIDSecret(c)
			if dnsConf.DNS.ID == idHide {
				dnsConf.DNS.ID = c.DNS.ID
			}
			if dnsConf.DNS.Secret == secretHide {
				dnsConf.DNS.Secret = c.DNS.Secret
			}
		}

		dnsConfArray = append(dnsConfArray, dnsConf)
	}
	conf.DnsConf = dnsConfArray

	if err := conf.SaveConfig(); err != nil {
		writeError(w, err.Error())
		return
	}

	util.ForceCompareGlobal = true
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

// HandleWebhookTest 测试 Webhook
func HandleWebhookTest(w http.ResponseWriter, r *http.Request) {
	var data struct {
		URL         string `json:"url"`
		RequestBody string `json:"requestBody"`
		Headers     string `json:"headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, "数据解析失败")
		return
	}

	if data.URL == "" {
		writeError(w, "请输入 Webhook URL")
		return
	}

	var domains = make([]*config.Domain, 1)
	domains[0] = &config.Domain{DomainName: "example.com", SubDomain: "test"}
	domains[0].UpdateStatus = config.UpdatedSuccess

	fakeDomains := &config.Domains{
		Ipv4Addr:    "127.0.0.1",
		Ipv4Domains: domains,
		Ipv6Addr:    "::1",
		Ipv6Domains: domains,
	}
	fakeConfig := &config.Config{
		Webhook: config.Webhook{
			WebhookURL:         data.URL,
			WebhookRequestBody: data.RequestBody,
			WebhookHeaders:     data.Headers,
		},
	}

	config.ExecWebhook(fakeDomains, fakeConfig)
	writeOK(w, nil)
}

// HandleStatus 获取系统状态
func HandleStatus(w http.ResponseWriter, r *http.Request) {
	conf, _ := config.GetConfigCached()
	writeOK(w, map[string]interface{}{
		"version":   "1.0.0",
		"username":  conf.Username,
	})
}
