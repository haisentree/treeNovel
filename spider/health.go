package spider

import (
	"io"
	"net/http"
	"time"
)

// SiteCheckResult 一次站点探测的结果。
type SiteCheckResult struct {
	OK         bool
	StatusCode int
	LatencyMS  int64
	Err        string
}

// CheckSite 探测适配器声明的首个域名，跟随跳转，
// 响应 2xx/3xx 视为可达。先试 https，网络层失败（部分站点只开 http）
// 再回退 http 重试一次。
//
// 注意：可达只说明域名活着——被注册商停放的域名也可能返回 200；
// 适配器选择器是否仍能解析页面，要用小章节数的试爬任务验证。
func CheckSite(ad SiteAdapter) SiteCheckResult {
	domains := ad.Domains()
	if len(domains) == 0 {
		return SiteCheckResult{Err: "适配器未声明域名"}
	}

	r := probe("https://" + domains[0])
	if !r.OK && r.StatusCode == 0 && r.Err != "" {
		r = probe("http://" + domains[0])
	}
	return r
}

func probe(target string) SiteCheckResult {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return SiteCheckResult{Err: err.Error()}
	}
	req.Header.Set("User-Agent", defaultUA)

	client := &http.Client{Timeout: 12 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return SiteCheckResult{LatencyMS: latency.Milliseconds(), Err: err.Error()}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10)) // 少量读取即断开

	return SiteCheckResult{
		OK:         resp.StatusCode < 400,
		StatusCode: resp.StatusCode,
		LatencyMS:  latency.Milliseconds(),
	}
}
