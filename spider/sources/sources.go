package sources

import (
	"errors"
	"fmt"
	"strings"

	"treeNovel/global"
	"treeNovel/models"
	"treeNovel/spider"
)

// ErrSiteDeleted 站点在管理页被删除（内置站点为软删墓碑）。
var ErrSiteDeleted = errors.New("站点已删除")

// all 全部已注册的站点适配器，新适配器在这里登记。
var all = []spider.SiteAdapter{
	KunnuAdapter{},
	Biqu22Adapter{},
	IbiquguAdapter{},
	XbiquwkAdapter{},
}

// splitDomains 按逗号/空格拆分域名列表（全角逗号兼容）。
func splitDomains(s string) []string {
	var domains []string
	for _, d := range strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == '，' || r == ' '
	}) {
		if d = strings.TrimSpace(d); d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// domainOverride 内置适配器的域名覆盖：用户在站点管理改过域名后
// （DomainsEdited），引擎允许域名与健康检查都以库里的为准。
type domainOverride struct {
	spider.SiteAdapter
	domains []string
}

func (o domainOverride) Domains() []string { return o.domains }

// applyDBOverride 按库中站点行调整代码适配器：
// 行已被用户删除（软删墓碑）→ 报错，不再可用；
// 页面填过解析规则覆盖 → 包装为选择器覆盖后的适配器（空规则不包装）；
// 用户改过域名 → 再包一层域名覆盖；
// 无行（未同步）或均未改过 → 原样返回。
func applyDBOverride(ad spider.SiteAdapter) (spider.SiteAdapter, error) {
	if global.DB == nil {
		return ad, nil
	}
	var row models.SiteStatus
	if err := global.DB.Unscoped().Where("source = ?", ad.Name()).First(&row).Error; err != nil {
		return ad, nil
	}
	if row.DeletedAt.Valid {
		return nil, fmt.Errorf("%w：%s", ErrSiteDeleted, ad.Name())
	}
	ov := Selectors{
		List:      row.ListSel,
		Title:     row.TitleSel,
		Content:   row.ContentSel,
		Mode:      row.ContentAlignment,
		Watermark: row.Watermark,
	}
	if w, ok := NewSelectorOverride(ad, ov); ok {
		ad = w
	}
	if row.DomainsEdited {
		if ds := splitDomains(row.Domains); len(ds) > 0 {
			return domainOverride{SiteAdapter: ad, domains: ds}, nil
		}
	}
	return ad, nil
}

// Get 按名称（适配器 Name()）取适配器。
// 内置站点若在页面上被删除，或改过域名/解析规则，此处会相应报错/覆盖。
func Get(name string) (spider.SiteAdapter, error) {
	for _, a := range all {
		if a.Name() == name {
			return applyDBOverride(a)
		}
	}
	return nil, fmt.Errorf("未知适配器 %q，可用: %v", name, Names())
}

// GetRaw 取未叠加库内覆盖的裸代码适配器：编辑页验证需要以
// 「表单当前值」为准构建适配器，而不是库里的旧覆盖。
func GetRaw(name string) (spider.SiteAdapter, error) {
	for _, a := range all {
		if a.Name() == name {
			return a, nil
		}
	}
	return nil, fmt.Errorf("未知适配器 %q，可用: %v", name, Names())
}

// Names 返回全部适配器名称。
func Names() []string {
	names := make([]string, 0, len(all))
	for _, a := range all {
		names = append(names, a.Name())
	}
	return names
}

// All 返回全部已注册适配器（供站点管理与健康检查使用），
// 页面上已删除的内置站点被排除。
func All() []spider.SiteAdapter {
	out := make([]spider.SiteAdapter, 0, len(all))
	for _, a := range all {
		ad, err := applyDBOverride(a)
		if err != nil {
			continue
		}
		out = append(out, ad)
	}
	return out
}
