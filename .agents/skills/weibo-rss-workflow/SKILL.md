---
# weibo-rss-workflow

> 只做事实提取：模块结构、包名、构建/测试命令、环境约束。规范正文指针到既有 skill，不复制。

## 项目形态
- 编码项目（Go 1.22+ 标准库 net/http，gorilla/feeds + golang-lru + x/sync 三个依赖）
- 非聊天/助理工作区

## 模块结构
- 入口：cmd/server/main.go
- 配置：internal/config/config.go（环境变量 + 各层缓存 TTL 常量）
- 订阅源接口：internal/source/source.go
- 路由：internal/web/server.go（与具体源解耦的通用注册）
- 缓存：internal/cache/（TTL-LRU + 策略层）
- 限流：internal/throttler/（串行 + 熔断冷却）
- 防风控：internal/anticrawl/（重试/钩子/Jitter）
- 公共上游客户端：internal/upstream/（Client + Fetcher）
- RSS 组装：internal/feed/
- 微博源：internal/source/weibo/（含访客 Cookie 轮换、domain2uid）
- Instagram 源：internal/source/instagram/

## 构建/测试/运行
- 构建：go build ./...
- 测试：go test ./...
- 静态检查：go vet ./...（提交前建议 gofmt -l . 为空）
- 开发运行：go run ./cmd/server
- 生产二进制：CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o weibo-rss ./cmd/server
- Go 工具链本机未装时可用 /tmp/go/bin/go（1.22.10，从 aliyun 镜像解压）；go mod 走 GOPROXY=https://goproxy.cn,direct
- 端口：默认 3000（环境变量 PORT）

## 环境硬约束
- 纯静态编译（CGO_ENABLED=0），无原生模块依赖
- 静态资源不依赖磁盘（首页已移除，纯 API 服务）
- 常驻内存 ~13MB；二进制 ~5.9MB；scratch 镜像 ~6MB
- 缓存 TTL 分级：XML/列表 15 分钟、用户信息 24 小时、长文/详情/域名 7 天、Instagram 1 小时
- 熔断冷却：微博系 10 分钟、Instagram 30 分钟
- 对上游克制：每接口组串行（并发 1）+ 请求前 0~100ms 抖动
- 容器部署：docker/Dockerfile（-f 指定）/ docker-compose.yml
- 对外行为基线：非法标识 404、风控/限流 503、缓存统计 GET /admin/cache-stats

## 已有脚本
- 无仓库内持久脚本；后续新增脚本统一登记到 scripts/agent-tools/
