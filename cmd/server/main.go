// weibo-rss 服务入口：组装依赖、启动 HTTP 服务、
// 访客 Cookie 轮换、缓存统计与优雅退出。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/zgq354/weibo-rss/internal/cache"
	"github.com/zgq354/weibo-rss/internal/config"
	"github.com/zgq354/weibo-rss/internal/source/instagram"
	"github.com/zgq354/weibo-rss/internal/source/weibo"
	"github.com/zgq354/weibo-rss/internal/web"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg := config.Load()
	memCache := cache.New(1000, log)

	// 数据源
	weiboSvc := weibo.NewService(cfg, memCache, log)
	instagramSvc := instagram.NewService(cfg, memCache, log)

	// 生命周期：Cookie 轮换与统计随 ctx 退出
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	weiboSvc.StartCookieRotation(ctx, config.CookieTTL)
	go logCacheStats(ctx, memCache, log)

	handler := web.NewHandler(web.Deps{
		Cache:     memCache,
		Collapse:  &singleflight.Group{},
		Weibo:     weiboSvc,
		Instagram: instagramSvc,
		Cfg:       cfg,
		Log:       log,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("weibo-rss start (go)", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("signal received, shutting down...")
	log.Info("[cache] stats", "size", memCache.Len(), "max", memCache.MaxEntries())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("forced shutdown after timeout", "err", err)
		os.Exit(1)
	}
	log.Info("HTTP server closed")
}

// logCacheStats 每 5 分钟输出一次缓存统计。
func logCacheStats(ctx context.Context, c *cache.Cache, log *slog.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Info("[cache] stats", "size", c.Len(), "max", c.MaxEntries())
		}
	}
}
