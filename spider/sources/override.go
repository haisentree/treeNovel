package sources

import (
	"github.com/PuerkitoBio/goquery"

	"treeNovel/spider"
)

// Selectors 一组解析规则：目录/标题/正文选择器、分段方式与水印关键词。
// 自定义站点整组存库；内置站点作为覆盖值存库，空字段用代码默认。
type Selectors struct {
	List      string // 目录章节链接选择器
	Title     string // 章节标题选择器
	Content   string // 正文容器（br 分段）或段落元素（p 分段）选择器
	Mode      string // br（默认）| p
	Watermark string // 水印关键词，命中的段落被过滤；空 = 不过滤
}

func (s Selectors) anySet() bool {
	return s.List != "" || s.Title != "" || s.Content != "" || s.Mode != "" || s.Watermark != ""
}

// effective 逐字段合并：覆盖值为空时取默认值。
func (s Selectors) effective(def Selectors) Selectors {
	if s.List == "" {
		s.List = def.List
	}
	if s.Title == "" {
		s.Title = def.Title
	}
	if s.Content == "" {
		s.Content = def.Content
	}
	if s.Mode == "" {
		s.Mode = def.Mode
	}
	if s.Watermark == "" {
		s.Watermark = def.Watermark
	}
	return s
}

// defaultSelProvider 内置适配器声明自己的默认解析规则，
// 供覆盖包装与编辑页占位提示使用。
type defaultSelProvider interface {
	defaultSelectors() Selectors
}

// DefaultSelectorsByName 内置站点的默认解析规则（编辑页占位提示用）。
func DefaultSelectorsByName(name string) (Selectors, bool) {
	for _, a := range all {
		if a.Name() != name {
			continue
		}
		if p, ok := a.(defaultSelProvider); ok {
			return p.defaultSelectors(), true
		}
		return Selectors{}, true // 内置但无默认声明：不开放选择器覆盖
	}
	return Selectors{}, false
}

// NewSelectorOverride 用覆盖规则包装适配器；适配器无默认规则声明时
// 原样返回（第二个返回值 false 表示未包装）。
func NewSelectorOverride(ad spider.SiteAdapter, ov Selectors) (spider.SiteAdapter, bool) {
	p, ok := ad.(defaultSelProvider)
	if !ok || !ov.anySet() {
		return ad, false
	}
	return SelectorOverride{SiteAdapter: ad, ov: ov, def: p.defaultSelectors()}, true
}

// SelectorOverride 库中/表单提交的解析规则覆盖：书名等元信息仍由
// 原适配器解析（各站模板不同），目录与章节正文按覆盖后的选择器解析。
type SelectorOverride struct {
	spider.SiteAdapter
	ov  Selectors
	def Selectors
}

func (o SelectorOverride) sel() Selectors { return o.ov.effective(o.def) }

// defaultSelectors 透传底层适配器的默认值：允许覆盖包装再叠加
// （外层覆盖优先，空字段落到底层默认）。
func (o SelectorOverride) defaultSelectors() Selectors {
	if p, ok := o.SiteAdapter.(defaultSelProvider); ok {
		return p.defaultSelectors()
	}
	return o.def
}

func (o SelectorOverride) ParseBook(pageURL string, doc *goquery.Document) (spider.BookMeta, []string, error) {
	meta, _, err := o.SiteAdapter.ParseBook(pageURL, doc)
	if err != nil {
		return meta, nil, err
	}
	return meta, biqugeLinksSel(pageURL, doc, o.sel().List), nil
}

func (o SelectorOverride) ParseChapterList(pageURL string, doc *goquery.Document) ([]string, error) {
	return biqugeLinksSel(pageURL, doc, o.sel().List), nil
}

func (o SelectorOverride) ParseChapter(_ string, doc *goquery.Document) (*spider.ChapterData, error) {
	s := o.sel()
	return parseChapterSel(doc, s.Title, s.Content, s.Mode, s.Watermark)
}
