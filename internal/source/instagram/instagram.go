// Package instagram 实现 Instagram RSS 数据源。
//
// 机制移植自 TS 版 modules/instagram/*：
//   - web_profile_info 接口，需携带 x-ig-app-id；
//   - 风控严格：401/403/429 视为风控，熔断冷却 30 分钟，优先保护出口 IP；
//   - 数据 TTL 拉长到 1 小时，减少对上游的请求频次。
package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zgq354/weibo-rss/internal/anticrawl"
	"github.com/zgq354/weibo-rss/internal/cache"
	"github.com/zgq354/weibo-rss/internal/config"
	"github.com/zgq354/weibo-rss/internal/source"
	"github.com/zgq354/weibo-rss/internal/throttler"
)

const (
	// Timeout 为单次上游请求超时。
	Timeout = 9 * time.Second
	// MockUA 为移动端 UA。
	MockUA = "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36"
	// IGAppID 为 Instagram web api 的公共 app id。
	IGAppID = "936619743392459"
)

// instagramPolicy 为数据缓存策略（info TTL 1 小时）。
var instagramPolicy = cache.SourcePolicy{KeyPrefix: "instagram-", InfoTTL: config.InstagramTTL}

// Media 为单条帖子（轮播子项复用同一结构）。
type Media struct {
	Shortcode  string
	DisplayURL string
	VideoURL   string
	IsVideo    bool
	TakenAt    time.Time
	Caption    string
	Children   []Media // 轮播图
}

// UserData 为用户数据聚合结果。
type UserData struct {
	Username    string
	Name        string
	Description string
	Media       []Media
}

// profileResp 为 web_profile_info 接口的精简结构。
type profileResp struct {
	Data struct {
		User struct {
			ID            string `json:"id"`
			Username      string `json:"username"`
			FullName      string `json:"full_name"`
			Biography     string `json:"biography"`
			TimelineMedia struct {
				Edges []struct {
					Node struct {
						Shortcode        string `json:"shortcode"`
						DisplayURL       string `json:"display_url"`
						VideoURL         string `json:"video_url"`
						IsVideo          bool   `json:"is_video"`
						TakenAtTimestamp int64  `json:"taken_at_timestamp"`
						Caption          struct {
							Edges []struct {
								Node struct {
									Text string `json:"text"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"edge_media_to_caption"`
						Sidecar *struct {
							Edges []struct {
								Node struct {
									DisplayURL string `json:"display_url"`
									VideoURL   string `json:"video_url"`
									IsVideo    bool   `json:"is_video"`
								} `json:"node"`
							} `json:"edges"`
						} `json:"edge_sidecar_to_children"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"edge_owner_to_timeline_media"`
		} `json:"user"`
	} `json:"data"`
}

// Service 为 Instagram 数据源服务。
type Service struct {
	cache  *cache.Cache
	client *http.Client
	cfg    config.Config
	hooks  *anticrawl.Hooks
	log    *slog.Logger
	runner *throttler.Throttler
}

// NewService 创建 Instagram 数据源服务；配置了 INSTAGRAM_PROXY 时走出站代理。
func NewService(cfg config.Config, c *cache.Cache, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.InstagramProxy != "" {
		if u, err := url.Parse(cfg.InstagramProxy); err == nil {
			transport.Proxy = http.ProxyURL(u)
		} else {
			log.Warn("[instagram] invalid proxy config, ignored", "proxy", cfg.InstagramProxy)
		}
	}
	return &Service{
		cache:  c,
		client: &http.Client{Transport: transport, Timeout: Timeout},
		cfg:    cfg,
		hooks: &anticrawl.Hooks{
			RiskyStatuses: []int{401, 403, 429},
		},
		log:    log,
		runner: throttler.New("instagram-web", log, 30*time.Minute),
	}
}

// FetchUserLatestPosts 获取用户最新帖子。
func (s *Service) FetchUserLatestPosts(ctx context.Context, username string) (*UserData, error) {
	return cache.MemoInfo(ctx, s.cache, instagramPolicy, username, func(ctx context.Context) (*UserData, error) {
		return s.getInstagramUserInfo(ctx, username)
	})
}

// getInstagramUserInfo 请求 web_profile_info 并解析。
func (s *Service) getInstagramUserInfo(ctx context.Context, username string) (*UserData, error) {
	var out *UserData
	err := s.runner.Run(ctx, func() error {
		time.Sleep(time.Duration(rand.IntN(100)) * time.Millisecond)
		resp, err := anticrawl.DoWithRetry(ctx, s.hooks, func(ctx context.Context) (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet,
				"https://www.instagram.com/api/v1/users/web_profile_info/?username="+url.QueryEscape(username), nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("User-Agent", MockUA)
			req.Header.Set("x-ig-app-id", IGAppID)
			req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
			req.Header.Set("Accept", "*/*")
			req.Header.Set("Referer", "https://www.instagram.com/"+username+"/")
			if s.cfg.InstagramCookie != "" {
				req.Header.Set("Cookie", s.cfg.InstagramCookie)
			}
			return s.client.Do(req)
		}, 2, time.Second)
		if err != nil {
			return anticrawl.HandleForbidden(ctx, s.hooks, err, s.runner.Trip)
		}
		defer resp.Body.Close()

		var r profileResp
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return err
		}
		u := r.Data.User
		if u.ID == "" || u.Username == "" {
			return fmt.Errorf("%w: username %s", source.ErrNotFound, username)
		}

		data := &UserData{
			Username:    u.Username,
			Name:        u.FullName,
			Description: u.Biography,
		}
		if data.Name == "" {
			data.Name = u.Username
		}
		for _, edge := range u.TimelineMedia.Edges {
			node := edge.Node
			m := Media{
				Shortcode:  node.Shortcode,
				DisplayURL: node.DisplayURL,
				VideoURL:   node.VideoURL,
				IsVideo:    node.IsVideo,
				TakenAt:    time.Unix(node.TakenAtTimestamp, 0),
			}
			if len(node.Caption.Edges) > 0 {
				m.Caption = strings.TrimSpace(node.Caption.Edges[0].Node.Text)
			}
			if node.Sidecar != nil {
				for _, child := range node.Sidecar.Edges {
					m.Children = append(m.Children, Media{
						DisplayURL: child.Node.DisplayURL,
						VideoURL:   child.Node.VideoURL,
						IsVideo:    child.Node.IsVideo,
					})
				}
			}
			data.Media = append(data.Media, m)
		}
		out = data
		return nil
	})
	return out, err
}

// MediaToHTML 将帖子转换为 RSS 正文 HTML：轮播/视频/图片三种渲染。
func (s *Service) MediaToHTML(m Media) string {
	text := strings.ReplaceAll(html.EscapeString(strings.TrimSpace(m.Caption)), "\n", "<br>")

	var b strings.Builder
	b.WriteString(text)

	renderVideo := func(node Media) {
		poster := s.wrapImageCache(node.DisplayURL)
		fmt.Fprintf(&b, `<br><video controls preload="metadata" poster="%s"><source src="%s" type="video/mp4" /></video>`,
			poster, node.VideoURL)
	}
	renderImage := func(node Media) {
		u := s.wrapImageCache(node.DisplayURL)
		fmt.Fprintf(&b, `<br><a href="%s" target="_blank"><img src="%s"></a>`, u, u)
	}

	if len(m.Children) > 0 {
		for _, child := range m.Children {
			if child.IsVideo && child.VideoURL != "" {
				renderVideo(child)
			} else if child.DisplayURL != "" {
				renderImage(child)
			}
		}
	} else if m.IsVideo && m.VideoURL != "" {
		renderVideo(m)
	} else if m.DisplayURL != "" {
		renderImage(m)
	}
	return b.String()
}

// wrapImageCache 包装图片反代前缀。
func (s *Service) wrapImageCache(rawURL string) string {
	if s.cfg.ImageCache == "" {
		return rawURL
	}
	return s.cfg.ImageCache + url.QueryEscape(rawURL)
}
