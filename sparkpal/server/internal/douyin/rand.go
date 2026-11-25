package douyin

import "crypto/rand"

// randHex 生成 n 字节随机数的十六进制串（长度 2n）
func randHex(n int) string {
	const hexdig = "0123456789abcdef"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// 极端情况下退化成全 0，不阻塞业务
		for i := range b {
			b[i] = 0
		}
	}
	out := make([]byte, n*2)
	for i, v := range b {
		out[i*2] = hexdig[v>>4]
		out[i*2+1] = hexdig[v&0x0f]
	}
	return string(out)
}
