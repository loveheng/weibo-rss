package weibo

// 本文件将微博源接入统一订阅源抽象（internal/source.Feed）：
// 路由模板、标识校验、XML 缓存策略、频道拼装、特有接口（domain2uid）
// 全部收拢在本包内，web 层不感知微博细节。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/zgq354/weibo-rss/internal/cache"
	"github.com/zgq354/weibo-rss/internal/config"
	"github.com/zgq354/weibo-rss/internal/feed"
	"github.com/zgq354/weibo-rss/internal/throttler"
)

var (
	uidRe    = regexp.MustCompile(`^[0-9]{10}$`)
	domainRe = regexp.MustCompile(`^[A-Za-z0-9]{3,20}$`)

	// weiboXMLPolicy 为 RSS XML 输出的缓存策略
	weiboXMLPolicy = cache.FeedPolicy{XMLKeyPrefix: "xml-", XMLTTL: config.RSSXMLTTL, Collapse: true}
)

// Name 实现 source.Feed。
func (s *Service) Name() string { return "weibo" }

// Route 实现 source.Feed：订阅标识为 10 位数字 uid。
func (s *Service) Route() string { return "/rss/user/{id}" }

// Policy 实现源 Feed。
func (s *Service) Policy() cache.FeedPolicy { return weiboXMLPolicy }

// Validate 实现源 Feed。
func (s *Service) Validate(id string) error {
	if !uidRe.MatchString(id) {
		return fmt.Errorf("找不到用户，传入 UID 格式有误。uid: %s", id)
	}
	return nil
}

// NotFoundMessage 实现 source.Feed。
func (s *Service) NotFoundMessage(id string) string {
	return "找不到用户，可能用户仅登录可见，不支持订阅。可以通过打开 https://m.weibo.cn/u/:uid 验证" +
		`（<a href="https://m.weibo.cn/u/` + id + `" target="_blank">uid: ` + id + `</a>）`
}

// Fetch 实现源 Feed：拉取数据并拼装 RSS 频道。
func (s *Service) Fetch(ctx context.Context, id string) (*feed.Channel, error) {
	data, err := s.FetchUserLatestWeibo(ctx, id)
	if err != nil {
		return nil, err
	}
	ch := &feed.Channel{
		SiteURL:     "https://weibo.com/" + id,
		Title:       data.ScreenName + "的微博",
		Description: data.Description,
		Items:       make([]feed.Item, 0, len(data.StatusList)),
	}
	for _, st := range data.StatusList {
		if st == nil {
			continue
		}
		ch.Items = append(ch.Items, feed.Item{
			Title:       FeedTitle(st),
			Description: StatusToHTML(s.cfg, st),
			Link:        "https://weibo.com/" + id + "/" + st.Bid,
			Time:        ParseWeiboTime(st.CreatedAt),
		})
	}
	return ch, nil
}

// RegisterExtra 实现 source.ExtraRoutes：注册微博特有的 domain2uid 接口。
func (s *Service) RegisterExtra(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/domain2uid", s.handleDomain2UID)
}

// handleDomain2UID 处理 GET /api/domain2uid?domain=xxx。
func (s *Service) handleDomain2UID(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if !domainRe.MatchString(domain) {
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "msg": "找不到用户，可能是地址格式不正确"})
		return
	}

	v, err, _ := s.sf.Do("domain:"+domain, func() (any, error) {
		return cache.Memo(r.Context(), s.cache, "dm-"+domain, config.DomainTTL, func(ctx context.Context) (string, error) {
			return s.FetchUIDByDomain(ctx, domain)
		})
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDomainNotFound):
			writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "msg": "找不到用户，可能是地址格式不正确"})
		case errors.Is(err, throttler.ErrThrottled):
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"success": false, "msg": "暂时无法拉取到数据，请稍后再试"})
		default:
			s.log.Error("domain2uid failed", "domain", domain, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "msg": "获取数据时发生了错误"})
		}
		return
	}
	uid, _ := v.(string)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "uid": uid})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
