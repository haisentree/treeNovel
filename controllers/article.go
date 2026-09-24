package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"treeNovel/models"
)

// Home 书架首页。
func Home(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"ArticleList": models.NewArticle().FindAllArticle(),
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
