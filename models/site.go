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

	// ---- 解析规则（Custom=true 的自定义站点；内置站点为覆盖值，
	// ---- 空值表示用代码适配器里的默认规则）----
	Custom           bool
	ListSel          string // 目录章节链接选择器，默认 #list dd a
	TitleSel         string // 章节标题选择器，默认 h1
	ContentSel       string // 正文容器选择器，默认 #content
	ContentAlignment string // 段落切分方式：br（<br> 分段，默认）| p（按 <p> 元素）
	Watermark        string // 水印关键词，命中的段落会被过滤
	CheckURL         string // 书籍示例页 URL：编辑页留存，供再次验证解析用

	// DomainsEdited 用户在页面上改过域名：EnsureSite 启动同步时
	// 不再用代码里的域名覆盖（站点换域名后无需改代码）。
	DomainsEdited bool
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

// FindSite 按 source 取单行（软删除的行查不到）。
func FindSite(source string) (SiteStatus, bool) {
	var row SiteStatus
	if err := global.DB.Where("source = ?", source).First(&row).Error; err != nil {
		return row, false
	}
	return row, true
}

// UpdateSiteDomains 内置站点改域名与解析规则覆盖：解析逻辑在代码里，
// 页面改动的是域名、选择器覆盖值与书籍示例 URL。
// DomainsEdited 置位后，EnsureSite 启动同步不再用代码域名覆盖。
func UpdateSiteDomains(source string, row SiteStatus) error {
	res := global.DB.Model(&SiteStatus{}).
		Where("source = ? AND custom = ?", source, false).
		Updates(map[string]any{
			"domains":           row.Domains,
			"domains_edited":    true,
			"list_sel":          row.ListSel,
			"title_sel":         row.TitleSel,
			"content_sel":       row.ContentSel,
			"content_alignment": row.ContentAlignment,
			"watermark":         row.Watermark,
			"check_url":         row.CheckURL,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("站点 %s 不存在或不是内置站点", source)
	}
	return nil
}

// UpdateCustomSite 自定义站点改域名与解析规则（站点标识不可改，
// 否则会和已入库书籍的 Source 归属断开）。
func UpdateCustomSite(source string, row SiteStatus) error {
	res := global.DB.Model(&SiteStatus{}).
		Where("source = ? AND custom = ?", source, true).
		Updates(map[string]any{
			"domains":           row.Domains,
			"list_sel":          row.ListSel,
			"title_sel":         row.TitleSel,
			"content_sel":       row.ContentSel,
			"content_alignment": row.ContentAlignment,
			"watermark":         row.Watermark,
			"check_url":         row.CheckURL,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("站点 %s 不存在或不是自定义站点", source)
	}
	return nil
}

// DeleteSite 删除站点：自定义站点连解析规则一起硬删；
// 内置站点软删作墓碑——EnsureSite 启动同步时不再重建，
// 效果等价于永久移除。已入库的书籍不受影响，只是无法再创建任务。
func DeleteSite(source string) error {
	var row SiteStatus
	if err := global.DB.Where("source = ?", source).First(&row).Error; err != nil {
		return fmt.Errorf("站点 %s 不存在", source)
	}
	if row.Custom {
		res := global.DB.Unscoped().Delete(&row)
		return res.Error
	}
	return global.DB.Delete(&row).Error
}

// EnsureSite 启动时同步：新适配器补行（默认启用），已有行只刷新域名，
// 不动启停状态与检测结果。
// 用户在页面上删除过的内置站点（软删墓碑）不重建；
// 用户改过域名的行（DomainsEdited）不被代码域名覆盖。
func EnsureSite(source, domains string) {
	var row SiteStatus
	if err := global.DB.Unscoped().Where("source = ?", source).First(&row).Error; err == nil {
		if row.DeletedAt.Valid {
			return
		}
		if !row.DomainsEdited && row.Domains != domains {
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

// IsSiteEnabled 站点是否启用；无记录时视为启用（新适配器未同步的防御），
// 已删除的内置站点（软删墓碑）视为停用。
func IsSiteEnabled(source string) bool {
	var row SiteStatus
	if err := global.DB.Unscoped().Where("source = ?", source).First(&row).Error; err != nil {
		return true
	}
	return row.Enabled && !row.DeletedAt.Valid
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
