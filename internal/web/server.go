// Package web 提供 HTTP 路由与请求处理。
//
// 路由层与具体订阅源解耦：
//   - 每个 source.Feed 的 RSS 路由由 registerFeed 通用逻辑注册；
//   - 源特有接口通过 source.ExtraRoutes 可选扩展（如微博的 domain2uid）；
//   - XML 缓存、请求合并、错误到 HTTP 状态码的映射均为统一实现。
package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/zgq354/weibo-rss/internal/anticrawl"
	"github.com/zgq354/weibo-rss/internal/cache"
	"github.com/zgq354/weibo-rss/internal/feed"
	"github.com/zgq354/weibo-rss/internal/httputil"
	"github.com/zgq354/weibo-rss/internal/source"
	"github.com/zgq354/weibo-rss/internal/throttler"
)

// Deps 为路由层依赖。
type Deps struct {
	Cache    *cache.Cache
	Collapse *singleflight.Group
	Sources  []source.Feed
	Log      *slog.Logger
}

// reqState 记录单次请求的缓存命中情况（用于访问日志）。
type reqState struct {
	hit int
}

type ctxKey struct{}

// NewHandler 组装全部路由。
func NewHandler(d Deps) http.Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Collapse == nil {
		d.Collapse = &singleflight.Group{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/cache-stats", d.handleCacheStats)

	for _, s := range d.Sources {
		d.registerFeed(mux, s)
		if er, ok := s.(source.ExtraRoutes); ok {
			er.RegisterExtra(mux)
		}
	}

	return d.logMiddleware(mux)
}

// registerFeed 注册一个订阅源的 RSS 路由（通用逻辑）。
func (d *Deps) registerFeed(mux *http.ServeMux, s source.Feed) {
	mux.HandleFunc("GET "+s.Route(), func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.Validate(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		xmlData, miss, err := cache.CachedFeed(r.Context(), d.Cache, d.Collapse, s.Policy(), id,
			func(ctx context.Context) (string, error) {
				ch, err := s.Fetch(ctx, id)
				if err != nil {
					return "", err
				}
				return feed.BuildRSS(*ch)
			})
		if err != nil {
			d.handleFeedError(w, s, id, err)
			return
		}

		writeXML(w, xmlData)
		setState(r).hit = boolToInt(!miss)
	})
}

// handleFeedError 将源错误统一映射为 HTTP 响应。
func (d *Deps) handleFeedError(w http.ResponseWriter, s source.Feed, id string, err error) {
	switch {
	case errors.Is(err, source.ErrNotFound):
		http.Error(w, s.NotFoundMessage(id), http.StatusNotFound)
	case errors.Is(err, throttler.ErrThrottled), errors.Is(err, anticrawl.ErrRisky):
		http.Error(w, "暂时无法拉取到数据，请稍后再试。", http.StatusServiceUnavailable)
	default:
		d.Log.Error("feed error", "source", s.Name(), "id", id, "err", err)
		http.Error(w, "未知错误，需管理员检查日志。", http.StatusInternalServerError)
	}
}

// handleCacheStats 处理 GET /admin/cache-stats。
func (d *Deps) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"stats": map[string]any{
			"size": d.Cache.Len(),
			"max":  d.Cache.MaxEntries(),
		},
	})
}

// logMiddleware 记录请求耗时与状态。
func (d *Deps) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		ctx := context.WithValue(r.Context(), ctxKey{}, &reqState{})
		next.ServeHTTP(rec, r.WithContext(ctx))
		st, _ := ctx.Value(ctxKey{}).(*reqState)
		hit := 0
		if st != nil {
			hit = st.hit
		}
		d.Log.Info("request",
			"status", rec.status,
			"method", r.Method,
			"path", r.URL.RequestURI(),
			"ip", r.RemoteAddr,
			"hit", hit,
			"duration", time.Since(start).String(),
		)
	})
}

// setState 取出（必要时创建）请求状态。
func setState(r *http.Request) *reqState {
	if st, ok := r.Context().Value(ctxKey{}).(*reqState); ok {
		return st
	}
	return &reqState{}
}

// recorder 记录响应状态码。
type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func writeXML(w http.ResponseWriter, xml string) {
	httputil.WriteXML(w, xml)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	httputil.WriteJSON(w, status, body)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
