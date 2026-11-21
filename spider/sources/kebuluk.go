package sources

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// KebulukAdapter 斗破苍穹网（dp.kebuluk.xyz，doupoxs.cc 的镜像站）。
//
// 页面特征（2026-10 校准）：
//   - UTF-8 编码；
//   - 书籍元信息在 og / og:novel:* meta（分类、状态缺省，留空）；
//   - 书籍页（/xiaoshuo/NNN/）同时是完整目录：#play_0 li a 为正序全量，
//     其上方另有一段「最新章节」倒序推荐位，不能按文档顺序取链接；
//   - 首页（/）是主推书《斗破苍穹》的着陆页，只有「开始阅读/目录」
//     入口而无完整目录，ParseBook 会跟进「目录」链接解析目录页；
//   - 章节页 h1 为标题，正文在 #contentn 内 <p> 分段，首尾混有站方
//     广告段（马上记住斗破苍穹网…/如果看累了…/斗破苍穹网提供…）。
type KebulukAdapter struct{}

func (KebulukAdapter) Name() string { return "kebuluk" }

func (KebulukAdapter) Domains() []string {
	return []string{"dp.kebuluk.xyz", "kebuluk.xyz"}
}

func (KebulukAdapter) defaultSelectors() Selectors {
	return Selectors{List: "#play_0 li a", Title: "h1", Content: "#contentn p", Mode: "p", Watermark: "斗破苍穹网"}
}

func (a KebulukAdapter) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	meta := biqugeMeta(doc)
	// og meta 缺作者时（如首页着陆页）退回「作者：」信息行
	if meta.Author == "" {
		meta.Author = spider.TrimLabel(strings.TrimSpace(doc.Find(".m-author").First().Text()))
	}
	if meta.Title == "" {
		return meta, nil, fmt.Errorf("书籍页解析不到书名: %s", pageURL)
	}
	links := biqugeLinksSel(pageURL, doc, "#play_0 li a")
	if len(links) == 0 {
		// 着陆页只有入口链接：跟进文本为「目录」的链接取完整目录页
		tocURL := kebulukTocEntry(pageURL, doc)
		if tocURL == "" {
			return meta, nil, fmt.Errorf("页面既无完整目录（#play_0）也无「目录」入口链接: %s", pageURL)
		}
		tocDoc, err := fetchPageDoc(tocURL)
		if err != nil {
			return meta, nil, fmt.Errorf("跟进目录页失败 %s: %w", tocURL, err)
		}
		links = biqugeLinksSel(tocURL, tocDoc, "#play_0 li a")
		if len(links) == 0 {
			return meta, nil, fmt.Errorf("目录页未解析到章节链接: %s", tocURL)
		}
		// 着陆页 og meta 不全（缺封面/简介），用目录页的补齐
		tocMeta := biqugeMeta(tocDoc)
		if meta.Author == "" {
			meta.Author = tocMeta.Author
		}
		if meta.Description == "" {
			meta.Description = tocMeta.Description
		}
		if meta.CoverURL == "" {
			meta.CoverURL = tocMeta.CoverURL
		}
		if meta.Category == "" {
			meta.Category = tocMeta.Category
		}
		if meta.Status == "" {
			meta.Status = tocMeta.Status
		}
	}
	return meta, links, nil
}

// kebulukTocEntry 在着陆页找文本为「目录」的站内入口链接。
func kebulukTocEntry(pageURL string, doc *goquery.Document) string {
	entry := ""
	doc.Find("a").Each(func(_ int, aEl *goquery.Selection) {
		if entry != "" || strings.TrimSpace(aEl.Text()) != "目录" {
			return
		}
		if href := aEl.AttrOr("href", ""); href != "" {
			entry = spider.AbsURL(pageURL, href)
		}
	})
	return entry
}

// ListPageURLs 目录不分页（单页全量）。
func (KebulukAdapter) ListPageURLs(string) []string { return nil }

func (KebulukAdapter) ParseChapterList(string, *goquery.Document) ([]string, error) {
	return nil, nil
}

func (a KebulukAdapter) ParseChapter(_ string, doc *goquery.Document) (*spider.ChapterData, error) {
	// 站内存在两种正文模板：p 分段（主流）与 br 分段（部分分类，正文直接
	// 落在 #contentn 文本流里，p 选择器只能命中广告段）。两种都解析并
	// 过滤广告后，取总字数更大的一边作为正文。
	title := strings.TrimSpace(doc.Find("h1").First().Text())
	if title == "" {
		return nil, fmt.Errorf("章节页解析不到标题（选择器 %q 未命中）", "h1")
	}
	p := filterKebulukParas(parseKebulukParas(doc, "#contentn p"), title)
	br := filterKebulukParas(spider.BrParagraphs(doc.Find("#contentn").First()), title)
	paras := p
	if kebulukParaLen(br) > kebulukParaLen(p) {
		paras = br
	}
	if len(paras) == 0 {
		return nil, fmt.Errorf("正文为空（#contentn 的 p/br 两种解析均无有效段落）")
	}
	return &spider.ChapterData{Title: title, Paragraphs: paras}, nil
}

// filterKebulukParas 过滤广告段落：含站名「斗破苍穹网」的水印段、
// 「收藏一下本页面」提示段、上一章/下一章导航段，以及与标题混排的
// 首段前缀（br 模板正文开头可能直接跟着章节标题）。
func filterKebulukParas(paras []string, title string) []string {
	out := make([]string, 0, len(paras))
	for _, p := range paras {
		if spider.IsWatermark(p, "斗破苍穹网") || strings.Contains(p, "收藏一下本页面") {
			continue
		}
		if strings.HasPrefix(p, "上一章") && strings.Contains(p, "下一章") {
			continue
		}
		p = stripPrefixIgnoreSpace(p, title)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// stripPrefixIgnoreSpace 忽略空白差异剥离 s 开头的 prefix，
// 未匹配时原样返回（仅去首尾空白）。
func stripPrefixIgnoreSpace(s, prefix string) string {
	isBlank := func(r rune) bool {
		return r == ' ' || r == '\u3000' || r == '\n' || r == '\t' || r == '\r'
	}
	sr := []rune(s)
	pr := []rune(prefix)
	pc := make([]rune, 0, len(pr))
	for _, r := range pr {
		if !isBlank(r) {
			pc = append(pc, r)
		}
	}
	if len(pc) == 0 {
		return strings.TrimSpace(s)
	}
	sc := make([]rune, 0, len(sr))
	idx := make([]int, 0, len(sr)) // sc[k] 对应 sr 中的 rune 下标
	for k, r := range sr {
		if isBlank(r) {
			continue
		}
		sc = append(sc, r)
		idx = append(idx, k)
	}
	if len(sc) < len(pc) {
		return strings.TrimSpace(s)
	}
	for k := range pc {
		if sc[k] != pc[k] {
			return strings.TrimSpace(s)
		}
	}
	// 匹配结束于 sr 的 rune 下标 idx[len(pc)-1]，从下一 rune 起为正文
	return strings.TrimSpace(string(sr[idx[len(pc)-1]+1:]))
}

// parseKebulukParas p 分段解析：收集选择器命中的非空段落文本。
func parseKebulukParas(doc *goquery.Document, sel string) []string {
	var paras []string
	doc.Find(sel).Each(func(_ int, p *goquery.Selection) {
		if t := strings.TrimSpace(p.Text()); t != "" {
			paras = append(paras, t)
		}
	})
	return paras
}

// kebulukParaLen 段落总字符数，用于比较 p/br 两种解析哪边更像正文。
func kebulukParaLen(paras []string) int {
	n := 0
	for _, p := range paras {
		n += len([]rune(p))
	}
	return n
}
