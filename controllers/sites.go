package controllers

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html/charset"

	"treeNovel/models"
	"treeNovel/spider"
	"treeNovel/spider/sources"
)

// AdminSites 站点管理页。
func AdminSites(c *gin.Context) {
	c.HTML(http.StatusOK, "sites.html", gin.H{"Sites": models.FindAllSites()})
}

// sourceNameRe 自定义站点名：小写字母/数字/下划线/中划线。
var sourceNameRe = regexp.MustCompile(`^[a-z0-9_-]{2,24}$`)

// AdminAddSite 添加自定义站点：解析规则存库，适配器运行时从库构建。
// 若填写了「验证书籍页 URL」，会先真实抓取并用所填选择器试解析，
// 解析失败（书名或章节目录为空）则拒绝保存——避免存进一个坏站点。
func AdminAddSite(c *gin.Context) {
	row := models.SiteStatus{
		Source:           strings.TrimSpace(c.PostForm("source")),
		Domains:          strings.TrimSpace(c.PostForm("domains")),
		ListSel:          strings.TrimSpace(c.PostForm("list_sel")),
		TitleSel:         strings.TrimSpace(c.PostForm("title_sel")),
		ContentSel:       strings.TrimSpace(c.PostForm("content_sel")),
		ContentAlignment: strings.TrimSpace(c.PostForm("content_mode")),
		Watermark:        strings.TrimSpace(c.PostForm("watermark")),
	}
	checkURL := strings.TrimSpace(c.PostForm("check_url"))

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "sites.html", gin.H{
			"Sites": models.FindAllSites(),
			"Error": msg,
			"Form": gin.H{
				"Source": row.Source, "Domains": row.Domains,
				"ListSel": row.ListSel, "TitleSel": row.TitleSel,
				"ContentSel": row.ContentSel, "Mode": row.ContentAlignment,
				"Watermark": row.Watermark, "CheckURL": checkURL,
			},
		})
	}

	if !sourceNameRe.MatchString(row.Source) {
		renderErr("站点标识需为 2~24 位小写字母/数字/下划线/中划线")
		return
	}
	if row.Domains == "" {
		renderErr("域名不能为空（多个用逗号分隔）")
		return
	}
	if row.ContentAlignment != "" && row.ContentAlignment != "br" && row.ContentAlignment != "p" {
		renderErr("正文分段只能是 br 或 p")
		return
	}
	// 有验证 URL 就必须试解析通过，保证存进去的站点可用
	if checkURL != "" {
		if msg := tryParseBook(row, checkURL); msg != "" {
			renderErr("验证书籍页失败：" + msg)
			return
		}
	}
	if err := models.CreateCustomSite(row); err != nil {
		renderErr(err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/sites")
}

// tryParseBook 用待存的选择器真实抓取并解析一页书籍页，
// 返回空串表示验证通过，否则返回错误说明。
func tryParseBook(row models.SiteStatus, bookURL string) string {
	req, err := http.NewRequest(http.MethodGet, bookURL, nil)
	if err != nil {
		return err.Error()
	}
	req.Header.Set("User-Agent", spiderDefaultUA)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "HTTP " + resp.Status
	}
	rd, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		rd = io.NopCloser(resp.Body)
	}
	doc, err := goquery.NewDocumentFromReader(rd)
	if err != nil {
		return "页面解析失败: " + err.Error()
	}
	ad := sources.FromSite(row)
	meta, links, err := ad.ParseBook(bookURL, doc)
	if err != nil {
		return err.Error()
	}
	if len(links) == 0 {
		return "目录选择器未解析到任何章节链接"
	}
	_ = meta
	return ""
}

// AdminDeleteSite 删除自定义站点（内置站点的行不可删）。
func AdminDeleteSite(c *gin.Context) {
	_ = models.DeleteCustomSite(c.Param("source"))
	c.Redirect(http.StatusFound, "/admin/sites")
}

// AdminCheckSite 探测单个站点并回显结果。
func AdminCheckSite(c *gin.Context) {
	source := c.Param("source")
	if ad, err := sources.GetAny(source); err == nil {
		r := spider.CheckSite(ad)
		models.SaveSiteCheck(source, r.OK, r.StatusCode, r.LatencyMS, r.Err)
	}
	c.Redirect(http.StatusFound, "/admin/sites")
}

// AdminCheckAllSites 并发探测全部站点（内置 + 自定义，
// 单站 12s 超时，总量约 12s 封顶）。
func AdminCheckAllSites(c *gin.Context) {
	ads, err := sources.AllAny()
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/sites")
		return
	}
	var wg sync.WaitGroup
	for _, ad := range ads {
		wg.Add(1)
		go func(ad spider.SiteAdapter) {
			defer wg.Done()
			r := spider.CheckSite(ad)
			models.SaveSiteCheck(ad.Name(), r.OK, r.StatusCode, r.LatencyMS, r.Err)
		}(ad)
	}
	wg.Wait()
	c.Redirect(http.StatusFound, "/admin/sites")
}

// AdminToggleSite 启用/停用站点。
func AdminToggleSite(c *gin.Context) {
	models.ToggleSiteEnabled(c.Param("source"))
	c.Redirect(http.StatusFound, "/admin/sites")
}

// spiderDefaultUA 与爬虫引擎一致的 UA。
const spiderDefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
