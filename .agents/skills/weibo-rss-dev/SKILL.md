# weibo-rss-dev

> 从现有代码提炼写法模式；后续新增代码优先对齐现有风格，而非引入新范式。

## 分层与依赖方向
- `src/app.ts` 只做组装，不写业务。
- `src/modules/` 下按职责拆分，weibo 域内再分子域。
- 上层不直接 import weibo/api 内部实现，优先走 `WeiboData` 对外的聚合方法。

## API 模块写法
- 采用工厂函数暴露方法，不直接导出实现类。
- 例：`createIndexAPI()`、`createDomainAPI()`。
- 每个 API 模块自行创建 `Throttler` 与 axios 实例，复用各自 `api/common.ts` 里的 `handleForbiddenErr`、`requestWithRetry`、`MOCK_UA`、`TIME_OUT`。

## 公共防风控设施（antiCrawl.ts）
- 统一实现在 `src/modules/antiCrawl.ts`：`requestWithRetry`（指数退避+抖动）、`handleForbiddenErr`（命中风控→先源钩子→再熔断）。
- 各源通过 `RiskControlHooks` 接口声明源特有风控特征：`riskyStatuses`（视为风控的状态码）、`onRiskDetected`（命中后的钩子，如刷新访客 Cookie）。
- 接入方式：在源的 `api/common.ts` 里 `createRiskControl({ riskyStatuses: [...] })` 生成 `riskControl`，再 re-export 绑定后的 `requestWithRetry` / `handleForbiddenErr`，保持 API 层 import 路径不变。
- 现有声明：weibo `[403, 418]`；instagram `[401, 403, 429]`。默认值为 `[401, 403, 418, 429]`。
- 新增源禁止自己手写重试/退避循环，一律复用 antiCrawl。

## 缓存约定（feedCache.ts）
- 公共实现在 `src/modules/feedCache.ts`，优先使用策略化封装而非裸调 `cache.memo`。
- `CachePolicy` 接口：`keyPrefix` + `infoTTL` / `listTTL`，供数据源级自定义；`memoWithPolicy(cache, policy, "info"|"list", ident, cb)` 自动拼 key 与 TTL。
- `FeedCachePolicy` 接口：`xmlKeyPrefix` + `xmlTTL` + `collapse`；RSS 路由的「请求合并 + XML 缓存」统一走 `cachedFeed(...)`，返回 `{ xmlData, cacheMiss }`。
- 策略对象在各自模块顶部集中声明（如 `weiboCachePolicy`、`instagramCachePolicy`、`weiboFeedPolicy`），TTL 引用 `config.cacheTTL.*`。
- 缓存 key 命名带域前缀：`weibo-info-`/`weibo-list-`（新）、`long-`、`dt-`（细粒度沿用）、XML 层 `xml-`、`instagram-xml-`。
- 细粒度、按内容 id 的缓存（如长文/详情）可保留裸 `cache.memo`，用户/列表级必须走 `memoWithPolicy`。
- 过期清理由缓存实现自身负责（现为 LRU MemoryCache）。

## 公共代理设施（proxy.ts）
- 统一实现在 `src/modules/proxy.ts`：`parseProxy`（URL → axios proxy 配置，非法返回 undefined）、`applyProxy(axiosConfig, proxyUrl)`。
- 各源在 `config.ts` 里声明 `<src>Proxy`（环境变量 `<SRC>_PROXY`，标准 URL 形式），在 `createXxxInstance()` 里用 `applyProxy({...buildAxiosConfig()}, config.<src>Proxy)` 一行接入。
- 新增源禁止手写 URL 解析/proxy 对象拼装，一律复用 applyProxy。

## 新增 RSS 源步骤速查
1. `api/common.ts`：`BASE_HEADERS`、`getCommonHeaders()`（Cookie 取 `config.<src>Cookie`）、`createRiskControl({...})` 声明风控特征并 re-export。
2. `api/xxxAPI.ts`：工厂函数 + `Throttler("<src>")` + axios 实例；try/catch 里 `return handleForbiddenErr(err, disable)`；`UserNotFoundError` 定义于此。
3. 聚合类（对齐 `instagram.ts`）：`cache` + 策略对象，`memoWithPolicy` 缓存对外方法。
4. `config.ts` 加 cookie/TTL 字段，`types.ts` 加类型并挂 `ctx`，`app.ts` 装配，`routes.ts` 加路由（用 `cachedFeed`）。
5. 错误分支约定：404 用户不存在/私密、503 `ThrottledError`、500 未知。

## 错误处理
- 自定义 Error 集中在 `routes.ts` 与对应 API 模块里定义，如 `UidInvalidError`、`UserNotFoundError`、`DomainNotFoundError`。
- 控制器层做类型判断，再返回 HTTP 状态码。
- 限流异常统一返回 503。

## Koa 上下文约定
- 用泛型 `Koa<RSSKoaState, RSSKoaContext>`。
- 把 `cache`、`weibo`、`instagram` 挂到 `ctx`，避免层层传参。
- `ctx.state.hit` 用来标记是否命中缓存。
