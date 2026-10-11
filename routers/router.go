package routers

import (
	"github.com/gin-gonic/gin"

	"treeNovel/admin"
	"treeNovel/controllers"
)

// Register 注册全部路由：公开阅读站 + /admin 后台。
func Register(r *gin.Engine) {
	// ---- 公开页面 ----
	r.GET("/", controllers.Home)
	r.GET("/article/:id", controllers.Article)
	r.GET("/chapter/:id", controllers.Chapter)

	// ---- 本地电子书（直接浏览 book/ 目录原文件） ----
	r.GET("/localbook", controllers.LocalBooks)
	r.GET("/localbook/view/:name", controllers.LocalArticle)
	r.GET("/localbook/read/:name/:num", controllers.LocalRead)
	r.GET("/localbook/download/:name", controllers.LocalDownload)

	// ---- 后台 ----
	r.GET("/admin/login", controllers.AdminLogin)
	r.POST("/admin/login", controllers.AdminLoginSubmit)
	r.POST("/admin/logout", controllers.AdminLogout)

	auth := r.Group("/admin", admin.Require())
	{
		auth.GET("", controllers.AdminDashboard)
		auth.GET("/tasks", controllers.AdminTasks)
		auth.POST("/tasks", controllers.AdminCreateTask)
		auth.POST("/tasks/:id/rerun", controllers.AdminRerunTask)
		auth.GET("/sites", controllers.AdminSites)
		auth.POST("/sites/add", controllers.AdminAddSite)
		auth.POST("/sites/check-all", controllers.AdminCheckAllSites)
		auth.GET("/sites/:source/edit", controllers.AdminEditSite)
		auth.POST("/sites/:source/update", controllers.AdminUpdateSite)
		auth.POST("/sites/:source/validate", controllers.AdminValidateSite)
		auth.POST("/sites/:source/check", controllers.AdminCheckSite)
		auth.POST("/sites/:source/delete", controllers.AdminDeleteSite)
		auth.POST("/sites/:source/toggle", controllers.AdminToggleSite)
		auth.GET("/books", controllers.AdminBooks)
		auth.POST("/books/:id/delete", controllers.AdminDeleteBook)
	}
}
