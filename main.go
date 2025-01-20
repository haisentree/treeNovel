package main

import (
	"fmt"
	"github.com/beego/beego/v2/server/web"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"strings"
	"treeNovel/global"
	"treeNovel/models"
	_ "treeNovel/routers"
)

func init() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
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
	web.AddFuncMap("ShowContent", ShowContent)
	web.Run("127.0.0.1:8080")
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
