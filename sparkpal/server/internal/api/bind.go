package api

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/fengyun0608/sparkpal/server/internal/douyin"
	"github.com/fengyun0608/sparkpal/server/internal/store"
)

/* ── 抖音绑定 ──
   流程：POST /bind/qr/enqueue → 拿 ticket + 二维码
        GET  /bind/qr/status  → 轮询，confirmed 后落库
        GET  /accounts        → 已绑定的抖音号
        POST /bind/unbind     → 解绑并清理
   二维码出码复用 Playwright 服务（QR_BASE），不在本进程里跑浏览器。
*/

type qrEnqueueResp struct {
	OK       bool   `json:"ok"`
	Ticket   string `json:"ticket"`
	Position int    `json:"position"`
	QR       string `json:"qr"`
	QRURL    string `json:"qrUrl"`
	ExpireAt int64  `json:"expireAt"`
}

// bindQR 入队，代理到 Playwright 服务。
func (s *Server) bindQR(w http.ResponseWriter, r *http.Request) {
	c, err := s.auth(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	if s.cfg.QRBase == "" {
		fail(w, http.StatusServiceUnavailable, "qr_disabled", "本服务器未启用扫码服务")
		return
	}
	var resp qrEnqueueResp
	if err := s.postJSON(s.cfg.QRBase+"/enqueue", nil, &resp); err != nil {
		slog.Error("[bind] 出码失败", "err", err)
		fail(w, http.StatusBadGateway, "qr_error", "扫码服务暂时不可用")
		return
	}
	ok(w, map[string]any{
		"ok":       true,
		"ticket":   resp.Ticket,
		"position": resp.Position,
		"qr":       resp.QR,
		"qrUrl":    resp.QRURL,
		"expireAt": resp.ExpireAt,
		"uid":      c.UID,
	})
}

// bindStatus 查扫码状态；confirmed 时把账号落库。
func (s *Server) bindStatus(w http.ResponseWriter, r *http.Request) {
	c, err := s.auth(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	ticket := strings.TrimSpace(r.URL.Query().Get("ticket"))
	if ticket == "" {
		fail(w, http.StatusBadRequest, "no_ticket", "缺少 ticket")
		return
	}
	var st struct {
		State    string `json:"state"`
		QR       string `json:"qr"`
		QRURL    string `json:"qrUrl"`
		Position int    `json:"position"`
		Cookie   string `json:"cookie"`
		Message  string `json:"message"`
	}
	if err := s.getJSON(s.cfg.QRBase+"/status?ticket="+ticket, &st); err != nil {
		fail(w, http.StatusBadGateway, "qr_error", "扫码服务暂时不可用")
		return
	}

	switch st.State {
	case "confirmed":
		if st.Cookie == "" {
			fail(w, http.StatusBadGateway, "no_cookie", "扫码成功但没拿到登录态，请重试")
			return
		}
		acc, err := s.bindAccount(r, c.UID, st.Cookie)
		if err != nil {
			slog.Error("[bind] 落库失败", "err", err)
			fail(w, http.StatusInternalServerError, "bind_failed", "绑定失败："+err.Error())
			return
		}
		ok(w, map[string]any{
			"ok": true, "state": "bound",
			"account": map[string]any{
				"id": acc.ID, "uid": acc.UID, "nickname": acc.Nickname,
				"avatar": acc.Avatar, "friends": acc.FriendCount,
			},
		})
	default:
		ok(w, map[string]any{
			"ok": true, "state": st.State,
			"qr": st.QR, "qrUrl": st.QRURL, "position": st.Position,
			"message": st.Message,
		})
	}
}

// bindAccount 用 cookie 解析出身份并落库。
func (s *Server) bindAccount(r *http.Request, userID, cookie string) (*store.DouyinAccount, error) {
	acc := douyin.NewAccount("", "", "", cookie)
	convs, err := s.dy.GetConversations(acc)
	if err != nil {
		return nil, err
	}
	if len(convs) == 0 {
		return nil, errNoConv
	}

	// 从会话 ID 里数出真正的自己（自己出现在每一条里）
	cnt := map[string]int{}
	for _, c := range convs {
		fs := strings.Split(c.ConvID, ":")
		if len(fs) == 4 {
			cnt[fs[2]]++
			cnt[fs[3]]++
		}
	}
	selfUID, best := "", 0
	for uid, n := range cnt {
		if uid != "" && n > best {
			selfUID, best = uid, n
		}
	}
	if selfUID == "" || best < 2 {
		return nil, errSelfUnknown
	}

	mySec := ""
	for _, c := range convs {
		if c.SelfSecUID != "" {
			mySec = c.SelfSecUID
			break
		}
	}

	// Account 里只有 UID（数字），sec_uid 我们自己记
	acc.UID = selfUID

	nick, avatar := "", ""
	if mySec != "" {
		if list, e := s.dy.FetchUsers(acc, []string{mySec}); e == nil && len(list) > 0 {
			for _, u := range list {
				// 只认 uid 完全相等，绝不猜
				if u.UID == selfUID {
					nick, avatar = u.Nickname, u.AvatarURL
					break
				}
			}
		}
	}

	saved, err := s.st.UpsertDouyin(r.Context(), userID, selfUID, mySec, cookie, nick, avatar)
	if err != nil {
		return nil, err
	}
	if err := s.st.ReplaceFriends(r.Context(), saved.ID, selfUID, convs); err != nil {
		slog.Error("[bind] 写好友失败", "err", err)
	}
	saved.FriendCount = len(convs)
	return saved, nil
}

// listAccounts 已绑定的抖音号。
func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	c, err := s.auth(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	rows, err := s.st.ListDouyin(r.Context(), c.UID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "db_error", "查询失败")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "uid": a.UID, "nickname": a.Nickname,
			"avatar": a.Avatar, "friends": a.FriendCount, "status": a.Status,
		})
	}
	ok(w, map[string]any{"ok": true, "accounts": out})
}
