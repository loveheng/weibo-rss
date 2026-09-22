# weibo-rss

简单的微博 RSS 订阅源生成器，可将某人最近发布的微博转换为符合 RSS Feed 标准的格式，供阅读器订阅。

让你不再错过喜欢的博主的动态更新，即使身处纷繁复杂中。

## 特点
1. 克制：严格限制程序对微博的并发请求，不产生额外压力
2. 省资源：基于 Go 实现，单二进制 + 纯内存缓存，常驻内存 ~15MB，镜像 ~7MB
3. 高可用：支持个人账号 Cookie 兜底与访客 Cookie 自动轮换，提升抗封禁能力
4. 多源：内置微博与 Instagram 订阅支持
5. 纯 API：无前端页面，仅提供 RSS 订阅与辅助接口

## 手动部署

依赖：`Go 1.22+`

安装并运行：
```
git clone https://github.com/zgq354/weibo-rss.git
cd weibo-rss
go run ./cmd/server
```

或编译为单二进制部署（静态资源已嵌入，无运行时依赖）：
```
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o weibo-rss ./cmd/server
./weibo-rss
```

程序将启动一个 HTTP Server，默认监听 `3000` 端口，还需另外配置域名、HTTP 反向代理等。

### 环境变量
- `PORT`：服务端口，默认 `3000`
- `WEIBO_COOKIE`：个人账号 Cookie，可显著提升抗封禁与抓取稳定性
- `WEIBO_PROXY`：微博上游代理，如 `http://user:pass@host:port`
- `INSTAGRAM_COOKIE`：Instagram 账号 Cookie
- `INSTAGRAM_PROXY`：Instagram 上游代理
- `IMAGE_CACHE`：图片反代前缀，默认百度图片反代

## Docker 部署

```
docker build -f docker/Dockerfile -t weibo-rss .
docker run --rm -p 3000:3000 weibo-rss
```

或使用 docker-compose：
```
docker compose up -d
```

## 接口

| 路径 | 说明 |
| --- | --- |
| `GET /rss/user/:uid` | 微博用户 RSS（uid 为 10 位数字） |
| `GET /rss/instagram/:username` | Instagram 用户 RSS |
| `GET /api/domain2uid?domain=xxx` | 微博自定义域名转 uid |
| `GET /admin/cache-stats` | 缓存统计 |

## 项目结构

```
├── cmd/server/          # 程序入口
├── internal/
│   ├── anticrawl/       # 防风控：重试退避 + 风控钩子链
│   ├── cache/           # LRU + TTL 缓存与缓存策略层
│   ├── config/          # 配置与缓存 TTL
│   ├── feed/            # RSS Channel/XML 组装
│   ├── source/          # 订阅源抽象（source.Feed 接口）
│   │   ├── weibo/       # 微博源
│   │   └── instagram/   # Instagram 源
│   ├── throttler/       # 串行限流 + 熔断冷却
│   └── web/             # HTTP 路由与中间件（与具体源解耦）
└── docker/Dockerfile    # 多阶段构建，scratch 极简镜像
```

## 新增订阅源

路由层与具体源完全解耦，新增一个源只需三步：

1. 在 `internal/source/<name>/` 实现满足 `internal/source.Feed` 接口的服务：
   - `Name` / `Route`（路由模板，如 `/rss/<name>/{id}`）/ `Policy`（XML 缓存策略）
   - `Validate`（标识格式校验）/ `Fetch`（拉取并拼装 `feed.Channel`）/ `NotFoundMessage`
   - 复用现成基础设施：`cache.SourcePolicy`（数据缓存）、`throttler.Throttler`（串行限流熔断）、`anticrawl`（风控重试钩子）
2. 如需 RSS 之外的接口（如微博的 domain2uid），实现可选的 `source.ExtraRoutes`。
3. 在 `cmd/server/main.go` 的 `Deps.Sources` 中注册即可，路由/缓存/请求合并/错误映射自动生效。

## 相关项目

* [RSSHub](https://github.com/DIYgod/RSSHub)

## License

MIT
