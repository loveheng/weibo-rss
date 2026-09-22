// Package cache 提供「LRU + 每 key TTL」的纯内存缓存。
//
// 设计机制移植自 TS 版 MemoryCache：
//   - 容量上限内按 LRU 淘汰；
//   - 每个条目可携带独立 TTL（未指定时使用默认值）；
//   - 命中读取不续期（语义与原版 lru-cache 的默认配置一致）。
package cache

import (
	"context"
	"log/slog"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// DefaultTTL 为调用方未指定 TTL 时的默认过期时间。
const DefaultTTL = 20 * time.Minute

type entry struct {
	value    any
	expireAt time.Time
}

// Cache 是并发安全的 LRU + TTL 缓存。
type Cache struct {
	lru        *lru.Cache[string, *entry]
	maxEntries int
	log        *slog.Logger
}

// New 创建缓存；maxEntries 为容量上限（原版默认 1000）。
func New(maxEntries int, log *slog.Logger) *Cache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	if log == nil {
		log = slog.Default()
	}
	c, err := lru.New[string, *entry](maxEntries)
	if err != nil {
		// 仅在 maxEntries 非法时发生，属编程错误
		panic(err)
	}
	return &Cache{lru: c, maxEntries: maxEntries, log: log}
}

// Set 写入条目；ttl <= 0 时使用 DefaultTTL。
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	c.log.Debug("[cache] set", "key", key)
	c.lru.Add(key, &entry{value: value, expireAt: time.Now().Add(ttl)})
}

// Get 读取条目，过期条目惰性剔除。
func (c *Cache) Get(key string) (any, bool) {
	e, ok := c.lru.Get(key)
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expireAt) {
		c.lru.Remove(key)
		return nil, false
	}
	return e.value, true
}

// Memo 等价于原版 CacheInterface.memo：
// 命中直接返回；未命中执行 fetch，成功后写缓存（失败不缓存）。
func Memo[T any](ctx context.Context, c *Cache, key string, ttl time.Duration, fetch func(ctx context.Context) (T, error)) (T, error) {
	if v, ok := c.Get(key); ok {
		if val, ok := v.(T); ok {
			return val, nil
		}
		// 类型不符按未命中处理（理论上仅发生在 key 复用冲突时）
	}
	val, err := fetch(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	c.Set(key, val, ttl)
	return val, nil
}

// Len 返回当前条目数（用于监控）。
func (c *Cache) Len() int { return c.lru.Len() }

// MaxEntries 返回容量上限。
func (c *Cache) MaxEntries() int { return c.maxEntries }
