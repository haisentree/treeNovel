package models

import "gorm.io/gorm"

// 爬虫任务状态
const (
	TaskQueued  = "queued"  // 已入队，等待执行
	TaskRunning = "running" // 执行中
	TaskSuccess = "success" // 抓取完成（或增量无新增）
	TaskFailed  = "failed"  // 执行失败，Message 存原因
)

// CrawlTask 后台创建的爬虫任务，由 tasks worker 串行执行。
type CrawlTask struct {
	gorm.Model
	Source      string // 站点适配器名
	BookURL     string `gorm:"not null"`
	MaxChapters int    // 章节上限（试爬用），0 不限
	Status      string `gorm:"default:queued;index"`
	Message     string `gorm:"type:text"`
	BookID      uint   // 抓到/更新到的书籍 ID，0 表示未关联
	SavedChapters int  // 本次新增章节数
}

func (t *CrawlTask) TableName() string { return "crawl_tasks" }
