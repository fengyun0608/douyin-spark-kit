package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// 抖音 cookie 必须加密入库。
// 密钥取自 JWT_SECRET（生产请换成独立密钥），用 SHA-256 派生 32 字节。

var cryptKey []byte

// SetCookieKey 由 main 在启动时注入。
func SetCookieKey(secret string) {
	sum := sha256.Sum256([]byte("sparkpal-cookie:" + secret))
	cryptKey = sum[:]
}

func encrypt(plain string) string {
	if plain == "" {
		return ""
	}
	if cryptKey == nil {
		return ""
	}
	block, err := aes.NewCipher(cryptKey)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return ""
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(out)
}

func decrypt(enc string) string {
	if enc == "" || cryptKey == nil {
		return ""
	}
	raw, err := base64.RawStdEncoding.DecodeString(enc)
	if err != nil {
		return ""
	}
	block, err := aes.NewCipher(cryptKey)
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	if len(raw) < gcm.NonceSize() {
		return ""
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return ""
	}
	return string(plain)
}

// ErrDecrypt 解密失败。
var ErrDecrypt = errors.New("cookie 解密失败")
