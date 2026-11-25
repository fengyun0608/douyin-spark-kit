package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/fengyun0608/sparkpal/server/internal/store"
)

type inviteReq struct {
	Code string `json:"code"`
}

// inviteCheck 只校验，不占用额度；真正占用在注册成功时。
func (s *Server) inviteCheck(w http.ResponseWriter, r *http.Request) {
	var req inviteReq
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, "bad_body", "请求格式不对")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		fail(w, http.StatusBadRequest, "empty_code", "请输入邀请码")
		return
	}
	it, err := s.st.GetInvite(code)
	if err != nil {
		if errors.Is(err, store.ErrInviteInvalid) {
			fail(w, http.StatusBadRequest, "invite_invalid", "邀请码无效或已用完")
			return
		}
		fail(w, http.StatusInternalServerError, "db_error", "查询失败")
		return
	}
	ok(w, map[string]any{
		"ok":        true,
		"code":      it.Code,
		"remaining": it.MaxUses - it.Used,
	})
}
