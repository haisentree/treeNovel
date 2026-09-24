# treeNovel
小说网站：多站点爬虫 + 内容展示 + 爬虫管理后台

## 使用技术

- gin（Web 层，服务端渲染 + `go:embed` 单二进制）
- colly + goquery（爬虫引擎与页面解析）
- gorm + sqlite（glebarez 纯 Go 驱动，免 CGO，可随意交叉编译）
- 自定义 CSS（亮/暗色自适应，无前端框架依赖）

## 目录结构

```
main.go               Web 入口（gin，默认 0.0.0.0:8083，-addr 可改）
models/               Article/Chapter/CrawlTask/SiteStatus 模型（爬虫与网站共用）
spider/
  adapter.go          SiteAdapter 接口 + 通用工具（URL 补全、标签前缀剥离、水印过滤）
  engine.go           Colly 抓取引擎：限速、超时、目录翻页终止、链接去重、增量更新
  health.go           站点可达性探测（HTTP 状态码 / 延迟 / 错误）
  sources/            站点适配器，每个站一个文件，新增站点在此注册
tasks/worker.go       爬虫任务串行执行器（后台轮询 queued 任务，断点续跑）
admin/auth.go         后台会话认证（ADMIN_PASSWORD）
controllers/          gin handlers（公开页 + /admin 后台）
views/  static/       页面模板与静态资源（go:embed 打进二进制）
cmd/spider/           爬虫 CLI 入口（与后台等效，适合 cron）
```

## 网站展示

启动：`go run .`（或 `go run . -addr 0.0.0.0:8083 -db test.db`）

- `/` 书籍列表
- `/article/:id` 书籍详情与章节目录
- `/chapter/:id` 章节正文

## 管理后台

入口 `/admin`，登录密码取环境变量 `ADMIN_PASSWORD`（默认 `admin`）：

```bash
ADMIN_PASSWORD=mypassword go run .
```

- **仪表盘**：书籍/章节/任务统计，最近任务
- **爬虫任务**：选站点 + 填书籍页 URL 入队（可设章节上限试爬），任务串行执行，
  有排队/执行中任务时页面自动刷新；失败会显示原因，可一键重跑
- **站点管理**：列出全部已注册站点适配器，一键探测可达性
  （HTTP 状态码 / 延迟 / 错误信息，支持单站检测与全部检测）；
  可停用站点——停用后创建任务会被拒绝，排队中的任务也会被 worker 拒绝执行
- **书库**：查看已入库书籍，删除书籍及其全部章节（硬删除，释放空间）

> 「可达」只代表域名活着（停放页也可能返回 200），适配器选择器是否仍匹配，
> 建议用小章节上限的试爬任务验证。

任务状态机 `queued → running → success/failed`；服务重启会把遗留 `running`
任务重置回 `queued` 续跑。已入库的书重复抓取为增量更新（只补缺失章节），
连载书可周期性重跑追更。

> 后台会话存于内存（重启需重新登录），未做 CSRF 防护，请勿暴露公网。

## 爬虫 CLI（可选，适合 cron 场景）

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

### 站点适配器状态

| source | 站点 | 状态 |
|---|---|---|
| kunnu | www.kunnu8.com（kunnu.com 跳转至此） | ✅ 可用，选择器 2026-09 校准 |
| 22biqu | www.22biqu.com | ❌ 域名已被注册商停放；适配器保留作翻页模式参考与存量数据溯源 |

新增站点：在 `spider/sources/` 新建文件实现 `spider.SiteAdapter` 接口（书籍页/目录页/正文页的解析），注册进 `sources.all` 即可，引擎负责其余全部（后台下拉框自动出现新站点）。

> 注意：GBK 编码站点需在适配器内自行转码（当前接入站点均为 UTF-8）；各站点内容均为未授权转载，本项目仅限本地学习使用，请勿公开部署。
