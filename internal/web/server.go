// Package web 提供 HTTP 路由与请求处理，对应 TS 版 routes.ts。
package web

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/zgq354/weibo-rss"
	"github.com/zgq354/weibo-rss/internal/anticrawl"
	"github.com/zgq354/weibo-rss/internal/cache"
	"github.com/zgq354/weibo-rss/internal/config"
	"github.com/zgq354/weibo-rss/internal/feed"
	"github.com/zgq354/weibo-rss/internal/source/instagram"
	"github.com/zgq354/weibo-rss/internal/source/weibo"
	"github.com/zgq354/weibo-rss/internal/throttler"
)

// 各 RSS 输出的 XML 缓存策略（Instagram 拉长到 1 小时保护出口 IP）。
var (
	weiboFeedPolicy     = cache.FeedPolicy{XMLKeyPrefix: "xml-", XMLTTL: config.RSSXMLTTL, Collapse: true}
	instagramFeedPolicy = cache.FeedPolicy{XMLKeyPrefix: "instagram-xml-", XMLTTL: config.InstagramTTL, Collapse: true}
)

var (
	uidRe      = regexp.MustCompile(`^[0-9]{10}$`)
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9._]{1,30}$`)
	domainRe   = regexp.MustCompile(`^[A-Za-z0-9]{3,20}$`)
)

// Deps 为路由层依赖。
type Deps struct {
	Cache     *cache.Cache
	Collapse  *singleflight.Group
	Weibo     *weibo.Service
	Instagram *instagram.Service
	Cfg       config.Config
	Log       *slog.Logger
}

// reqState 记录单次请求的缓存命中情况（用于访问日志）。
type reqState struct {
	hit int
}

type ctxKey struct{}

// NewHandler 组装全部路由（Go 1.22 ServeMux 方法匹配）。
func NewHandler(d Deps) http.Handler {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Collapse == nil {
		d.Collapse = &singleflight.Group{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /rss/user/{id}", d.handleWeiboFeed)
	mux.HandleFunc("GET /rss/instagram/{username}", d.handleInstagramFeed)
	mux.HandleFunc("GET /api/domain2uid", d.handleDomain2UID)
	mux.HandleFunc("GET /admin/cache-stats", d.handleCacheStats)

	// 静态资源来自嵌入的 public/（根包 weiborss）
	sub, err := fs.Sub(weiborss.Public, "public")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServerFS(sub))

	return d.logMiddleware(mux)
}

// handleWeiboFeed 处理 GET /rss/user/{id}。
func (d *Deps) handleWeiboFeed(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("id")
	if !uidRe.MatchString(uid) {
		http.Error(w, "找不到用户，传入 UID 格式有误。uid: "+uid, http.StatusNotFound)
		return
	}

	xmlData, miss, err := cache.CachedFeed(r.Context(), d.Cache, d.Collapse, weiboFeedPolicy, uid,
		func(ctx context.Context) (string, error) {
			data, err := d.Weibo.FetchUserLatestWeibo(ctx, uid)
			if err != nil {
				return "", err
			}
			items := make([]feed.Item, 0, len(data.StatusList))
			for _, st := range data.StatusList {
				if st == nil {
					continue
				}
				items = append(items, feed.Item{
					Title:       weibo.FeedTitle(st),
					Description: weibo.StatusToHTML(d.Cfg, st),
					Link:        "https://weibo.com/" + uid + "/" + st.Bid,
					Time:        weibo.ParseWeiboTime(st.CreatedAt),
				})
			}
			return feed.BuildRSS(
				"https://weibo.com/"+uid,
				data.ScreenName+"的微博",
				data.Description,
				items,
			)
		})
	if err != nil {
		d.handleFeedError(w, err, "uid: "+uid)
		return
	}

	writeXML(w, xmlData)
	setState(r).hit = boolToInt(!miss)
}

// handleInstagramFeed 处理 GET /rss/instagram/{username}。
func (d *Deps) handleInstagramFeed(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if !usernameRe.MatchString(username) {
		http.Error(w, "用户名格式有误。username: "+username, http.StatusNotFound)
		return
	}

	xmlData, miss, err := cache.CachedFeed(r.Context(), d.Cache, d.Collapse, instagramFeedPolicy, username,
		func(ctx context.Context) (string, error) {
			data, err := d.Instagram.FetchUserLatestPosts(ctx, username)
			if err != nil {
				return "", err
			}
			items := make([]feed.Item, 0, len(data.Media))
			for _, m := range data.Media {
				title := m.Caption
				if runes := []rune(title); len(runes) > 25 {
					title = string(runes[:25])
				}
				items = append(items, feed.Item{
					Title:       title,
					Description: d.Instagram.MediaToHTML(m),
					Link:        "https://www.instagram.com/p/" + m.Shortcode + "/",
					Time:        m.TakenAt,
				})
			}
			return feed.BuildRSS(
				"https://www.instagram.com/"+data.Username+"/",
				data.Name+" (@"+data.Username+") 的 Instagram",
				data.Description,
				items,
			)
		})
	if err != nil {
		d.handleFeedError(w, err, "username: "+username)
		return
	}

	writeXML(w, xmlData)
	setState(r).hit = boolToInt(!miss)
}

// handleDomain2UID 处理 GET /api/domain2uid?domain=xxx。
func (d *Deps) handleDomain2UID(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if !domainRe.MatchString(domain) {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "msg": "找不到用户，可能是地址格式不正确"})
		return
	}

	v, err, _ := d.Collapse.Do("domain:"+domain, func() (any, error) {
		return cache.Memo(r.Context(), d.Cache, "dm-"+domain, config.DomainTTL, func(ctx context.Context) (string, error) {
			return d.Weibo.FetchUIDByDomain(ctx, domain)
		})
	})
	if err != nil {
		if errors.Is(err, weibo.ErrDomainNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "msg": "找不到用户，可能是地址格式不正确"})
			return
		}
		d.Log.Error("domain2uid failed", "domain", domain, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "msg": "获取数据时发生了错误"})
		return
	}
	uid, _ := v.(string)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "uid": uid})
	setState(r).hit = boolToInt(v != nil)
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

// handleFeedError 将数据源错误映射为 HTTP 响应。
func (d *Deps) handleFeedError(w http.ResponseWriter, err error, ident string) {
	switch {
	case errors.Is(err, weibo.ErrUserNotFound):
		http.Error(w,
			"找不到用户，可能用户仅登录可见，不支持订阅。可以通过打开 https://m.weibo.cn/u/:uid 验证（uid: "+ident+"）",
			http.StatusNotFound)
	case errors.Is(err, instagram.ErrUserNotFound):
		http.Error(w, "找不到用户，可能用户名有误、用户不存在或为私密账号。"+ident, http.StatusNotFound)
	case errors.Is(err, throttler.ErrThrottled), errors.Is(err, anticrawl.ErrRisky):
		http.Error(w, "暂时无法拉取到数据，请稍后再试。"+ident, http.StatusServiceUnavailable)
	default:
		d.Log.Error("feed error", "ident", ident, "err", err)
		http.Error(w, "未知错误，需管理员检查日志。"+ident, http.StatusInternalServerError)
	}
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
	w.Header().Set("Content-Type", "text/xml")
	_, _ = w.Write([]byte(xml))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
