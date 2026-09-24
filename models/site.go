package models

import (
	"time"

	"gorm.io/gorm"

	"treeNovel/global"
)

// SiteStatus 爬虫站点的管理状态与健康检查结果。
// 适配器本身在代码里注册（sources.all），这里的行由启动时
// EnsureSite 按适配器名同步创建，Enable/检测结果持久化在库里。
type SiteStatus struct {
	gorm.Model
	Source     string `gorm:"uniqueIndex"` // 适配器名
	Domains    string // 逗号分隔的域名列表
	Enabled    bool   `gorm:"default:true"`
	Reachable  bool   // 最近一次检测结果
	StatusCode int    // HTTP 状态码（0 = 请求未完成）
	LatencyMS  int64
	Error      string
	CheckedAt  *time.Time
}

func (s *SiteStatus) TableName() string { return "site_statuses" }

// EnsureSite 启动时同步：新适配器补行（默认启用），已有行只刷新域名，
// 不动启停状态与检测结果。
func EnsureSite(source, domains string) {
	var row SiteStatus
	if err := global.DB.Where("source = ?", source).First(&row).Error; err == nil {
		if row.Domains != domains {
			global.DB.Model(&row).Update("domains", domains)
		}
		return
	}
	global.DB.Create(&SiteStatus{Source: source, Domains: domains, Enabled: true})
}

// FindAllSites 全部站点行。
func FindAllSites() []SiteStatus {
	var rows []SiteStatus
	global.DB.Order("source ASC").Find(&rows)
	return rows
}

// EnabledSiteNames 处于启用状态的适配器名。
func EnabledSiteNames() []string {
	var rows []SiteStatus
	global.DB.Where("enabled = ?", true).Order("source ASC").Find(&rows)
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Source)
	}
	return names
}

// IsSiteEnabled 站点是否启用；无记录时视为启用（新适配器未同步的防御）。
func IsSiteEnabled(source string) bool {
	var row SiteStatus
	if err := global.DB.Where("source = ?", source).First(&row).Error; err != nil {
		return true
	}
	return row.Enabled
}

// ToggleSiteEnabled 翻转启停状态。
func ToggleSiteEnabled(source string) {
	global.DB.Model(&SiteStatus{}).Where("source = ?", source).
		Update("enabled", gorm.Expr("NOT enabled"))
}

// SaveSiteCheck 写入一次探测结果。
func SaveSiteCheck(source string, ok bool, statusCode int, latencyMS int64, errMsg string) {
	global.DB.Model(&SiteStatus{}).Where("source = ?", source).Updates(map[string]any{
		"reachable":   ok,
		"status_code": statusCode,
		"latency_ms":  latencyMS,
		"error":       errMsg,
		"checked_at":  time.Now(),
	})
}
