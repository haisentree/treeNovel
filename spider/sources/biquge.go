package sources

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// ---------- 笔趣阁系（jieqi CMS 模板）共用解析 ----------
//
// 页面特征（2026-09 校准，ibiqugu / xbiquwk 两站同模板）：
//   - UTF-8 编码；
//   - 书籍元信息在 og / og:novel:* meta 标签；
//   - 目录在 #list dd a，单页全量、无翻页，按阅读顺序排列；
//   - 章节页 h1 为标题，正文在 #content，<br> 分段，
//     可混有 #content_tip 提示位与站名水印。

// biqugeMeta 从 og meta 取书籍元信息，og 缺书名时退回 h1。
func biqugeMeta(doc *goquery.Document) spider.BookMeta {
	var meta spider.BookMeta
	get := func(prop string) string {
		sel := doc.Find(`meta[property="` + prop + `"]`).First()
		if sel.Length() == 0 { // 个别站用 name 而非 property
			sel = doc.Find(`meta[name="` + prop + `"]`).First()
		}
		return strings.TrimSpace(sel.AttrOr("content", ""))
	}
	meta.Title = get("og:title")
	meta.Author = get("og:novel:author")
	meta.Category = get("og:novel:category")
	meta.Status = get("og:novel:status")
	meta.Description = get("og:description")
	meta.CoverURL = get("og:image")
	if meta.Title == "" {
		meta.Title = strings.TrimSpace(doc.Find("#info h1, h1").First().Text())
	}
	return meta
}

// biqugeLinks 取默认目录选择器（#list dd a）的章节链接。
func biqugeLinks(pageURL string, doc *goquery.Document) []string {
	return biqugeLinksSel(pageURL, doc, "#list dd a")
}

// biqugeLinksSel 取目录选择器下的章节链接（保持目录顺序），
// 限定在本书路径下（过滤推荐位等站内链接）并按 URL 去重。
func biqugeLinksSel(pageURL string, doc *goquery.Document, sel string) []string {
	return sectionLinks(pageURL, doc.Find(sel))
}

// sectionLinks 从给定 <a> 集合提取章节链接（保持目录顺序），
// 限定在本书路径下（过滤推荐位等站内链接）并按 URL 去重。
// 目录页为文件形态（…/index_2.html）时按其所在目录判定本书路径。
func sectionLinks(pageURL string, aSel *goquery.Selection) []string {
	bookPath := ""
	if u, err := url.Parse(pageURL); err == nil {
		p := u.Path
		if strings.HasSuffix(p, ".html") || strings.HasSuffix(p, ".htm") {
			if i := strings.LastIndex(p, "/"); i >= 0 {
				p = p[:i+1]
			}
		}
		bookPath = strings.TrimSuffix(p, "/")
	}
	var links []string
	seen := map[string]bool{}
	aSel.Each(func(_ int, aEl *goquery.Selection) {
		href, ok := aEl.Attr("href")
		if !ok || href == "" {
			return
		}
		abs := spider.AbsURL(pageURL, href)
		if u, err := url.Parse(abs); err != nil ||
			(bookPath != "" && !strings.HasPrefix(u.Path, bookPath)) {
			return
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		links = append(links, abs)
	})
	return links
}

// biqugeChapter 解析章节页：h1 标题 + #content 正文（<br> 分段），
// 按 siteName 过滤站名水印段落。
func biqugeChapter(doc *goquery.Document, siteName string) (*spider.ChapterData, error) {
	return parseChapterSel(doc, "h1", "#content", "br", siteName)
}

// parseChapterSel 按选择器解析章节页：标题选择器取章节标题；
// 正文选择器在 br 分段时匹配容器、p 分段时匹配段落元素本身；
// watermark 非空时过滤命中水印的段落。
func parseChapterSel(doc *goquery.Document, titleSel, contentSel, mode, watermark string) (*spider.ChapterData, error) {
	title := strings.TrimSpace(doc.Find(titleSel).First().Text())
	if title == "" {
		return nil, fmt.Errorf("章节页解析不到标题（选择器 %q 未命中）", titleSel)
	}
	data := &spider.ChapterData{Title: title}
	var paras []string
	if mode == "p" {
		doc.Find(contentSel).Each(func(_ int, p *goquery.Selection) {
			if t := strings.TrimSpace(p.Text()); t != "" {
				paras = append(paras, t)
			}
		})
	} else {
		paras = spider.BrParagraphs(doc.Find(contentSel))
	}
	for _, p := range paras {
		if p == "" || (watermark != "" && spider.IsWatermark(p, watermark)) {
			continue
		}
		data.Paragraphs = append(data.Paragraphs, p)
	}
	if len(data.Paragraphs) == 0 {
		return nil, fmt.Errorf("正文为空（选择器 %q 未命中）", contentSel)
	}
	return data, nil
}
