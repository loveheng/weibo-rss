package weibo

// 本文件对应 TS 版 modules/weibo/api/*：四个上游接口。
// 通用骨架（限流/抖动/重试/风控）由 upstream.Fetcher 提供，
// 这里只保留各接口的 URL、头部与响应解析。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/loveheng/weibo-rss/internal/source"
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

// indexHeaders 为 getIndex 系列接口的公共附加头部。
func indexHeaders(referer string) map[string]string {
	return map[string]string{
		"Mweibo-Pwa":       "1",
		"Referer":          referer,
		"X-Requested-With": "XMLHttpRequest",
	}
}

// getIndexUserInfo 拉取用户信息：昵称、简介、内容容器 id。
func (s *Service) getIndexUserInfo(ctx context.Context, uid string) (userInfo, error) {
	var r indexInfoResp
	err := s.fetcher.JSON(ctx, s.indexRunner, http.MethodGet,
		fmt.Sprintf("https://m.weibo.cn/api/container/getIndex?type=uid&value=%s", uid),
		indexHeaders("https://m.weibo.cn/u/"+uid), &r)
	if err != nil {
		return userInfo{}, err
	}
	if r.OK != 1 || len(r.Data.TabsInfo.Tabs) < 2 {
		return userInfo{}, fmt.Errorf("%w: uid %s", source.ErrNotFound, uid)
	}
	return userInfo{
		ScreenName:  r.Data.UserInfo.ScreenName,
		Description: r.Data.UserInfo.Description,
		ContainerID: r.Data.TabsInfo.Tabs[1].ContainerID,
	}, nil
}

// getWeiboContentList 拉取内容列表，过滤出有效微博卡片。
func (s *Service) getWeiboContentList(ctx context.Context, uid, containerID string) ([]*Status, error) {
	var r indexListResp
	err := s.fetcher.JSON(ctx, s.indexRunner, http.MethodGet,
		fmt.Sprintf("https://m.weibo.cn/api/container/getIndex?type=uid&value=%s&containerid=%s", uid, containerID),
		indexHeaders("https://m.weibo.cn/u/"+uid), &r)
	if err != nil {
		return nil, err
	}
	var out []*Status
	for _, card := range r.Data.Cards {
		if card.Mblog != nil {
			out = append(out, card.Mblog)
		}
	}
	return out, nil
}

// getWeiboLongText 拉取长文全文。
func (s *Service) getWeiboLongText(ctx context.Context, id string) (string, error) {
	var r extendResp
	err := s.fetcher.JSON(ctx, s.longTextRunner, http.MethodGet,
		fmt.Sprintf("https://m.weibo.cn/statuses/extend?id=%s", id),
		indexHeaders("https://m.weibo.cn/detail/"+id), &r)
	if err != nil {
		return "", err
	}
	if r.Data.LongTextContent == "" {
		return "", fmt.Errorf("weibo: long text empty, id %s", id)
	}
	return r.Data.LongTextContent, nil
}

// getWeiboDetail 拉取单条微博详情（长文接口失败时的兜底）。
func (s *Service) getWeiboDetail(ctx context.Context, id string) (*Status, error) {
	var r struct {
		Data *Status `json:"data"`
	}
	err := s.fetcher.JSON(ctx, s.detailRunner, http.MethodGet,
		fmt.Sprintf("https://m.weibo.cn/statuses/show?id=%s", id),
		indexHeaders("https://m.weibo.cn/detail/"+id), &r)
	if err != nil {
		return nil, err
	}
	if r.Data == nil {
		return nil, fmt.Errorf("weibo: detail empty, id %s", id)
	}
	return r.Data, nil
}

// getUIDByDomain 将自定义域名转换为 uid：
// 请求 m.weibo.cn/<domain> 并跟随重定向，从最终路径 /u/<uid> 提取。
func (s *Service) getUIDByDomain(ctx context.Context, domain string) (string, error) {
	resp, err := s.fetcher.Do(ctx, s.domainRunner, http.MethodGet,
		fmt.Sprintf("https://m.weibo.cn/%s?&jumpfrom=weibocom", domain), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))

	// 与 TS 版 res.request.path.split("/u/")[1] 语义一致
	uid, ok := extractUID(resp.Request.URL.Path)
	if !ok {
		return "", fmt.Errorf("%w: domain %s", ErrDomainNotFound, domain)
	}
	return uid, nil
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
