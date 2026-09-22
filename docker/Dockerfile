# Go 版 weibo-rss 极简镜像（多阶段构建）
# 构建：docker build -f docker/Dockerfile -t weibo-rss-go .
# 运行：docker run --rm -p 3000:3000 -e WEIBO_COOKIE=xxx weibo-rss-go
# 最终镜像 ~7MB（单二进制 + CA 证书，静态资源已通过 go:embed 嵌入）

# 编译阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /weibo-rss ./cmd/server

# 运行阶段：scratch，仅保留二进制与 CA 根证书
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /weibo-rss /weibo-rss
EXPOSE 3000
ENTRYPOINT ["/weibo-rss"]
