package sources

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// KunnuAdapter 鲲弩小说（www.kunnu8.com，kunnu.com 会跳转到此）。
//
// 页面特征（2026-09 校准）：
//   - UTF-8 编码；
//   - 书籍页同时是完整目录，无翻页；
//   - 书籍信息在 .book-describe，正文在章节页 #nr1。
type KunnuAdapter struct{}

func (KunnuAdapter) Name() string { return "kunnu" }
func (KunnuAdapter) Domains() []string {
	return []string{"www.kunnu8.com", "kunnu8.com", "www.kunnu.com", "kunnu.com"}
}

func (a KunnuAdapter) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	var meta spider.BookMeta

	desc := doc.Find(".book-describe")
	meta.Title = strings.TrimSpace(desc.Find("h1").First().Text())
	desc.Find("p").Each(func(_ int, p *goquery.Selection) {
		t := strings.TrimSpace(p.Text())
		switch {
		case strings.HasPrefix(t, "作者"):
			meta.Author = spider.TrimLabel(t)
		case strings.HasPrefix(t, "类型"), strings.HasPrefix(t, "分类"):
			meta.Category = spider.TrimLabel(t)
		case strings.HasPrefix(t, "状态"):
			meta.Status = spider.TrimLabel(t)
		}
	})
	// 简选取第一个 .describe-html，第二个是站方的推荐位
	meta.Description = strings.TrimSpace(desc.Find(".describe-html").First().Text())
	if src, ok := doc.Find(".book-img img").First().Attr("src"); ok {
		meta.CoverURL = spider.AbsURL(pageURL, src)
	}
	if meta.Title == "" {
		return meta, nil, fmt.Errorf("书籍页解析不到书名: %s", pageURL)
	}

	// 目录在 .book-list；只保留本书路径下的链接（/fanren/NNNNN.htm 形态），
	// 过滤推荐位等其他站内链接
	bookPath := ""
	if u, err := url.Parse(pageURL); err == nil {
		bookPath = u.Path
	}
	var links []string
	doc.Find(".book-list li a").Each(func(_ int, aEl *goquery.Selection) {
		href, ok := aEl.Attr("href")
		if !ok {
			return
		}
		abs := spider.AbsURL(pageURL, href)
		if bookPath != "" {
			if u, err := url.Parse(abs); err != nil || !strings.HasPrefix(u.Path, bookPath) {
				return
			}
		}
		links = append(links, abs)
	})
	return meta, links, nil
}

// ListPageURLs 目录不分页。
func (KunnuAdapter) ListPageURLs(string) []string { return nil }

func (KunnuAdapter) ParseChapterList(string, *goquery.Document) ([]string, error) {
	return nil, nil
}

func (KunnuAdapter) ParseChapter(_ string, doc *goquery.Document) (*spider.ChapterData, error) {
	title := strings.TrimSpace(doc.Find("#nr_title").First().Text())
	if title == "" {
		return nil, fmt.Errorf("章节页解析不到标题")
	}
	data := &spider.ChapterData{Title: title}
	doc.Find("#nr1 p").Each(func(_ int, p *goquery.Selection) {
		t := strings.TrimSpace(p.Text())
		// 正文中混有「-鲲-弩-小-说…」形式的水印段落，按站点名过滤
		if t == "" || spider.IsWatermark(t, "kunnu") {
			return
		}
		data.Paragraphs = append(data.Paragraphs, t)
	})
	return data, nil
}
