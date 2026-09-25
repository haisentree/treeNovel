// 爬虫 CLI：按站点适配器抓取一本书存入 sqlite（与后台任务等效，适合 cron）。
//
// 用法：
//
//	go run ./cmd/spider list
//	go run ./cmd/spider --source kunnu --url https://www.kunnu8.com/fanren/
//	go run ./cmd/spider --source kunnu --url ... --max 5        # 试爬前 5 章
//	go run ./cmd/spider --db /path/test.db --source kunnu --url ...
//
// 已入库的书重复执行会增量更新（只补缺失章节），中断后重跑即可续爬。
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	"treeNovel/models"
	"treeNovel/spider"
	"treeNovel/spider/sources"
)

var (
	dbPath  string
	source  string
	bookURL string
	max     int
	delay   time.Duration
)

func main() {
	root := &cobra.Command{
		Use:   "spider",
		Short: "treeNovel 小说爬虫 CLI",
		Long:  "按站点适配器抓取一本书存入 sqlite。已入库的书重复执行为增量更新，\n中断后重跑即可续爬，连载书可周期性执行追更。",
		RunE:  run,
	}
	root.Flags().StringVar(&dbPath, "db", "test.db", "sqlite 数据库路径")
	root.Flags().StringVar(&source, "source", "", "站点适配器名（见 list 子命令）")
	root.Flags().StringVar(&bookURL, "url", "", "书籍页地址")
	root.Flags().IntVar(&max, "max", 0, "最多抓取章节数（试爬用），0 不限制")
	root.Flags().DurationVar(&delay, "delay", time.Second, "同站请求间隔")
	root.MarkFlagRequired("source")
	root.MarkFlagRequired("url")

	root.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部可用站点适配器",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("可用站点适配器:", sources.Names())
			return nil
		},
	})

	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// 先开库再解析适配器：自定义站点的规则存在库里，GetAny 需要查库
	db, err := models.InitDB(dbPath, true)
	if err != nil {
		return err
	}

	ad, err := sources.GetAny(source)
	if err != nil {
		return err
	}

	bookID, saved, err := spider.CrawlBook(db, ad, bookURL, spider.Options{
		Delay:       delay,
		MaxChapters: max,
	})
	if err != nil {
		return err
	}
	log.Printf("完成：书籍 id=%d，本次新增 %d 章", bookID, saved)
	return nil
}
