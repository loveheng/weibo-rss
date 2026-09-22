// Package upstream 提供面向「受限上游 API」的公共抓取基础设施，
// 供各订阅源复用，避免每个源重复实现：
//   - client.go：出站代理、超时、移动端 UA 与基础头部、可插拔 Cookie 提供器；
//   - fetcher.go：将 throttler（串行熔断）与 anticrawl（重试/风控钩子）
//     组合成通用请求骨架（Do / JSON）；
//   - anticrawl.go：请求重试、风控响应识别与命中后的钩子/熔断回调链；
//   - throttler.go：串行队列 + 熔断冷却限流器。
package upstream

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// MobileUA 为默认的移动端 UA。
const MobileUA = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36"

// DefaultTimeout 为单次上游请求的默认超时。
const DefaultTimeout = 9 * time.Second

// Options 为客户端配置。
type Options struct {
	// Timeout 为单次请求超时，<=0 时使用 DefaultTimeout
	Timeout time.Duration
	// ProxyURL 为出站代理（如 http://user:pass@host:port），为空则直连
	ProxyURL string
	// UserAgent 为请求 UA，为空则使用 MobileUA
	UserAgent string
	// BaseHeaders 为每次请求都附带的基础头部（可被单次请求覆盖）
	BaseHeaders map[string]string
	// Cookie 为可插拔的 Cookie 提供器（如微博访客 Cookie 轮换），可为 nil
	Cookie func() string
}

// Client 是通用上游 HTTP 客户端。
type Client struct {
	http        *http.Client
	ua          string
	baseHeaders map[string]string
	cookie      func() string
}

// NewClient 创建客户端；代理配置非法时记录告警并直连。
func NewClient(opts Options, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if opts.ProxyURL != "" {
		if u, err := url.Parse(opts.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		} else {
			log.Warn("[upstream] invalid proxy config, ignored", "proxy", opts.ProxyURL)
		}
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = MobileUA
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		http:        &http.Client{Transport: transport, Timeout: timeout},
		ua:          ua,
		baseHeaders: opts.BaseHeaders,
		cookie:      opts.Cookie,
	}
}

// Do 发起上游请求：附带 UA、基础头部与 Cookie，headers 中的同名键覆盖基础头部；
// body 为请求体（GET 请求传 nil）。
func (c *Client) Do(ctx context.Context, method, rawURL string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	for k, v := range c.baseHeaders {
		req.Header.Set(k, v)
	}
	if c.cookie != nil {
		if cookie := c.cookie(); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}
