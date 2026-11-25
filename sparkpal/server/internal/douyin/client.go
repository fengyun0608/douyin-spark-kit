package douyin

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

/* ══════════════════════════════════════════════════════════════
   抖音 IM 客户端

   域名：imapi.douyin.com（不是 www.douyin.com）
   鉴权：只要 Cookie 里的 sessionid 等，不需要 a_bogus 签名
   ══════════════════════════════════════════════════════════════ */

const (
	imHost  = "imapi.douyin.com"
	imScheme = "https"

	// 默认 UA，和抓包一致
	// 注意：这个会影响服务端的风控判断，尽量别乱改
	DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/158.0.0.0 Safari/537.36 Edg/158.0.0.0"

	// f3 字段的固定盐值（抓包逆向得出，改了会被拒 status=7905）
	msgSalt = uint64(7479746146479407679)
)

// Account 一个抖音账号的运行时
type Account struct {
	ID     string // DouyinAccount.id
	UserID string // 星语用户
	UID    string // 我自己的数字 uid（从 conv_id 或 api 得出）
	Cookie string // 完整 cookie 串
	UA     string // User-Agent

	seq    atomic.Uint64 // 请求序号，必须严格递增
	mu     sync.Mutex
	client *http.Client
}

// NewAccount 建一个账号运行时
func NewAccount(id, userID, uid, cookie string) *Account {
	a := &Account{
		ID:     id,
		UserID: userID,
		UID:    uid,
		Cookie: cookie,
		UA:     DefaultUA,
		client: &http.Client{
			Timeout: 25 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	// 从 10000 起步，和浏览器一致
	a.seq.Store(10000)
	return a
}

// NextSeq 取下一个序号
func (a *Account) NextSeq() uint64 {
	return a.seq.Add(1)
}

// Post 向 imapi 发一个 protobuf 请求
func (a *Account) Post(path string, body []byte) ([]byte, error) {
	url := imScheme + "://" + imHost + path

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", a.UA)
	req.Header.Set("Cookie", a.Cookie)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", "https://www.douyin.com")
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(data), 200))
	}
	return data, nil
}

/* ── 响应解析 ── */

// Resp 通用响应头
type Resp struct {
	Cmd        uint64
	Seq        uint64
	StatusCode uint64
	StatusMsg  string // f4，通常是 "OK"
	LogID      string // f7
	NowMs      uint64 // f10
	SelfUID    uint64 // f13
	Biz        []byte // f6，业务数据（命令 + payload）
}

// ParseResp 解析响应外壳
func ParseResp(b []byte) (*Resp, error) {
	fs, err := ParseFields(b)
	if err != nil {
		return nil, err
	}
	r := &Resp{}
	if f := GetField(fs, 1); f != nil && f.Wire == 0 {
		r.Cmd = f.Uint
	}
	if f := GetField(fs, 2); f != nil && f.Wire == 0 {
		r.Seq = f.Uint
	}
	if f := GetField(fs, 3); f != nil && f.Wire == 0 {
		r.StatusCode = f.Uint
	}
	if f := GetField(fs, 4); f != nil && f.Wire == 2 {
		r.StatusMsg = string(f.Bytes)
	}
	if f := GetField(fs, 7); f != nil && f.Wire == 2 {
		r.LogID = string(f.Bytes)
	}
	if f := GetField(fs, 10); f != nil && f.Wire == 0 {
		r.NowMs = f.Uint
	}
	if f := GetField(fs, 13); f != nil && f.Wire == 0 {
		r.SelfUID = f.Uint
	}
	if f := GetField(fs, 6); f != nil && f.Wire == 2 {
		r.Biz = f.Bytes
	}
	return r, nil
}

// BizResult 业务层结果（f6 里再套一层的 JSON）
type BizResult struct {
	StatusCode int    `json:"status_code"`
	Tips       string `json:"tips"`
	RawCheck   int    `json:"raw_check_code"`
}

// ParseBizJSON 从 f6 里找那段 JSON
//
// 结构：f6 → f100 → 里面有 "client_message_id" + "{"status_code":...}"
func ParseBizJSON(biz []byte) (*BizResult, error) {
	if len(biz) == 0 {
		return nil, fmt.Errorf("空的业务数据")
	}
	fs, err := ParseFields(biz)
	if err != nil {
		return nil, err
	}
	// 找所有 bytes 字段里的 JSON
	for _, f := range fs {
		if f.Wire != 2 {
			continue
		}
		s := string(f.Bytes)
		if idx := indexOf(s, `"status_code"`); idx >= 0 {
			// 用宽松解析
			r := &BizResult{}
			r.StatusCode = extractInt(s, `"status_code":`)
			r.Tips = extractStr(s, `"tips":"`)
			r.RawCheck = extractInt(s, `"raw_check_code":`)
			return r, nil
		}
	}
	return nil, fmt.Errorf("业务数据里没有 status_code")
}

/* ── 小工具 ── */

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// extractInt 从 JSON 字符串里抠整数
func extractInt(s, key string) int {
	i := indexOf(s, key)
	if i < 0 {
		return 0
	}
	i += len(key)
	v := 0
	neg := false
	for i < len(s) && (s[i] == ' ' || s[i] == '"') {
		i++
	}
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		v = v*10 + int(s[i]-'0')
		i++
	}
	if neg {
		v = -v
	}
	return v
}

// extractStr 从 JSON 字符串里抠字符串值
func extractStr(s, prefix string) string {
	i := indexOf(s, prefix)
	if i < 0 {
		return ""
	}
	i += len(prefix)
	start := i
	for i < len(s) && s[i] != '"' {
		if s[i] == '\\' {
			i++
		}
		i++
	}
	if i > len(s) {
		return ""
	}
	return s[start:i]
}

// Log 带账号标记的日志
func (a *Account) Log(msg string, kv ...any) {
	args := append([]any{"account", a.ID}, kv...)
	slog.Info("[douyin] "+msg, args...)
}
