package models

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"treeNovel/global"
)

// InitDB 打开 sqlite（WAL + busy_timeout，允许爬虫写入与网站读取并存），
// 完成建表迁移并写入 global.DB。
// quiet=true 关闭 gorm SQL 日志——爬虫的高频去重查询（not found）会刷屏。
func InitDB(path string, quiet bool) (*gorm.DB, error) {
	cfg := &gorm.Config{}
	if quiet {
		cfg.Logger = logger.Discard
	}
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"), cfg)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(Models...); err != nil {
		return nil, err
	}
	global.DB = db
	return db, nil
}
