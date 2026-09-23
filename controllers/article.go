package controllers

import (
	"github.com/beego/beego/v2/server/web"
	"strconv"
	"strings"
	"treeNovel/models"
)

type ArticleController struct {
	web.Controller
}

func (c *ArticleController) GetHome() {
	articleModel := models.NewArticle()
	articleList := articleModel.FindAllArticle()
	c.Data["ArticleList"] = articleList
	c.TplName = "index.html"
}

func (c *ArticleController) GetArticle() {
	articleID := c.Ctx.Input.Param(":id")

	temp, _ := strconv.ParseUint(articleID, 10, 64)
	articleIDUint := uint(temp)
	articleModel := models.NewArticle()
	articleDetail := articleModel.FindArticleByID(articleIDUint)

	c.Data["ArticleDetail"] = articleDetail
	c.Data["ArticleID"] = articleIDUint
	c.TplName = "article.html"
}

func (c *ArticleController) GetChapter() {
	chapterID := c.Ctx.Input.Param(":id")

	temp, _ := strconv.ParseUint(chapterID, 10, 64)
	chapterIDUint := uint(temp)
	chapterModel := models.NewChapter()
	chapterDetail := chapterModel.FindChapterByID(chapterIDUint)
	prevChapter, nextChapter := chapterModel.FindPrevNext(chapterIDUint)

	contentList := strings.Split(chapterDetail.Content, "$$")

	c.Data["ChapterDetail"] = chapterDetail
	c.Data["ContentList"] = contentList
	c.Data["PrevChapter"] = prevChapter
	c.Data["NextChapter"] = nextChapter

	c.TplName = "content.html"
}
