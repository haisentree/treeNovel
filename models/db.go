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
	backfillSiteBools(db)
	global.DB = db
	return db, nil
}

// backfillSiteBools 旧库布尔列补零值：AutoMigrate 给已有表新增列时
// 旧行为 NULL（SQLite 加列不带默认值），而「custom = ?」等值比较
// 匹配不到 NULL，内置站点的域名编辑会因此报「不存在」。幂等。
// enabled 按其建列默认回填为启用，其余布尔列回填为零值 false。
func backfillSiteBools(db *gorm.DB) {
	for _, col := range []string{"custom", "domains_edited", "reachable"} {
		db.Exec("UPDATE site_statuses SET " + col + " = 0 WHERE " + col + " IS NULL")
	}
	db.Exec("UPDATE site_statuses SET enabled = 1 WHERE enabled IS NULL")
}
