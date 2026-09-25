// treeNovel Web 服务入口（gin + go:embed 单二进制）。
//
// 启动：go run ./cmd/server
//   --addr 0.0.0.0:8083   监听地址
//   --db   test.db        sqlite 路径
//   --debug               gin 调试模式
// 后台登录密码取环境变量 ADMIN_PASSWORD，默认 admin。
package main

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"treeNovel"
	"treeNovel/admin"
	"treeNovel/models"
	"treeNovel/routers"
	"treeNovel/spider/sources"
	"treeNovel/tasks"
)

var (
	addr   string
	dbPath string
	debug  bool
)

func main() {
	root := &cobra.Command{
		Use:   "server",
		Short: "treeNovel Web 服务：小说阅读站 + 爬虫管理后台",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run()
		},
		SilenceUsage: true,
	}
	root.Flags().StringVar(&addr, "addr", "0.0.0.0:8083", "监听地址")
	root.Flags().StringVar(&dbPath, "db", "test.db", "sqlite 数据库路径")
	root.Flags().BoolVar(&debug, "debug", false, "gin 调试模式")
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := models.InitDB(dbPath, false)
	if err != nil {
		return err
	}

	// 同步站点注册表到库（站点管理页的数据来源），
	// 并把内置名集合交给 models 供自定义站点查重
	models.CodeSiteNames = sources.Names()
	for _, ad := range sources.All() {
		models.EnsureSite(ad.Name(), strings.Join(ad.Domains(), ", "))
	}

	admin.SetPassword(os.Getenv("ADMIN_PASSWORD"))
	tasks.StartWorker(db)

	r := gin.New()
	r.Use(gin.LoggerWithWriter(log.Writer()), gin.Recovery())

	// 模板与静态资源全部来自根包嵌入，部署只依赖二进制 + db 文件
	tmpl := template.Must(template.ParseFS(treeNovel.Assets, "views/*.html", "views/admin/*.html"))
	r.SetHTMLTemplate(tmpl)

	staticFS, err := fs.Sub(treeNovel.Assets, "static")
	if err != nil {
		return err
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
	log.Printf("http server running on http://%s | 后台 /admin，密码: %s", addr, adminPass)
	return r.Run(addr)
}
