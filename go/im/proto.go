// Package im 提供抖音私信发送所需的最小编解码实现。
//
// 抖音的 IM 接口用的不是标准 .proto，字段是自己拼的。
// 这里手写 protobuf 的 varint / length-delimited 编解码，避免拉 protoc。
package im

import (
	"encoding/binary"
	"fmt"
)

/* ══════════════ 编码 ══════════════ */

// PutVarint 写入 varint
func PutVarint(dst []byte, v uint64) []byte {
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], v)
	return append(dst, tmp[:n]...)
}

// Tag 写入 field 头
//
// wireType: 0=varint 1=64bit 2=length-delimited 5=32bit
func Tag(dst []byte, field int, wireType int) []byte {
	return PutVarint(dst, uint64(field)<<3|uint64(wireType))
}

// PUint 写 varint 字段
func PUint(dst []byte, field int, v uint64) []byte {
	dst = Tag(dst, field, 0)
	return PutVarint(dst, v)
}

// PBytes 写 length-delimited 字段
func PBytes(dst []byte, field int, b []byte) []byte {
	dst = Tag(dst, field, 2)
	dst = PutVarint(dst, uint64(len(b)))
	return append(dst, b...)
}

// PStr 写字符串字段
func PStr(dst []byte, field int, s string) []byte {
	return PBytes(dst, field, []byte(s))
}

// Cat 拼接多段
func Cat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// KVPair 构造一个 key/value 对（value 通常是 JSON 字符串）
func KVPair(key string, value []byte) []byte {
	return Cat(PStr(nil, 1, key), PBytes(nil, 2, value))
}

/* ══════════════ 解码 ══════════════ */

// Field 一个解析出来的字段
type Field struct {
	Num  int
	Wire int
	Uint uint64   // wire=0
	Data []byte   // wire=2
}

// ParseFields 解析 protobuf 为字段列表
//
// 遇到不认识的 wire type 会返回错误，调用方自己决定要不要忽略。
func ParseFields(b []byte) ([]Field, error) {
	var out []Field
	i := 0
	for i < len(b) {
		tag, n := binary.Uvarint(b[i:])
		if n <= 0 {
			return out, fmt.Errorf("bad tag at %d", i)
		}
		i += n
		num := int(tag >> 3)
		wire := int(tag & 7)

		f := Field{Num: num, Wire: wire}
		switch wire {
		case 0:
			v, n2 := binary.Uvarint(b[i:])
			if n2 <= 0 {
				return out, fmt.Errorf("bad varint at %d", i)
			}
			i += n2
			f.Uint = v
		case 2:
			l, n2 := binary.Uvarint(b[i:])
			if n2 <= 0 {
				return out, fmt.Errorf("bad len at %d", i)
			}
			i += n2
			if i+int(l) > len(b) {
				return out, fmt.Errorf("overflow at %d", i)
			}
			f.Data = b[i : i+int(l)]
			i += int(l)
		case 1:
			i += 8
		case 5:
			i += 4
		default:
			return out, fmt.Errorf("unsupported wire %d", wire)
		}
		out = append(out, f)
	}
	return out, nil
}

// GetField 取第一个指定编号的字段
func GetField(fs []Field, num int) *Field {
	for i := range fs {
		if fs[i].Num == num {
			return &fs[i]
		}
	}
	return nil
}

// GetFields 取全部指定编号
func GetFields(fs []Field, num int) []Field {
	var out []Field
	for _, f := range fs {
		if f.Num == num {
			out = append(out, f)
		}
	}
	return out
}
