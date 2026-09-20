----
# weibo-rss-index

> 只做“功能 → 代码落点 + 文档落点”索引。禁止铺满细节；只登记域级锚点。

## 应用启动
- src/app.ts
- src/config.ts

## 路由与 RSS
- src/modules/routes.ts
- docs/feature/feature.md

## 微博数据抓取
- src/modules/weibo/weibo.ts
- src/modules/weibo/api/indexAPI.ts
- src/modules/weibo/api/longTextAPI.ts
- src/modules/weibo/api/detailAPI.ts
- src/modules/weibo/api/domainAPI.ts
- src/modules/weibo/api/common.ts

## 缓存与限流
- src/modules/cache.ts
- src/modules/throttler.ts

## 日志
- src/modules/logger.ts

## 类型与工具
- src/types.ts
- src/utils.ts

## 部署
- Dockerfile
- docker-compose.yml
- process.json
- .github/workflows/docker-image.yml
