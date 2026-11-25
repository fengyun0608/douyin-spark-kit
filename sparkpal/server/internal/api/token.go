package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// 手写 HS256 JWT：只为避免引入依赖，格式与标准一致。

type claims struct {
	UID      string `json:"uid"`
	Plan     string `json:"plan"`
	Mode     string `json:"mode"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

var errBadToken = errors.New("token invalid")

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(secret string, payload string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(payload))
	return b64(m.Sum(nil))
}

func issueToken(secret, uid, plan, mode string, ttl time.Duration) string {
	head := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	c := claims{UID: uid, Plan: plan, Mode: mode,
		IssuedAt: time.Now().Unix(), Expires: time.Now().Add(ttl).Unix()}
	body, _ := json.Marshal(c)
	payload := head + "." + b64(body)
	return payload + "." + sign(secret, payload)
}

func parseToken(secret, token string) (*claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errBadToken
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(secret, payload)), []byte(parts[2])) {
		return nil, errBadToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errBadToken
	}
	var c claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, errBadToken
	}
	if c.Expires < time.Now().Unix() {
		return nil, errBadToken
	}
	return &c, nil
}
