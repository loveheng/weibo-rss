package weibo

// 本文件对应 TS 版 weibo.ts 的 Service 编排层：
//   - 数据缓存策略（info/list/长文/详情各层独立 TTL）；
//   - 长文填充（失败降级到详情接口，允许失败）；
//   - 转发微博递归填充。

import (
	"context"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/loveheng/weibo-rss/internal/cache"
	"github.com/loveheng/weibo-rss/internal/config"
	"github.com/loveheng/weibo-rss/internal/upstream"
)

// 微博源各层缓存策略（key 前缀沿用原 long-/dt- 约定）。
var (
	weiboPolicy    = cache.SourcePolicy{KeyPrefix: "weibo-", InfoTTL: config.IndexInfoTTL, ListTTL: config.StatusListTTL}
	longTextPolicy = cache.SourcePolicy{KeyPrefix: "long-", InfoTTL: config.LongTextTTL}
	detailPolicy   = cache.SourcePolicy{KeyPrefix: "dt-", InfoTTL: config.DetailTTL}
	domainTTL      = config.DomainTTL
)

// Status 为微博条目的精简模型，只保留 RSS 生成必需字段；
// 微博上游 JSON 字段杂乱且可空，未用到的字段由 encoding/json 自动忽略。
type Status struct {
	ID          string `json:"id"`
	MID         string `json:"mid"`
	Bid         string `json:"bid"`
	StatusTitle string `json:"status_title"`
	CreatedAt   string `json:"created_at"`
	Text        string `json:"text"`
	IsLongText  bool   `json:"isLongText"`
	User        *struct {
		ID         int64  `json:"id"`
		ScreenName string `json:"screen_name"`
	} `json:"user"`
	Pics []struct {
		Large struct {
			URL string `json:"url"`
		} `json:"large"`
	} `json:"pics"`
	Retweeted *Status `json:"retweeted_status"`
}

// UserData 为用户数据聚合结果。
type UserData struct {
	UID         string
	ScreenName  string
	Description string
	StatusList  []*Status
}

// Service 为微博数据源服务，实现 source.Feed 与 source.ExtraRoutes。
type Service struct {
	cfg     config.Config
	cache   *cache.Cache
	client  *Client
	fetcher *upstream.Fetcher
	hooks   *upstream.Hooks
	log     *slog.Logger
	sf      *singleflight.Group

	indexRunner    *upstream.Throttler
	detailRunner   *upstream.Throttler
	longTextRunner *upstream.Throttler
	domainRunner   *upstream.Throttler
}

// NewService 创建微博数据源服务。
func NewService(cfg config.Config, c *cache.Cache, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	client := NewClient(cfg, log)
	s := &Service{
		cfg:            cfg,
		cache:          c,
		client:         client,
		hooks:          client.Hooks(),
		log:            log,
		sf:             &singleflight.Group{},
		indexRunner:    upstream.New("weibo-index", log, upstream.DefaultCooldown),
		detailRunner:   upstream.New("weibo-detail", log, upstream.DefaultCooldown),
		longTextRunner: upstream.New("weibo-longText", log, upstream.DefaultCooldown),
		domainRunner:   upstream.New("weibo-domain", log, upstream.DefaultCooldown),
	}
	s.fetcher = upstream.NewFetcher(client.Up(), s.hooks)
	return s
}

// StartCookieRotation 启动访客 Cookie 轮换（应传入随主程序退出的 ctx）。
func (s *Service) StartCookieRotation(ctx context.Context, interval time.Duration) {
	go s.client.StartCookieRotation(ctx, interval)
}

// FetchUserLatestWeibo 获取用户最新微博（含长文填充）。
func (s *Service) FetchUserLatestWeibo(ctx context.Context, uid string) (*UserData, error) {
	info, err := cache.MemoInfo(ctx, s.cache, weiboPolicy, uid, func(ctx context.Context) (userInfo, error) {
		return s.getIndexUserInfo(ctx, uid)
	})
	if err != nil {
		return nil, err
	}

	statusList, err := cache.MemoList(ctx, s.cache, weiboPolicy, uid, func(ctx context.Context) ([]*Status, error) {
		statuses, err := s.getWeiboContentList(ctx, uid, info.ContainerID)
		if err != nil {
			return nil, err
		}
		// 并发填充长文；单条失败静默降级（与原版一致），不中断整体
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(5)
		for _, st := range statuses {
			st := st
			g.Go(func() error {
				s.fillStatusWithLongText(gctx, st)
				return nil
			})
		}
		_ = g.Wait()
		return statuses, nil
	})
	if err != nil {
		return nil, err
	}

	return &UserData{
		UID:         uid,
		ScreenName:  info.ScreenName,
		Description: info.Description,
		StatusList:  statusList,
	}, nil
}

// FetchUIDByDomain 将自定义域名转换为 uid。
func (s *Service) FetchUIDByDomain(ctx context.Context, domain string) (string, error) {
	return cache.Memo(ctx, s.cache, "dm-"+domain, domainTTL, func(ctx context.Context) (string, error) {
		return s.getUIDByDomain(ctx, domain)
	})
}

// fillStatusWithLongText 就地填充长文内容：
//   - isLongText 条目先请求长文接口，失败降级到详情接口；
//   - 转发的微博递归填充；
//   - 所有失败均静默降级（返回原始内容），与原版 catch 语义一致。
func (s *Service) fillStatusWithLongText(ctx context.Context, st *Status) {
	if st.IsLongText {
		text, err := cache.MemoInfo(ctx, s.cache, longTextPolicy, st.ID, func(ctx context.Context) (string, error) {
			return s.getWeiboLongText(ctx, st.ID)
		})
		if err != nil {
			s.log.Error("longText failed, fallback to detail", "status", st.ID, "err", err)
			detail, derr := cache.MemoInfo(ctx, s.cache, detailPolicy, st.ID, func(ctx context.Context) (*Status, error) {
				return s.getWeiboDetail(ctx, st.ID)
			})
			if derr != nil {
				s.log.Error("detail fallback failed", "status", st.ID, "err", derr)
			} else {
				st.Text = detail.Text
				if detail.Pics != nil {
					st.Pics = detail.Pics
				}
			}
		} else {
			st.Text = text
		}
	}
	if st.Retweeted != nil {
		s.fillStatusWithLongText(ctx, st.Retweeted)
	}
}
