package sources

import (
	"errors"
	"fmt"

	"github.com/PuerkitoBio/goquery"

	"treeNovel/global"
	"treeNovel/models"
	"treeNovel/spider"
)

// BiqugeSelectors 笔趣阁系默认解析规则：自定义站点选择器留空时的缺省值。
var BiqugeSelectors = Selectors{List: "#list dd a", Title: "h1", Content: "#content", Mode: "br"}

// CustomAdapter 页面添加的自定义站点：解析规则存在 models.SiteStatus
// 行里，按笔趣阁系模板解析（默认选择器见 BiqugeSelectors）。
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
// 内置站点被删除时透传 ErrSiteDeleted，调用方拿到明确原因而不是「未知适配器」。
func GetAny(name string) (spider.SiteAdapter, error) {
	ad, err := Get(name)
	if err == nil {
		return ad, nil
	}
	if errors.Is(err, ErrSiteDeleted) {
		return nil, err
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

func (a CustomAdapter) Domains() []string { return splitDomains(a.Row.Domains) }

func (a CustomAdapter) listSelOrDefault() string {
	if a.Row.ListSel != "" {
		return a.Row.ListSel
	}
	return BiqugeSelectors.List
}

func (a CustomAdapter) titleSelOrDefault() string {
	if a.Row.TitleSel != "" {
		return a.Row.TitleSel
	}
	return BiqugeSelectors.Title
}

func (a CustomAdapter) contentSelOrDefault() string {
	if a.Row.ContentSel != "" {
		return a.Row.ContentSel
	}
	return BiqugeSelectors.Content
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
	return parseChapterSel(doc, a.titleSelOrDefault(), a.contentSelOrDefault(),
		a.Row.ContentAlignment, a.Row.Watermark)
}
