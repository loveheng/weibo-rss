// Package eastmoney 实现东方财富股吧 RSS 数据源。
//
// 通用 HTTP/重试/风控骨架已下沉到 internal/upstream，
// 本包只保留东方财富特有逻辑：
//   - 个人主页两个接口：发帖列表（postCenterList）与回复列表（myreply）；
//   - 接口为公开数据，默认无需 Cookie，可选配置 EASTMONEY_COOKIE 兜底；
//   - callbackParam 为空时返回标准 JSON，无需 JSONP 剥壳。
package eastmoney

import (
	"log/slog"

	"github.com/loveheng/weibo-rss/internal/cache"
	"github.com/loveheng/weibo-rss/internal/config"
	"github.com/loveheng/weibo-rss/internal/upstream"
)

// desktopUA 为股吧接口使用的桌面端 UA（与浏览器请求保持一致）。
const desktopUA = "Mozilla/5.0 (X11; Linux x86_64; rv:154.0) Gecko/20100101 Firefox/154.0"

// Service 为东方财富数据源服务。
// Service 本身实现 source.Feed（发帖订阅），ReplyFeed 提供回复订阅。
type Service struct {
	cfg     config.Config
	cache   *cache.Cache
	log     *slog.Logger
	runner  *upstream.Throttler
	fetcher *upstream.Fetcher
}

// NewService 创建东方财富数据源服务；配置了 EASTMONEY_PROXY 时走出站代理。
func NewService(cfg config.Config, c *cache.Cache, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	client := upstream.NewClient(upstream.Options{
		ProxyURL:  cfg.EastmoneyProxy,
		UserAgent: desktopUA,
		Cookie:    func() string { return cfg.EastmoneyCookie },
	}, log)
	return &Service{
		cfg:     cfg,
		cache:   c,
		log:     log,
		runner:  upstream.New("eastmoney", log, upstream.DefaultCooldown),
		fetcher: upstream.NewFetcher(client, &upstream.Hooks{}),
	}
}
