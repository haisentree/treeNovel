package localbook

import (
	"path/filepath"
	"testing"
)

// 用仓库 book/ 下的真实电子书做冒烟测试；没有书时跳过。
func TestParseLocalBook(t *testing.T) {
	matches, _ := filepath.Glob("../book/*.azw")
	m2, _ := filepath.Glob("../book/*.mobi")
	matches = append(matches, m2...)
	if len(matches) == 0 {
		t.Skip("book/ 下没有 .azw/.mobi 测试文件")
	}
	b, err := ParseFile(matches[0])
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", matches[0], err)
	}
	t.Logf("书名=%q 作者=%q 语言=%q 出版=%q", b.Meta.Title, b.Meta.Author, b.Meta.Language, b.Meta.Publisher)
	t.Logf("简介=%q", truncate(b.Description, 50))
	t.Logf("小节数=%d", len(b.Sections))
	total := 0
	for i, s := range b.Sections {
		total += s.Chars
		if i < 6 || i >= len(b.Sections)-2 {
			t.Logf("  [%2d] %-16s %5d 字", i+1, truncate(s.Title, 16), s.Chars)
		}
	}
	t.Logf("总字数=%d", total)

	if b.Meta.Author == "" {
		t.Error("作者为空，EXTH 解析可能失效")
	}
	if total < 100000 {
		t.Errorf("正文字数 %d 过少，解压可能不完整", total)
	}
	if len(b.Sections) < 10 {
		t.Errorf("小节数 %d 过少，拆分可能失效", len(b.Sections))
	}
	// 抽查中间小节的段落质量：不应有大量乱码
	mid := b.Sections[len(b.Sections)/2]
	if len(mid.Paragraphs) == 0 {
		t.Fatalf("小节 %q 无段落", mid.Title)
	}
	sample := mid.Paragraphs[len(mid.Paragraphs)/2]
	t.Logf("样本段落(%q): %s", mid.Title, truncate(sample, 60))
	if cjk := cjkCount(sample); cjk < runeLen(sample)/2 {
		t.Errorf("样本段落中文占比过低: %d/%d", cjk, runeLen(sample))
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
