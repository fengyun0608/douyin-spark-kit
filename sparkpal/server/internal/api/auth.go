package api

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

/* ── 验证码（内存态，重启失效；够 MVP 用）── */

type codeEntry struct {
	code    string
	expire  time.Time
	lastAt  time.Time
	sentDay int
	day     string
}

var (
	codeMu sync.Mutex
	codes  = map[string]*codeEntry{}
)

var phoneRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

func genCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "000000"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

type smsSendReq struct {
	Phone string `json:"phone"`
}

// smsSend 发验证码。
//
// 生产：SMS_WEBHOOK 指向星语现有短信通道，这里只做转发。
// 开发：没配 webhook 就把验证码打进日志，方便自测。
func (s *Server) smsSend(w http.ResponseWriter, r *http.Request) {
	var req smsSendReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "bad_body", "请求格式不对")
		return
	}
	phone := strings.TrimSpace(req.Phone)
	if !phoneRe.MatchString(phone) {
		fail(w, http.StatusBadRequest, "bad_phone", "手机号格式不对")
		return
	}

	today := time.Now().Format("2006-01-02")
	codeMu.Lock()
	e := codes[phone]
	if e == nil {
		e = &codeEntry{day: today}
		codes[phone] = e
	}
	if e.day != today {
		e.day, e.sentDay = today, 0
	}
	if time.Since(e.lastAt) < 60*time.Second {
		codeMu.Unlock()
		fail(w, http.StatusTooManyRequests, "too_fast", "请 60 秒后再试")
		return
	}
	if e.sentDay >= 10 {
		codeMu.Unlock()
		fail(w, http.StatusTooManyRequests, "daily_limit", "今天验证码次数用完了")
		return
	}
	e.code = genCode()
	e.expire = time.Now().Add(5 * time.Minute)
	e.lastAt = time.Now()
	e.sentDay++
	code := e.code
	codeMu.Unlock()

	if s.cfg.SMSWebhook == "" {
		slog.Warn("[sms] 未配置 SMS_WEBHOOK，验证码走日志（仅开发）", "phone", mask(phone), "code", code)
		ok(w, map[string]any{"ok": true, "dev": true, "hint": "开发模式：验证码见服务端日志"})
		return
	}

	body, _ := json.Marshal(map[string]string{"phone": phone, "code": code, "scene": "sparkpal_login"})
	req2, _ := http.NewRequest(http.MethodPost, s.cfg.SMSWebhook, bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	if s.cfg.SMSKey != "" {
		req2.Header.Set("X-Key", s.cfg.SMSKey)
	}
	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req2)
	if err != nil {
		slog.Error("[sms] 网关请求失败", "err", err)
		fail(w, http.StatusBadGateway, "sms_gateway", "短信服务暂时不可用")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Error("[sms] 网关返回错误", "status", resp.StatusCode)
		fail(w, http.StatusBadGateway, "sms_gateway", "短信服务暂时不可用")
		return
	}
	ok(w, map[string]any{"ok": true})
}

type smsVerifyReq struct {
	Phone      string `json:"phone"`
	Code       string `json:"code"`
	InviteCode string `json:"inviteCode"`
	DeviceID   string `json:"deviceId"`
	Platform   string `json:"platform"`
}

// smsVerify 校验验证码 → 建号/登录 → 发 token。
func (s *Server) smsVerify(w http.ResponseWriter, r *http.Request) {
	var req smsVerifyReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "bad_body", "请求格式不对")
		return
	}
	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.Code)
	if !phoneRe.MatchString(phone) {
		fail(w, http.StatusBadRequest, "bad_phone", "手机号格式不对")
		return
	}

	// 邀请码：官方侧强制（免费期也拦），自建侧不需要
	if s.cfg.NeedInvite {
		ic := strings.ToUpper(strings.TrimSpace(req.InviteCode))
		if ic == "" {
			fail(w, http.StatusBadRequest, "invite_required", "需要邀请码")
			return
		}
		if _, err := s.st.GetInvite(ic); err != nil {
			fail(w, http.StatusBadRequest, "invite_invalid", "邀请码无效或已用完")
			return
		}
	}

	codeMu.Lock()
	e := codes[phone]
	bad := e == nil || e.code != code || time.Now().After(e.expire)
	if !bad {
		delete(codes, phone)
	}
	codeMu.Unlock()
	if bad {
		fail(w, http.StatusBadRequest, "code_invalid", "验证码不对或已过期")
		return
	}

	u, err := s.st.UpsertUserByPhone(phone)
	if err != nil {
		slog.Error("[auth] 建号失败", "err", err)
		fail(w, http.StatusInternalServerError, "db_error", "登录失败")
		return
	}

	if s.cfg.NeedInvite && req.InviteCode != "" {
		ic := strings.ToUpper(strings.TrimSpace(req.InviteCode))
		if err := s.st.ConsumeInvite(ic); err == nil {
			s.st.SetInviteCode(u.ID, ic)
		}
	}
	s.st.TouchDevice(u.ID, req.DeviceID, req.Platform)

	token := issueToken(s.cfg.Secret, u.ID, u.Plan, s.cfg.Mode, 30*24*time.Hour)
	ok(w, map[string]any{
		"ok":       true,
		"token":    token,
		"uid":      u.ID,
		"plan":     u.Plan,
		"mode":     s.cfg.Mode,
		"expireIn": 30 * 24 * 3600,
	})
}

type pairReq struct {
	Code     string `json:"code"`
	DeviceID string `json:"deviceId"`
	Platform string `json:"platform"`
}

// pair 自建服务器配对：输入 PAIR_CODE 换长期 token。
func (s *Server) pair(w http.ResponseWriter, r *http.Request) {
	if s.cfg.PairCode == "" {
		fail(w, http.StatusBadRequest, "pair_disabled", "本服务器未启用配对码，可直接使用")
		return
	}
	var req pairReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "bad_body", "请求格式不对")
		return
	}
	if strings.TrimSpace(req.Code) != s.cfg.PairCode {
		fail(w, http.StatusUnauthorized, "pair_invalid", "配对码不对")
		return
	}

	u, err := s.st.UpsertUserByPhone("local:" + req.DeviceID)
	if err != nil {
		// 没有手机号的自建用户，用设备 id 占位；失败就退化成本地单用户
		slog.Warn("[pair] 建号失败，使用临时身份", "err", err)
		u = nil
	}
	uid, plan := "local", "unlimited"
	if u != nil {
		uid, plan = u.ID, "unlimited"
	}
	if req.DeviceID != "" && u != nil {
		s.st.TouchDevice(u.ID, req.DeviceID, req.Platform)
	}

	token := issueToken(s.cfg.Secret, uid, plan, s.cfg.Mode, 180*24*time.Hour)
	ok(w, map[string]any{"ok": true, "token": token, "uid": uid, "plan": plan, "mode": s.cfg.Mode})
}

var errNoCode = errors.New("no code")

func mask(phone string) string {
	if len(phone) < 11 {
		return phone
	}
	return phone[:3] + "****" + phone[7:]
}
