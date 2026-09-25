package sources

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// IbiquguAdapter 香书小说（www.ibiqugu.net，xbiquge.la 会跳转至此）。
// 笔趣阁系模板，解析逻辑见 biquge.go。
type IbiquguAdapter struct{}

func (IbiquguAdapter) Name() string { return "ibiqugu" }
func (IbiquguAdapter) Domains() []string {
	return []string{"www.ibiqugu.net", "ibiqugu.net", "www.xbiquge.la", "xbiquge.la"}
}

func (a IbiquguAdapter) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	meta := biqugeMeta(doc)
	if meta.Title == "" {
		return meta, nil, fmt.Errorf("书籍页解析不到书名: %s", pageURL)
	}
	return meta, biqugeLinks(pageURL, doc), nil
}

// ListPageURLs 目录不分页（单页全量）。
func (IbiquguAdapter) ListPageURLs(string) []string { return nil }

func (IbiquguAdapter) ParseChapterList(string, *goquery.Document) ([]string, error) {
	return nil, nil
}

func (a IbiquguAdapter) ParseChapter(_ string, doc *goquery.Document) (*spider.ChapterData, error) {
	// 正文中混有「最新网址：www.ibiqugu.net」提示段，按站点名过滤
	return biqugeChapter(doc, "ibiqugu")
}
