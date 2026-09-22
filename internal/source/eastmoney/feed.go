package eastmoney

// 本文件将东方财富源接入统一订阅源抽象（internal/source.Feed）：
// Service 实现发帖订阅，ReplyFeed 实现回复订阅，两者共用同一客户端。
// 路由注册、XML 缓存、请求合并、错误映射均由 web 层通用逻辑处理。

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/loveheng/weibo-rss/internal/cache"
	"github.com/loveheng/weibo-rss/internal/config"
	"github.com/loveheng/weibo-rss/internal/feed"
)

var (
	uidRe = regexp.MustCompile(`^[0-9]{8,20}$`)
	tagRe = regexp.MustCompile(`<[^>]*>`)
)

// 缓存策略：列表与 XML 输出均为 15 分钟。
// 两个订阅源使用不同的 XML key 前缀，避免同 uid 时请求合并键冲突。
var (
	emPostListPolicy  = cache.SourcePolicy{KeyPrefix: "eastmoney-post-"}
	emReplyListPolicy = cache.SourcePolicy{KeyPrefix: "eastmoney-reply-"}
	emPostXMLPolicy   = cache.FeedPolicy{XMLKeyPrefix: "eastmoney-post-xml-", XMLTTL: config.RSSXMLTTL, Collapse: true}
	emReplyXMLPolicy  = cache.FeedPolicy{XMLKeyPrefix: "eastmoney-reply-xml-", XMLTTL: config.RSSXMLTTL, Collapse: true}
)

// Name 实现 source.Feed。
func (s *Service) Name() string { return "eastmoney-post" }

// Route 实现 source.Feed：订阅标识为股吧 uid。
func (s *Service) Route() string { return "/rss/eastmoney/post/{id}" }

// Policy 实现 source.Feed。
func (s *Service) Policy() cache.FeedPolicy { return emPostXMLPolicy }

// Validate 实现 source.Feed。
func (s *Service) Validate(id string) error {
	if !uidRe.MatchString(id) {
		return fmt.Errorf("uid 格式有误。uid: %s", id)
	}
	return nil
}

// NotFoundMessage 实现 source.Feed。
func (s *Service) NotFoundMessage(id string) string {
	return "找不到用户，可能 uid 有误或该用户暂无发帖。uid: " + id
}

// Fetch 实现 source.Feed：拉取发帖列表并拼装 RSS 频道。
func (s *Service) Fetch(ctx context.Context, id string) (*feed.Channel, error) {
	posts, err := cache.MemoList(ctx, s.cache, emPostListPolicy, id, func(ctx context.Context) ([]postItem, error) {
		return s.fetchPosts(ctx, id)
	})
	if err != nil {
		return nil, err
	}

	ch := &feed.Channel{
		SiteURL:     "https://i.eastmoney.com/" + id,
		Title:       "股吧动态",
		Description: "东方财富股吧最新发帖",
		Items:       make([]feed.Item, 0, len(posts)),
	}
	if len(posts) > 0 {
		nick := posts[0].PostUser.Nickname
		ch.Title = nick + " 的股吧动态"
		ch.Description = "东方财富股吧用户 " + nick + " 的最新发帖"
	}
	for _, p := range posts {
		ch.Items = append(ch.Items, feed.Item{
			Title:       postTitleOf(p),
			Description: s.postToHTML(p),
			Link:        postLink(p),
			Time:        parseEmTime(p.PostPublishTime),
		})
	}
	return ch, nil
}

// ReplyFeed 为博主回复订阅源，实现 source.Feed。
type ReplyFeed struct {
	svc *Service
}

// ReplyFeed 返回回复订阅源。
func (s *Service) ReplyFeed() *ReplyFeed { return &ReplyFeed{svc: s} }

// Name 实现 source.Feed。
func (f *ReplyFeed) Name() string { return "eastmoney-reply" }

// Route 实现 source.Feed：订阅标识为股吧 uid。
func (f *ReplyFeed) Route() string { return "/rss/eastmoney/reply/{id}" }

// Policy 实现 source.Feed。
func (f *ReplyFeed) Policy() cache.FeedPolicy { return emReplyXMLPolicy }

// Validate 实现 source.Feed。
func (f *ReplyFeed) Validate(id string) error { return f.svc.Validate(id) }

// NotFoundMessage 实现 source.Feed。
func (f *ReplyFeed) NotFoundMessage(id string) string {
	return "找不到用户，可能 uid 有误或该用户暂无回复。uid: " + id
}

// Fetch 实现 source.Feed：拉取回复列表并拼装 RSS 频道。
func (f *ReplyFeed) Fetch(ctx context.Context, id string) (*feed.Channel, error) {
	replies, err := cache.MemoList(ctx, f.svc.cache, emReplyListPolicy, id, func(ctx context.Context) ([]replyItem, error) {
		return f.svc.fetchReplies(ctx, id)
	})
	if err != nil {
		return nil, err
	}

	ch := &feed.Channel{
		SiteURL:     "https://i.eastmoney.com/" + id,
		Title:       "股吧回复",
		Description: "东方财富股吧最新回复",
		Items:       make([]feed.Item, 0, len(replies)),
	}
	if len(replies) > 0 {
		nick := replies[0].ReplyUser.Nickname
		ch.Title = nick + " 的股吧回复"
		ch.Description = "东方财富股吧用户 " + nick + " 的最新回复"
	}
	for _, r := range replies {
		ch.Items = append(ch.Items, feed.Item{
			Title:       replyTitleOf(r),
			Description: replyToHTML(f.svc.cfg, r),
			Link:        replyLink(r),
			Time:        parseEmTime(r.ReplyPublishTime),
		})
	}
	return ch, nil
}

// postTitleOf 生成帖子标题：优先帖子标题，否则截取正文。
func postTitleOf(p postItem) string {
	title := strings.TrimSpace(p.PostTitle)
	if title == "" {
		title = strings.TrimSpace(tagRe.ReplaceAllString(p.PostContent, ""))
	}
	if runes := []rune(title); len(runes) > 30 {
		title = string(runes[:30]) + "…"
	}
	return title
}

// postLink 生成股吧帖子链接。
func postLink(p postItem) string {
	if p.PostGuba.StockbarCode == "" || p.PostID == 0 {
		return "https://i.eastmoney.com/"
	}
	return fmt.Sprintf("https://guba.eastmoney.com/news,%s,%d.html", p.PostGuba.StockbarCode, p.PostID)
}

// postToHTML 将帖子转为 RSS 正文：正文 + 视频/配图（走图片反代）。
func (s *Service) postToHTML(p postItem) string {
	var b strings.Builder
	b.WriteString(p.PostContent)
	if p.PostVideoURL != "" {
		fmt.Fprintf(&b, `<br><video controls preload="metadata" src="%s"></video>`, p.PostVideoURL)
	}
	for _, pic := range p.PostPicURL {
		if pic == "" {
			continue
		}
		fmt.Fprintf(&b, `<br><img src="%s">`, s.cfg.WrapImageURL(pic))
	}
	if p.PostFrom != "" {
		fmt.Fprintf(&b, `<br><small>来自 %s</small>`, html.EscapeString(p.PostFrom))
	}
	return b.String()
}

// replyTitleOf 生成回复标题：回复对象 + 回复内容摘要。
func replyTitleOf(r replyItem) string {
	var title string
	switch {
	case r.SourceReplyText != "":
		title = "回复 @" + r.SourceReplyUser + "：" + r.SourceReplyText
	case r.SourcePostTitle != "":
		title = "评论《" + r.SourcePostTitle + "》"
	default:
		title = r.ReplyText
	}
	title = strings.TrimSpace(tagRe.ReplaceAllString(title, ""))
	if runes := []rune(title); len(runes) > 30 {
		title = string(runes[:30]) + "…"
	}
	return title
}

// replyToHTML 将回复转为 RSS 正文：回复内容 + 引用被回复内容/原帖。
func replyToHTML(cfg config.Config, r replyItem) string {
	var b strings.Builder
	b.WriteString(html.EscapeString(strings.TrimSpace(r.ReplyText)))
	if r.ReplyPicture != "" {
		fmt.Fprintf(&b, `<br><img src="%s">`, cfg.WrapImageURL(r.ReplyPicture))
	}
	switch {
	case r.SourceReplyText != "":
		who := r.SourceReplyUser
		if who == "" {
			who = "楼主"
		}
		fmt.Fprintf(&b, `<br><blockquote>回复 @%s：%s</blockquote>`,
			html.EscapeString(who), html.EscapeString(strings.TrimSpace(r.SourceReplyText)))
	case r.SourcePostTitle != "":
		fmt.Fprintf(&b, `<br><blockquote>原帖：《%s》</blockquote>`, html.EscapeString(r.SourcePostTitle))
	}
	return b.String()
}

// replyLink 生成回复所在帖子的链接。
func replyLink(r replyItem) string {
	if r.ReplyGuba.StockbarCode == "" || r.SourcePostID == 0 {
		return "https://i.eastmoney.com/"
	}
	return fmt.Sprintf("https://guba.eastmoney.com/news,%s,%d.html", r.ReplyGuba.StockbarCode, r.SourcePostID)
}
