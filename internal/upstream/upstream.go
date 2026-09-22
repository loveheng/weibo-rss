// Package upstream 提供面向「受限上游 API」的公共 HTTP 客户端，
// 供各订阅源复用，避免每个源重复实现：
//   - 出站代理、超时、移动端 UA 与基础头部、可插拔 Cookie 提供器；
//   - Fetcher 将 throttler（串行熔断）与 anticrawl（重试/风控钩子）
//     组合成通用请求骨架（Do / JSON）。
package upstream

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/zgq354/weibo-rss/internal/anticrawl"
	"github.com/zgq354/weibo-rss/internal/throttler"
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

// Fetcher 组合 Client 与风控钩子，提供「限流 + 抖动 + 重试 + 风控处理」
// 的通用请求骨架。
type Fetcher struct {
	Client    *Client
	Hooks     *anticrawl.Hooks
	Retries   int
	BaseDelay time.Duration
}

// NewFetcher 创建 Fetcher，填充重试参数默认值（2 次、1s 基准退避）。
func NewFetcher(client *Client, hooks *anticrawl.Hooks) *Fetcher {
	return &Fetcher{Client: client, Hooks: hooks, Retries: 2, BaseDelay: time.Second}
}

// Do 在限流器内执行带抖动、重试与风控处理的请求：
//   - 冷却期内或风控熔断时返回 throttler.ErrThrottled / anticrawl.ErrRisky；
//   - 成功时返回未读取的响应体（调用方负责关闭）。
func (f *Fetcher) Do(ctx context.Context, runner *throttler.Throttler, method, rawURL string, headers map[string]string) (*http.Response, error) {
	var resp *http.Response
	err := runner.Run(ctx, func() error {
		if err := anticrawl.Jitter(ctx, 100*time.Millisecond); err != nil {
			return err
		}
		r, err := anticrawl.DoWithRetry(ctx, f.Hooks, func(ctx context.Context) (*http.Response, error) {
			return f.Client.Do(ctx, method, rawURL, nil, headers)
		}, f.Retries, f.BaseDelay)
		if err != nil {
			return anticrawl.HandleForbidden(ctx, f.Hooks, err, runner.Trip)
		}
		resp = r
		return nil
	})
	return resp, err
}

// JSON 同 Do，并将响应体解码进 out（随后关闭响应体）。
func (f *Fetcher) JSON(ctx context.Context, runner *throttler.Throttler, method, rawURL string, headers map[string]string, out any) error {
	resp, err := f.Do(ctx, runner, method, rawURL, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}
