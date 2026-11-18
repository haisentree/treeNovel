package sources

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"

	"treeNovel/spider"
)

// fetchPageDoc 适配器内主动跟进页面时使用（着陆页跟进「目录」、
// 章节内翻页跟进「下一页」）：带 UA 抓取并按响应编码转成 goquery
// 文档。引擎经 colly 的常规抓取不走这里。
func fetchPageDoc(pageURL string) (*goquery.Document, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", spider.DefaultUA)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	rd, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		rd = io.NopCloser(resp.Body)
	}
	return goquery.NewDocumentFromReader(rd)
}
