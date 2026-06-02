# SEAS Docker 容器化设计文档

## 概述

本文档描述 SEAS 后端服务的 Docker 容器化方案，实现"一键构建镜像 + 一键部署运行"的目标。

## 设计决策

### 1. 多阶段构建（Multi-stage Build）

采用两阶段构建：
- **Stage 1（builder）**：基于 `golang:1.25-bookworm`，安装 `gcc` 工具链，编译生成二进制
- **Stage 2（runner）**：基于 `debian:bookworm-slim`，仅保留运行所需的最小环境

**理由**：
- SEAS 依赖 `github.com/mattn/go-sqlite3`，该库使用 CGO，需要 gcc 编译
- Debian 的 glibc 比 Alpine 的 musl 对 CGO 兼容性更好，避免运行时符号问题
- 镜像从单阶段的 ~500MB 压缩到 ~150MB，收益明显

### 2. 配置打包进镜像

Docker 运行时使用内置的 `configs/config.docker.yaml`，不依赖外部挂载。

**理由**：
- 自包含，镜像即部署单元，`docker run` 即可启动
- 敏感信息（API Key、JWT Secret）由构建者自行替换后构建

### 3. Docker Compose 编排

提供 `docker-compose.yml`，同时拉起 `seas` + `redis` 两个服务。

**理由**：
- SEAS 依赖 Redis（验证码、会话等），Compose 保证环境完整性
- SQLite 数据通过 Docker Volume 持久化，容器重启不丢数据
- 对外仅暴露 8000 端口，Redis 不暴露到宿主机

## 文件结构

```
项目根目录
├── Dockerfile                    # 多阶段构建定义
├── docker-compose.yml            # 服务编排
├── configs/
│   └── config.docker.yaml        # Docker 环境专用配置
├── .dockerignore                 # 构建忽略规则
└── Makefile                      # 新增 docker-* 命令
```

## Dockerfile 详解

```dockerfile
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

WORKDIR /app

# 安装 CA 证书（HTTPS 调用 LLM API 需要）
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

# 复制二进制和配置
COPY --from=builder /app/bin/seas ./bin/seas
COPY --from=builder /app/configs/config.docker.yaml ./configs/config.yaml

# 创建数据目录并设置非 root 用户
RUN mkdir -p /app/data && groupadd -r seas && useradd -r -g seas seas && chown -R seas:seas /app
USER seas

# 暴露 HTTP 端口
EXPOSE 8000

# 启动命令
ENTRYPOINT ["./bin/seas", "-conf", "./configs/config.yaml"]
```

## docker-compose.yml 详解

```yaml
version: "3.8"

services:
  redis:
    image: redis:7-alpine
    container_name: seas-redis
    restart: unless-stopped
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    networks:
      - seas-network
    # Redis 不暴露到宿主机，仅 seas 服务内网访问

  seas:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: seas-app
    restart: unless-stopped
    ports:
      - "8000:8000"
    volumes:
      - seas_data:/app/data    # SQLite 数据库持久化
    depends_on:
      - redis
    networks:
      - seas-network

volumes:
  seas_data:
    driver: local
  redis_data:
    driver: local

networks:
  seas-network:
    driver: bridge
```

## 配置适配（config.docker.yaml）

与 `config.yaml` 的差异：

| 字段 | 原值 | Docker 值 | 说明 |
|------|------|----------|------|
| `server.http.addr` | `0.0.0.0:8000` | `0.0.0.0:8000` | 不变，已适配 |
| `data.database.source` | `file:./data/seas.db?...` | `file:/app/data/seas.db?...` | 绝对路径，避免 CWD 问题 |
| `data.redis.addr` | `localhost:6379` | `redis:6379` | Compose 服务名解析 |

## Makefile 新增命令

| 命令 | 作用 |
|------|------|
| `make docker-build` | 构建 Docker 镜像，标签 `seas:latest` |
| `make docker-run` | 单容器运行（仅验证容器能否启动，无 Redis，功能不完整） |
| `make docker-compose-up` | Docker Compose 后台启动完整环境 |
| `make docker-compose-down` | 停止并清理 Compose 环境 |
| `make docker-clean` | 删除 seas 镜像、停止的容器及相关 volume |

## 部署流程

### 首次部署

```bash
# 1. 克隆代码并进入目录
git clone <repo> && cd SEAS

# 2. 修改 configs/config.docker.yaml 中的敏感信息（API Key、JWT Secret 等）

# 3. 一键构建并启动
make docker-compose-up

# 4. 访问服务
curl http://localhost:8000
```

### 更新版本

```bash
# 拉取最新代码
git pull

# 重新构建并启动
make docker-compose-down
make docker-compose-up
```

## 安全检查清单

- [x] 不使用 root 运行（`bookworm-slim` 默认非 root，可进一步增强）
- [x] 敏感信息通过配置文件注入，不硬编码在 Dockerfile
- [x] 仅暴露必要的 8000 端口
- [x] Redis 不暴露到宿主机
- [x] 使用 Docker Volume 持久化数据
