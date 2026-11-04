package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// 公开资源先匿名访问，鉴权或限流时再使用已有令牌；跨域下载不转发令牌。
func (p *Publisher) Download(ctx context.Context, endpoint string) (*http.Response, error) {
	client := *p.client
	client.Timeout = 2 * time.Minute
	origin, err := url.Parse(p.base)
	if err != nil {
		return nil, err
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("GitHub 下载重定向次数过多")
		}
		sameOrigin := req.URL.Scheme == origin.Scheme && req.URL.Host == origin.Host
		if !sameOrigin {
			req.Header.Del("Authorization")
			if req.URL.Scheme != "https" || req.URL.Host != "codeload.github.com" {
				return errors.New("GitHub 下载重定向到不受支持的地址")
			}
		}
		return nil
	}
	token := ""
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base+endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "Trylet/0.1")
		req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("连接 GitHub 失败：%w", err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if attempt == 0 && (resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 || resp.StatusCode == 429) {
			token, err = p.token()
			if err != nil {
				return nil, fmt.Errorf("读取 GitHub 令牌失败：%w", err)
			}
			if token != "" {
				continue
			}
		}
		var diagnostic struct{ Message string }
		_ = json.Unmarshal(data, &diagnostic)
		if resp.StatusCode == 404 || resp.StatusCode == 401 {
			diagnostic.Message = "仓库不可访问或为空；私有仓库请在设置中保存有 Contents 读取权限的令牌"
		} else if diagnostic.Message == "" {
			diagnostic.Message = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("GitHub 请求失败（%d）：%s", resp.StatusCode, diagnostic.Message)
	}
	return nil, errors.New("GitHub 鉴权失败")
}
