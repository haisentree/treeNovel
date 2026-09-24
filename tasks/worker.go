// Package tasks 实现爬虫任务的串行执行器：
// 轮询数据库里 status=queued 的最早任务，逐个执行并回写状态。
//
// 串行的原因：SQLite 是单写者，爬虫本身带同站限速，
// 一本一本抓最稳，也避免了并发爬虫把目标站打挂。
package tasks

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"treeNovel/models"
	"treeNovel/spider"
	"treeNovel/spider/sources"
)

const pollInterval = 2 * time.Second

// StartWorker 启动后台执行器。服务重启时遗留的 running 任务
// 重置回 queued，实现断点续跑（增量更新本来就幂等）。
func StartWorker(db *gorm.DB) {
	if err := db.Model(&models.CrawlTask{}).
		Where("status = ?", models.TaskRunning).
		Update("status", models.TaskQueued).Error; err != nil {
		log.Printf("[tasks] 重置遗留 running 任务失败: %v", err)
	}
	go func() {
		for {
			runNext(db)
			time.Sleep(pollInterval)
		}
	}()
}

// runNext 取最早的 queued 任务执行；没有任务时静默返回。
func runNext(db *gorm.DB) {
	var task models.CrawlTask
	// 轮询查询关掉 gorm 日志，避免每 2 秒刷一条 record not found
	err := db.Session(&gorm.Session{Logger: logger.Discard}).
		Where("status = ?", models.TaskQueued).
		Order("id ASC").First(&task).Error
	if err != nil {
		return // 无排队任务，正常情况
	}

	db.Model(&task).Updates(map[string]any{
		"status":  models.TaskRunning,
		"message": "",
	})
	log.Printf("[task#%d] 开始执行 %s %s", task.ID, task.Source, task.BookURL)

	ad, err := sources.Get(task.Source)
	if err == nil && !models.IsSiteEnabled(task.Source) {
		err = fmt.Errorf("站点 %s 已在后台停用，请到站点管理启用后重跑", task.Source)
	}
	if err == nil {
		var bookID uint
		var saved int
		bookID, saved, err = spider.CrawlBook(db, ad, task.BookURL, spider.Options{
			MaxChapters: task.MaxChapters,
		})
		if err == nil {
			finish(db, &task, models.TaskSuccess, bookID, saved, fmt.Sprintf("完成，本次新增 %d 章", saved))
			return
		}
	}
	finish(db, &task, models.TaskFailed, 0, 0, err.Error())
}

func finish(db *gorm.DB, task *models.CrawlTask, status string, bookID uint, saved int, msg string) {
	db.Model(task).Updates(map[string]any{
		"status":         status,
		"message":        msg,
		"book_id":        bookID,
		"saved_chapters": saved,
	})
	log.Printf("[task#%d] %s: %s", task.ID, status, msg)
}
