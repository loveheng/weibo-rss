// Package weibo 实现微博 RSS 数据源。
//
// 机制移植自 TS 版 modules/weibo/*：
//   - 访客 Cookie 轮换（genvisitor2 提取 SUB，30 分钟周期，静默失败）；
//   - 个人账号 Cookie 兜底（配置了 WEIBO_COOKIE 时优先使用）；
//   - 出站代理支持；四个上游接口各自持有独立的串行熔断限流器。
package weibo

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zgq354/weibo-rss/internal/anticrawl"
	"github.com/zgq354/weibo-rss/internal/config"
)

const (
	// Timeout 为单次上游请求超时（原版 3000*3ms）。
	Timeout = 9 * time.Second
	// MockUA 为移动端 UA。
	MockUA = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36"
)

// Client 为微博上游 HTTP 客户端，管理访客 Cookie 的获取与轮换。
type Client struct {
	http           *http.Client
	log            *slog.Logger
	cfg            config.Config
	mu             sync.Mutex
	visitorCookie  string
	riskyHook      *anticrawl.Hooks
	rotationCancel context.CancelFunc
}

// NewClient 创建客户端；配置了 WEIBO_PROXY 时走出站代理。
func NewClient(cfg config.Config, log *slog.Logger) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.WeiboProxy != "" {
		if u, err := url.Parse(cfg.WeiboProxy); err == nil {
			transport.Proxy = http.ProxyURL(u)
		} else {
			log.Warn("[weibo] invalid proxy config, ignored", "proxy", cfg.WeiboProxy)
		}
	}
	return &Client{
		http: &http.Client{Transport: transport, Timeout: Timeout},
		log:  log,
		cfg:  cfg,
	}
}

// Hooks 返回微博源的风控钩子：403/418 视为风控，命中后先刷新访客 Cookie。
func (c *Client) Hooks() *anticrawl.Hooks {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.riskyHook == nil {
		c.riskyHook = &anticrawl.Hooks{
			RiskyStatuses:  []int{403, 418},
			OnRiskDetected: c.RefreshVisitorCookie,
		}
	}
	return c.riskyHook
}

// cookie 返回请求应携带的 Cookie：个人账号 Cookie 优先，否则使用访客 Cookie。
func (c *Client) cookie() string {
	if c.cfg.WeiboCookie != "" {
		return c.cfg.WeiboCookie
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.visitorCookie
}

// do 发起上游请求，自动附带公共头部与 Cookie。
func (c *Client) do(ctx context.Context, method, rawURL string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", MockUA)
	req.Header.Set("Referer", "https://m.weibo.cn/")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	if cookie := c.cookie(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.http.Do(req)
}

// RefreshVisitorCookie 请求 genvisitor2 获取访客 Cookie（SUB）；
// 失败时静默（依赖后续请求的重试机制兜底），与原版语义一致。
func (c *Client) RefreshVisitorCookie(ctx context.Context) error {
	form := url.Values{
		"cb":   {"visitor_gray_callback"},
		"tid":  {""},
		"from": {"weibo"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://visitor.passport.weibo.cn/visitor/genvisitor2", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", MockUA)

	resp, err := c.http.Do(req)
	if err != nil {
		c.log.Warn("[visitor] refresh cookie failed", "err", err)
		return nil
	}
	defer resp.Body.Close()

	var sub []string
	for _, ck := range resp.Cookies() {
		if strings.HasPrefix(ck.Name, "SUB") {
			sub = append(sub, ck.Name+"="+ck.Value)
		}
	}
	if len(sub) > 0 {
		c.mu.Lock()
		c.visitorCookie = strings.Join(sub, "; ")
		c.mu.Unlock()
		c.log.Debug("[visitor] cookie refreshed")
	}
	return nil
}

// StartCookieRotation 启动访客 Cookie 周期轮换，阻塞至 ctx 取消。
func (c *Client) StartCookieRotation(ctx context.Context, interval time.Duration) {
	_ = c.RefreshVisitorCookie(ctx) // 启动时先获取一次
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.RefreshVisitorCookie(ctx)
		}
	}
}

// Close 停止后台轮换（预留，轮换生命周期由调用方 ctx 控制）。
func (c *Client) Close() {
	if c.rotationCancel != nil {
		c.rotationCancel()
	}
}
