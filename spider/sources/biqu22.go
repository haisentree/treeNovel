package sources

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// Biqu22Adapter 新笔趣阁（www.22biqu.com），本项目最初的数据来源。
//
// ⚠️ 2026-09 探测时该域名已被注册商停放，站点不可用。
// 保留此适配器的目的：
//  1. 存量数据 Article.Source == "22biqu"，保留同名适配器便于溯源；
//  2. 作为「目录翻页」模式的参考实现（kunnu 是无翻页的对照例子）。
//
// 选择器由旧版爬虫（third/spider，已删除）的绝对 XPath 翻译成
// nth-child 链，未经过线上验证，若站点复活需先核对。
type Biqu22Adapter struct{}

func (Biqu22Adapter) Name() string { return "22biqu" }
func (Biqu22Adapter) Domains() []string {
	return []string{"www.22biqu.com", "22biqu.com"}
}

// 信息区与目录区的根：body 下第 4 个 div
const (
	biqu22InfoSel    = "body > div:nth-child(4) > div:first-child > div > div > div:nth-child(2) > div:first-child"
	biqu22DescSel    = "body > div:nth-child(4) > div:first-child > div > div > div:nth-child(2) > div:nth-child(2)"
	biqu22CoverSel   = "body > div:nth-child(4) > div:first-child > div > div > div:first-child img"
	biqu22ListSel    = "body > div:nth-child(4) > div:nth-child(2) > div:first-child > div:nth-child(2) ul li a"
	biqu22ChTitleSel = "#container h1"
	biqu22ContentSel = "body > div:nth-child(4) > div:first-child > div > div > div:nth-child(2) > div:nth-child(3) p"
)

func (Biqu22Adapter) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	var meta spider.BookMeta

	info := doc.Find(biqu22InfoSel)
	meta.Title = strings.TrimSpace(info.Find("h1").First().Text())
	// 元信息是「作者：xxx」「分类：xxx」「状态：xxx」三行 p
	info.Find("div p").Each(func(_ int, p *goquery.Selection) {
		t := strings.TrimSpace(p.Text())
		switch {
		case strings.HasPrefix(t, "作者"):
			meta.Author = spider.TrimLabel(t)
		case strings.HasPrefix(t, "分类"), strings.HasPrefix(t, "类型"):
			meta.Category = spider.TrimLabel(t)
		case strings.HasPrefix(t, "状态"):
			meta.Status = spider.TrimLabel(t)
		}
	})
	meta.Description = strings.TrimSpace(doc.Find(biqu22DescSel).First().Text())
	if src, ok := doc.Find(biqu22CoverSel).First().Attr("src"); ok {
		meta.CoverURL = spider.AbsURL(pageURL, src)
	}
	if meta.Title == "" {
		return meta, nil, fmt.Errorf("书籍页解析不到书名: %s", pageURL)
	}

	var links []string
	doc.Find(biqu22ListSel).Each(func(_ int, aEl *goquery.Selection) {
		if href, ok := aEl.Attr("href"); ok {
			links = append(links, spider.AbsURL(pageURL, href))
		}
	})
	return meta, links, nil
}

// ListPageURLs 该站目录按 url + "N/" 分页（第 2 页起）。
// 目录页数未知，给一个足够大的上限，由引擎在「无新章节/访问失败」时终止。
func (Biqu22Adapter) ListPageURLs(bookURL string) []string {
	var urls []string
	for i := 2; i <= 500; i++ {
		urls = append(urls, strings.TrimSuffix(bookURL, "/")+"/"+strconv.Itoa(i)+"/")
	}
	return urls
}

func (Biqu22Adapter) ParseChapterList(pageURL string, doc *goquery.Document) ([]string, error) {
	var links []string
	doc.Find(biqu22ListSel).Each(func(_ int, aEl *goquery.Selection) {
		if href, ok := aEl.Attr("href"); ok {
			links = append(links, spider.AbsURL(pageURL, href))
		}
	})
	return links, nil
}

func (Biqu22Adapter) ParseChapter(_ string, doc *goquery.Document) (*spider.ChapterData, error) {
	title := strings.TrimSpace(doc.Find(biqu22ChTitleSel).First().Text())
	if title == "" {
		return nil, fmt.Errorf("章节页解析不到标题")
	}
	data := &spider.ChapterData{Title: title}
	doc.Find(biqu22ContentSel).Each(func(_ int, p *goquery.Selection) {
		if t := strings.TrimSpace(p.Text()); t != "" {
			data.Paragraphs = append(data.Paragraphs, t)
		}
	})
	return data, nil
}
