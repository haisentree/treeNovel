package sources

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/global"
	"treeNovel/models"
	"treeNovel/spider"
)

// CustomAdapter 页面添加的自定义站点：解析规则存在 models.SiteStatus
// 行里，按笔趣阁系模板解析（默认选择器见各 selOrDefault）。
// 与 biquge.go 共用解析工具。
type CustomAdapter struct {
	Row models.SiteStatus
}

// FromSite 由库行构建适配器。
func FromSite(row models.SiteStatus) spider.SiteAdapter {
	return CustomAdapter{Row: row}
}

// GetAny 统一的适配器解析：先查代码注册表，再查库里的自定义站点。
// 任务创建、worker、CLI、健康检查都用它。
func GetAny(name string) (spider.SiteAdapter, error) {
	if ad, err := Get(name); err == nil {
		return ad, nil
	}
	if global.DB != nil {
		var row models.SiteStatus
		if err := global.DB.Where("source = ? AND custom = ?", name, true).
			First(&row).Error; err == nil {
			return FromSite(row), nil
		}
	}
	return nil, fmt.Errorf("未知适配器 %q，可用: %v", name, Names())
}

// AllAny 全部可用适配器：代码注册表 + 库中的自定义站点。
func AllAny() ([]spider.SiteAdapter, error) {
	out := All()
	if global.DB != nil {
		var rows []models.SiteStatus
		if err := global.DB.Where("custom = ?", true).Find(&rows).Error; err != nil {
			return out, err
		}
		for _, row := range rows {
			out = append(out, FromSite(row))
		}
	}
	return out, nil
}

func (a CustomAdapter) Name() string { return a.Row.Source }

func (a CustomAdapter) Domains() []string {
	var domains []string
	for _, d := range strings.FieldsFunc(a.Row.Domains, func(r rune) bool {
		return r == ',' || r == '，' || r == ' '
	}) {
		if d = strings.TrimSpace(d); d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

func (a CustomAdapter) listSelOrDefault() string {
	if a.Row.ListSel != "" {
		return a.Row.ListSel
	}
	return "#list dd a"
}

func (a CustomAdapter) titleSelOrDefault() string {
	if a.Row.TitleSel != "" {
		return a.Row.TitleSel
	}
	return "h1"
}

func (a CustomAdapter) contentSelOrDefault() string {
	if a.Row.ContentSel != "" {
		return a.Row.ContentSel
	}
	return "#content"
}

func (a CustomAdapter) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	meta := biqugeMeta(doc)
	if meta.Title == "" {
		return meta, nil, fmt.Errorf("书籍页解析不到书名（og:title 缺失），请确认该站是否为笔趣阁系模板: %s", pageURL)
	}
	return meta, biqugeLinksSel(pageURL, doc, a.listSelOrDefault()), nil
}

// ListPageURLs 自定义站点按单页全量目录处理（不支持翻页模板）。
func (CustomAdapter) ListPageURLs(string) []string { return nil }

func (a CustomAdapter) ParseChapterList(pageURL string, doc *goquery.Document) ([]string, error) {
	return biqugeLinksSel(pageURL, doc, a.listSelOrDefault()), nil
}

func (a CustomAdapter) ParseChapter(_ string, doc *goquery.Document) (*spider.ChapterData, error) {
	title := strings.TrimSpace(doc.Find(a.titleSelOrDefault()).First().Text())
	if title == "" {
		return nil, fmt.Errorf("章节页解析不到标题（选择器 %q 未命中）", a.titleSelOrDefault())
	}
	data := &spider.ChapterData{Title: title}

	content := doc.Find(a.contentSelOrDefault())
	var paras []string
	if a.Row.ContentAlignment == "p" {
		content.Each(func(_ int, p *goquery.Selection) {
			if t := strings.TrimSpace(p.Text()); t != "" {
				paras = append(paras, t)
			}
		})
	} else {
		paras = spider.BrParagraphs(content)
	}
	for _, p := range paras {
		if p == "" || (a.Row.Watermark != "" && spider.IsWatermark(p, a.Row.Watermark)) {
			continue
		}
		data.Paragraphs = append(data.Paragraphs, p)
	}
	if len(data.Paragraphs) == 0 {
		return nil, fmt.Errorf("正文为空（选择器 %q 未命中）", a.contentSelOrDefault())
	}
	return data, nil
}
