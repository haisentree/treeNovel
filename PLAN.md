# treeNovel 技术栈升级与爬虫管理后台 — 任务计划书

> 生成时间：2026-09-24 ｜ 执行模式：无人值守 ｜ 状态：✅ 已完成（全部任务卡验收通过）

## 一、目标

1. **技术栈迁移**：Web 层从 beego 迁移到 gin，模板与静态资源用 `go:embed` 打进单二进制；`models/`、`spider/` 两层保持不变，`test.db` 数据无缝沿用。
2. **新增管理后台**：一个简单的 `/admin`，用于管理爬虫站点与爬虫任务——在页面上选择站点、填入书籍 URL 即可发起抓取，任务在后台串行执行并可查看状态，同时可管理已入库书籍。

## 二、目标技术栈

| 层 | 技术 | 说明 |
|---|---|---|
| Web 框架 | gin | 路由 + 服务端渲染 |
| 模板 | html/template（gin 内置） | 沿用现有 3 个视图，数据键名不变 |
| 静态资源 | go:embed | views/ 与 static/ 嵌入二进制，部署 = 一个文件 + 一个 db |
| ORM | gorm + glebarez/sqlite | 不变，纯 Go 驱动免 CGO，WAL + busy_timeout |
| 爬虫 | colly + goquery + SiteAdapter 架构 | 不变 |

## 三、模块设计

```
main.go               gin 入口：flag（-addr/-db/-admin-pass）、embed、DB 初始化、启动任务 worker
routers/router.go     全部路由注册（公开 + /admin）
controllers/article.go  公开页 handlers（书架/详情/阅读，逻辑与原 beego 版一致）
controllers/admin.go    后台 handlers（登录、仪表盘、任务、书籍管理）
admin/auth.go          会话认证：内存 session + cookie，密码取 ADMIN_PASSWORD（默认 admin）
tasks/worker.go        任务执行器：轮询 queued 任务 → 串行执行 CrawlBook → 回写状态
models/task.go         CrawlTask 模型（source/url/max/status/message/book_id/saved）
views/admin/*.html     后台页面（登录、仪表盘、任务、书籍）
spider/engine.go       小改：CrawlBook 返回 (bookID, saved, error)，供任务回写
```

### 任务状态机

`queued → running → success | failed`；服务启动时把遗留的 `running` 重置为 `queued`（断点续跑）。
同一时刻只执行一个任务（SQLite 单写者 + 爬虫限速，串行最稳）。

### 路由清单

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/`、`/article/:id`、`/chapter/:id` | 公开阅读站（不变） |
| GET/POST | `/admin/login` | 登录 |
| POST | `/admin/logout` | 退出 |
| GET | `/admin` | 仪表盘：统计 + 最近任务 |
| GET/POST | `/admin/tasks` | 任务列表 / 创建爬虫任务（选站点 + 填 URL） |
| POST | `/admin/tasks/:id/rerun` | 以相同参数重新入队 |
| GET | `/admin/books` | 书籍管理列表 |
| POST | `/admin/books/:id/delete` | 删除书籍及其章节 |

## 四、任务卡片（与 Todo 同步）

| # | 任务 | 验收标准 |
|---|---|---|
| 1 | 生成任务计划书 | 本文档 |
| 2 | 通读现有代码 | 掌握 controllers/views/adapter/sources/cmd/conf 全貌 |
| 3 | 技术栈迁移 beego → gin | `go build` 通过，go.mod 无 beego；views/static 改为 embed |
| 4 | 公开页回归 | `/`、`/article/1`、`/chapter/1` 返回 200 且内容与迁移前一致 |
| 5 | CrawlTask 模型 + 任务 worker | 建表成功；queued 任务被自动执行并回写状态；重启能续跑 |
| 6 | 后台认证 | 未登录访问 /admin 跳登录页；密码错误有提示 |
| 7 | 后台页面 | 仪表盘/任务/书籍三页可用，bootstrap 风格统一 |
| 8 | 端到端验证 | 登录 → 创建任务（kunnu, max=2）→ 任务执行 → 状态与入库正确 |
| 9 | 更新 README | 反映新栈与新用法 |

## 五、风险与对策

- **外网不可达**：任务会以 failed + 错误信息呈现，机制仍可验证；不影响交付。
- **模板迁移**：现有视图无 beego 专有函数，纯标准语法，预期零改动。
- **并发写库**：沿用 WAL + busy_timeout，且任务串行执行，无新增写冲突。
- **回滚**：git 干净起点，任一阶段失败可 `git checkout .` 整体回退。
