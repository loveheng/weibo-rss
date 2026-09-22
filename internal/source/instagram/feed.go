package instagram

// 本文件将 Instagram 源接入统一订阅源抽象（internal/source.Feed）。

import (
	"context"
	"fmt"
	"regexp"

	"github.com/loveheng/weibo-rss/internal/cache"
	"github.com/loveheng/weibo-rss/internal/config"
	"github.com/loveheng/weibo-rss/internal/feed"
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9._]{1,30}$`)

	// instagramXMLPolicy：XML 缓存拉长到 1 小时，优先保护出口 IP
	instagramXMLPolicy = cache.FeedPolicy{XMLKeyPrefix: "instagram-xml-", XMLTTL: config.InstagramTTL, Collapse: true}
)

// Name 实现 source.Feed。
func (s *Service) Name() string { return "instagram" }

// Route 实现 source.Feed：订阅标识为 Instagram 用户名。
func (s *Service) Route() string { return "/rss/instagram/{id}" }

// Policy 实现 source.Feed。
func (s *Service) Policy() cache.FeedPolicy { return instagramXMLPolicy }

// Validate 实现 source.Feed。
func (s *Service) Validate(id string) error {
	if !usernameRe.MatchString(id) {
		return fmt.Errorf("用户名格式有误。username: %s", id)
	}
	return nil
}

// NotFoundMessage 实现 source.Feed。
func (s *Service) NotFoundMessage(id string) string {
	return "找不到用户，可能用户名有误、用户不存在或为私密账号。username: " + id
}

// Fetch 实现 source.Feed：拉取数据并拼装 RSS 频道。
func (s *Service) Fetch(ctx context.Context, id string) (*feed.Channel, error) {
	data, err := s.FetchUserLatestPosts(ctx, id)
	if err != nil {
		return nil, err
	}
	ch := &feed.Channel{
		SiteURL:     "https://www.instagram.com/" + data.Username + "/",
		Title:       data.Name + " (@" + data.Username + ") 的 Instagram",
		Description: data.Description,
		Items:       make([]feed.Item, 0, len(data.Media)),
	}
	for _, m := range data.Media {
		title := m.Caption
		if runes := []rune(title); len(runes) > 25 {
			title = string(runes[:25])
		}
		ch.Items = append(ch.Items, feed.Item{
			Title:       title,
			Description: s.MediaToHTML(m),
			Link:        "https://www.instagram.com/p/" + m.Shortcode + "/",
			Time:        m.TakenAt,
		})
	}
	return ch, nil
}
