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
	// 首尾广告段多含站名「斗破苍穹网」，按水印过滤
	data, err := parseChapterSel(doc, "h1", "#contentn p", "p", "斗破苍穹网")
	if err != nil {
		return nil, err
	}
	// 「如果看累了，收藏一下本页面」提示段不含站名，按文案特征过滤
	out := data.Paragraphs[:0]
	for _, p := range data.Paragraphs {
		if strings.Contains(p, "收藏一下本页面") {
			continue
		}
		out = append(out, p)
	}
	data.Paragraphs = out
	return data, nil
}
