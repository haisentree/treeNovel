# treeNovel
小说网站：多站点爬虫 + 内容展示

## 使用技术

- beego（Web 展示）
- colly + goquery（爬虫引擎与页面解析）
- gorm + sqlite（glebarez 纯 Go 驱动，免 CGO，可随意交叉编译）
- bootstrap（页面样式）

## 目录结构

```
main.go               Web 入口（beego，默认 0.0.0.0:8083，-addr 可改）
models/               Article/Chapter 模型（爬虫与网站共用）
spider/
  adapter.go          SiteAdapter 接口 + 通用工具（URL 补全、标签前缀剥离、水印过滤）
  engine.go           Colly 抓取引擎：限速、UA、目录翻页终止、链接去重、增量更新
  sources/            站点适配器，每个站一个文件，新增站点在此注册
cmd/spider/           爬虫 CLI 入口
views/  static/ conf/ 页面模板与静态资源
```

## 爬虫用法

```bash
# 查看可用站点适配器
go run ./cmd/spider -list

# 抓取一本书（鲲弩小说的《凡人修仙传》）
go run ./cmd/spider -source kunnu -url https://www.kunnu8.com/fanren/

# 试爬前 5 章验证适配器
go run ./cmd/spider -source kunnu -url https://www.kunnu8.com/fanren/ -max 5

# 指定数据库与请求间隔
go run ./cmd/spider -db test.db -delay 2s -source kunnu -url ...
```

已入库的书重复执行会增量更新（只补缺失章节），中断后重跑即可续爬，连载书可周期性执行追更。

### 站点适配器状态

| source | 站点 | 状态 |
|---|---|---|
| kunnu | www.kunnu8.com（kunnu.com 跳转至此） | ✅ 可用，选择器 2026-09 校准 |
| 22biqu | www.22biqu.com | ❌ 域名已被注册商停放；适配器保留作翻页模式参考与存量数据溯源 |

新增站点：在 `spider/sources/` 新建文件实现 `spider.SiteAdapter` 接口（书籍页/目录页/正文页的解析），注册进 `sources.all` 即可，引擎负责其余全部。

> 注意：GBK 编码站点需在适配器内自行转码（当前接入站点均为 UTF-8）；各站点内容均为未授权转载，本项目仅限本地学习使用，请勿公开部署。

## 网站展示

启动：`go run .`（或 `go run . -addr 0.0.0.0:8083`）

- `/` 书籍列表
- `/article/:id` 书籍详情与章节目录
- `/chapter/:id` 章节正文

![](https://raw.githubusercontent.com/haisentree/imageBed/main/image2024/QQ2025122-1093.gif)

## 总结

内容爬取与展示功能没问题，没有做导航、分页、分类等功能，因为差不多一本数和相关的章节就要占用5MB空间，云服务器空间有限，就没继续做下去了。
