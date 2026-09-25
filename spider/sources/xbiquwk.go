package sources

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// XbiquwkAdapter 笔尖中文（www.xbiquwk.com，biquwx.la 会跳转至此）。
// 笔趣阁系模板，解析逻辑见 biquge.go。
type XbiquwkAdapter struct{}

func (XbiquwkAdapter) Name() string { return "xbiquwk" }
func (XbiquwkAdapter) Domains() []string {
	return []string{"www.xbiquwk.com", "xbiquwk.com", "www.biquwx.la", "biquwx.la"}
}

func (a XbiquwkAdapter) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	meta := biqugeMeta(doc)
	if meta.Title == "" {
		return meta, nil, fmt.Errorf("书籍页解析不到书名: %s", pageURL)
	}
	return meta, biqugeLinks(pageURL, doc), nil
}

// ListPageURLs 目录不分页（单页全量）。
func (XbiquwkAdapter) ListPageURLs(string) []string { return nil }

func (XbiquwkAdapter) ParseChapterList(string, *goquery.Document) ([]string, error) {
	return nil, nil
}

func (a XbiquwkAdapter) ParseChapter(_ string, doc *goquery.Document) (*spider.ChapterData, error) {
	return biqugeChapter(doc, "xbiquwk")
}
