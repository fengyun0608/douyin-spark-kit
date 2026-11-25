package douyin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

/* ══════════════════════════════════════════════════════════════
   用户资料查询

   域名：www.douyin.com（注意不是 imapi）
   路径：POST /aweme/v1/web/im/user/info/
   编码：form-urlencoded
   鉴权：只要 cookie —— 不需要 a_bogus 签名（实测 2026-11 有效）

   一次可以查多个 sec_uid。
   ══════════════════════════════════════════════════════════════ */

const (
	webHost = "https://www.douyin.com"

	// 基础 query 参数（服务端会校验，缺了返回 status_code 非 0）
	userInfoQS = "device_platform=webapp&aid=6383&channel=channel_pc_web" +
		"&pc_client_type=1&version_code=170400&version_name=17.4.0" +
		"&cookie_enabled=true&platform=PC"
)

// UserInfo 一个用户的资料
type UserInfo struct {
	UID       string `json:"uid"`
	SecUID    string `json:"sec_uid"`
	Nickname  string `json:"nickname"`
	UniqueID  string `json:"unique_id"`   // 抖音号
	ShortID   string `json:"short_id"`
	Signature string `json:"signature"`
	AvatarURL string `json:"avatarUrl"`
	Follow    int    `json:"followStatus"`
	Follower  int    `json:"followerStatus"`
	IsBlock   bool   `json:"isBlock"`
}

// userInfoResp 接口返回
type userInfoResp struct {
	StatusCode int `json:"status_code"`
	Data       []struct {
		UID       string `json:"uid"`
		SecUID    string `json:"sec_uid"`
		Nickname  string `json:"nickname"`
		UniqueID  string `json:"unique_id"`
		ShortID   string `json:"short_id"`
		Signature string `json:"signature"`
		Follow    int    `json:"follow_status"`
		IsBlock   bool   `json:"is_block"`
		AvatarThumb struct {
			URLList []string `json:"url_list"`
		} `json:"avatar_thumb"`
	} `json:"data"`
}

// FetchUsers 批量查用户资料（最多 20 个一次）
func (m *Manager) FetchUsers(acc *Account, secUIDs []string) ([]UserInfo, error) {
	if len(secUIDs) == 0 {
		return nil, nil
	}
	if len(secUIDs) > 20 {
		secUIDs = secUIDs[:20]
	}

	// body: sec_user_ids=["...","..."]
	idJSON, _ := json.Marshal(secUIDs)
	form := url.Values{}
	form.Set("sec_user_ids", string(idJSON))
	formBody := form.Encode()

	u := webHost + "/aweme/v1/web/im/user/info/?" + userInfoQS

	req, err := http.NewRequest(http.MethodPost, u, strings.NewReader(formBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", acc.UA)
	req.Header.Set("Cookie", acc.Cookie)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "https://www.douyin.com/chat")
	req.Header.Set("Accept", "application/json, text/plain, */*")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
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

	var r userInfoResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("解析失败: %w", err)
	}
	if r.StatusCode != 0 {
		return nil, fmt.Errorf("status_code=%d", r.StatusCode)
	}

	out := make([]UserInfo, 0, len(r.Data))
	for _, d := range r.Data {
		ui := UserInfo{
			UID:       d.UID,
			SecUID:    d.SecUID,
			Nickname:  d.Nickname,
			UniqueID:  d.UniqueID,
			ShortID:   d.ShortID,
			Signature: d.Signature,
			Follow:    d.Follow,
			IsBlock:   d.IsBlock,
		}
		if len(d.AvatarThumb.URLList) > 0 {
			ui.AvatarURL = d.AvatarThumb.URLList[0]
		}
		out = append(out, ui)
	}
	return out, nil
}

// FetchUserName 只查一个用户的昵称
func (m *Manager) FetchUserName(acc *Account, secUID string) (string, error) {
	list, err := m.FetchUsers(acc, []string{secUID})
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("查不到")
	}
	return list[0].Nickname, nil
}

// LookupByDouyinID 用抖音号查用户
//
// 走 /aweme/v1/web/discover/search/（需要签名，这里先不实现）
// 备选：直接拼 https://www.douyin.com/user/<抖音号> 不行，只认 sec_uid
//
// 实际用法是用户在 App 里粘贴抖音号，我们用搜索接口查。
// 暂时留空，后面用 Node 转发。
func LookupByDouyinID(acc *Account, douyinID string) ([]UserInfo, error) {
	return nil, fmt.Errorf("抖音号搜索需要签名，暂未实现")
}

var _ = bytes.NewReader
