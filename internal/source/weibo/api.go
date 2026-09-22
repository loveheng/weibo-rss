package weibo

// 本文件对应 TS 版 modules/weibo/api/*：四个上游接口。
// 每个接口各自持有独立的串行熔断限流器（index/detail/longText/domain），
// 请求前加入 0~100ms 随机抖动，命中风控时先走钩子（刷新 Cookie）再熔断。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/zgq354/weibo-rss/internal/anticrawl"
	"github.com/zgq354/weibo-rss/internal/source"
)

// ErrDomainNotFound 表示自定义域名无法转换为 uid。
var ErrDomainNotFound = errors.New("weibo: domain not found")

// userInfo 为用户信息解析结果。
type userInfo struct {
	ScreenName  string
	Description string
	ContainerID string
}

// indexInfoResp 为 getIndex 用户信息接口的精简结构（脏 JSON 只取必需字段）。
type indexInfoResp struct {
	OK   int `json:"ok"`
	Data struct {
		UserInfo struct {
			ScreenName  string `json:"screen_name"`
			Description string `json:"description"`
		} `json:"userInfo"`
		TabsInfo struct {
			Tabs []struct {
				ContainerID string `json:"containerid"`
			} `json:"tabs"`
		} `json:"tabsInfo"`
	} `json:"data"`
}

// indexListResp 为 getIndex 内容列表接口的精简结构；
// cards 中混有广告/热门卡片（无 mblog），用指针字段自然过滤。
type indexListResp struct {
	OK   int `json:"ok"`
	Data struct {
		Cards []struct {
			Mblog *Status `json:"mblog"`
		} `json:"cards"`
	} `json:"data"`
}

// extendResp 为长文接口的精简结构。
type extendResp struct {
	Data struct {
		LongTextContent string `json:"longTextContent"`
	} `json:"data"`
}

// getIndexUserInfo 拉取用户信息：昵称、简介、内容容器 id。
func (s *Service) getIndexUserInfo(ctx context.Context, uid string) (userInfo, error) {
	var out userInfo
	err := s.indexRunner.Run(ctx, func() error {
		jitter()
		resp, err := anticrawl.DoWithRetry(ctx, s.hooks, func(ctx context.Context) (*http.Response, error) {
			return s.client.do(ctx, http.MethodGet,
				fmt.Sprintf("https://m.weibo.cn/api/container/getIndex?type=uid&value=%s", uid),
				map[string]string{
					"Mweibo-Pwa":       "1",
					"Referer":          "https://m.weibo.cn/u/" + uid,
					"X-Requested-With": "XMLHttpRequest",
				})
		}, 2, time.Second)
		if err != nil {
			return anticrawl.HandleForbidden(ctx, s.hooks, err, s.indexRunner.Trip)
		}
		defer resp.Body.Close()

		var r indexInfoResp
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return err
		}
		if r.OK != 1 || len(r.Data.TabsInfo.Tabs) < 2 {
			return fmt.Errorf("%w: uid %s", source.ErrNotFound, uid)
		}
		out = userInfo{
			ScreenName:  r.Data.UserInfo.ScreenName,
			Description: r.Data.UserInfo.Description,
			ContainerID: r.Data.TabsInfo.Tabs[1].ContainerID,
		}
		return nil
	})
	return out, err
}

// getWeiboContentList 拉取内容列表，过滤出有效微博卡片。
func (s *Service) getWeiboContentList(ctx context.Context, uid, containerID string) ([]*Status, error) {
	var out []*Status
	err := s.indexRunner.Run(ctx, func() error {
		jitter()
		resp, err := anticrawl.DoWithRetry(ctx, s.hooks, func(ctx context.Context) (*http.Response, error) {
			return s.client.do(ctx, http.MethodGet,
				fmt.Sprintf("https://m.weibo.cn/api/container/getIndex?type=uid&value=%s&containerid=%s", uid, containerID),
				map[string]string{
					"Mweibo-Pwa":       "1",
					"Referer":          "https://m.weibo.cn/u/" + uid,
					"X-Requested-With": "XMLHttpRequest",
				})
		}, 2, time.Second)
		if err != nil {
			return anticrawl.HandleForbidden(ctx, s.hooks, err, s.indexRunner.Trip)
		}
		defer resp.Body.Close()

		var r indexListResp
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return err
		}
		out = out[:0]
		for _, card := range r.Data.Cards {
			if card.Mblog != nil {
				out = append(out, card.Mblog)
			}
		}
		return nil
	})
	return out, err
}

// getWeiboLongText 拉取长文全文。
func (s *Service) getWeiboLongText(ctx context.Context, id string) (string, error) {
	var content string
	err := s.longTextRunner.Run(ctx, func() error {
		jitter()
		resp, err := anticrawl.DoWithRetry(ctx, s.hooks, func(ctx context.Context) (*http.Response, error) {
			return s.client.do(ctx, http.MethodGet,
				fmt.Sprintf("https://m.weibo.cn/statuses/extend?id=%s", id),
				map[string]string{
					"Mweibo-Pwa":       "1",
					"Referer":          "https://m.weibo.cn/detail/" + id,
					"X-Requested-With": "XMLHttpRequest",
				})
		}, 2, time.Second)
		if err != nil {
			return anticrawl.HandleForbidden(ctx, s.hooks, err, s.longTextRunner.Trip)
		}
		defer resp.Body.Close()

		var r extendResp
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return err
		}
		if r.Data.LongTextContent == "" {
			return fmt.Errorf("weibo: long text empty, id %s", id)
		}
		content = r.Data.LongTextContent
		return nil
	})
	return content, err
}

// getWeiboDetail 拉取单条微博详情（长文接口失败时的兜底）。
func (s *Service) getWeiboDetail(ctx context.Context, id string) (*Status, error) {
	var out *Status
	err := s.detailRunner.Run(ctx, func() error {
		jitter()
		resp, err := anticrawl.DoWithRetry(ctx, s.hooks, func(ctx context.Context) (*http.Response, error) {
			return s.client.do(ctx, http.MethodGet,
				fmt.Sprintf("https://m.weibo.cn/statuses/show?id=%s", id),
				map[string]string{
					"Mweibo-Pwa":       "1",
					"Referer":          "https://m.weibo.cn/detail/" + id,
					"X-Requested-With": "XMLHttpRequest",
				})
		}, 2, time.Second)
		if err != nil {
			return anticrawl.HandleForbidden(ctx, s.hooks, err, s.detailRunner.Trip)
		}
		defer resp.Body.Close()

		// 详情接口的微博数据位于 data 字段
		var r struct {
			Data *Status `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return err
		}
		if r.Data == nil {
			return fmt.Errorf("weibo: detail empty, id %s", id)
		}
		out = r.Data
		return nil
	})
	return out, err
}

// getUIDByDomain 将自定义域名转换为 uid：
// 请求 m.weibo.cn/<domain> 并跟随重定向，从最终路径 /u/<uid> 提取。
func (s *Service) getUIDByDomain(ctx context.Context, domain string) (string, error) {
	var uid string
	err := s.domainRunner.Run(ctx, func() error {
		jitter()
		resp, err := anticrawl.DoWithRetry(ctx, s.hooks, func(ctx context.Context) (*http.Response, error) {
			return s.client.do(ctx, http.MethodGet,
				fmt.Sprintf("https://m.weibo.cn/%s?&jumpfrom=weibocom", domain),
				map[string]string{"User-Agent": MockUA})
		}, 2, time.Second)
		if err != nil {
			return anticrawl.HandleForbidden(ctx, s.hooks, err, s.domainRunner.Trip)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

		// 与 TS 版 res.request.path.split("/u/")[1] 语义一致
		extracted, ok := extractUID(resp.Request.URL.Path)
		if !ok {
			return fmt.Errorf("%w: domain %s", ErrDomainNotFound, domain)
		}
		uid = extracted
		return nil
	})
	return uid, err
}

// extractUID 从路径中提取 /u/ 之后的 uid 部分。
func extractUID(path string) (string, bool) {
	_, rest, found := strings.Cut(path, "/u/")
	if !found || rest == "" {
		return "", false
	}
	// 截断查询串与尾随路径段
	if i := strings.IndexAny(rest, "?&/"); i >= 0 {
		rest = rest[:i]
	}
	return rest, rest != ""
}

// jitter 引入 0~100ms 随机抖动，与原版 waitMs(random*100) 一致。
func jitter() {
	time.Sleep(time.Duration(rand.IntN(100)) * time.Millisecond)
}
