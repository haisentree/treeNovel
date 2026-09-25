package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"treeNovel/global"
)

// CodeSiteNames 内置适配器名集合，由入口（cmd/server）启动时注入，
// 供新增自定义站点时查重（models 不能反向 import sources，避免循环依赖）。
var CodeSiteNames []string

// SiteStatus 爬虫站点的管理状态与健康检查结果。
// 代码适配器在启动时由 EnsureSite 按注册表同步建行；
// Custom=true 的行是页面上添加的自定义站点，解析规则也存库。
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

	// ---- 自定义站点（Custom=true）的解析规则，空值用笔趣阁系默认 ----
	Custom      bool
	ListSel     string // 目录章节链接选择器，默认 #list dd a
	TitleSel    string // 章节标题选择器，默认 h1
	ContentSel  string // 正文容器选择器，默认 #content
	ContentAlignment string // 段落切分方式：br（<br> 分段，默认）| p（按 <p> 元素）
	Watermark   string // 水印关键词，命中的段落会被过滤
}

func (s *SiteStatus) TableName() string { return "site_statuses" }

// CreateCustomSite 新增自定义站点；名字与内置适配器或已有行冲突时报错。
func CreateCustomSite(row SiteStatus) error {
	for _, name := range CodeSiteNames {
		if row.Source == name {
			return fmt.Errorf("站点名 %s 与内置适配器冲突", row.Source)
		}
	}
	var exist SiteStatus
	if err := global.DB.Where("source = ?", row.Source).First(&exist).Error; err == nil {
		return fmt.Errorf("站点名 %s 已存在", row.Source)
	}
	row.Custom = true
	row.Enabled = true
	return global.DB.Create(&row).Error
}

// DeleteCustomSite 删除自定义站点行（内置站点的行不可删）。
func DeleteCustomSite(source string) error {
	res := global.DB.Unscoped().
		Where("source = ? AND custom = ?", source, true).Delete(&SiteStatus{})
	if res.RowsAffected == 0 {
		return fmt.Errorf("站点 %s 不存在或不是自定义站点", source)
	}
	return res.Error
}

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
