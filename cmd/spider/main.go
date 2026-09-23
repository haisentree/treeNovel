// 爬虫 CLI：按站点适配器抓取一本书存入 sqlite。
//
// 用法：
//
//	go run ./cmd/spider -list
//	go run ./cmd/spider -source kunnu -url https://www.kunnu8.com/fanren/
//	go run ./cmd/spider -source kunnu -url ... -max 5        # 试爬前 5 章
//	go run ./cmd/spider -source kunnu -url ... -db /path/test.db
//
// 已入库的书重复执行会增量更新（只补缺失章节），中断后重跑即可续爬。
package main

import (
	"flag"
	"log"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"treeNovel/models"
	"treeNovel/spider"
	"treeNovel/spider/sources"
)

func main() {
	dbPath := flag.String("db", "test.db", "sqlite 数据库路径")
	source := flag.String("source", "", "站点适配器名（见 -list）")
	bookURL := flag.String("url", "", "书籍页地址")
	max := flag.Int("max", 0, "最多抓取章节数（试爬用），0 不限制")
	delay := flag.Duration("delay", time.Second, "同站请求间隔")
	list := flag.Bool("list", false, "列出全部可用适配器")
	flag.Parse()

	if *list {
		log.Printf("可用站点适配器: %v", sources.Names())
		return
	}
	if *source == "" || *bookURL == "" {
		flag.Usage()
		log.Fatal("必须指定 -source 和 -url")
	}

	ad, err := sources.Get(*source)
	if err != nil {
		log.Fatal(err)
	}

	// 爬虫自己有进度日志，关掉 gorm 的 SQL 输出（去重查询会频繁触发 not found）
	db, err := gorm.Open(sqlite.Open(*dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"),
		&gorm.Config{Logger: logger.Discard})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.AutoMigrate(models.Models...); err != nil {
		log.Fatal(err)
	}

	if err := spider.CrawlBook(db, ad, *bookURL, spider.Options{
		Delay:       *delay,
		MaxChapters: *max,
	}); err != nil {
		log.Fatal(err)
	}
}
