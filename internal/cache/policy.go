package cache

// 本文件对应 TS 版 feedCache.ts：
// 各数据源通过 Policy 声明自己的 key 前缀与各层 TTL，
// 共享同一套「memo + 请求合并」组合逻辑。

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

// DefaultInfoTTL / DefaultListTTL 为源策略未指定时的兜底 TTL。
const (
	DefaultInfoTTL = 15 * time.Minute
	DefaultListTTL = 15 * time.Minute
)

// SourcePolicy 为数据源级缓存策略（接口结果缓存）。
type SourcePolicy struct {
	// KeyPrefix 为缓存 key 前缀，如 "instagram-info-"
	KeyPrefix string
	// InfoTTL 为用户信息类结果的 TTL，0 表示使用默认值
	InfoTTL time.Duration
	// ListTTL 为内容列表类结果的 TTL，0 表示使用默认值
	ListTTL time.Duration
}

// FeedPolicy 为 RSS XML 输出级缓存策略。
type FeedPolicy struct {
	// XMLKeyPrefix 为 XML 缓存 key 前缀，如 "instagram-xml-"
	XMLKeyPrefix string
	// XMLTTL 为 XML 缓存 TTL，0 表示使用默认值
	XMLTTL time.Duration
	// Collapse 为 false 时禁用请求合并
	Collapse bool
}

// MemoInfo 按策略缓存「用户信息」类结果，key 形如 `<prefix>info-<ident>`。
func MemoInfo[T any](ctx context.Context, c *Cache, p SourcePolicy, ident string, fetch func(ctx context.Context) (T, error)) (T, error) {
	ttl := p.InfoTTL
	if ttl <= 0 {
		ttl = DefaultInfoTTL
	}
	return Memo(ctx, c, p.KeyPrefix+"info-"+ident, ttl, fetch)
}

// MemoList 按策略缓存「内容列表」类结果，key 形如 `<prefix>list-<ident>`。
func MemoList[T any](ctx context.Context, c *Cache, p SourcePolicy, ident string, fetch func(ctx context.Context) (T, error)) (T, error) {
	ttl := p.ListTTL
	if ttl <= 0 {
		ttl = DefaultListTTL
	}
	return Memo(ctx, c, p.KeyPrefix+"list-"+ident, ttl, fetch)
}

type feedResult struct {
	xml  string
	miss bool
}

// CachedFeed 为 RSS 路由通用的「请求合并 + XML 缓存」封装，等价于 TS 版 cachedFeed：
// collapse 未命中且 memo 未命中时才真正执行 fetchFeed；
// 返回值 miss 仅在真正生产数据时为 true（与原版 cacheMiss 语义一致）。
func CachedFeed(
	ctx context.Context,
	c *Cache,
	g *singleflight.Group,
	p FeedPolicy,
	ident string,
	fetchFeed func(ctx context.Context) (string, error),
) (xml string, miss bool, err error) {
	run := func() (feedResult, error) {
		xml, err := fetchFeed(ctx)
		if err != nil {
			return feedResult{}, err
		}
		// xml 为空视为该次无数据：仍写缓存，避免空结果击穿上游
		return feedResult{xml: xml, miss: xml != ""}, nil
	}

	if !p.Collapse {
		res, err := run()
		return res.xml, res.miss, err
	}

	v, err, _ := g.Do(p.XMLKeyPrefix+ident, func() (any, error) {
		return run()
	})
	if err != nil {
		return "", false, err
	}
	res := v.(feedResult)
	return res.xml, res.miss, nil
}
