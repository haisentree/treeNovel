package localbook

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

// docBoundary 一份独立 XHTML 文档的开始（epub 转制的 MOBI 按文档分章）。
var docBoundary = regexp.MustCompile(`(?i)<\?xml|<!DOCTYPE|<html[\s>]`)

// junkRun 标题/段落里连续出现的排版垃圾字符（坏记录边界混入）。
var junkRun = regexp.MustCompile(`[<>=~*_#&$@|\\]{3,}`)

// skipTitles 目录/封面类文档：无阅读价值，整份丢弃。
var skipTitles = []string{"目录", "contents", "table of contents", "cover", "封面", "版权", "copyright"}

// descTitles 可以摘出内容简介的小节标题关键词。
var descTitles = []string{"内容简介", "内容提要", "内容介绍"}

// splitHTML 把整段 HTML 流拆成小节：先按文档边界切开，每份文档再按
// h1~h6 标题细分为小节（如「第一章」下的编号小节 1/2/3...），小节标题
// 形如「第一章 1」。封面页与解压出的二进制残留按中文字符数量/占比过滤。
// 返回小节列表与内容简介（如有「内容简介」小节）。
func splitHTML(html string) ([]Section, string) {
	var sections []Section
	description := ""
	for _, chunk := range docBoundary.Split(html, -1) {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		ss, desc := splitDoc(chunk)
		if desc != "" {
			description = desc
		}
		sections = append(sections, ss...)
	}
	return sections, description
}

// splitDoc 拆一份文档，返回小节与内容简介（该文档是内容简介页时）。
func splitDoc(chunk string) ([]Section, string) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(chunk))
	if err != nil {
		return nil, ""
	}
	docTitle := cleanText(doc.Find("title").First().Text())
	for _, kw := range skipTitles {
		if strings.EqualFold(docTitle, kw) ||
			strings.Contains(strings.ToLower(docTitle), strings.ToLower(kw)) {
			return nil, ""
		}
	}

	var out []Section
	// 章级标题（文档里第一个标题）；后续短标题（编号小节）带上它作前缀
	parent := ""
	cur := func() *Section {
		if n := len(out); n > 0 {
			return &out[n-1]
		}
		return nil
	}

	elems := doc.Find("h1, h2, h3, h4, h5, h6, p")
	elems.Each(func(_ int, s *goquery.Selection) {
		text := cleanText(s.Text())
		if text == "" {
			return
		}
		switch node := s.Get(0).Data; node {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			title := text
			if parent == "" {
				parent = title
			} else if runeLen(title) <= 8 && !strings.Contains(title, "章") && !strings.Contains(title, "部") {
				title = parent + " " + title
			}
			out = append(out, Section{Title: title})
		default: // <p>
			c := cur()
			if c == nil { // 标题前的正文：挂到文档标题下
				if docTitle == "" {
					return
				}
				out = append(out, Section{Title: docTitle})
				c = cur()
			}
			c.Paragraphs = append(c.Paragraphs, text)
			c.Chars += runeLen(text)
		}
	})

	// 无标题无段落但整段有正文的文档（罕见排版）：整文档兜底成一个小节
	if len(out) == 0 {
		if body := cleanText(doc.Find("body").Text()); cjkCount(body) >= 10 {
			return []Section{{
				Title:      docTitle,
				Paragraphs: strings.Split(body, "\n"),
				Chars:      cjkCount(body),
			}}, ""
		}
		return nil, ""
	}

	var result []Section
	description := ""
	for _, s := range out {
		if !sectionWorthy(s) {
			continue // 纯标题小节、封面页、二进制残留
		}
		if descTitlesMatch(s.Title) && s.Chars >= 30 && description == "" {
			description = brief(s.Paragraphs)
		}
		result = append(result, s)
	}
	return result, description
}

// sectionWorthy 小节要有一定量的中文正文才算可读内容。
func sectionWorthy(s Section) bool {
	text := strings.Join(s.Paragraphs, "")
	n := cjkCount(text)
	return n >= 10 && n*5 >= runeLen(text) // 中文 ≥10 字且占比 ≥ 20%
}

func descTitlesMatch(title string) bool {
	for _, kw := range descTitles {
		if strings.Contains(title, kw) {
			return true
		}
	}
	return false
}

// brief 把段落列表拼成截断的简介文本。
func brief(paras []string) string {
	s := cleanText(strings.Join(paras, " "))
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300]) + "……"
	}
	return s
}

func cjkCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}
	return n
}

func runeLen(s string) int { return len([]rune(s)) }

// cleanText 压掉垃圾字符与全部空白（正文按段落保存，段落内不需要换行）。
func cleanText(s string) string {
	s = junkRun.Split(s, 2)[0]
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) || r == 0x3000 || r == 0xfeff {
			space = b.Len() > 0
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
