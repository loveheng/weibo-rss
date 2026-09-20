FROM node:20-alpine AS builder
WORKDIR /app

<<<<<<< HEAD
# 安装构建 leveldown 原生 C++ 模块所需的系统工具
RUN apk add --no-cache python3 make g++

# 全局安装 pnpm
RUN npm i -g pnpm
# 全局安装 pnpm（固定版本，保证 CI 构建可复现）
RUN npm i -g pnpm@12.5.1

# 复制依赖定义
COPY package.json pnpm-lock.yaml ./
# 复制依赖定义（pnpm-workspace.yaml 的 allowBuilds 授权 leveldown 编译脚本）
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./

# 复制项目代码
# 先装依赖再拷代码，代码改动可复用依赖层缓存
RUN pnpm install --frozen-lockfile --unsafe-perm

# 复制项目代码并完成打包
COPY . .

# 允许原生模块脚本自动编译，并完成打包
RUN pnpm config set only-built-dependencies "*" && \
    pnpm install --unsafe-perm && \
    pnpm build
RUN pnpm build

# 2. 运行阶段
FROM node:18-alpine
LABEL maintainer="https://github.com/zgq354/weibo-rss"
WORKDIR /app

# 安装运行所需的系统工具（包含 dumb-init 和 leveldown 编译依赖）
RUN apk add --no-cache dumb-init python3 make g++

# 全局安装 pnpm
RUN npm install -g pnpm
RUN npm i -g pnpm@12.5.1

# 复制依赖定义
COPY package.json pnpm-lock.yaml ./
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./

# 允许原生模块编译并安装生产依赖
RUN pnpm config set only-built-dependencies "*" && \
    pnpm install --prod --no-frozen-lockfile --unsafe-perm
# 安装生产依赖（alpine 无预编译二进制，leveldown 在此步编译）
RUN pnpm install --prod --frozen-lockfile --unsafe-perm

# 编译完成后清理构建依赖，精简镜像
RUN apk del python3 make g++

# 复制编译产物和源码
COPY . .
=======
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
>>>>>>> a9a32b2 (项目架构升级)
COPY --from=builder /app/dist ./dist
COPY package.json ./

EXPOSE 3000
CMD ["node", "dist/app.js"]
