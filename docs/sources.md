# 订阅源文档

本文档描述 weibo-rss 当前内置的全部 RSS 订阅源：路由、标识格式、数据更新频率、缓存策略与配置项。

所有源共用统一的订阅源抽象 `internal/source.Feed`，由 web 层统一处理路由注册、XML 缓存、并发请求合并（request collapsing）与错误映射，各源只需实现自身的数据拉取与拼装逻辑。

## 订阅源总览

| 订阅源 | 路由 | 标识（{id}） | 数据缓存 TTL | XML 缓存 TTL |
| --- | --- | --- | --- | --- |
| 微博用户 | `GET /rss/user/:uid` | 10 位数字 uid | 15 分钟 | 15 分钟 |
| Instagram 用户 | `GET /rss/instagram/:username` | 用户名（字母/数字/`.`/`_`，1-30 位） | 1 小时 | 1 小时 |
| 东方财富股吧 · 发帖 | `GET /rss/eastmoney/post/:uid` | 8-20 位数字 uid | 15 分钟 | 15 分钟 |
| 东方财富股吧 · 回复 | `GET /rss/eastmoney/reply/:uid` | 8-20 位数字 uid | 15 分钟 | 15 分钟 |

> XML 缓存开启请求合并（Collapse）：多个阅读器同时请求同一订阅时，只有一次真实上游拉取。

## 微博源（weibo）

- 路由模板：`/rss/user/{id}`
- 标识格式：10 位数字 uid（如 `1195230310`）
- 数据来源：微博移动端接口（`m.weibo.cn`），支持访客 Cookie 自动轮换与个人账号 Cookie 兜底
- 输出内容：用户昵称与简介作为频道标题/描述；每条微博生成条目（标题、正文 HTML、微博链接、发布时间），图片经 `IMAGE_CACHE` 反代

### 辅助接口

| 路径 | 说明 |
| --- | --- |
| `GET /api/domain2uid?domain=xxx` | 微博自定义域名转 uid，结果缓存 7 天 |

### 订阅方式

1. 已知 uid：直接订阅 `https://<你的域名>/rss/user/<uid>`
2. 只有自定义域名（如 `https://weibo.com/xxx`）：先访问 `/api/domain2uid?domain=xxx` 获取 uid

### 常见错误

- uid 格式错误 → 404：`找不到用户，传入 UID 格式有误`
- 用户不存在或仅登录可见 → 404：`找不到用户，可能用户仅登录可见，不支持订阅。可以通过打开 https://m.weibo.cn/u/:uid 验证`

## Instagram 源（instagram）

- 路由模板：`/rss/instagram/{id}`
- 标识格式：Instagram 用户名，`[a-zA-Z0-9._]` 组成，1-30 位
- 数据来源：Instagram Web 接口，依赖 `INSTAGRAM_COOKIE`
- 缓存策略：XML 与数据均缓存 1 小时，优先保护出口 IP 不被风控
- 输出内容：条目标题为文案前 25 字，正文包含图片/视频 HTML，条目链接指向 `instagram.com/p/<shortcode>/`

### 常见错误

- 用户名格式错误 → 404：`用户名格式有误`
- 用户不存在或私密账号 → 404：`找不到用户，可能用户名有误、用户不存在或为私密账号`

> 注意：Instagram 源无 Cookie 或 Cookie 失效时基本无法拉取，部署时必须配置 `INSTAGRAM_COOKIE`。

## 东方财富股吧源（eastmoney）

同一用户可分别订阅发帖与回复，两者共用同一上游客户端与 Cookie，XML 缓存 key 前缀不同，互不冲突。

### 发帖订阅

- 路由模板：`/rss/eastmoney/post/{id}`
- 标识格式：8-20 位数字股吧 uid
- 输出内容：频道标题为「<昵称> 的股吧动态」；条目标题优先帖子标题（无标题时截取正文前 30 字），正文包含帖子内容、视频、配图（走图片反代）与发帖来源；条目链接指向股吧帖子页

### 回复订阅

- 路由模板：`/rss/eastmoney/reply/{id}`
- 标识格式：同上
- 输出内容：频道标题为「<昵称> 的股吧回复」；条目标题为「回复 @某人：…」或「评论《原帖》」，正文包含回复内容、引用的上下文与配图；条目链接指向所在帖子页

### 常见错误

- uid 格式错误 → 404：`uid 格式有误`
- 用户不存在或暂无内容 → 404：`找不到用户，可能 uid 有误或该用户暂无发帖/回复`

## 通用行为

### 缓存

- RSS XML 输出默认缓存 15 分钟，feed 中 `<ttl>` 字段为 15 分钟
- 上游数据与 XML 分层缓存，均存储在进程内存（LRU，上限 1000 条）
- 可通过 `GET /admin/cache-stats` 查看缓存使用情况

### 错误映射

| 情况 | HTTP 状态 |
| --- | --- |
| 标识格式错误 / 目标不存在 | 404 |
| 上游限流 / 触发风控（熔断中） | 503 |
| 其他内部错误 | 500 |

### 相关环境变量

| 变量 | 作用 | 影响的源 |
| --- | --- | --- |
| `WEIBO_COOKIE` | 个人账号 Cookie 兜底 | 微博 |
| `WEIBO_PROXY` | 上游代理 | 微博 |
| `INSTAGRAM_COOKIE` | 账号 Cookie（必需） | Instagram |
| `INSTAGRAM_PROXY` | 上游代理 | Instagram |
| `EASTMONEY_COOKIE` | 可选 Cookie 兜底 | 东方财富 |
| `EASTMONEY_PROXY` | 上游代理 | 东方财富 |
| `IMAGE_CACHE` | 图片反代前缀 | 全部（正文图片） |
