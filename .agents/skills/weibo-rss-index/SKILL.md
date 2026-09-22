---
# weibo-rss-index

> 只做“功能 → 代码落点 + 文档落点”索引。禁止铺满细节；只登记域级锚点。

## 应用启动与装配
- cmd/server/main.go
- internal/config/config.go

## 订阅源抽象（新增源先看这里）
- internal/source/source.go（source.Feed 接口 + 可选 ExtraRoutes）
- README.md「新增订阅源」章节

## 路由与 RSS
- internal/web/server.go（通用注册/错误映射/访问日志）
- internal/feed/feed.go（Channel + RSS XML 组装）
- docs/feature/feature.md

## 微博数据抓取
- internal/source/weibo/weibo.go（编排：缓存策略 + 长文填充）
- internal/source/weibo/api.go（4 个上游接口 + 解析）
- internal/source/weibo/client.go（访客 Cookie 轮换/个人 Cookie 兜底）
- internal/source/weibo/feed.go（source.Feed 实现 + domain2uid）
- internal/source/weibo/render.go（StatusToHTML/标题/时间解析）

## Instagram 数据抓取
- internal/source/instagram/instagram.go（客户端 + 接口 + MediaToHTML）
- internal/source/instagram/feed.go（source.Feed 实现）

## 缓存与限流
- internal/cache/cache.go（TTL-LRU + 泛型 Memo）
- internal/cache/policy.go（SourcePolicy/FeedPolicy/CachedFeed）
- internal/throttler/throttler.go（串行队列 + 熔断冷却）

## 防风控与上游客户端
- internal/anticrawl/anticrawl.go（重试退避/风控钩子/Jitter）
- internal/upstream/upstream.go（公共上游 Client + Fetcher 骨架）

## 公共工具
- internal/httputil/httputil.go（WriteXML/WriteJSON）
- assets.go（go:embed 静态资源；已移除首页，暂无静态目录）

## 部署
- docker/Dockerfile（多阶段构建，scratch）
- docker-compose.yml
- .github/workflows/docker-image.yml（Go + 多架构镜像）
