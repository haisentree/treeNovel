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

// CategoryCount 分类导航条目：分类名 + 该分类下的书籍数。
type CategoryCount struct {
	Name  string
	Count int64
}

// FindAllCategories 统计各分类的书籍数，按数量降序、同名按名称排序。
func (a *Article) FindAllCategories() []CategoryCount {
	var rows []CategoryCount
	global.DB.Model(&Article{}).
		Select("category AS name, COUNT(*) AS count").
		Where("category <> ''").
		Group("category").
		Order("count DESC, name").
		Scan(&rows)
	return rows
}

// FindByCategory 按分类查询文章列表（精确匹配）。
func (a *Article) FindByCategory(category string) []Article {
	var articles []Article
	global.DB.Where("category = ?", category).Find(&articles)
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

// FindPrevNext 返回同一本书内相邻的上一章/下一章（按 id 顺序），不存在则为 nil。
func (c *Chapter) FindPrevNext(id uint) (prev, next *Chapter) {
	var cur Chapter
	if global.DB.Where("id = ?", id).First(&cur).Error != nil {
		return nil, nil
	}
	var p Chapter
	if global.DB.Where("article_id = ? AND id < ?", cur.ArticleID, id).
		Order("id DESC").First(&p).Error == nil {
		prev = &p
	}
	var n Chapter
	if global.DB.Where("article_id = ? AND id > ?", cur.ArticleID, id).
		Order("id ASC").First(&n).Error == nil {
		next = &n
	}
	return prev, next
}
