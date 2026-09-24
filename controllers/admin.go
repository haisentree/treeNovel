package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"treeNovel/admin"
	"treeNovel/global"
	"treeNovel/models"
	"treeNovel/spider/sources"
)

// ---------- 登录 / 退出 ----------

// AdminLogin 后台登录页。
func AdminLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{})
}

// AdminLoginSubmit 处理登录表单。
func AdminLoginSubmit(c *gin.Context) {
	if admin.Login(c, c.PostForm("password")) {
		c.Redirect(http.StatusFound, "/admin")
		return
	}
	c.HTML(http.StatusUnauthorized, "login.html", gin.H{"Error": "密码错误"})
}

// AdminLogout 退出登录。
func AdminLogout(c *gin.Context) {
	admin.Logout(c)
	c.Redirect(http.StatusFound, "/admin/login")
}

// ---------- 仪表盘 ----------

// AdminDashboard 后台首页：统计 + 最近任务。
func AdminDashboard(c *gin.Context) {
	type statusCount struct {
		Status string
		N      int64 `gorm:"column:n"`
	}
	var (
		books, chapters int64
		taskByStatus    []statusCount
		recent          []models.CrawlTask
	)
	global.DB.Model(&models.Article{}).Count(&books)
	global.DB.Model(&models.Chapter{}).Count(&chapters)
	global.DB.Model(&models.CrawlTask{}).
		Select("status, count(*) as n").Group("status").Scan(&taskByStatus)

	taskCount := map[string]int64{}
	for _, s := range taskByStatus {
		taskCount[s.Status] = s.N
	}
	global.DB.Order("id DESC").Limit(10).Find(&recent)

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Books": books, "Chapters": chapters,
		"TaskCount": taskCount, "Recent": recent,
	})
}

// ---------- 任务管理 ----------

// enabledSiteNames 创建表单下拉框用：启用的站点；同步异常时退回全部注册名。
func enabledSiteNames() []string {
	if names := models.EnabledSiteNames(); len(names) > 0 {
		return names
	}
	return sources.Names()
}

// AdminTasks 任务列表 + 创建表单。
func AdminTasks(c *gin.Context) {
	var taskList []models.CrawlTask
	global.DB.Order("id DESC").Limit(200).Find(&taskList)

	hasActive := false
	for _, t := range taskList {
		if t.Status == models.TaskQueued || t.Status == models.TaskRunning {
			hasActive = true
			break
		}
	}
	c.HTML(http.StatusOK, "tasks.html", gin.H{
		"Tasks":     taskList,
		"Sources":   enabledSiteNames(),
		"HasActive": hasActive,
	})
}

// AdminCreateTask 校验并创建爬虫任务（入队后由 worker 串行执行）。
func AdminCreateTask(c *gin.Context) {
	source := strings.TrimSpace(c.PostForm("source"))
	bookURL := strings.TrimSpace(c.PostForm("url"))
	max, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("max")))

	renderErr := func(msg string) {
		c.HTML(http.StatusBadRequest, "tasks.html", gin.H{
			"Sources": enabledSiteNames(),
			"Error":   msg,
			"Form":    gin.H{"Source": source, "URL": bookURL, "Max": c.PostForm("max")},
		})
	}

	if _, err := sources.Get(source); err != nil {
		renderErr("未知站点适配器：" + source)
		return
	}
	if !models.IsSiteEnabled(source) {
		renderErr("站点 " + source + " 已停用，请到站点管理启用")
		return
	}
	if !strings.HasPrefix(bookURL, "http://") && !strings.HasPrefix(bookURL, "https://") {
		renderErr("书籍页地址必须以 http:// 或 https:// 开头")
		return
	}
	if max < 0 {
		renderErr("章节上限不能为负")
		return
	}

	if err := global.DB.Create(&models.CrawlTask{
		Source:      source,
		BookURL:     bookURL,
		MaxChapters: max,
		Status:      models.TaskQueued,
	}).Error; err != nil {
		renderErr("任务入库失败：" + err.Error())
		return
	}
	c.Redirect(http.StatusFound, "/admin/tasks")
}

// AdminRerunTask 以相同参数重新入队一个新任务（保留历史记录）。
func AdminRerunTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var origin models.CrawlTask
	if err := global.DB.First(&origin, uint(id)).Error; err == nil {
		global.DB.Create(&models.CrawlTask{
			Source:      origin.Source,
			BookURL:     origin.BookURL,
			MaxChapters: origin.MaxChapters,
			Status:      models.TaskQueued,
			Message:     "重跑自任务 #" + strconv.FormatUint(uint64(origin.ID), 10),
		})
	}
	c.Redirect(http.StatusFound, "/admin/tasks")
}

// ---------- 书籍管理 ----------

// bookRow 书籍管理列表的一行（带章节计数）。
type bookRow struct {
	ID           uint
	Title        string
	Author       string
	Source       string
	CoverURL     string
	ChapterCount int64
	UpdatedAt    time.Time
}

// AdminBooks 已入库书籍列表。
func AdminBooks(c *gin.Context) {
	var rows []bookRow
	global.DB.Model(&models.Article{}).
		Select(`articles.id, articles.title, articles.author, articles.source, articles.cover_url,
			articles.updated_at,
			(SELECT COUNT(*) FROM chapters
			 WHERE chapters.article_id = articles.id AND chapters.deleted_at IS NULL) AS chapter_count`).
		Order("articles.id DESC").
		Scan(&rows)

	c.HTML(http.StatusOK, "books.html", gin.H{"Books": rows})
}

// AdminDeleteBook 删除书籍及其全部章节（硬删除，释放空间）。
func AdminDeleteBook(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	db := global.DB
	db.Unscoped().Where("article_id = ?", uint(id)).Delete(&models.Chapter{})
	db.Unscoped().Delete(&models.Article{}, uint(id))
	c.Redirect(http.StatusFound, "/admin/books")
}
