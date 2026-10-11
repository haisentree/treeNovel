package controllers

import (
	"errors"
	"fmt"
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
// 若填写了「验证书籍页 URL」，会先真实抓取并试解析书籍页与第一章，
// 失败则拒绝保存——避免存进一个坏站点。
func AdminAddSite(c *gin.Context) {
	row := models.SiteStatus{
		Source:           strings.TrimSpace(c.PostForm("source")),
		Domains:          strings.TrimSpace(c.PostForm("domains")),
		ListSel:          strings.TrimSpace(c.PostForm("list_sel")),
		TitleSel:         strings.TrimSpace(c.PostForm("title_sel")),
		ContentSel:       strings.TrimSpace(c.PostForm("content_sel")),
		ContentAlignment: strings.TrimSpace(c.PostForm("content_mode")),
		Watermark:        strings.TrimSpace(c.PostForm("watermark")),
		CheckURL:         strings.TrimSpace(c.PostForm("check_url")),
	}

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "sites.html", gin.H{
			"Sites":    models.FindAllSites(),
			"Error":    msg,
			"Form":     addFormH(row),
			"Defaults": sources.BiqugeSelectors,
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
	if !validContentMode(row.ContentAlignment) {
		renderErr("正文分段只能是 br 或 p")
		return
	}
	// 有验证 URL 就必须试解析通过，保证存进去的站点可用
	if row.CheckURL != "" {
		if _, err := probeBook(sources.FromSite(row), row.CheckURL); err != nil {
			renderErr("验证书籍页失败：" + err.Error())
			return
		}
	}
	if err := models.CreateCustomSite(row); err != nil {
		renderErr(err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/sites")
}

// validContentMode 正文分段取值校验：空串按默认 br 处理。
func validContentMode(mode string) bool {
	return mode == "" || mode == "br" || mode == "p"
}

// addFormH 添加站点页出错回填用（模板按 index .Form "X" 取值）。
func addFormH(row models.SiteStatus) gin.H {
	return gin.H{
		"Source": row.Source, "Domains": row.Domains,
		"ListSel": row.ListSel, "TitleSel": row.TitleSel,
		"ContentSel": row.ContentSel, "Mode": row.ContentAlignment,
		"Watermark": row.Watermark, "CheckURL": row.CheckURL,
	}
}

// editFormH 编辑页出错/验证回填用。
func editFormH(row models.SiteStatus) gin.H {
	return gin.H{
		"Domains": row.Domains,
		"ListSel": row.ListSel, "TitleSel": row.TitleSel,
		"ContentSel": row.ContentSel, "Mode": row.ContentAlignment,
		"Watermark": row.Watermark, "CheckURL": row.CheckURL,
	}
}

// readSiteForm 读编辑页表单（域名、解析规则、验证书籍页 URL）。
func readSiteForm(c *gin.Context) models.SiteStatus {
	return models.SiteStatus{
		Domains:          strings.TrimSpace(c.PostForm("domains")),
		ListSel:          strings.TrimSpace(c.PostForm("list_sel")),
		TitleSel:         strings.TrimSpace(c.PostForm("title_sel")),
		ContentSel:       strings.TrimSpace(c.PostForm("content_sel")),
		ContentAlignment: strings.TrimSpace(c.PostForm("content_mode")),
		Watermark:        strings.TrimSpace(c.PostForm("watermark")),
		CheckURL:         strings.TrimSpace(c.PostForm("check_url")),
	}
}

// editDefaults 编辑页选择器占位提示：内置站点用代码默认值，
// 自定义站点用笔趣阁系缺省值。
func editDefaults(row models.SiteStatus) sources.Selectors {
	if !row.Custom {
		if d, ok := sources.DefaultSelectorsByName(row.Source); ok {
			return d
		}
	}
	return sources.BiqugeSelectors
}

// effMode 编辑页分段方式的实际生效值：表单 > 库行 > 代码默认。
func effMode(row, form models.SiteStatus, def sources.Selectors) string {
	switch {
	case form.ContentAlignment != "":
		return form.ContentAlignment
	case row.ContentAlignment != "":
		return row.ContentAlignment
	default:
		return def.Mode
	}
}

// formAdapter 按表单规则构建验证用适配器：是否自定义以库里已存的行
// 为准（表单里没有该信息），规则值用表单提交的；内置站点在裸代码
// 适配器上叠加表单覆盖（Get 返回的适配器带着库内旧覆盖，
// 不适合做「以表单为准」的验证）。
func formAdapter(source string, stored, form models.SiteStatus) (spider.SiteAdapter, error) {
	if stored.Custom {
		return sources.FromSite(form), nil
	}
	ad, err := sources.GetRaw(source)
	if err != nil {
		return nil, err
	}
	ov := sources.Selectors{
		List: form.ListSel, Title: form.TitleSel, Content: form.ContentSel,
		Mode: form.ContentAlignment, Watermark: form.Watermark,
	}
	if w, ok := sources.NewSelectorOverride(ad, ov); ok {
		ad = w
	}
	return ad, nil
}

// bookProbe 试解析一本书的结果：书籍页元信息 + 目录规模 + 第一章正文。
type bookProbe struct {
	Title      string
	Author     string
	Category   string
	Status     string
	Chapters   int
	FirstTitle string
	Paragraphs int
	Preview    []string // 第一章开头两段
}

// probeBook 真实抓取验证书籍页并解析，再抓第一章验证正文规则，
// 能完整解析才认为「可以爬到书」。err 为 nil 表示通过。
// 书籍页通过但章节失败时，错误信息带上已解析到的书名与章节数。
func probeBook(ad spider.SiteAdapter, bookURL string) (*bookProbe, error) {
	doc, err := fetchDoc(bookURL)
	if err != nil {
		return nil, err
	}
	meta, links, err := ad.ParseBook(bookURL, doc)
	if err != nil {
		return nil, fmt.Errorf("书籍页解析失败: %w", err)
	}
	if len(links) == 0 {
		return nil, errors.New("目录选择器未解析到任何章节链接")
	}
	p := &bookProbe{
		Title: meta.Title, Author: meta.Author,
		Category: meta.Category, Status: meta.Status,
		Chapters: len(links),
	}
	doc1, err := fetchDoc(links[0])
	if err != nil {
		return p, fmt.Errorf("书籍页 OK（《%s》共 %d 章），但抓第一章失败: %w", meta.Title, len(links), err)
	}
	data, err := ad.ParseChapter(links[0], doc1)
	if err != nil {
		return p, fmt.Errorf("书籍页 OK（《%s》共 %d 章），但第一章解析失败: %w", meta.Title, len(links), err)
	}
	p.FirstTitle = data.Title
	p.Paragraphs = len(data.Paragraphs)
	if len(data.Paragraphs) > 0 {
		p.Preview = append(p.Preview, truncatePara(data.Paragraphs[0]))
		if len(data.Paragraphs) > 1 {
			p.Preview = append(p.Preview, truncatePara(data.Paragraphs[1]))
		}
	}
	return p, nil
}

// truncatePara 预览段落截到 120 字。
func truncatePara(s string) string {
	r := []rune(s)
	if len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}

// fetchDoc 抓取页面并按响应编码转成 goquery 文档。
func fetchDoc(pageURL string) (*goquery.Document, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", spiderDefaultUA)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errors.New("HTTP " + resp.Status)
	}
	rd, err := charset.NewReader(resp.Body, resp.Header.Get("Content-Type"))
	if err != nil {
		rd = io.NopCloser(resp.Body)
	}
	return goquery.NewDocumentFromReader(rd)
}

// AdminEditSite 站点编辑页：内置站点可改域名与解析规则覆盖（留空用代码默认），
// 自定义站点可改域名与全部解析规则。
func AdminEditSite(c *gin.Context) {
	row, ok := models.FindSite(c.Param("source"))
	if !ok {
		c.Redirect(http.StatusFound, "/admin/sites")
		return
	}
	def := editDefaults(row)
	c.HTML(http.StatusOK, "site_edit.html", gin.H{
		"Site":     row,
		"Defaults": def,
		"EffMode":  effMode(row, models.SiteStatus{}, def),
	})
}

// AdminUpdateSite 保存站点编辑。填写「验证书籍页 URL」时先真实抓取
// 试解析（内置站点用代码规则+表单覆盖、自定义站点用编辑后的规则），
// 通过才保存；示例 URL 一并存库供下次直接验证。
func AdminUpdateSite(c *gin.Context) {
	source := c.Param("source")
	stored, ok := models.FindSite(source)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/sites")
		return
	}
	form := readSiteForm(c)

	renderErr := func(msg string) {
		def := editDefaults(stored)
		c.HTML(http.StatusBadRequest, "site_edit.html", gin.H{
			"Site":     stored,
			"Error":    msg,
			"Form":     editFormH(form),
			"Defaults": def,
			"EffMode":  effMode(stored, form, def),
		})
	}

	if form.Domains == "" {
		renderErr("域名不能为空（多个用逗号分隔）")
		return
	}
	if !validContentMode(form.ContentAlignment) {
		renderErr("正文分段只能是 br 或 p")
		return
	}
	// 验证用适配器：内置站点 = 代码规则 + 表单覆盖；自定义 = 表单规则
	ad, err := formAdapter(source, stored, form)
	if err != nil {
		renderErr(err.Error())
		return
	}
	if form.CheckURL != "" {
		if _, err := probeBook(ad, form.CheckURL); err != nil {
			renderErr("验证书籍页失败：" + err.Error())
			return
		}
	}

	if stored.Custom {
		err = models.UpdateCustomSite(source, form)
	} else {
		err = models.UpdateSiteDomains(source, form)
	}
	if err != nil {
		renderErr(err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/sites")
}

// AdminValidateSite 试解析书籍页 + 第一章并把结果回显在编辑页，
// 不保存任何修改——用于回答「这个站现在能不能爬到书」。
func AdminValidateSite(c *gin.Context) {
	source := c.Param("source")
	stored, ok := models.FindSite(source)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/sites")
		return
	}
	form := readSiteForm(c)
	def := editDefaults(stored)

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "site_edit.html", gin.H{
			"Site":     stored,
			"Error":    msg,
			"Form":     editFormH(form),
			"Defaults": def,
			"EffMode":  effMode(stored, form, def),
		})
	}

	if form.CheckURL == "" {
		renderErr("请先填写「验证书籍页 URL」再验证")
		return
	}
	if form.Domains == "" {
		renderErr("域名不能为空（多个用逗号分隔）")
		return
	}
	if !validContentMode(form.ContentAlignment) {
		renderErr("正文分段只能是 br 或 p")
		return
	}
	ad, err := formAdapter(source, stored, form)
	if err != nil {
		renderErr(err.Error())
		return
	}
	p, err := probeBook(ad, form.CheckURL)
	if err != nil {
		renderErr("验证失败：" + err.Error())
		return
	}
	c.HTML(http.StatusOK, "site_edit.html", gin.H{
		"Site":     stored,
		"Form":     editFormH(form),
		"Defaults": def,
		"EffMode":  effMode(stored, form, def),
		"Result":   p,
	})
}

// AdminDeleteSite 删除站点：自定义站点连解析规则一起硬删；
// 内置站点软删作墓碑，服务重启后也不会恢复。
func AdminDeleteSite(c *gin.Context) {
	_ = models.DeleteSite(c.Param("source"))
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
