package util

import (
	"log"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var logLang = language.English
var logPrinter = message.NewPrinter(logLang)

func init() {

	message.SetString(language.English, "配置文件已保存在: %s", "Config file has been saved to: %s")

	message.SetString(language.English, "你的IP %s 没有变化, 域名 %s", "Your's IP %s has not changed! Domain: %s")
	message.SetString(language.English, "新增域名解析 %s 成功! IP: %s", "Added domain %s successfully! IP: %s")
	message.SetString(language.English, "新增域名解析 %s 失败! 异常信息: %s", "Failed to add domain %s! Result: %s")

	message.SetString(language.English, "更新域名解析 %s 成功! IP: %s", "Updated domain %s successfully! IP: %s")
	message.SetString(language.English, "更新域名解析 %s 失败! 异常信息: %s", "Failed to updated domain %s! Result: %s")

	message.SetString(language.English, "你的IPv4未变化, 未触发 %s 请求", "Your's IPv4 has not changed, %s request has not been triggered")
	message.SetString(language.English, "你的IPv6未变化, 未触发 %s 请求", "Your's IPv6 has not changed, %s request has not been triggered")

	// http_util
	message.SetString(language.English, "异常信息: %s", "Exception: %s")
	message.SetString(language.English, "查询域名信息发生异常! %s", "Failed to query domain info! %s")
	message.SetString(language.English, "返回内容: %s ,返回状态码: %d", "Response body: %s ,Response status code: %d")
	message.SetString(language.English, "通过接口获取IPv4失败! 接口地址: %s", "Failed to get IPv4 from %s")
	message.SetString(language.English, "通过接口获取IPv6失败! 接口地址: %s", "Failed to get IPv6 from %s")
	message.SetString(language.English, "将不会触发Webhook, 仅在第 3 次失败时触发一次Webhook, 当前失败次数：%d", "Webhook will not be triggered, only trigger once when the third failure, current failure times: %d")
	message.SetString(language.English, "在DNS服务商中未找到根域名: %s", "Root domain not found in DNS provider: %s")

	// webhook
	message.SetString(language.English, "Webhook配置中的URL不正确", "Webhook url is incorrect")
	message.SetString(language.English, "Webhook中的 RequestBody JSON 无效", "Webhook RequestBody JSON is invalid")
	message.SetString(language.English, "Webhook调用成功! 返回数据：%s", "Successfully called Webhook! Response body: %s")
	message.SetString(language.English, "Webhook调用失败! 异常信息：%s", "Failed to call Webhook! Exception: %s")
	message.SetString(language.English, "Webhook Header不正确: %s", "Webhook header is invalid: %s")
	message.SetString(language.English, "请输入Webhook的URL", "Please enter the Webhook url")

	// callback
	message.SetString(language.English, "Callback的URL不正确", "Callback url is incorrect")
	message.SetString(language.English, "Callback调用成功, 域名: %s, IP: %s, 返回数据: %s", "Successfully called Callback! Domain: %s, IP: %s, Response body: %s")
	message.SetString(language.English, "Callback调用失败, 异常信息: %s", "Failed to call Callback! Exception: %s")

	// save
	message.SetString(language.English, "密码不安全！尝试使用更复杂的密码", "Password is not secure! Try using a more complex password")
	message.SetString(language.English, "第 %s 个配置未填写域名", "The %s config does not fill in the domain")

	// config
	message.SetString(language.English, "从网卡获得IPv4失败", "Failed to get IPv4 from network card")
	message.SetString(language.English, "从网卡中获得IPv4失败! 网卡名: %s", "Failed to get IPv4 from network card! Network card name: %s")
	message.SetString(language.English, "获取IPv4结果失败! 接口: %s ,返回值: %s", "Failed to get IPv4 result! Interface: %s ,Result: %s")
	message.SetString(language.English, "获取%s结果失败! 未能成功执行命令：%s, 错误：%q, 退出状态码：%s", "Failed to get %s result! Command: %s, Error: %q, Exit status code: %s")
	message.SetString(language.English, "获取%s结果失败! 命令: %s, 标准输出: %q", "Failed to get %s result! Command: %s, Stdout: %q")
	message.SetString(language.English, "从网卡获得IPv6失败", "Failed to get IPv6 from network card")
	message.SetString(language.English, "从网卡中获得IPv6失败! 网卡名: %s", "Failed to get IPv6 from network card! Network card name: %s")
	message.SetString(language.English, "获取IPv6结果失败! 接口: %s ,返回值: %s", "Failed to get IPv6 result! Interface: %s ,Result: %s")
	message.SetString(language.English, "未找到第 %d 个IPv6地址! 将使用第一个IPv6地址", "%dth IPv6 address not found! Will use the first IPv6 address")
	message.SetString(language.English, "IPv6匹配表达式 %s 不正确! 最小从1开始", "IPv6 match expression %s is incorrect! Minimum start from 1")
	message.SetString(language.English, "IPv6将使用正则表达式 %s 进行匹配", "IPv6 will use regular expression %s for matching")
	message.SetString(language.English, "匹配成功! 匹配到地址: %s", "Match successfully! Matched address: %s")
	message.SetString(language.English, "没有匹配到任何一个IPv6地址, 将使用第一个地址", "No IPv6 address matched, will use the first address")
	message.SetString(language.English, "未能获取IPv4地址, 将不会更新", "Failed to get IPv4 address, will not update")
	message.SetString(language.English, "未能获取IPv6地址, 将不会更新", "Failed to get IPv6 address, will not update")

	// domains
	message.SetString(language.English, "域名: %s 不正确", "The domain %s is incorrect")
	message.SetString(language.English, "域名: %s 解析失败", "The domain %s resolution failed")
	message.SetString(language.English, "域名 %s 解析未找到，且因添加了参数 %s=%s 导致无法创建。本次更新已被忽略", "DNS resolution for domain %s was not found, and the creation failed due to the added parameter %s=%s. This update has been ignored.")
	message.SetString(language.English, "IPv6未改变, 将等待 %d 次后与DNS服务商进行比对", "IPv6 has not changed, will wait %d times to compare with DNS provider")
	message.SetString(language.English, "IPv4未改变, 将等待 %d 次后与DNS服务商进行比对", "IPv4 has not changed, will wait %d times to compare with DNS provider")

	message.SetString(language.English, "本机DNS异常! 将默认使用 %s, 可参考文档通过 -dns 自定义 DNS 服务器", "Local DNS exception! Will use %s by default, you can use -dns to customize DNS server")
	message.SetString(language.English, "等待网络连接: %s", "Waiting for network connection: %s")
	message.SetString(language.English, "%s 后重试...", "Retry after %s")
	message.SetString(language.English, "网络已连接", "The network is connected")

	// webhook通知
	message.SetString(language.English, "未改变", "no changed")
	message.SetString(language.English, "失败", "failed")
	message.SetString(language.English, "成功", "success")

	// Login
	message.SetString(language.English, "用户名或密码错误", "Username or password is incorrect")
	message.SetString(language.English, "登录失败次数过多，请稍后再试", "Too many login failures, please try again later")
	message.SetString(language.English, "用户名 %s 的密码已重置成功! 请重启ddns-go", "The password of username %s has been reset successfully! Please restart ddns-go")
	message.SetString(language.English, "配置文件 %s 不存在, 可通过-c指定配置文件", "Config file %s does not exist, you can specify the configuration file through -c")

	// DNS 提供商相关日志
	message.SetString(language.English, "查询域名 %s 信息发生异常! %v", "Failed to query domain %s info! %v")
	message.SetString(language.English, "在DNS服务商中未找到域名: %s", "Domain not found in DNS provider: %s")
	message.SetString(language.English, "IP %s 没有变化，域名 %s", "IP %s has not changed, domain %s")

}

func Log(key string, args ...interface{}) {
	log.Println(LogStr(key, args...))
}

func LogStr(key string, args ...interface{}) string {
	return logPrinter.Sprintf(key, args...)
}

func InitLogLang(lang string) string {
	newLang := language.English
	if strings.HasPrefix(lang, "zh") {
		newLang = language.Chinese
	}
	if newLang != logLang {
		logLang = newLang
		logPrinter = message.NewPrinter(logLang)
	}
	return logLang.String()
}
