package sources

import (
	"fmt"

	"treeNovel/spider"
)

// all 全部已注册的站点适配器，新适配器在这里登记。
var all = []spider.SiteAdapter{
	KunnuAdapter{},
	Biqu22Adapter{},
}

// Get 按名称（适配器 Name()）取适配器。
func Get(name string) (spider.SiteAdapter, error) {
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
