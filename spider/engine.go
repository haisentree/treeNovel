package spider

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"gorm.io/gorm"

	"treeNovel/models"
)

// Options 抓取选项。
type Options struct {
	// Delay 同站两次请求的最小间隔（礼貌限速），实际会叠加同幅度的随机抖动
	Delay time.Duration
	// MaxChapters 最多抓取的章节数，0 表示不限（试爬时可用来小规模验证）
	MaxChapters int
	// UserAgent 请求头，空则用默认值
	UserAgent string
}

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func (o *Options) fill() {
	if o.Delay <= 0 {
		o.Delay = time.Second
	}
	if o.UserAgent == "" {
		o.UserAgent = defaultUA
	}
}

// CrawlBook 抓取一本书：书籍页 -> 目录翻页 -> 逐章正文页。
//
// 行为约定：
//   - 书籍按「来源+作者+标题」去重，已存在时复用记录并做增量更新
//     （只补缺失章节），所以中断后重跑即可续爬，连载书也可重复执行追更；
//   - 章节按「标题+书籍ID」去重；
//   - 段落沿用旧库的 "$$" 分隔约定入库，展示端按 "$$" 切回段落；
//   - 返回值：书籍 ID（便于调用方关联）与本次新入库的章节数。
func CrawlBook(db *gorm.DB, ad SiteAdapter, bookURL string, opt Options) (bookID uint, saved int, err error) {
	opt.fill()

	c := colly.NewCollector(
		colly.UserAgent(opt.UserAgent),
		colly.AllowedDomains(ad.Domains()...),
	)
	// 慢站点大页面（整页目录）容易超过 colly 默认超时；
	// 且部分站点对 Go 默认 TLS 握手响应极慢，统一放宽
	c.SetRequestTimeout(60 * time.Second)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSHandshakeTimeout = 60 * time.Second
	tr.ResponseHeaderTimeout = 60 * time.Second
	c.WithTransport(tr)
	if err := c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Delay:       opt.Delay,
		RandomDelay: opt.Delay,
	}); err != nil {
		return 0, 0, fmt.Errorf("设置限速: %w", err)
	}

	// Colly 默认同步访问：Visit 返回时回调已执行完，
	// 因此每次响应把解析好的 DOM 存起来，Visit 后交给适配器。
	var lastDoc *goquery.Document
	c.OnResponse(func(r *colly.Response) {
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			log.Printf("[spider] 解析页面失败 %s: %v", r.Request.URL, err)
		}
		lastDoc = doc
	})
	visit := func(pageURL string) (*goquery.Document, error) {
		lastDoc = nil
		if err := c.Visit(pageURL); err != nil {
			return nil, fmt.Errorf("访问 %s: %w", pageURL, err)
		}
		if lastDoc == nil {
			return nil, fmt.Errorf("访问 %s: 未取得页面内容", pageURL)
		}
		return lastDoc, nil
	}

	// 1. 书籍页
	bookDoc, err := visit(bookURL)
	if err != nil {
		return 0, 0, err
	}
	meta, firstPageLinks, err := ad.ParseBook(bookURL, bookDoc)
	if err != nil {
		return 0, 0, fmt.Errorf("解析书籍页: %w", err)
	}

	// 2. 查找或创建书籍记录（同来源+作者+标题视为同一本）
	var article models.Article
	if err := db.Where("author = ? AND source = ? AND title = ?",
		meta.Author, ad.Name(), meta.Title).First(&article).Error; err == nil {
		log.Printf("[spider] 《%s》已在库中(id=%d)，增量更新", meta.Title, article.ID)
	} else {
		article = models.Article{
			Author:      meta.Author,
			Title:       meta.Title,
			Description: meta.Description,
			Category:    meta.Category,
			CoverURL:    meta.CoverURL,
			Status:      meta.Status,
			Source:      ad.Name(),
		}
		if err := db.Create(&article).Error; err != nil {
			return 0, 0, fmt.Errorf("写入书籍: %w", err)
		}
		log.Printf("[spider] 新书入库 《%s》(id=%d)", meta.Title, article.ID)
	}

	// 3. 收集目录链接：第一页 + 翻页，直到访问失败或没有新链接
	var chapterURLs []string
	seen := make(map[string]bool)
	addLinks := func(links []string) int {
		n := 0
		for _, l := range links {
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			chapterURLs = append(chapterURLs, l)
			n++
		}
		return n
	}
	addLinks(firstPageLinks)
	for _, listURL := range ad.ListPageURLs(bookURL) {
		doc, err := visit(listURL)
		if err != nil {
			// 翻过最后一页后站点通常返回 404，视为目录结束
			log.Printf("[spider] 目录翻页结束于 %s", listURL)
			break
		}
		links, err := ad.ParseChapterList(listURL, doc)
		if err != nil {
			log.Printf("[spider] 解析目录页失败 %s: %v", listURL, err)
			break
		}
		if addLinks(links) == 0 {
			log.Printf("[spider] 目录页无新增章节，视为已到末页: %s", listURL)
			break
		}
	}
	log.Printf("[spider] 《%s》目录共 %d 章", meta.Title, len(chapterURLs))

	if opt.MaxChapters > 0 && len(chapterURLs) > opt.MaxChapters {
		chapterURLs = chapterURLs[:opt.MaxChapters]
		log.Printf("[spider] 试爬模式，只抓前 %d 章", opt.MaxChapters)
	}

	// 4. 逐章抓取入库
	for i, chapterURL := range chapterURLs {
		doc, err := visit(chapterURL)
		if err != nil {
			log.Printf("[spider] 跳过章节 %s: %v", chapterURL, err)
			continue
		}
		data, err := ad.ParseChapter(chapterURL, doc)
		if err != nil {
			log.Printf("[spider] 跳过章节 %s: %v", chapterURL, err)
			continue
		}

		var exist models.Chapter
		if err := db.Where("title = ? AND article_id = ?", data.Title, article.ID).
			First(&exist).Error; err == nil {
			continue
		}
		chapter := models.Chapter{
			ArticleID: article.ID,
			Title:     data.Title,
			Content:   strings.Join(data.Paragraphs, "$$"),
		}
		if err := db.Create(&chapter).Error; err != nil {
			log.Printf("[spider] 章节入库失败 《%s》: %v", data.Title, err)
			continue
		}
		saved++
		if saved%50 == 0 {
			log.Printf("[spider] 进度 %d/%d", i+1, len(chapterURLs))
		}
	}
	log.Printf("[spider] 《%s》完成，本次新入库 %d 章", meta.Title, saved)
	return article.ID, saved, nil
}
