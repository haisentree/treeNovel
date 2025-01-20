package models

import (
	"gorm.io/gorm"
	"treeNovel/global"
)

type Article struct {
	gorm.Model
	Author      string
	Title       string
	Description string `gorm:"type:text"`
	Category    string
	CoverURL    string
	Status      string
	Source      string
	Chapters    []Chapter `gorm:"foreignKey:ArticleID"`
}

type Chapter struct {
	gorm.Model
	ArticleID uint
	Title     string

	Content string `gorm:"type:text"`
}

// ===============================================文章=============================================
func NewArticle() *Article {
	return &Article{}
}
func (a *Article) TableName() string {
	return "articles"
}

// 分页获取全部文章信息
func (a *Article) FindAllArticle() []Article {
	var articles []Article
	global.DB.Find(&articles)
	return articles
}

// 通过分类查询文章列表
func (a *Article) FindArticleByID(id uint) *Article {
	var article Article
	global.DB.Model(&Article{}).Preload("Chapters").Where("id = ?", id).First(&article)
	//global.DB.Model(article).Association("Chapter").Find(&article.Chapters)
	return &article
}

// ===============================================章节=============================================
func NewChapter() *Chapter {
	return &Chapter{}
}
func (c *Chapter) TableName() string {
	return "chapters"
}

func (c *Chapter) FindChapterByID(id uint) Chapter {
	var chapter Chapter
	global.DB.Where("id = ?", id).First(&chapter)
	return chapter
}
