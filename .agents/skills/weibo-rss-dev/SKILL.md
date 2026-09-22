# weibo-rss-dev

> 从现有代码提炼写法模式；后续新增代码优先对齐现有风格，而非引入新范式。

## 分层与依赖方向
- `cmd/server/main.go` 只做组装（依赖注入、信号处理），不写业务。
- `internal/` 下按职责分包；web 层与具体订阅源完全解耦。
- 依赖方向：`web → source（接口）→ 基础设施（cache/throttler/anticrawl/upstream）`；
  源之间互不 import；基础设施不 import 源与 web。

## 订阅源接入模式（source.Feed 接口）
- 每个源在 `internal/source/<name>/` 实现接口：`Name / Route / Policy / Validate / Fetch / NotFoundMessage`。
- `Fetch` 返回 `feed.Channel`（频道信息 + 条目），不感知 XML 细节。
- 需要额外接口时实现 `source.ExtraRoutes`（参考微博 `RegisterExtra` 的 domain2uid）。
- 错误约定（web 层统一映射）：`source.ErrNotFound` → 404（文案走 `NotFoundMessage`）；
  `throttler.ErrThrottled` / `anticrawl.ErrRisky` → 503；其余 → 500 并记日志。
- 标识校验（`Validate`）返回的错误文本直接作为 404 响应体，不包装哨兵错误。

## 上游请求写法（upstream.Fetcher）
- 每个源一个 `upstream.Fetcher`（组合公共 `Client` + 源的 `anticrawl.Hooks`），
  禁止手写 transport/proxy/UA 拼装和重试循环。
- JSON 接口：`fetcher.JSON(ctx, runner, method, url, headers, &respStruct)`。
- 非常规请求（如跟随重定向取 URL）：`fetcher.Do(...)` 拿原始响应自行处理。
- 源的公共附加头部写成包内小函数（参考 weibo `indexHeaders(referer)`）。
- 上游脏 JSON 用精简 struct 解析：只定义必需字段，可空字段用指针
  （如 `Mblog *Status` 天然过滤无正文的卡片），`encoding/json` 自动忽略未知字段。

## 防风控设施（anticrawl）
- `DoWithRetry`：指数退避 + 随机抖动；风控状态码不重试，返回 `StatusError`。
- `HandleForbidden`：命中风控 → 先 `OnRiskDetected` 源钩子（如刷新 Cookie）→ 再熔断。
- 源通过 `Hooks{RiskyStatuses, OnRiskDetected}` 声明特征：
  weibo `[403, 418]`；instagram `[401, 403, 429]`；默认 `[401, 403, 418, 429]`。
- 请求前抖动一律用 `anticrawl.Jitter(ctx, max)`，不要裸 `time.Sleep`。

## 缓存约定（cache 包）
- 接口结果缓存走策略化封装：`cache.MemoInfo / MemoList`（`SourcePolicy`：keyPrefix + infoTTL/listTTL）。
- RSS XML 缓存 + 请求合并统一走 `cache.CachedFeed`（`FeedPolicy`：xmlKeyPrefix + xmlTTL + collapse），
  由 web 层通用逻辑调用，源内不重复实现。
- 细粒度按内容 id 的缓存（长文/详情）用裸 `cache.Memo` + 包内策略变量（`long-`/`dt-` 前缀沿用）。
- 缓存 key 命名带域前缀：`weibo-info-`/`weibo-list-`/`long-`/`dt-`/`xml-`/`instagram-xml-`/`dm-`。
- TTL 常量集中在 `internal/config/config.go`。
- 失败结果不缓存（Memo 语义），空字符串结果会缓存以防击穿。

## 限流写法（throttler）
- 每个上游接口组一个独立 `Throttler`（串行 concurrency=1 + 熔断冷却），
  参考微博四个 runner（index/detail/longText/domain）与 instagram 单 runner（30 分钟冷却）。
- 熔断恢复按时间戳比较（`Trip` 记录 `brokenAt`），不要用定时器重置。

## 图片与公共工具
- 图片反代前缀一律 `cfg.WrapImageURL(u)`（config 方法），不要手写 QueryEscape 拼接。
- 响应写出用 `httputil.WriteXML / WriteJSON`。
- HTML 拼装优先 `strings.Builder` + `fmt.Fprintf`，与 `StatusToHTML`/`MediaToHTML` 风格对齐。

## 测试约定
- 测试文件与被测包同目录（`_test.go`），标准库 `testing`。
- 上游解析类测试用带噪音的真实 JSON 样本锁定行为（参考 weibo `parse_test.go`）。
- 并发/时序类用短时延验证（参考 throttler_test 的串行与熔断用例）。

## 新增 RSS 源步骤速查
1. `internal/source/<name>/`：`upstream.NewClient`（源专属 BaseHeaders/Cookie/Proxy env）+
   `upstream.NewFetcher`（声明 `Hooks.RiskyStatuses`）。
2. API 函数：`fetcher.JSON` + 精简 struct 解析；用户不存在包装 `source.ErrNotFound`。
3. 实现 `source.Feed` 六方法；需要额外接口再实现 `source.ExtraRoutes`。
4. `internal/config` 加 Cookie/Proxy env 字段与 TTL 常量。
5. `cmd/server/main.go` 的 `Deps.Sources` 注册一行，路由/缓存/合并/错误映射自动生效。
6. 补包内 `_test.go`（解析与校验优先）。
