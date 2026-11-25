package douyin

import "strconv"

// itoa 在星语里定义在 recall.go；伙伴只搬了发消息与拉会话，
// 这里补一个同等实现，避免为了一个函数把整个撤回模块也搬过来。
func itoa(n int) string { return strconv.Itoa(n) }
