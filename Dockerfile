FROM node:20-alpine AS builder
WORKDIR /app

# 复制依赖定义
COPY package.json package-lock.json ./

# 安装依赖并构建
RUN npm ci && \
    npm run build

# 运行阶段
FROM node:20-alpine
LABEL maintainer="https://github.com/zgq354/weibo-rss"
WORKDIR /app

# 复制编译产物和依赖
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/dist ./dist
COPY package.json ./

EXPOSE 3000
CMD ["node", "dist/app.js"]
