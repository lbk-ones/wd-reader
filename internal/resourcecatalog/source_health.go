package resourcecatalog

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SourceHealth 是一次访问检测的结果，用于书源列表展示状态和失败原因。
type SourceHealth struct {
	// Status 为 available、blocked 或 unavailable，分别表示可访问、访问受限和访问失败。
	Status string `json:"status"`
	// Detail 保存 HTTP 状态码或请求错误等诊断信息，前端在状态图标的提示中展示。
	Detail string `json:"detail,omitempty"`
}

// CheckSource 在 5 秒内探测书源主页或文件索引。检测只访问探测入口，避免占用站点的搜索配额。
// 检测结果描述该入口的可访问性，搜索和章节请求仍可能受各自的限制。
func CheckSource(ctx context.Context, client *http.Client, source Source) SourceHealth {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// 优先使用配置的主页。未填写主页的成品来源使用文件索引或搜索地址所属站点的首页。
	target := source.HomeURL
	switch {
	case source.Kind == CrawlerKind:
		client = newNovelClient(client)
	case target != "":
	case source.Kind == FileIndexKind:
		target = source.IndexURL
	default:
		u, err := url.Parse(strings.ReplaceAll(source.SearchURL, "{query}", "test"))
		if err != nil {
			return SourceHealth{Status: "unavailable", Detail: err.Error()}
		}
		target = u.Scheme + "://" + u.Host + "/"
	}
	if !IsWebURL(target) {
		return SourceHealth{Status: "unavailable", Detail: "无效的书源地址"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return SourceHealth{Status: "unavailable", Detail: err.Error()}
	}
	req.Header.Set("User-Agent", catalogUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return SourceHealth{Status: "unavailable", Detail: err.Error()}
	}
	defer resp.Body.Close()
	detail := fmt.Sprintf("HTTP %d", resp.StatusCode)
	// 将验证码、登录要求、限流等访问限制与服务器错误分别报告。
	switch {
	case resp.Header.Get("Cf-Mitigated") == "challenge":
		return SourceHealth{Status: "blocked", Detail: detail}
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusUnavailableForLegalReasons:
		return SourceHealth{Status: "blocked", Detail: detail}
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return SourceHealth{Status: "unavailable", Detail: detail}
	}
	// 某些验证页返回 2xx，只读取前 16 KiB 检查页面特征，避免下载整个首页或索引。
	sample, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return SourceHealth{Status: "unavailable", Detail: err.Error()}
	}
	if isChallengePage(sample) {
		return SourceHealth{Status: "blocked", Detail: detail}
	}
	return SourceHealth{Status: "available", Detail: detail}
}

// isChallengePage 检查响应开头是否含常见的浏览器验证标题或脚本路径。
func isChallengePage(data []byte) bool {
	head := bytes.ToLower(data[:min(len(data), 16*1024)])
	return bytes.Contains(head, []byte("/cdn-cgi/challenge-platform/")) ||
		bytes.Contains(head, []byte("<title>just a moment")) ||
		bytes.Contains(head, []byte("<title>attention required! | cloudflare"))
}
