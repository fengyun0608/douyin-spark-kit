package im

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

/* ══════════════════════════════════════════════════════════════
   抖音私信客户端

   只用 cookie，不需要 a_bogus，不需要 WebView。

   最容易踩的三个坑：
     ① salt 不是时间戳，改了会 7905
     ② 单向关注必须用该会话自己的 salt，否则 7100
     ③ 序号必须严格递增，重复会 req.body is nil
   ══════════════════════════════════════════════════════════════ */

const (
	imHost = "imapi.douyin.com"

	// 互关好友可用的账号级 salt。
	// 单向关注 + 有聊天记录的会话，必须换成该会话自己的 salt。
	AccountSalt uint64 = 7479746146479407679

	// 发送命令号
	cmdSend = 100
)

// DefaultUA 用桌面浏览器 UA，移动 UA 容易触发风控
const DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/158.0.0.0 Safari/537.36 Edg/158.0.0.0"

// Account 一个已登录的抖音账号
type Account struct {
	UserID string // 自己的数字 uid
	SecUID string // 自己的 sec_uid
	Cookie string
	UA     string

	seq atomic.Uint64
	cli *http.Client
}

// NewAccount 创建账号对象。seq 从 10000 开始，保证只增不减。
func NewAccount(userID, secUID, cookie string) *Account {
	a := &Account{
		UserID: userID,
		SecUID: secUID,
		Cookie: cookie,
		UA:     DefaultUA,
		cli:    &http.Client{Timeout: 20 * time.Second},
	}
	a.seq.Store(10000)
	return a
}

// NextSeq 取下一个序号
func (a *Account) NextSeq() uint64 { return a.seq.Add(1) }

// Post 发一个 protobuf 请求
func (a *Account) Post(path string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, "https://"+imHost+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	ua := a.UA
	if ua == "" {
		ua = DefaultUA
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Cookie", a.Cookie)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Referer", "https://www.douyin.com/")

	resp, err := a.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return raw, nil
}

// Resp 解析后的响应
type Resp struct {
	Cmd        int
	Seq        uint64
	StatusCode int
	StatusMsg  string
	LogID      uint64
	Biz        []byte // field 6，业务数据
}

// ParseResp 解析 imapi 响应
func ParseResp(raw []byte) (*Resp, error) {
	fs, err := ParseFields(raw)
	if err != nil {
		return nil, err
	}
	r := &Resp{}
	if f := GetField(fs, 1); f != nil {
		r.Cmd = int(f.Uint)
	}
	if f := GetField(fs, 2); f != nil {
		r.Seq = f.Uint
	}
	if f := GetField(fs, 3); f != nil {
		r.StatusCode = int(int64(f.Uint))
	}
	if f := GetField(fs, 4); f != nil {
		r.StatusMsg = string(f.Data)
	}
	if f := GetField(fs, 6); f != nil {
		r.Biz = f.Data
	}
	return r, nil
}

/* ══════════════ 发送 ══════════════ */

// SendResult 发送结果
type SendResult struct {
	OK        bool
	Code      int
	Msg       string
	LogID     uint64
	MessageID string
}

// SendText 给某个会话发一条文本
//
// convID 形如 0:1:<自己的uid>:<对方uid>
// salt  互关用 AccountSalt；单向关注必须传该会话自己的 salt
//
// 返回的 SendResult.OK 只代表接口层面成功。
// 抖音可能静默丢弃（风控），要确认对方真收到，
// 建议隔几秒重新拉一次会话列表，看最后一条是不是自己发的内容。
func SendText(a *Account, convID string, salt uint64, text string) (*SendResult, error) {
	if salt == 0 {
		salt = AccountSalt
	}

	seq := a.NextSeq()

	// field5 里的 kv 对，value 都是 JSON 字符串
	payload := map[string]any{
		"conversation_id": convID,
		"content":         text,
		"msg_type":        7, // 文本
	}
	pj, _ := json.Marshal(payload)

	inner := Cat(
		PBytes(nil, 1, KVPair("s:send_message", pj)),
	)

	body := Cat(
		PUint(nil, 1, cmdSend),   // cmd
		PUint(nil, 2, seq),       // 序号，严格递增
		PUint(nil, 3, salt),      // salt，不是时间戳
		PBytes(nil, 5, inner),    // kv 列表
	)

	raw, err := a.Post("/v1/message/send", body)
	if err != nil {
		return nil, err
	}

	r, err := ParseResp(raw)
	if err != nil {
		return nil, err
	}

	out := &SendResult{
		OK:    r.StatusCode == 0,
		Code:  r.StatusCode,
		Msg:   r.StatusMsg,
		LogID: r.LogID,
	}
	if !out.OK {
		out.Msg = HumanError(r.StatusCode, out.Msg)
	}
	return out, nil
}

// HumanError 把错误码翻译成能看懂的提示
func HumanError(code int, fallback string) string {
	switch code {
	case 0:
		return "成功"
	case 7100:
		return "需要先和 TA 聊一句才能发（或 salt 不匹配）"
	case 7905:
		return "触发风控：检查 salt 是否被改动"
	case -14:
		return "登录已过期，重新扫码"
	default:
		if fallback != "" {
			return fallback
		}
		return fmt.Sprintf("发送失败（%d）", code)
	}
}
