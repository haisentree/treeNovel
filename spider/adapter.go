// Package spider 实现多站点小说爬虫。
//
// 结构分两层：
//   - engine.go：通用抓取引擎（Colly），负责限速、UA、目录翻页终止、
//     链接去重、入库与增量更新，不关心具体站点的页面结构；
//   - sources/：站点适配器，每个目标站点实现 SiteAdapter 接口，
//     只做「DOM -> 数据」的解析。
//
// 新增一个站点的步骤：在 sources/ 下新建文件实现 SiteAdapter，
// 注册进 sources.all，即完成接入。
package spider

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
)

// BookMeta 书籍元信息，由适配器从书籍页解析。
type BookMeta struct {
	Title       string
	Author      string
	Category    string
	Status      string
	Description string
	CoverURL    string
}

// ChapterData 章节内容，由适配器从章节页解析。
type ChapterData struct {
	Title      string
	Paragraphs []string
}

// SiteAdapter 目标站点适配器。
//
// 注意：goquery 按 UTF-8 解析页面，GBK 编码的站点需要适配器内
// 自行转码后再解析（目前接入的站点均为 UTF-8）。
type SiteAdapter interface {
	// Name 站点唯一标识，写入 Article.Source
	Name() string

	// Domains 引擎允许访问的域名（含跳转目标域名）
	Domains() []string

	// ParseBook 解析书籍页（通常同时是目录第一页），
	// 返回元信息和章节链接（尽量保持目录顺序）。
	ParseBook(pageURL string, doc *goquery.Document) (BookMeta, []string, error)

	// ListPageURLs 返回除第一页外的目录页地址；
	// 目录不分页的站点返回 nil。
	ListPageURLs(bookURL string) []string

	// ParseChapterList 解析后续目录页，返回章节链接。
	ParseChapterList(pageURL string, doc *goquery.Document) ([]string, error)

	// ParseChapter 解析章节正文页。
	ParseChapter(pageURL string, doc *goquery.Document) (*ChapterData, error)
}

// AbsURL 以 pageURL 为基准，把可能相对的 href 解析成绝对地址。
func AbsURL(pageURL, href string) string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

// TrimLabel 去掉「作者：」「状态：」这类标签前缀（全角/半角冒号都兼容）。
// 注意 IndexAny 返回字节下标，必须按 rune 宽度推进，否则会把
// 全角冒号（3 字节）从中间切开产生非法 UTF-8。
func TrimLabel(s string) string {
	if i := strings.IndexAny(s, "：:"); i >= 0 {
		_, size := utf8.DecodeRuneInString(s[i:])
		return strings.TrimSpace(s[i+size:])
	}
	return strings.TrimSpace(s)
}

// IsWatermark 判断段落是否为站方水印，如「-鲲-弩-小-说w ww ^ k u n n u^ c o m.」。
// 做法：只保留字母/数字（含中文）后检查是否包含站点名，容忍各种混淆插入。
func IsWatermark(paragraph, siteName string) bool {
	var b strings.Builder
	for _, r := range paragraph {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	s := b.String()
	return strings.Contains(s, strings.ToLower(siteName))
}
