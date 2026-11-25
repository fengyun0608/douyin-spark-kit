package douyin

import (
	"encoding/binary"
	"fmt"
)

/* ══════════════════════════════════════════════════════════════
   手写 protobuf 编解码

   抖音的 IM 接口用 protobuf，但字段定义没公开。
   这里按抓包逆向出来的结构手写，不用 .proto 文件。

   两个坑：
     ① 时间戳/大整数必须用 uint64，不能转 int（会溢出）
     ② 序号必须严格递增，否则服务端当重放拒绝
   ══════════════════════════════════════════════════════════════ */

// Field 一个 protobuf 字段
type Field struct {
	Num   int
	Wire  int // 0=varint, 2=bytes, 1=fixed64, 5=fixed32
	Uint  uint64
	Bytes []byte
}

/* ── 编码 ── */

// putVarint 追加一个 varint
func putVarint(b []byte, v uint64) []byte {
	for v > 0x7f {
		b = append(b, byte(v&0x7f)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func putTag(b []byte, num, wire int) []byte {
	return putVarint(b, uint64(num)<<3|uint64(wire))
}

// PBUint 无符号整数字段
func PBUint(num int, v uint64) []byte {
	b := putTag(nil, num, 0)
	return putVarint(b, v)
}

// PBStr 字符串/字节字段
func PBStr(num int, s string) []byte {
	return PBBytes(num, []byte(s))
}

// PBBytes 字节字段
func PBBytes(num int, v []byte) []byte {
	b := putTag(nil, num, 2)
	b = putVarint(b, uint64(len(v)))
	return append(b, v...)
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

/* ── 解码 ── */

// readVarint 读一个 varint
func readVarint(b []byte, pos int) (uint64, int, error) {
	var v uint64
	var shift uint
	for pos < len(b) {
		c := b[pos]
		pos++
		v |= uint64(c&0x7f) << shift
		if c&0x80 == 0 {
			return v, pos, nil
		}
		shift += 7
		if shift > 63 {
			return 0, pos, fmt.Errorf("varint 太长")
		}
	}
	return 0, pos, fmt.Errorf("varint 被截断")
}

// ParseFields 解析一层 protobuf
func ParseFields(b []byte) ([]Field, error) {
	out := make([]Field, 0, 16)
	pos := 0
	for pos < len(b) {
		tv, np, err := readVarint(b, pos)
		if err != nil {
			return out, err
		}
		pos = np
		num := int(tv >> 3)
		wire := int(tv & 7)

		switch wire {
		case 0:
			v, np2, err := readVarint(b, pos)
			if err != nil {
				return out, err
			}
			pos = np2
			out = append(out, Field{Num: num, Wire: 0, Uint: v})
		case 2:
			ln, np2, err := readVarint(b, pos)
			if err != nil {
				return out, err
			}
			pos = np2
			end := pos + int(ln)
			if end > len(b) {
				return out, fmt.Errorf("字段 %d 越界", num)
			}
			out = append(out, Field{Num: num, Wire: 2, Bytes: b[pos:end]})
			pos = end
		case 1:
			if pos+8 > len(b) {
				return out, fmt.Errorf("fixed64 越界")
			}
			out = append(out, Field{Num: num, Wire: 1, Bytes: b[pos : pos+8]})
			pos += 8
		case 5:
			if pos+4 > len(b) {
				return out, fmt.Errorf("fixed32 越界")
			}
			out = append(out, Field{Num: num, Wire: 5, Bytes: b[pos : pos+4]})
			pos += 4
		default:
			return out, fmt.Errorf("不支持的 wire type %d", wire)
		}
	}
	return out, nil
}

// EncodeFields 把字段列表编回去
func EncodeFields(fs []Field) []byte {
	var out []byte
	for _, f := range fs {
		switch f.Wire {
		case 0:
			out = append(out, PBUint(f.Num, f.Uint)...)
		case 2:
			out = append(out, PBBytes(f.Num, f.Bytes)...)
		default:
			out = putTag(out, f.Num, f.Wire)
			out = append(out, f.Bytes...)
		}
	}
	return out
}

/* ── 小工具 ── */

// GetField 取第一个指定编号的字段
func GetField(fs []Field, num int) *Field {
	for i := range fs {
		if fs[i].Num == num {
			return &fs[i]
		}
	}
	return nil
}

// GetFields 取所有指定编号的字段
func GetFields(fs []Field, num int) []Field {
	out := make([]Field, 0, 2)
	for _, f := range fs {
		if f.Num == num {
			out = append(out, f)
		}
	}
	return out
}

// ReadUint32BE 大端读 4 字节（备用）
func ReadUint32BE(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return binary.BigEndian.Uint32(b[:4])
}
