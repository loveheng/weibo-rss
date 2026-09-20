---
# weibo-rss-workflow

> 只做事实提取：模块结构、包名、构建/测试命令、环境约束。规范正文指针到既有 skill，不复制。

## 项目形态
- 编码项目（Node.js + TypeScript + Koa）
- 非聊天/助理工作区

## 模块结构
- 入口：src/app.ts
- 配置：src/config.ts（可加载自定义 config.js）
- 类型：src/types.ts
- 工具：src/utils.ts
- 路由：src/modules/routes.ts
- 缓存：src/modules/cache.ts
- 限流：src/modules/throttler.ts
- 日志：src/modules/logger.ts
- 微博：src/modules/weibo/weibo.ts
- 微博 API：src/modules/weibo/api/*.ts

## 构建/测试/运行
- 安装：pnpm i
- 构建：pnpm build（tsc）
- 开发：pnpm dev（cross-env DEBUG=1 ts-node-dev src/app.ts）
- 启动：pnpm serve（node dist/app.js）
- 测试：pnpm test（jest）
- 端口：默认 3000（可通过环境变量 PORT 指定）

## 环境硬约束
- 依赖 Node.js 与 pnpm
- LevelDB 依赖 leveldown 原生模块，部分环境需单独处理构建
- 默认 TTL：rssTTL 15 分钟；cacheTTL 按接口类型分级
- 对外限流：np-queue，默认 10 分钟重试间隔
- 容器部署：Dockerfile / docker-compose.yml / pm2 process.json

## 已有脚本
- 无仓库内持久脚本；后续新增脚本统一登记到 scripts/agent-tools/
