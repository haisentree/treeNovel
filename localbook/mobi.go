// Package localbook 解析本地电子书原文件（MOBI/AZW/PRC，即 Kindle MOBI7 格式），
// 不依赖 calibre 等外部工具，供 Web 端直接在线浏览 book/ 目录下的原文件。
//
// 解析流程：PDB 记录表 -> 记录 0 中的 PalmDOC/MOBI 头与 EXTH 元数据 ->
// 文本记录 PalmDOC 解压拼接 -> 按文档/标题拆分为小节（split.go）。
package localbook

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// ErrDRM 文件带 DRM 保护，无法解析（也不应绕过）。
var ErrDRM = errors.New("电子书带 DRM 保护，无法解析")

// Meta 书籍元数据（来自 MOBI 头与 EXTH 记录）。
type Meta struct {
	Title     string
	Author    string
	Publisher string
	Language  string
}

// Section 一个阅读小节：标题 + 段落列表。
type Section struct {
	Title      string
	Paragraphs []string
	Chars      int // 正文字数（去空白后）
}

// Book 一本解析完成的电子书。
type Book struct {
	Meta
	Description string // 内容简介（取自「内容简介/前言」类小节，可能为空）
	Sections    []Section

	html string // 解压拼接后的完整 HTML 流，拆分完成后清空
}

// ParseFile 解析一个 MOBI/AZW 原文件。
func ParseFile(path string) (*Book, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, err := parseMOBI(data)
	if err != nil {
		return nil, err
	}
	if b.Meta.Title == "" { // 元数据缺失时用文件名兜底
		b.Meta.Title = baseTitle(path)
	}
	b.Sections, b.Description = splitHTML(b.html)
	b.html = ""
	return b, nil
}

// ============================================================ PDB 记录表 ============================================================

// pdbRecords 取出全部 PDB 记录。部分制作工具（如 cnepub epubBuilder）写出的
// 记录表起始偏移不标准，这里在几个常见位置里挑出能通过合法性校验的那一个。
func pdbRecords(data []byte) ([][]byte, error) {
	if len(data) < 78 || string(data[60:64]) != "BOOK" {
		return nil, fmt.Errorf("不是 PDB/MOBI 格式文件")
	}
	creator := string(data[64:68])
	if creator != "MOBI" && creator != "TEXt" {
		return nil, fmt.Errorf("不支持的 PDB 类型 %q", creator)
	}
	num := int(binary.BigEndian.Uint16(data[76:78]))
	if num == 0 {
		return nil, fmt.Errorf("PDB 记录表为空")
	}
	for _, start := range []int{78, 79, 80} { // 标准为 78，坏工具会偏移 1~2 字节
		if recs, ok := tryRecords(data, start, num); ok {
			return recs, nil
		}
	}
	return nil, fmt.Errorf("PDB 记录表无法定位（文件可能损坏）")
}

func tryRecords(data []byte, start, num int) ([][]byte, bool) {
	if start+num*8 > len(data) {
		return nil, false
	}
	offs := make([]int, num)
	for i := 0; i < num; i++ {
		p := start + i*8
		offs[i] = int(data[p])<<16 | int(data[p+1])<<8 | int(data[p+2])
	}
	// 记录 0 必须在记录表之后，且偏移严格递增、不越界
	if offs[0] < start+num*8 || offs[num-1] >= len(data) {
		return nil, false
	}
	for i := 1; i < num; i++ {
		if offs[i] <= offs[i-1] {
			return nil, false
		}
	}
	recs := make([][]byte, num)
	for i := range recs {
		end := len(data)
		if i+1 < num {
			end = offs[i+1]
		}
		recs[i] = data[offs[i]:end]
	}
	return recs, true
}

// ============================================================ 记录 0：PalmDOC/MOBI 头 ============================================================

func parseMOBI(data []byte) (*Book, error) {
	recs, err := pdbRecords(data)
	if err != nil {
		return nil, err
	}
	r0 := recs[0]
	if len(r0) < 14 {
		return nil, fmt.Errorf("PalmDOC 头不完整")
	}
	compression := binary.BigEndian.Uint16(r0[0:2])
	if compression != 1 && compression != 2 {
		if compression == 17480 {
			return nil, fmt.Errorf("暂不支持 HUFF/CDIC 压缩（.azw3/KF8），请使用 MOBI7 格式（.azw/.mobi/.prc）")
		}
		return nil, fmt.Errorf("未知的压缩类型 %d", compression)
	}
	if encryption := binary.BigEndian.Uint16(r0[12:14]); encryption != 0 {
		return nil, ErrDRM
	}

	b := &Book{}
	textEncoding := uint32(65001)
	extraFlags := uint16(0)
	if len(r0) >= 20 && string(r0[16:20]) == "MOBI" && len(r0) >= 24 {
		hlen := int(binary.BigEndian.Uint32(r0[20:24]))
		if hlen >= 16 && 16+hlen <= len(r0) {
			if len(r0) >= 32 {
				textEncoding = binary.BigEndian.Uint32(r0[28:32])
			}
			if hlen >= 0xE4 { // MOBI 头足够长才有 extraFlags 与 EXTH
				extraFlags = binary.BigEndian.Uint16(r0[0xF2 : 0xF2+2])
				b.parseEXTH(r0[16+hlen:])
			}
		}
	}
	b.Meta.Language = strings.TrimSpace(b.Meta.Language)

	raw := textStream(recs, compression, extraFlags)
	b.html = decodeText(raw, textEncoding)
	return b, nil
}

// parseEXTH 解析 EXTH 记录里的书名(503)/作者(100)/出版者(101)/语言(524)。
func (b *Book) parseEXTH(ext []byte) {
	if len(ext) < 12 || string(ext[0:4]) != "EXTH" {
		return
	}
	count := int(binary.BigEndian.Uint32(ext[8:12]))
	p := 12
	for i := 0; i < count && p+8 <= len(ext); i++ {
		typ := binary.BigEndian.Uint32(ext[p : p+4])
		size := int(binary.BigEndian.Uint32(ext[p+4 : p+8]))
		if size < 8 || p+size > len(ext) {
			break
		}
		val := strings.TrimSpace(string(ext[p+8 : p+size]))
		switch typ {
		case 100:
			if b.Meta.Author == "" {
				b.Meta.Author = val
			}
		case 101:
			if b.Meta.Publisher == "" {
				b.Meta.Publisher = val
			}
		case 503:
			b.Meta.Title = stripBookBrackets(val)
		case 524:
			if b.Meta.Language == "" {
				b.Meta.Language = val
			}
		}
		p += size
	}
}

// stripBookBrackets 去掉中文书名号，如 《幻夜》 -> 幻夜。
func stripBookBrackets(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "《")
	s = strings.TrimSuffix(s, "》")
	return s
}

// ============================================================ 文本流 ============================================================

// textStream 解出正文记录并拼接为完整字节流。
//
// 注意 PalmDOC 的回引用可以跨记录，所以所有记录解压进同一个缓冲。
// 部分工具写出的 recordCount/extraFlags 字段本身就是坏的（如 cnepub 产物
// recordCount=4826 而实际记录 273 条），因此对头字段一律做范围保护，
// 尾部附加条目只裁剪合理长度，坏值直接跳过宁多不裁——垃圾字节随后会被
// 拆分阶段按「无中文内容」过滤掉，而误裁会真丢正文。
func textStream(recs [][]byte, compression uint16, extraFlags uint16) []byte {
	count := 0
	if len(recs) > 1 {
		if r0 := recs[0]; len(r0) >= 8 {
			count = int(binary.BigEndian.Uint16(r0[6:8]))
		}
		if count <= 0 || count > len(recs)-1 {
			count = len(recs) - 1
		}
	}
	out := make([]byte, 0, 1<<20)
	for i := 1; i <= count; i++ {
		src := trimExtra(recs[i], extraFlags)
		switch compression {
		case 1:
			out = append(out, src...)
		default:
			palmDocDecompress(src, &out)
		}
	}
	return out
}

// trimExtra 去掉记录尾部附加数据。flag 1 是多字节字符跨界标记（末字节低 2 位），
// 其余 flag 是「变长整数（值=条目字节数）」。对变长值设 8 字节上限：
// 合法值一般 ≤5，坏工具会写出上万字节——那种一律不裁。
func trimExtra(rec []byte, flags uint16) []byte {
	for e := 0; e < 16 && len(rec) > 0; e++ {
		flag := uint16(1) << e
		if flags&flag == 0 {
			continue
		}
		if flag == 1 {
			n := int(rec[len(rec)-1] & 3)
			if n+1 <= len(rec) {
				rec = rec[:len(rec)-n-1]
			}
		} else if n := trailingEntryLen(rec); n > 0 && n <= 8 && n <= len(rec) {
			rec = rec[:len(rec)-n]
		}
	}
	return rec
}

// trailingEntryLen 从记录末尾反向读一个变长整数（每字节低 7 位，遇最高位为 1 停止）。
func trailingEntryLen(rec []byte) int {
	bitpos, result := 0, 0
	for i := len(rec) - 1; i >= 0 && bitpos < 28; i-- {
		v := rec[i]
		result |= int(v&0x7f) << bitpos
		bitpos += 7
		if v&0x80 != 0 {
			return result
		}
	}
	return result
}

// palmDocDecompress 解压一条 PalmDOC 记录并追加到 out。
func palmDocDecompress(src []byte, out *[]byte) {
	buf := *out
	i := 0
	for i < len(src) {
		c := src[i]
		i++
		switch {
		case c == 0: // 字面量 NUL（坏数据占位，保留以免错位）
			buf = append(buf, 0)
		case c <= 8: // 后跟 1~8 个字面量
			n := int(c)
			if i+n > len(src) {
				n = len(src) - i
			}
			buf = append(buf, src[i:i+n]...)
			i += n
		case c <= 0x7f: // ASCII 字面量
			buf = append(buf, c)
		case c <= 0xbf: // 14 位回引用：<0bbb bbaa aaaa aaLL L> 距离 11 位，长度 3~10
			if i >= len(src) {
				i = len(src)
				break
			}
			pair := int(c)<<8 | int(src[i])
			i++
			dist := (pair >> 3) & 0x7ff
			length := pair&7 + 3
			if dist > len(buf) { // 坏数据防御：距离截断到已有输出
				dist = len(buf)
			}
			if dist == 0 { // 距离为 0 是坏数据，按 1 处理避免死循环
				dist = 1
			}
			// 每轮从「当前末尾往前 dist」处取字节：dist < length 时
			// 刚拷出的字节会再次被引用，这是 LZ77 重叠拷贝的固有语义
			for k := 0; k < length; k++ {
				buf = append(buf, buf[len(buf)-dist])
			}
		default: // 空格 + 高位字符
			buf = append(buf, ' ', c^0x80)
		}
	}
	*out = buf
}

// ============================================================ 解码 ============================================================

// decodeText 按 MOBI 头声明的编码解码，并去掉控制字符（保留换行类空白）。
func decodeText(raw []byte, enc uint32) string {
	var s string
	switch enc {
	case 1252:
		s = decodeCP1252(raw)
	default:
		// 65001=UTF-8；字段值可疑时按内容探测
		if enc != 65001 && !utf8.Valid(raw) && looksLatin(raw) {
			s = decodeCP1252(raw)
		} else {
			s = strings.ToValidUTF8(string(raw), "\ufffd")
		}
	}
	return cleanControl(s)
}

func looksLatin(raw []byte) bool {
	bad := 0
	for i := 0; i < len(raw) && i < 4096; i++ {
		if raw[i] < 0x20 && raw[i] != '\t' && raw[i] != '\n' && raw[i] != '\r' {
			bad++
		}
	}
	return bad > len(raw)/100
}

func cleanControl(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' || r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// decodeCP1252 Windows-1252 解码（0x80~0x9F 区域有映射表，其余同 Latin-1）。
func decodeCP1252(raw []byte) string {
	cp1252Extra := [32]rune{
		0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
		0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
		0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
		0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
	}
	runes := make([]rune, 0, len(raw))
	for _, c := range raw {
		if c >= 0x80 && c <= 0x9F {
			r := cp1252Extra[c-0x80]
			if r == 0x81 || r == 0x8D || r == 0x8F || r == 0x90 || r == 0x9D {
				r = utf8.RuneError
			}
			runes = append(runes, r)
		} else {
			runes = append(runes, rune(c))
		}
	}
	return string(runes)
}
