// Package source 定义订阅源的统一抽象。
//
// 新增一个订阅源的步骤：
//  1. 在 internal/source/<name>/ 实现一个满足 Feed 接口的服务；
//  2. 在 cmd/server/main.go 的 Deps.Sources 中注册；
//  3. 路由注册、XML 缓存、请求合并、错误映射均由 web 层通用逻辑处理。
package source

import (
	"context"
	"errors"
	"net/http"

	"github.com/zgq354/weibo-rss/internal/cache"
	"github.com/zgq354/weibo-rss/internal/feed"
)

// 通用错误，web 层据此映射 HTTP 状态码；各源应使用 %w 包装返回。
var (
	// ErrInvalidID 表示订阅标识（uid/用户名等）格式不合法
	ErrInvalidID = errors.New("source: invalid id")
	// ErrNotFound 表示目标不存在或不可订阅
	ErrNotFound = errors.New("source: not found")
)

// Feed 是一个 RSS 订阅源的抽象。
type Feed interface {
	// Name 返回源标识，用于日志。
	Name() string
	// Route 返回 RSS 路由模板（Go 1.22 ServeMux 风格），如 "/rss/user/{id}"；
	// 模板中的 {id} 即订阅标识的路径参数名。
	Route() string
	// Policy 返回该源 RSS XML 的缓存策略。
	Policy() cache.FeedPolicy
	// Validate 校验订阅标识格式；不合法时返回错误，
	// 错误文本将直接作为 404 响应体返回给用户。
	Validate(id string) error
	// Fetch 拉取数据并组装频道；错误应包装 ErrNotFound（→404），
	// 或透传 throttler.ErrThrottled / anticrawl.ErrRisky（→503）。
	Fetch(ctx context.Context, id string) (*feed.Channel, error)
	// NotFoundMessage 返回目标不存在时的用户文案（404 响应体）。
	NotFoundMessage(id string) string
}

// ExtraRoutes 是可选扩展接口：源需要注册 RSS 之外的
// 特有接口（如微博的 domain2uid）时实现它。
type ExtraRoutes interface {
	RegisterExtra(mux *http.ServeMux)
}
