// Package treeNovel 根包：持有嵌入的 web 资源。
// go:embed 只能访问本包目录下的文件，因此入口挪到 cmd/ 后，
// views/ 与 static/ 的嵌入语句留在仓库根包，供 cmd/server 使用。
package treeNovel

import "embed"

//go:embed views static
var Assets embed.FS
