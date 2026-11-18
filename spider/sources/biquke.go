package sources

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// BiqukeAdapter 零点看书（www.biquke.info）。
//
// 页面特征（2026-10 校准）：
//   - UTF-8 编码；
//   - 书籍元信息在 og / og:novel:* meta，齐全；
//   - 目录分页：书籍页（/lNNN-MMM/）只含前 20 章，在 ul.section-list
//     的最后一个（其前一个是「最新章节」倒序推荐位，不可取），
//     后续页为 index_2.html、index_3.html…；越界页码站点不报 404
//     而是重复返回已有内容，靠引擎的链接去重终止翻页；
//   - 章节页 h1.title 为标题，正文在 #content（<br> 分段），首段是
//     「标题 (第N/M页)」翻页标记；长章节内部还有 _2.html、_3.html
//     分页，由「下一页」链接串接，ParseChapter 会自动跟进拼接。
type BiqukeAdapter struct{}

func (BiqukeAdapter) Name() string { return "biquke" }

func (BiqukeAdapter) Domains() []string {
	return []string{"www.biquke.info", "biquke.info"}
}

func (BiqukeAdapter) defaultSelectors() Selectors {
	return Selectors{List: "ul.section-list li a", Title: "h1.title", Content: "#content", Mode: "br", Watermark: "biquke"}
}

func (a BiqukeAdapter) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	meta := biqugeMeta(doc)
	if meta.Title == "" {
		return meta, nil, fmt.Errorf("书籍页解析不到书名: %s", pageURL)
	}
	return meta, biqukeTocLinks(pageURL, doc), nil
}

// biqukeTocLinks 取「正文」目录段的章节链接：页面里 ul.section-list
// 有两个（最新章节倒序位 + 正文正序位），取最后一个。
func biqukeTocLinks(pageURL string, doc *goquery.Document) []string {
	return sectionLinks(pageURL, doc.Find("ul.section-list").Last().Find("a"))
}

// tocPageCap 目录分页候选页数上限（单页 20 章，即 4000 章）。
// 越界页站点会重复返回已有内容，引擎按「无新增章节」提前停止，
// 因此多生成的候选页每本只多花一次请求。
const tocPageCap = 200

// ListPageURLs 预生成 index_2..index_200 候选目录页。
func (BiqukeAdapter) ListPageURLs(bookURL string) []string {
	if !strings.HasSuffix(bookURL, "/") {
		bookURL += "/"
	}
	urls := make([]string, 0, tocPageCap-1)
	for i := 2; i <= tocPageCap; i++ {
		urls = append(urls, bookURL+"index_"+strconv.Itoa(i)+".html")
	}
	return urls
}

func (a BiqukeAdapter) ParseChapterList(pageURL string, doc *goquery.Document) ([]string, error) {
	return biqukeTocLinks(pageURL, doc), nil
}

func (a BiqukeAdapter) ParseChapter(pageURL string, doc *goquery.Document) (*spider.ChapterData, error) {
	title := strings.TrimSpace(doc.Find("h1.title").First().Text())
	if title == "" {
		return nil, fmt.Errorf("章节页解析不到标题: %s", pageURL)
	}
	data := &spider.ChapterData{Title: title}
	visited := map[string]bool{pageURL: true}
	for cur := doc; cur != nil; {
		biqukeAppendContent(data, cur, title)
		nextURL := biqukeNextPage(pageURL, cur)
		if nextURL == "" || visited[nextURL] {
			break
		}
		visited[nextURL] = true
		nextDoc, err := fetchPageDoc(nextURL)
		if err != nil {
			return nil, fmt.Errorf("跟进章节分页失败 %s: %w", nextURL, err)
		}
		pageURL, cur = nextURL, nextDoc
	}
	if len(data.Paragraphs) == 0 {
		return nil, fmt.Errorf("正文为空: %s", title)
	}
	return data, nil
}

// biqukeAppendContent 把一页 #content 的段落拼进 data，
// 跳过「标题 (第N/M页)」翻页标记段与页面残留的孤立标点（如模板
// 泄漏出的单个「>」）。
func biqukeAppendContent(data *spider.ChapterData, doc *goquery.Document, title string) {
	for _, p := range spider.BrParagraphs(doc.Find("#content")) {
		if biqukePageMarkRe.MatchString(p) && strings.HasPrefix(p, title) {
			continue
		}
		if biqukeJunkRe.MatchString(p) {
			continue
		}
		data.Paragraphs = append(data.Paragraphs, p)
	}
}

// biqukeNextPage 找「下一页」链接（章节内分页），没有则返回空串。
func biqukeNextPage(pageURL string, doc *goquery.Document) string {
	next := ""
	doc.Find("a").Each(func(_ int, aEl *goquery.Selection) {
		if next != "" || strings.TrimSpace(aEl.Text()) != "下一页" {
			return
		}
		if href := aEl.AttrOr("href", ""); href != "" {
			next = spider.AbsURL(pageURL, href)
		}
	})
	return next
}

// biqukePageMarkRe 章节内翻页标记「(第1/2页)」。
var biqukePageMarkRe = regexp.MustCompile(`\(第\d+/\d+页\)`)

// biqukeJunkRe 纯 ASCII 标点/符号组成的残留段（不含任何文字数字）。
var biqukeJunkRe = regexp.MustCompile(`^[[:punct:]]+$`)
