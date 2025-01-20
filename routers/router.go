package routers

import (
	"github.com/beego/beego/v2/server/web"
	"treeNovel/controllers"
)

func init() {
	web.CtrlGet("/", (*controllers.ArticleController).GetHome)

	web.CtrlGet("/article/:id", (*controllers.ArticleController).GetArticle)
	web.CtrlGet("/chapter/:id", (*controllers.ArticleController).GetChapter)
}
