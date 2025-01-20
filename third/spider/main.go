package main

import (
	"fmt"
	"github.com/antchfx/htmlquery"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

var DB *gorm.DB

const SOURCE = "22biqu"
const BASE_URL = "https://www.22biqu.com"

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

func init() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		fmt.Println(err)
	}

	err = db.AutoMigrate(&Article{}, &Chapter{})
	if err != nil {
		fmt.Println(err)
	}
	DB = db
}

func main() {
	// GetContent("https://www.22biqu.com/biqu100/40517611.html")
	GetArticle("https://www.22biqu.com/biqu71672/")
	// https://www.22biqu.com/biqu71669/
	// https://www.22biqu.com/biqu77577/
}

func GetArticle(url string) {
	http.Header{}.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
	}
	// fmt.Println(resp.Status)

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	title := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/h1/text()`)
	author := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/div/p[1]/text()`)
	status := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/div/p[3]/text()`)
	description := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[2]/text()`)
	categroy := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/div/p[2]/text()`)
	image := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[1]/img`).Attr

	article := &Article{
		Author:      author.Data[13:],
		Title:       title.Data,
		Description: description.Data,
		Status:      status.Data[13:],
		Category:    categroy.Data[13:],
		CoverURL:    image[1].Val,
		Source:      SOURCE,
	}
	findArticle := &Article{}
	// 来源、作者、标题，三者相同，就不存储到数据库中了
	DB.Where("author = ? AND source = ? AND title = ?", author.Data[13:], SOURCE, title.Data).First(&findArticle)
	if findArticle.ID != 0 {
		fmt.Println("存在,该条文章信息已经存储在数据库中")
		return
	}
	DB.Create(article)
	// 爬取第一页章节列表，具体章节的链接
	var currentFirstChapter string
	chapterList := htmlquery.Find(doc, `/html/body/div[4]/div[2]/div[1]/div[2]/ul/li/a`)
	var urlList []string
	for k, v := range chapterList {
		if k == 0 {
			currentFirstChapter = v.Attr[0].Val
			fmt.Println(currentFirstChapter)
		}
		urlList = append(urlList, v.Attr[0].Val)
	}
	// 爬取下一页章节列表，具体文章的链接
	for i := 2; i < 1000; i++ {
		url_2 := url + strconv.Itoa(i) + "/"
		resp2, err := http.Get(url_2)
		fmt.Println("url:", url_2)
		if err != nil {
			fmt.Println(err)
		}
		doc2, err := htmlquery.Parse(resp2.Body)
		if err != nil {
			fmt.Println(err)
		}
		chapterList2 := htmlquery.Find(doc2, `/html/body/div[4]/div[2]/div[1]/div[2]/ul/li/a`)
		fmt.Println("cp", chapterList2[1].Attr[0])
		for k2, v2 := range chapterList2 {
			if k2 == 0 {
				if currentFirstChapter == v2.Attr[0].Val {
					fmt.Println(currentFirstChapter)
					fmt.Println("没有新的章节列表")
					goto END
				} else {
					currentFirstChapter = v2.Attr[0].Val
				}
			}
			urlList = append(urlList, v2.Attr[0].Val)
		}
	}
END:
	fmt.Println(urlList)
	fmt.Println(len(urlList))
	for _, v := range urlList {
		url_3 := BASE_URL + v
		GetContent(url_3, article.ID)
	}
}

func GetContent(url string, article_id uint) {
	http.Header{}.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
	}
	// fmt.Println(resp.Status)
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	title := htmlquery.FindOne(doc, `//*[@id="container"]/div/div/div[2]/h1/text()`)
	contentList := htmlquery.Find(doc, `/html/body/div[4]/div/div/div[2]/div[3]/p/text()`)
	content := ""
	for _, v := range contentList {
		content = content + "$$" + v.Data
	}
	//fmt.Println(title.Data)
	//fmt.Println(content)
	chapter := &Chapter{
		ArticleID: article_id,
		Title:     title.Data,
		Content:   content,
	}
	findChapter := &Chapter{}
	DB.Where("title = ? AND article_id = ?", title.Data, article_id).First(&findChapter)
	if findChapter.ID != 0 {
		fmt.Println("该章节存在与数据库中--", title.Data)
		return
	}
	DB.Create(chapter)
}
