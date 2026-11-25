package api

import (
	"database/sql"
	"net/http"
	"time"
)

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{"ok": true, "ts": time.Now().Unix()})
}

// meta 是整个客户端的分叉点：
// 客户端不判断「是不是官方」，只看这里返回的能力。
func (s *Server) meta(w http.ResponseWriter, r *http.Request) {
	quota := map[string]any{
		"plan":        "unlimited",
		"maxAccounts": -1,
		"maxFriends":  -1,
		"maxDaily":    -1,
	}
	if s.cfg.Billing {
		quota = map[string]any{
			"plan":        "free",
			"maxAccounts": s.cfg.FreeMaxAccounts,
			"maxFriends":  s.cfg.FreeMaxFriends,
			"maxDaily":    s.cfg.FreeMaxDaily,
		}
	}
	ok(w, map[string]any{
		"app":          "sparkpal",
		"apiVersion":   1,
		"name":         s.cfg.Name,
		"official":     s.cfg.Official,
		"billing":      s.cfg.Billing,
		"needPairCode": s.cfg.PairCode != "",
		"needInvite":   s.cfg.NeedInvite,
		"quota":        quota,
		"serverTime":   time.Now().Unix(),
	})
}

// me 返回当前身份与额度。
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	c, err := s.auth(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, "unauthorized", "请先登录")
		return
	}
	u, err := s.st.GetUser(c.UID)
	if err != nil {
		fail(w, http.StatusUnauthorized, "unauthorized", "账号不存在")
		return
	}
	ok(w, map[string]any{
		"ok":       true,
		"uid":      u.ID,
		"phone":    str(u.Phone),
		"email":    str(u.Email),
		"nickname": str(u.Nickname),
		"plan":     u.Plan,
		"planExpireAt": func() any {
			if u.PlanExpireAt.Valid {
				return u.PlanExpireAt.Time
			}
			return nil
		}(),
		"mode": s.cfg.Mode,
	})
}

func str(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

// auth 从 Authorization 头取身份。
func (s *Server) auth(r *http.Request) (*claims, error) {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return parseToken(s.cfg.Secret, h[7:])
	}
	return nil, errBadToken
}
