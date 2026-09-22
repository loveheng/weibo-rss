// Package config 管理运行配置（环境变量）与各层缓存 TTL。
package config

import (
	"net/url"
	"os"
	"time"
)

// 各层缓存 TTL，与 TS 版 config.ts 中的 cacheTTL 保持一致。
const (
	RSSXMLTTL     = 15 * time.Minute   // RSS XML 输出缓存
	StatusListTTL = 15 * time.Minute   // 微博内容列表
	IndexInfoTTL  = 24 * time.Hour     // 用户信息（昵称/容器 id）
	LongTextTTL   = 7 * 24 * time.Hour // 长文全文
	DetailTTL     = 7 * 24 * time.Hour // 详情兜底
	DomainTTL     = 7 * 24 * time.Hour // domain -> uid
	InstagramTTL  = 1 * time.Hour      // Instagram 数据（优先保护出口 IP）
	CookieTTL     = 30 * time.Minute   // 访客 Cookie 轮换周期
)

// RSSFeedTTL 为 RSS feed 的 <ttl> 字段值（分钟）。
const RSSFeedTTL = 15

// Config 为服务运行配置，全部支持环境变量覆盖。
type Config struct {
	Port            string // PORT，默认 3000
	ImageCache      string // 图片反代前缀
	WeiboCookie     string // WEIBO_COOKIE，个人账号 Cookie 兜底
	WeiboProxy      string // WEIBO_PROXY，上游代理 URL
	InstagramCookie string // INSTAGRAM_COOKIE
	InstagramProxy  string // INSTAGRAM_PROXY
}

// Load 从环境变量读取配置。
func Load() Config {
	cfg := Config{
		Port:       envOr("PORT", "3000"),
		ImageCache: envOr("IMAGE_CACHE", "https://image.baidu.com/search/down?url="),
	}
	if v := os.Getenv("WEIBO_COOKIE"); v != "" {
		cfg.WeiboCookie = v
	}
	if v := os.Getenv("WEIBO_PROXY"); v != "" {
		cfg.WeiboProxy = v
	}
	if v := os.Getenv("INSTAGRAM_COOKIE"); v != "" {
		cfg.InstagramCookie = v
	}
	if v := os.Getenv("INSTAGRAM_PROXY"); v != "" {
		cfg.InstagramProxy = v
	}
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// WrapImageURL 包装图片反代前缀；未配置 ImageCache 时返回原始 URL。
func (c Config) WrapImageURL(rawURL string) string {
	if c.ImageCache == "" {
		return rawURL
	}
	return c.ImageCache + url.QueryEscape(rawURL)
}
