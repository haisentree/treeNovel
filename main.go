// treeNovel Web 入口：gin + go:embed 单二进制。
//
// 启动：go run .（默认 0.0.0.0:8083）
//   -addr  监听地址        -db   sqlite 路径（默认 test.db）
//   -debug gin 调试模式
// 后台登录密码取环境变量 ADMIN_PASSWORD，默认 admin。
package main

import (
	"embed"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"treeNovel/admin"
	"treeNovel/global"
	"treeNovel/models"
	"treeNovel/routers"
	"treeNovel/spider/sources"
	"treeNovel/tasks"
)

//go:embed views static
var assets embed.FS

var (
	addr   = flag.String("addr", "0.0.0.0:8083", "http service address")
	dbPath = flag.String("db", "test.db", "sqlite database path")
	debug  = flag.Bool("debug", false, "gin debug mode")
)

func main() {
	flag.Parse()
	if !*debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// WAL + busy_timeout：允许爬虫写入与网站读取并存
	db, err := gorm.Open(sqlite.Open(*dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.AutoMigrate(models.Models...); err != nil {
		log.Fatal(err)
	}
	global.DB = db

	// 同步站点注册表到库（站点管理页的数据来源）
	for _, ad := range sources.All() {
		models.EnsureSite(ad.Name(), strings.Join(ad.Domains(), ", "))
	}

	admin.SetPassword(os.Getenv("ADMIN_PASSWORD"))
	tasks.StartWorker(db)

	r := gin.New()
	r.Use(gin.LoggerWithWriter(log.Writer()), gin.Recovery())

	// 模板与静态资源全部来自 embed，部署只依赖二进制 + db 文件
	tmpl := template.Must(template.ParseFS(assets, "views/*.html", "views/admin/*.html"))
	r.SetHTMLTemplate(tmpl)

	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		log.Fatal(err)
	}
	r.GET("/static/*filepath", func(c *gin.Context) {
		p := c.Param("filepath")
		if strings.HasSuffix(p, "/") { // 不提供目录列表
			c.Status(http.StatusNotFound)
			return
		}
		c.FileFromFS(p, http.FS(staticFS))
	})

	routers.Register(r)

	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin（默认，可用环境变量 ADMIN_PASSWORD 覆盖）"
	}
	log.Printf("http server running on http://%s | 后台 /admin，密码: %s", *addr, adminPass)
	if err := r.Run(*addr); err != nil {
		log.Fatal(err)
	}
}
