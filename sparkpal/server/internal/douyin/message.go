package douyin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

/* ══════════════════════════════════════════════════════════════
   抖音 IM 消息收发

   命令号（field 8 里那个子字段的编号）：
     100  = 发消息
     301  = get_by_conversation   拉指定会话消息
     604  = mark_read             标记已读
     2043 = get_message_by_init   拉全部会话 + 最近消息
     2048 = get_user_message
   ══════════════════════════════════════════════════════════════ */

const (
	CmdSend           = 100
	CmdGetByConv      = 301
	CmdMarkRead       = 604
	CmdConvListInit   = 2043
	CmdGetUserMessage = 2048
)

/* ── 公共头部 ── */

// fingerprint 浏览器指纹（field 15 的那组 kv）
func fingerprint() [][2]string {
	ua := DefaultUA
	return [][2]string{
		{"session_aid", "6383"},
		{"session_did", "0"},
		{"app_name", "douyin_pc"},
		{"priority_region", "cn"},
		{"user_agent", ua},
		{"cookie_enabled", "true"},
		{"browser_language", "zh-CN"},
		{"browser_platform", "Win32"},
		{"browser_name", "Mozilla"},
		{"browser_version", ua[8:]},
		{"browser_online", "true"},
		{"screen_width", "1707"},
		{"screen_height", "1067"},
		{"referer", ""},
		{"timezone_name", "Asia/Shanghai"},
		{"deviceId", "0"},
		{"is-retry", "0"},
	}
}

// buildHeader 组装请求外壳
func buildHeader(seq uint64, cmdField uint64, cmdPayload []byte) []byte {
	cmdBlock := PBBytes(int(cmdField), cmdPayload)

	parts := [][]byte{
		PBUint(1, cmdField),
		PBUint(2, seq),
		PBStr(3, "0.1.8"),
		PBStr(4, ""),
		PBUint(5, 3),
		PBUint(6, 0),
		PBStr(7, "0d50935:feat/pc-im-group"),
		cmdBlock,
		PBStr(9, "0"),
		PBStr(11, "douyin_pc"),
		PBStr(14, "360000"),
	}
	for _, kv := range fingerprint() {
		parts = append(parts, PBBytes(15, Cat(PBStr(1, kv[0]), PBStr(2, kv[1]))))
	}
	parts = append(parts,
		PBUint(18, 1),
		PBStr(21, "douyin_web"),
		PBStr(22, "web_sdk"),
	)
	return Cat(parts...)
}

func kvPair(k, v string) []byte {
	return Cat(PBStr(1, k), PBStr(2, v))
}

/* ── 发消息 ── */

type textJSON struct {
	AweType       int   `json:"aweType"`
	Type          int   `json:"type"`
	RichTextInfos []any `json:"richTextInfos"`
	Text          string `json:"text"`
}

// NewUUID 简易 UUID v4
func NewUUID() string {
	return fmt.Sprintf("%s-%s-4%s-%s%s-%s",
		randHex(4), randHex(2), randHex(3), randHex(1), randHex(3), randHex(6))
}

// buildSendBody 用抓包模板构造发消息请求
//
// 只改三处：顶层序号、命令100 的 conv_id、命令100 的 text JSON。
// 其余字段（尤其 f3 = msgSalt）绝不能动。
func buildSendBody(seq uint64, convID string, salt uint64, text string) ([]byte, string) {
	clientID := NewUUID()

	raw, err := base64.StdEncoding.DecodeString(sendTemplateB64)
	if err != nil {
		return nil, clientID
	}
	top, err := ParseFields(raw)
	if err != nil {
		return nil, clientID
	}

	// ① 顶层序号
	for i := range top {
		if top[i].Num == 2 && top[i].Wire == 0 {
			top[i].Uint = seq
			break
		}
	}

	// ② 进 field 8 → 命令 100
	for i := range top {
		if top[i].Num != 8 || top[i].Wire != 2 {
			continue
		}
		cmdFields, err := ParseFields(top[i].Bytes)
		if err != nil || len(cmdFields) == 0 {
			break
		}
		biz, err := ParseFields(cmdFields[0].Bytes)
		if err != nil {
			break
		}

		now := time.Now()
		stime := fmt.Sprintf("%d.%04d", now.UnixMilli(), now.Nanosecond()/100000)

		for j := range biz {
			switch biz[j].Num {
			case 1: // conv_id
				if biz[j].Wire == 2 {
					biz[j].Bytes = []byte(convID)
				}
			case 3: // 会话 salt —— 每个会话不同，填错就 7100
				if biz[j].Wire == 0 && salt > 0 {
					biz[j].Uint = salt
				}
			case 4: // 正文 JSON
				if biz[j].Wire == 2 {
					b, _ := json.Marshal(textJSON{
						AweType:       700,
						Type:          0,
						RichTextInfos: []any{},
						Text:          text,
					})
					biz[j].Bytes = b
				}
			case 5: // kv 列表
				if biz[j].Wire != 2 {
					continue
				}
				kv, err := ParseFields(biz[j].Bytes)
				if err != nil || len(kv) < 2 || kv[0].Wire != 2 || kv[1].Wire != 2 {
					continue
				}
				switch string(kv[0].Bytes) {
				case "s:client_message_id":
					kv[1].Bytes = []byte(clientID)
					biz[j].Bytes = EncodeFields(kv)
				case "s:stime":
					kv[1].Bytes = []byte(stime)
					biz[j].Bytes = EncodeFields(kv)
				}
			case 8: // 尾部的 client_message_id
				if biz[j].Wire == 2 {
					biz[j].Bytes = []byte(clientID)
				}
			}
		}

		cmdFields[0].Bytes = EncodeFields(biz)
		top[i].Bytes = EncodeFields(cmdFields)
		break
	}

	return EncodeFields(top), clientID
}

// SendResult 发送结果
type SendResult struct {
	OK         bool   `json:"ok"`
	MsgID      string `json:"msgId"`
	ServerTime int64  `json:"serverTime"`
	Error      string `json:"error,omitempty"`
	Tips       string `json:"tips,omitempty"`
}

// SendText 给指定会话发一条文本
func (m *Manager) SendText(acc *Account, convID string, salt uint64, text string) *SendResult {
	// salt 为 0 表示「互关直发」，用账号默认 salt
	// 实测：互关的人（follow_status=2）不需要会话专属 salt
	if salt == 0 {
		salt = msgSalt
	}

	seq := acc.NextSeq()
	body, clientID := buildSendBody(seq, convID, salt, text)
	if body == nil {
		return &SendResult{Error: "构造请求失败"}
	}

	raw, err := acc.Post("/v1/message/send", body)
	if err != nil {
		return &SendResult{Error: err.Error()}
	}

	r, err := ParseResp(raw)
	if err != nil {
		return &SendResult{Error: "解析响应失败: " + err.Error()}
	}
	if r.StatusMsg != "OK" {
		return &SendResult{Error: r.StatusMsg}
	}

	if biz, err := ParseBizJSON(r.Biz); err == nil {
		if biz.StatusCode != 0 {
			return &SendResult{
				Error: humanError(biz.StatusCode, biz.Tips),
				Tips:  biz.Tips,
			}
		}
	}

	acc.Log("发消息成功", "conv", convID, "len", len(text), "clientMsgId", clientID)

	return &SendResult{
		OK:         true,
		MsgID:      r.LogID,
		ServerTime: int64(r.NowMs),
	}
}

/* ── 会话列表 ── */

// Conversation 一个会话
type Conversation struct {
	ConvID      string `json:"convId"`
	UID         string `json:"uid"`
	SecUID      string `json:"secUid"`
	Nickname    string `json:"nickname"`
	LastMsg     string `json:"lastMsg"`
	LastTime    int64  `json:"lastTime"`
	UnreadCount int    `json:"unread"`
	SparkDays   int    `json:"sparkDays"`
	AvatarURL   string `json:"avatar"`
	SparkLastAt int64  `json:"sparkLastAt"` // 上次互动（unix 秒）
	SparkDeadAt int64  `json:"sparkDeadAt"` // 火花到期（unix 秒）
	Salt        uint64 `json:"-"`           // 会话 salt（发消息填 f3）
	SelfSecUID  string `json:"-"`           // 我自己的 sec_uid（解析时顺带记下）
}

// buildConvListBody 构造「拉全部会话」请求
//
// 用抓包模板：服务端对字段顺序和 f6 有校验，自己拼会报 req.body is nil。
// 只改 field2（序号）。
func buildConvListBody(seq uint64) []byte {
	raw, err := base64.StdEncoding.DecodeString(convListTemplateB64)
	if err != nil {
		return nil
	}
	fs, err := ParseFields(raw)
	if err != nil {
		return nil
	}
	for i := range fs {
		if fs[i].Num == 2 && fs[i].Wire == 0 {
			fs[i].Uint = seq
			break
		}
	}
	return EncodeFields(fs)
}

// GetConversations 拉所有会话
func (m *Manager) GetConversations(acc *Account) ([]Conversation, error) {
	seq := acc.NextSeq()
	body := buildConvListBody(seq)
	if body == nil {
		return nil, fmt.Errorf("构造请求失败")
	}

	raw, err := acc.Post("/v1/message/get_message_by_init", body)
	if err != nil {
		return nil, err
	}
	if len(raw) < 20 {
		return nil, fmt.Errorf("响应太小: %s", string(raw))
	}

	r, err := ParseResp(raw)
	if err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if r.StatusMsg != "OK" {
		return nil, fmt.Errorf("status: %s", r.StatusMsg)
	}

	return parseConversations(r.Biz, acc.UID)
}

// parseConversations 解析会话列表
//
// 结构（get_message_by_init，命令 2043）：
//   命令 payload.f1[n]  = 第 n 个会话对象
//     会话对象.f1      = 元信息
//       元信息.f1      = conv_id
//       元信息.f6      = 参与者列表（f1=数字uid, f5=sec_uid）
//       元信息.f10     = 火花天数
//     会话对象.f2[n]   = 消息（跳过）
func parseConversations(biz []byte, myUID string) ([]Conversation, error) {
	// 抖音在响应 f6 里又套了一层「命令号 + 载荷」，先脱掉
	biz = unwrapCmdLayer(biz)

	fs, err := ParseFields(biz)
	if err != nil {
		return nil, err
	}

	// ① 先收集所有会话的原始对象，顺便从 convId 里数出"谁是真正的自己"
	//
	// 为什么不能直接信 myUID：
	//   换账号/换 cookie 后，账号表里的 uid 可能和会话里的对不上。
	//   这时 parseParticipants 一个都排除不掉，会把"我这一侧"当成好友，
	//   结果整张好友列表全是自己。
	//   自己一定出现在每一条会话里，所以取出现次数最多的那个 uid。
	raws := make([][]byte, 0, 64)
	cnt := map[string]int{}
	for _, f := range fs {
		if f.Num != 1 || f.Wire != 2 {
			continue
		}
		raws = append(raws, f.Bytes)
		if cid := convIDOf(f.Bytes); cid != "" {
			parts := strings.Split(cid, ":")
			if len(parts) == 4 {
				cnt[parts[2]]++
				cnt[parts[3]]++
			}
		}
	}
	selfUID := myUID
	best := 0
	for uid, n := range cnt {
		if uid == "" {
			continue
		}
		// 自己应当是"几乎每条都有"的那个；对方一般只出现一次
		if n > best {
			best = n
			selfUID = uid
		}
	}
	if selfUID != "" && selfUID != myUID {
		slog.Warn("[douyin] 会话里的自己和账号表不一致，按会话为准",
			"accountUID", myUID, "selfUID", selfUID, "hits", best, "total", len(raws))
	}

	out := make([]Conversation, 0, 64)
	withSec := 0
	for _, rb := range raws {
		c, ok := parseOneConv(rb, selfUID)
		if !ok {
			continue
		}
		if c.SecUID != "" {
			withSec++
		}
		out = append(out, c)
	}

	slog.Info("[douyin] 会话解析", "total", len(out), "withSecUid", withSec, "selfUID", selfUID)
	if len(out) > 0 {
		c0 := out[0]
		slog.Info("[douyin] 样例",
			"conv", c0.ConvID,
			"uid", c0.UID,
			"secLen", len(c0.SecUID),
			"spark", c0.SparkDays)
	}
	return out, nil
}

// unwrapCmdLayer 脱掉响应里「命令号 + 载荷」这一层
//
// 形如：f6 = { f2043 = <真正载荷> }  → 返回 <真正载荷>
func unwrapCmdLayer(b []byte) []byte {
	fs, err := ParseFields(b)
	if err != nil || len(fs) != 1 {
		return b
	}
	if fs[0].Wire != 2 || fs[0].Num < 100 {
		return b
	}
	return fs[0].Bytes
}

// parseOneConv 解析单个会话对象
func parseOneConv(obj []byte, myUID string) (Conversation, bool) {
	var c Conversation

	fs, err := ParseFields(obj)
	if err != nil {
		return c, false
	}

	// 只要元信息（第一个 f1）
	var meta []byte
	for _, f := range fs {
		if f.Num == 1 && f.Wire == 2 {
			meta = f.Bytes
			break
		}
	}
	if meta == nil {
		return c, false
	}

	mfs, err := ParseFields(meta)
	if err != nil {
		return c, false
	}

	for _, f := range mfs {
		switch f.Num {
		case 1:
			if f.Wire == 2 && isConvID(string(f.Bytes)) {
				c.ConvID = string(f.Bytes)
			}
		case 6:
			if f.Wire == 2 {
				parseParticipants(f.Bytes, myUID, &c)
			}
		}
	}

	// 火花天数在 kv 里：a:consecutive_chat = "天数:上次互动:到期时间:标志:标志"
	if v, ok := findKv(meta, "a:consecutive_chat"); ok {
		parts := strings.Split(v, ":")
		if len(parts) >= 1 {
			if n, err := strconv.Atoi(parts[0]); err == nil {
				c.SparkDays = n
			}
		}
		if len(parts) >= 2 {
			if n, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				c.SparkLastAt = n
			}
		}
		if len(parts) >= 3 {
			if n, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
				c.SparkDeadAt = n
			}
		}
	}

	// 从任意一条消息里取会话 salt（消息的 f5，就是发送时要填的 f3）
	for _, f := range fs {
		if f.Num != 2 || f.Wire != 2 {
			continue
		}
		mf, err := ParseFields(f.Bytes)
		if err != nil {
			continue
		}
		if v := GetField(mf, 5); v != nil && v.Wire == 0 && v.Uint > 0 {
			c.Salt = v.Uint
			break
		}
	}

	if c.ConvID == "" {
		return c, false
	}
	return c, true
}

// parseParticipants 从参与者列表挑出「对方」
func parseParticipants(b []byte, myUID string, c *Conversation) {
	pfs, err := ParseFields(b)
	if err != nil {
		return
	}
	for _, p := range pfs {
		if p.Num != 1 || p.Wire != 2 {
			continue
		}
		members, err := ParseFields(p.Bytes)
		if err != nil {
			continue
		}
		var uid, sec string
		for _, m := range members {
			switch m.Num {
			case 1: // 数字 uid（varint；偶尔是 bytes）
				if m.Wire == 0 {
					uid = strconv.FormatUint(m.Uint, 10)
				} else if m.Wire == 2 {
					uid = string(m.Bytes)
				}
			case 5: // sec_uid
				if m.Wire == 2 && isSecUID(string(m.Bytes)) {
					sec = string(m.Bytes)
				}
			}
		}
		if uid == "" {
			continue
		}
		if uid == myUID {
			// 记下自己的 sec_uid，登录时用来查自己的昵称
			if sec != "" && c.SelfSecUID == "" {
				c.SelfSecUID = sec
			}
			continue
		}
		if c.UID == "" || c.UID == myUID {
			c.UID = uid
			c.SecUID = sec
		} else if c.SecUID == "" && sec != "" {
			c.SecUID = sec
		}
	}
}

// isConvID 判断是不是 "0:x:数字:数字"
func isConvID(s string) bool {
	if len(s) < 7 || len(s) > 64 {
		return false
	}
	if s[0] != '0' || s[1] != ':' {
		return false
	}
	colon := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == ':' {
			colon++
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return colon == 3
}

// isSecUID 判断是不是 sec_uid
func isSecUID(s string) bool {
	return len(s) >= 30 && len(s) <= 200 && strings.HasPrefix(s, "MS4wLjABAAAA")
}

// ConvIDFor 用对方的数字 uid 拼出 1v1 会话 id
func ConvIDFor(myUID, otherUID string) string {
	return "0:1:" + myUID + ":" + otherUID
}

// findKv 在嵌套 protobuf 里找 {f1:key, f2:value} 形态的 kv 对
//
// 抖音把会话的各种属性都塞在这种 kv 里（a:xxx / s:xxx）。
func findKv(b []byte, want string) (string, bool) {
	var walk func(buf []byte, depth int) (string, bool)
	walk = func(buf []byte, depth int) (string, bool) {
		if depth > 6 {
			return "", false
		}
		fs, err := ParseFields(buf)
		if err != nil {
			return "", false
		}
		var k, v []byte
		for _, f := range fs {
			if f.Wire != 2 {
				continue
			}
			switch f.Num {
			case 1:
				if k == nil {
					k = f.Bytes
				}
			case 2:
				if v == nil {
					v = f.Bytes
				}
			}
		}
		if k != nil && v != nil {
			key := string(k)
			if len(key) > 1 && len(key) < 48 && key[1] == ':' {
				if key == want {
					return string(v), true
				}
				return "", false
			}
		}
		for _, f := range fs {
			if f.Wire == 2 {
				if got, ok := walk(f.Bytes, depth+1); ok {
					return got, true
				}
			}
		}
		return "", false
	}
	return walk(b, 0)
}

// humanError 把抖音的 status_code 翻成人话
//
// 常见码：
//   7100 = 会话不存在 / 不是好友关系
//   7905 = 风控拦截
//   -14  = 登录态失效
func humanError(code int, tips string) string {
	if tips != "" {
		return tips
	}
	switch code {
	case 7100:
		return "需要先和 TA 聊一句才能续火花"
	case 7905:
		return "被抖音风控拦了，歇一会儿再试"
	case -14:
		return "登录已过期，请重新登录抖音"
	case 1000:
		return "对方已注销或封号"
	default:
		return "发送失败（错误码 " + itoa(code) + "）"
	}
}

// convIDOf 只从会话对象里取 conv_id，不做其它解析
func convIDOf(obj []byte) string {
	fs, err := ParseFields(obj)
	if err != nil {
		return ""
	}
	var meta []byte
	for _, f := range fs {
		if f.Num == 1 && f.Wire == 2 {
			meta = f.Bytes
			break
		}
	}
	if meta == nil {
		return ""
	}
	mfs, err := ParseFields(meta)
	if err != nil {
		return ""
	}
	for _, f := range mfs {
		if f.Num == 1 && f.Wire == 2 && isConvID(string(f.Bytes)) {
			return string(f.Bytes)
		}
	}
	return ""
}
