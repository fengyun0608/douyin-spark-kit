// Package config 读取运行参数。
//
// 设计要点：官方与自建的差异全部由环境变量决定，
// 客户端不判断「是不是官方」，只读 /api/spark/meta 返回的能力。
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config 是服务端的全部运行参数。
type Config struct {
	Port    string
	DBDSN   string
	Secret  string
	Mode    string // cloud | self
	Official bool
	Billing  bool
	// 邀请码独立于计费：现在免费期也要拦人，所以单独一个开关
	NeedInvite bool
	PairCode string
	Name     string

	// 扫码服务地址（Playwright），留空则关闭扫码绑定
	QRBase string

	// 短信：留空则走开发模式（验证码打到日志）
	SMSWebhook string
	SMSKey     string

	// 额度（cloud 模式下按 plan 覆盖）
	FreeMaxAccounts int
	FreeMaxFriends  int
	FreeMaxDaily    int
}

// Load 从环境变量读取，缺省值面向自建（self）场景。
func Load() Config {
	c := Config{
		Port:            env("PORT", "3210"),
		DBDSN:           env("DB_DSN", ""),
		Secret:          env("JWT_SECRET", ""),
		Mode:            env("MODE", "self"),
		Official:        envBool("OFFICIAL", false),
		Billing:         envBool("BILLING", false),
		PairCode:        env("PAIR_CODE", ""),
		Name:            env("SERVER_NAME", "伙伴服务器"),
		QRBase:          env("QR_BASE", ""),
		SMSWebhook:      env("SMS_WEBHOOK", ""),
		SMSKey:          env("SMS_KEY", ""),
		FreeMaxAccounts: envInt("FREE_MAX_ACCOUNTS", 1),
		FreeMaxFriends:  envInt("FREE_MAX_FRIENDS", 10),
		FreeMaxDaily:    envInt("FREE_MAX_DAILY", 5),
	}

	// 邀请码默认跟计费走，但可以单独打开（免费期也要控节奏）
	c.NeedInvite = envBool("NEED_INVITE", c.Billing)

	// 自建模式默认不需要配对码——否则第一次用就卡住。
	// 但如果是公网部署，建议设置 PAIR_CODE。
	if c.Mode == "cloud" && c.PairCode == "" {
		slog.Warn("[config] cloud 模式未设置 PAIR_CODE，配对接口将关闭")
	}
	if c.Secret == "" {
		slog.Warn("[config] 未设置 JWT_SECRET，临时生成（重启后所有 token 失效）")
		c.Secret = randomSecret()
	}
	return c
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func randomSecret() string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 48)
	seed := os.Getpid()*31 + int(os.Getppid())
	for i := range b {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		b[i] = alphabet[seed%len(alphabet)]
	}
	return string(b)
}
