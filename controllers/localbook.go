package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"treeNovel/localbook"
)

// BookDir 本地电子书目录，server 启动时由 --bookdir 指定。
var BookDir = "book"

var localLib *localbook.Library

// SetBookDir 设置本地书库目录（server 启动时调用）。
func SetBookDir(dir string) {
	BookDir = dir
	localLib = nil
}

// lib 惰性创建书库实例，避免包初始化顺序问题。
func lib() *localbook.Library {
	if localLib == nil {
		localLib = localbook.NewLibrary(BookDir)
	}
	return localLib
}

// localToc 目录页条目：小节序号（1 起）+ 标题。
type localToc struct {
	Num   int
	Title string
	Chars int
}

// LocalBooks 本地书库列表页：直接浏览 book/ 目录下的电子书原文件。
func LocalBooks(c *gin.Context) {
	items, err := lib().List()
	if err != nil {
		c.HTML(http.StatusOK, "local_books.html", gin.H{
			"Items": nil,
			"Error": err.Error(),
		})
		return
	}
	c.HTML(http.StatusOK, "local_books.html", gin.H{
		"Items": items,
		"Error": "",
	})
}

// LocalArticle 本地书籍详情与目录页。
func LocalArticle(c *gin.Context) {
	name := c.Param("name")
	book, err := lib().Get(name)
	if err != nil {
		c.String(http.StatusNotFound, "无法打开 %s：%v", name, err)
		return
	}
	toc := make([]localToc, 0, len(book.Sections))
	total := 0
	for i, s := range book.Sections {
		toc = append(toc, localToc{Num: i + 1, Title: s.Title, Chars: s.Chars})
		total += s.Chars
	}
	title := book.Meta.Title
	if title == "" {
		title = name
	}
	c.HTML(http.StatusOK, "local_article.html", gin.H{
		"Name":       name,
		"Book":       book,
		"Title":      title,
		"Toc":        toc,
		"TotalChars": total,
	})
}

// LocalRead 本地书籍阅读页，num 为 1 起的小节序号。
func LocalRead(c *gin.Context) {
	name := c.Param("name")
	num, _ := strconv.Atoi(c.Param("num"))

	book, err := lib().Get(name)
	if err != nil {
		c.String(http.StatusNotFound, "无法打开 %s：%v", name, err)
		return
	}
	if num < 1 || num > len(book.Sections) {
		c.String(http.StatusNotFound, "小节序号超出范围（本书共 %d 节）", len(book.Sections))
		return
	}

	h := gin.H{
		"Name":      name,
		"Num":       num,
		"Total":     len(book.Sections),
		"BookTitle": book.Meta.Title,
		"Section":   book.Sections[num-1],
	}
	if num > 1 {
		h["PrevNum"] = num - 1
		h["PrevTitle"] = book.Sections[num-2].Title
	}
	if num < len(book.Sections) {
		h["NextNum"] = num + 1
		h["NextTitle"] = book.Sections[num].Title
	}
	c.HTML(http.StatusOK, "local_read.html", h)
}

// LocalDownload 下载电子书原文件。
func LocalDownload(c *gin.Context) {
	name := c.Param("name")
	path, err := lib().Path(name)
	if err != nil {
		c.String(http.StatusNotFound, "文件不存在：%v", err)
		return
	}
	c.FileAttachment(path, name)
}
