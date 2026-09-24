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
		auth.POST("/sites/check-all", controllers.AdminCheckAllSites)
		auth.POST("/sites/:source/check", controllers.AdminCheckSite)
		auth.POST("/sites/:source/toggle", controllers.AdminToggleSite)
		auth.GET("/books", controllers.AdminBooks)
		auth.POST("/books/:id/delete", controllers.AdminDeleteBook)
	}
}
