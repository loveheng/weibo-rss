// Package weibo 实现微博 RSS 数据源。
//
// 通用 HTTP/重试/风控骨架已下沉到 internal/upstream，
// 本包只保留微博特有逻辑：
//   - 访客 Cookie 轮换（genvisitor2 提取 SUB，30 分钟周期，静默失败）；
//   - 个人账号 Cookie 兜底（配置了 WEIBO_COOKIE 时优先使用）；
//   - 四个上游接口各自持有独立的串行熔断限流器。
package weibo

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zgq354/weibo-rss/internal/config"
	"github.com/zgq354/weibo-rss/internal/upstream"
)

// Client 为微博上游客户端：组合公共 HTTP 客户端并管理访客 Cookie 轮换。
type Client struct {
	up            *upstream.Client
	log           *slog.Logger
	cfg           config.Config
	mu            sync.Mutex
	visitorCookie string
	riskyHook     *upstream.Hooks
}

// NewClient 创建客户端；配置了 WEIBO_PROXY 时走出站代理。
func NewClient(cfg config.Config, log *slog.Logger) *Client {
	c := &Client{log: log, cfg: cfg}
	c.up = upstream.NewClient(upstream.Options{
		ProxyURL:  cfg.WeiboProxy,
		UserAgent: upstream.MobileUA,
		BaseHeaders: map[string]string{
			"Referer":         "https://m.weibo.cn/",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		},
		Cookie: c.cookie,
	}, log)
	return c
}

// Up 暴露公共客户端（供 Service 组装 Fetcher）。
func (c *Client) Up() *upstream.Client { return c.up }

// Hooks 返回微博源的风控钩子：403/418 视为风控，命中后先刷新访客 Cookie。
func (c *Client) Hooks() *upstream.Hooks {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.riskyHook == nil {
		c.riskyHook = &upstream.Hooks{
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

// RefreshVisitorCookie 请求 genvisitor2 获取访客 Cookie（SUB）；
// 失败时静默（依赖后续请求的重试机制兜底），与原版语义一致。
func (c *Client) RefreshVisitorCookie(ctx context.Context) error {
	form := url.Values{
		"cb":   {"visitor_gray_callback"},
		"tid":  {""},
		"from": {"weibo"},
	}
	resp, err := c.up.Do(ctx, http.MethodPost,
		"https://visitor.passport.weibo.cn/visitor/genvisitor2",
		strings.NewReader(form.Encode()),
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
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
