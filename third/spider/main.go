package main

import (
	"fmt"
	"github.com/antchfx/htmlquery"
	"net/http"
)

func main() {
	// GetContent("https://www.22biqu.com/biqu100/40517611.html")
	http.Header{}.Add("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0")
	resp, err := http.Get("https://www.22biqu.com/biqu77577/")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(resp.Status)

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	title := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/h1/text()`)
	author := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/div/p[1]/text()`)
	status := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/div/p[3]/text()`)
	description := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[2]/text()`)
	image := htmlquery.FindOne(doc, `/html/body/div[4]/div[1]/div/div/div[2]/div[1]/img`).Attr
	fmt.Println(title.Data)
	fmt.Println(author.Data)
	fmt.Println(status.Data)
	fmt.Println(description.Data)
	fmt.Println(image)
}

func GetContent(url string) {
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
	fmt.Println(title.Data)
	fmt.Println(content)
}
