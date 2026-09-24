package controllers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"treeNovel/models"
	"treeNovel/spider"
	"treeNovel/spider/sources"
)

// AdminSites 站点管理页。
func AdminSites(c *gin.Context) {
	c.HTML(http.StatusOK, "sites.html", gin.H{"Sites": models.FindAllSites()})
}

// AdminCheckSite 探测单个站点并回显结果。
func AdminCheckSite(c *gin.Context) {
	source := c.Param("source")
	ad, err := sources.Get(source)
	if err == nil {
		r := spider.CheckSite(ad)
		models.SaveSiteCheck(source, r.OK, r.StatusCode, r.LatencyMS, r.Err)
	}
	c.Redirect(http.StatusFound, "/admin/sites")
}

// AdminCheckAllSites 并发探测全部站点（单站 12s 超时，总量约 12s 封顶）。
func AdminCheckAllSites(c *gin.Context) {
	var wg sync.WaitGroup
	for _, ad := range sources.All() {
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
