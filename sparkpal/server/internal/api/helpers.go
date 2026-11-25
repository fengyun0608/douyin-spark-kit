package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fengyun0608/sparkpal/server/internal/douyin"
)

var (
	errNoConv      = errors.New("这个抖音号没有任何会话")
	errSelfUnknown = errors.New("解析不出你的身份，请重试")
)

// getJSON / postJSON：给扫码服务用的小工具。
func (s *Server) getJSON(u string, out any) error {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	return s.doJSON(req, out)
}

func (s *Server) postJSON(u string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.doJSON(req, out)
}

func (s *Server) doJSON(req *http.Request, out any) error {
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return errors.New("扫码服务返回 " + resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// dyAccount 返回当前账号绑定的第一个抖音号（没有则 nil）。
func (s *Server) dyAccountOf(r *http.Request, userID string) (*douyin.Account, string, error) {
	rows, err := s.st.ListDouyin(r.Context(), userID)
	if err != nil || len(rows) == 0 {
		return nil, "", err
	}
	a := rows[0]
	cookie, err := s.st.Cookie(a.ID)
	if err != nil || cookie == "" {
		return nil, "", err
	}
	return douyin.NewAccount(a.ID, a.UID, a.SecUID, cookie), a.ID, nil
}
