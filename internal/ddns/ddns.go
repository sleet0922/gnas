package ddns

import (
	"sync"
	"time"

	"github.com/jeessy2/ddns-go/v6/config"
	"github.com/jeessy2/ddns-go/v6/dns"
	"github.com/jeessy2/ddns-go/v6/util"
)

// DDNS 封装 ddns-go 核心逻辑
type DDNS struct {
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	interval time.Duration
}

// New 创建 DDNS 实例
func New(interval time.Duration) *DDNS {
	return &DDNS{
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动 DDNS 定时任务
func (d *DDNS) Start() {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	d.running = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	conf, _ := config.GetConfigCached()
	conf.CompatibleConfig()
	util.InitLogLang(conf.Lang)

	go func() {
		d.RunOnce()
		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.RunOnce()
			case <-d.stopCh:
				return
			}
		}
	}()
}

// Stop 停止 DDNS 定时任务
func (d *DDNS) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.running {
		close(d.stopCh)
		d.running = false
	}
}

// RunOnce 执行一次 DDNS 更新
func (d *DDNS) RunOnce() {
	dns.RunOnce()
}

// IsRunning 是否正在运行
func (d *DDNS) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}

// GetConfig 获取当前配置
func GetConfig() (*config.Config, error) {
	conf, err := config.GetConfigCached()
	return &conf, err
}

// SaveConfig 保存配置
func SaveConfig(conf *config.Config) error {
	return conf.SaveConfig()
}

// GetNetInterfaces 获取网卡信息
func GetNetInterfaces() (ipv4 []config.NetInterface, ipv6 []config.NetInterface, err error) {
	return config.GetNetInterface()
}

// ForceCompare 强制比对
func ForceCompare() {
	util.ForceCompareGlobal = true
}

// DNS 提供商列表
var DNSProviders = []struct {
	Name string
	Label string
}{
	{"alidns", "阿里云"},
	{"aliesa", "阿里云 ESA"},
	{"tencentcloud", "腾讯云"},
	{"dnspod", "Dnspod"},
	{"cloudflare", "Cloudflare"},
	{"huaweicloud", "华为云"},
	{"callback", "Callback"},
	{"baiducloud", "百度云"},
	{"porkbun", "Porkbun"},
	{"godaddy", "GoDaddy"},
	{"namecheap", "Namecheap"},
	{"namesilo", "NameSilo"},
	{"dynadot", "Dynadot"},
	{"vercel", "Vercel"},
	{"dynv6", "Dynv6"},
	{"spaceship", "Spaceship"},
	{"nowcn", "Nowcn"},
	{"eranet", "Eranet"},
	{"tnethk", "Tnethk"},
	{"gcore", "Gcore"},
	{"edgeone", "EdgeOne"},
	{"nsone", "IBM NS1 Connect"},
	{"rainyun", "雨云"},
	{"hipmdnsmgr", "HiPMDnsMgr"},
	{"cloudns", "ClouDNS"},
	{"dnsla", "DNSLA"},
	{"trafficroute", "TrafficRoute"},
	{"name_com", "Name.com"},
}
