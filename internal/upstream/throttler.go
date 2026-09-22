package upstream

// 本文件实现「串行队列 + 熔断冷却」限流器。
//
// 设计机制移植自 TS 版 Throttler：
//   - concurrency = 1：同一时刻只允许一个上游请求在飞（克制原则）；
//   - 熔断：上游命中风控后调用 Trip()，冷却期内所有请求直接失败；
//   - 冷却恢复：按时间戳比较判断恢复（而非定时器重置），多次熔断不会堆叠定时器。

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"
)

// ErrThrottled 表示当前处于熔断冷却期，请求被拒绝。
var ErrThrottled = errors.New("upstream: circuit broken")

// DefaultCooldown 为默认熔断冷却时长。
const DefaultCooldown = 10 * time.Minute

// Throttler 是单并发限流器。
type Throttler struct {
	name     string
	log      *slog.Logger
	cooldown time.Duration
	sem      chan struct{}
	brokenAt atomic.Int64 // 熔断时刻的 UnixMilli，0 表示未熔断
}

// New 创建限流器；cooldown <= 0 时使用 DefaultCooldown。
func New(name string, log *slog.Logger, cooldown time.Duration) *Throttler {
	if cooldown <= 0 {
		cooldown = DefaultCooldown
	}
	if log == nil {
		log = slog.Default()
	}
	return &Throttler{
		name:     name,
		log:      log,
		cooldown: cooldown,
		sem:      make(chan struct{}, 1),
	}
}

// available 判断当前是否可用（熔断是否已过冷却期）。
func (t *Throttler) available() bool {
	at := t.brokenAt.Load()
	if at == 0 {
		return true
	}
	return time.Since(time.UnixMilli(at)) >= t.cooldown
}

// Trip 触发熔断，进入冷却期。
func (t *Throttler) Trip() {
	t.brokenAt.Store(time.Now().UnixMilli())
	t.log.Info("[Throttled] disabled", "name", t.name)
}

// Run 以串行方式执行 fn：
//   - 冷却期内直接返回 ErrThrottled；
//   - 并发调用会排队（容量 1），获取资格后二次校验熔断状态。
func (t *Throttler) Run(ctx context.Context, fn func() error) error {
	if !t.available() {
		return ErrThrottled
	}
	select {
	case t.sem <- struct{}{}:
		defer func() { <-t.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	// 排队等待期间可能已被其他请求熔断
	if !t.available() {
		return ErrThrottled
	}
	return fn()
}
