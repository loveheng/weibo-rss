# weibo-rss-dev

> 从现有代码提炼写法模式；后续新增代码优先对齐现有风格，而非引入新范式。

## 分层与依赖方向
- `src/app.ts` 只做组装，不写业务。
- `src/modules/` 下按职责拆分，weibo 域内再分子域。
- 上层不直接 import weibo/api 内部实现，优先走 `WeiboData` 对外的聚合方法。

## API 模块写法
- 采用工厂函数暴露方法，不直接导出实现类。
- 例：`createIndexAPI()`、`createDomainAPI()`。
- 每个 API 模块自行创建 `Throttler` 与 axios 实例，复用 `handleForbiddenErr`、`MOCK_UA`、`TIME_OUT`。

## 错误处理
- 自定义 Error 集中在 `routes.ts` 与对应 API 模块里定义，如 `UidInvalidError`、`UserNotFoundError`、`DomainNotFoundError`。
- 控制器层做类型判断，再返回 HTTP 状态码。
- 限流异常统一返回 503。

## 缓存约定
- 优先用 `CacheInterface.memo()` 缓存接口结果。
- 过期清理由 `LevelCache.startScheduleCleanJob()` 负责。
- 缓存 key 命名建议带域前缀：`xml-`、`dm-`、`info-`、`list-`、`long-`、`dt-`。

## Koa 上下文约定
- 用泛型 `Koa<RSSKoaState, RSSKoaContext>`。
- 把 `cache` 和 `weibo` 挂到 `ctx`，避免层层传参。
- `ctx.state.hit` 用来标记是否命中缓存。

## 工具调用格式教训
- `edit_file` 的 XML 参数必须成对闭合。
- 在生成 `