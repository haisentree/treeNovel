package main

import (
	"flag"
	"fmt"
	"strings"
	"treeNovel/global"
	"treeNovel/models"
	_ "treeNovel/routers"

	"github.com/beego/beego/v2/server/web"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var addr = flag.String("addr", "0.0.0.0:8083", "http service address")

func init() {
	// WAL + busy_timeout：允许爬虫写入与网站读取并存
	db, err := gorm.Open(sqlite.Open("test.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		fmt.Println(err)
	}

	err = db.AutoMigrate(models.Models...)
	if err != nil {
		fmt.Println(err)
	}
	global.DB = db
}

func main() {
	flag.Parse()

	web.AddFuncMap("ShowContent", ShowContent)
	web.Run(*addr)
}

// 去除$$符号，增加p标签
func ShowContent(in string) (out string) {
	contentList := strings.Split(in, "$$")
	out = ""
	for _, v := range contentList {
		out = out + "<p>" + v + "</p>"
	}
	fmt.Println(out)
	return out
}
