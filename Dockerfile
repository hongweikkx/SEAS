# Stage 1: 构建
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# 安装 CGO 编译依赖
RUN apt-get update && apt-get install -y --no-install-recommends gcc && rm -rf /var/lib/apt/lists/*

# 复制依赖文件并下载（利用 Docker 缓存层）
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并构建
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o bin/seas ./cmd/seas

# Stage 2: 运行
FROM debian:bookworm-slim

# 构建参数：敏感信息（构建时传入，不进入镜像历史层）
ARG LLM_API_KEY
ARG JWT_SECRET
ARG WECHAT_TOKEN

WORKDIR /app

# 安装 CA 证书（HTTPS 调用 LLM API 需要）
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates gettext-base && rm -rf /var/lib/apt/lists/*

# 复制二进制和配置
COPY --from=builder /app/bin/seas ./bin/seas
COPY --from=builder /app/configs/config.docker.yaml ./configs/config.yaml

# 替换配置文件中的占位符为构建参数值
RUN if [ -n "$LLM_API_KEY" ]; then sed -i "s|\\${LLM_API_KEY}|$LLM_API_KEY|g" ./configs/config.yaml; fi && \
    if [ -n "$JWT_SECRET" ]; then sed -i "s|\\${JWT_SECRET}|$JWT_SECRET|g" ./configs/config.yaml; fi && \
    if [ -n "$WECHAT_TOKEN" ]; then sed -i "s|\\${WECHAT_TOKEN}|$WECHAT_TOKEN|g" ./configs/config.yaml; fi

# 创建数据目录并设置非 root 用户
RUN mkdir -p /app/data && groupadd -r seas && useradd -r -g seas seas && chown -R seas:seas /app
USER seas

# 暴露 HTTP 端口
EXPOSE 8000

# 启动命令
ENTRYPOINT ["./bin/seas", "-conf", "./configs/config.yaml"]
