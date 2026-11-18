package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"treeNovel/models"
)

// Home 书架首页，支持 ?category= 按分类筛选。
func Home(c *gin.Context) {
	category := strings.TrimSpace(c.Query("category"))
	articleModel := models.NewArticle()
	all := articleModel.FindAllArticle()
	list := all
	if category != "" {
		list = articleModel.FindByCategory(category)
	}
	c.HTML(http.StatusOK, "index.html", gin.H{
		"ArticleList":    list,
		"Categories":     articleModel.FindAllCategories(),
		"ActiveCategory": category,
		"Total":          len(all),
	})
}

// Article 书籍详情与章节目录。
func Article(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	c.HTML(http.StatusOK, "article.html", gin.H{
		"ArticleDetail": models.NewArticle().FindArticleByID(uint(id)),
		"ArticleID":     uint(id),
	})
}

// Chapter 章节正文阅读页。
func Chapter(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	chapterModel := models.NewChapter()
	chapterDetail := chapterModel.FindChapterByID(uint(id))
	prevChapter, nextChapter := chapterModel.FindPrevNext(uint(id))

	c.HTML(http.StatusOK, "content.html", gin.H{
		"ChapterDetail": chapterDetail,
		"ContentList":   strings.Split(chapterDetail.Content, "$$"),
		"PrevChapter":   prevChapter,
		"NextChapter":   nextChapter,
	})
}
