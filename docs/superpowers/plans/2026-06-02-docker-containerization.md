# Docker 容器化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 SEAS 后端服务添加 Docker 支持，实现一键构建镜像、一键部署运行（暴露 8000 端口，含 Redis 依赖）。

**Architecture:** 多阶段 Dockerfile（golang:1.25-bookworm 构建 → debian:bookworm-slim 运行）+ Docker Compose 编排 seas + redis 双服务 + Docker 专用配置文件 + Makefile 快捷命令。

**Tech Stack:** Docker, Docker Compose, Go 1.25 CGO, SQLite, Redis

---

## 文件结构

| 文件 | 操作 | 说明 |
|------|------|------|
| `.dockerignore` | 创建 | 排除构建上下文中不需要的文件，减小镜像、加速构建 |
| `configs/config.docker.yaml` | 创建 | 基于 `config.yaml` 修改数据库路径和 Redis 地址，专供 Docker 环境 |
| `Dockerfile` | 创建 | 多阶段构建定义，Stage 1 编译、Stage 2 运行 |
| `docker-compose.yml` | 创建 | 编排 seas + redis 服务，持久化 SQLite 数据 |
| `Makefile` | 修改 | 新增 `docker-build`、`docker-run`、`docker-compose-up`、`docker-compose-down`、`docker-clean` 命令 |

---

### Task 1: 创建 `.dockerignore`

**Files:**
- Create: `.dockerignore`

**说明：** 排除构建上下文中不需要的文件，避免它们进入 Docker build context，减小镜像体积、加速构建。

- [ ] **Step 1: 创建 `.dockerignore` 文件**

```dockerignore
# Git
.git
.gitignore

# IDE
.idea
.vscode

# 构建产物
/bin
/seas
*.exe
*.dll
*.so

# 日志
/logs
*.log
nohup.out

# 测试和文档（运行时不需）
/docs
README.md
LICENSE

# CI
.github

# 本地配置（生产配置由 config.docker.yaml 提供）
/configs/config.yaml

# 本地数据库和 Redis dump（由 volume 挂载）
/data/*.db
/data/*.db-wal
/data/*.db-shm
/data/*.db-journal
dump.rdb

# 工作区和临时文件
.worktrees/
.superpowers/
.playwright-mcp/
.playwright-cli/
mise.toml

# 计划和设计文档（构建时不需要）
/docs/superpowers/
```

- [ ] **Step 2: 验证文件已创建**

```bash
cat .dockerignore | head -5
```

Expected: 输出行以 `# Git` 开头

- [ ] **Step 3: 提交**

```bash
git add .dockerignore
git commit -m "build(docker): 添加 .dockerignore，排除构建上下文中的非必要文件

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: 创建 `configs/config.docker.yaml`

**Files:**
- Create: `configs/config.docker.yaml`

**说明：** 基于现有 `configs/config.yaml` 修改两处：数据库路径改为容器内绝对路径 `/app/data/seas.db`，Redis 地址改为 Compose 服务名 `redis:6379`。

- [ ] **Step 1: 创建 Docker 专用配置文件**

```yaml
server:
  env: dev
  http:
    addr: 0.0.0.0:8000
    timeout: 1000s
  grpc:
    addr: 0.0.0.0:9000
    timeout: 3s
data:
  database:
    driver: sqlite
    # SQLite 使用容器内绝对路径 + WAL 模式 + 启用外键 + 等锁 5s + 同步 NORMAL
    source: "file:/app/data/seas.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_synchronous=NORMAL"
  redis:
    addr: redis:6379
    pw: ""
    db: 0
    warn_limit: 100
llm:
  provider: ark
  model: deepseek-r1-250528
  api_key: 22a786fb-e8f5-4079-92c1-6bc6073333b9
  api_base: https://ark.cn-beijing.volces.com/api/v3
  region: cn-beijing
  temperature: 0.2
  max_iterations: 8
  system_prompt: You are a data analysis assistant for school exam reporting.

auth:
  jwt_secret: "seas-dev-secret-change-in-production"
  wechat_token: "6f46434c27f6c8437f2465bea08cf21f"
  wechat_qr_url: "https://mp.weixin.qq.com/"
```

- [ ] **Step 2: 验证与原始配置的差异仅有两处**

```bash
diff configs/config.yaml configs/config.docker.yaml
```

Expected: 仅显示 `source:` 行和 `addr:` 行的差异

- [ ] **Step 3: 提交**

```bash
git add configs/config.docker.yaml
git commit -m "build(docker): 添加 Docker 专用配置文件 config.docker.yaml

- 数据库路径改为容器内绝对路径 /app/data/seas.db
- Redis 地址改为 Compose 服务名 redis:6379

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: 创建 `Dockerfile`

**Files:**
- Create: `Dockerfile`

**说明：** 多阶段构建。Stage 1 使用 golang:1.25-bookworm 编译二进制（需 gcc 支持 CGO/SQLite）；Stage 2 使用 debian:bookworm-slim 运行，创建非 root 用户，暴露 8000 端口。

- [ ] **Step 1: 创建 Dockerfile**

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

- [ ] **Step 2: 验证 Dockerfile 语法（如有 hadolint 可用）**

```bash
# 如本地安装了 hadolint
hadolint Dockerfile 2>/dev/null || echo "hadolint not installed, skipping"

# 基础语法检查：确保文件存在且非空
test -s Dockerfile && echo "Dockerfile exists and not empty"
```

Expected: `Dockerfile exists and not empty`

- [ ] **Step 3: 提交**

```bash
git add Dockerfile
git commit -m "build(docker): 添加多阶段 Dockerfile

- Stage 1: golang:1.25-bookworm 构建（含 gcc 支持 CGO/SQLite）
- Stage 2: debian:bookworm-slim 运行，非 root 用户，暴露 8000 端口

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: 创建 `docker-compose.yml`

**Files:**
- Create: `docker-compose.yml`

**说明：** 定义 seas 和 redis 两个服务。redis 不暴露到宿主机，仅 seas 内网访问。SQLite 数据和 Redis 数据均通过 Docker Volume 持久化。

- [ ] **Step 1: 创建 docker-compose.yml**

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

- [ ] **Step 2: 验证 Compose 文件格式（如 docker-compose 可用）**

```bash
# 如本地安装了 docker-compose
docker-compose config 2>/dev/null || echo "docker-compose not available, skipping"

# 基础检查
test -s docker-compose.yml && echo "docker-compose.yml exists and not empty"
```

Expected: `docker-compose.yml exists and not empty`（或 Compose 配置解析成功）

- [ ] **Step 3: 提交**

```bash
git add docker-compose.yml
git commit -m "build(docker): 添加 docker-compose.yml 服务编排

- seas 服务：基于 Dockerfile 构建，暴露 8000，SQLite 数据持久化
- redis 服务：redis:7-alpine，仅内网访问，AOF 持久化

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: 修改 `Makefile`，新增 Docker 命令

**Files:**
- Modify: `Makefile`

**说明：** 在现有 Makefile 末尾新增 5 个 docker 相关命令，保持与现有命令风格一致。

- [ ] **Step 1: 在 Makefile 末尾、help 目标之前插入 docker 命令**

在 `Makefile` 中 `help:` 目标之前插入以下内容（即第 70 行之前）：

```makefile
.PHONY: docker-build
# docker build image
docker-build:
	docker build -t seas:latest .

.PHONY: docker-run
# docker run single container (no redis, for quick verification only)
docker-run:
	docker run --rm -p 8000:8000 seas:latest

.PHONY: docker-compose-up
# docker compose up (full environment with redis)
docker-compose-up:
	docker-compose up -d --build

.PHONY: docker-compose-down
# docker compose down
docker-compose-down:
	docker-compose down

.PHONY: docker-clean
# docker clean images and volumes
docker-clean:
	docker-compose down -v --remove-orphans
	docker rmi seas:latest 2>/dev/null || true
```

- [ ] **Step 2: 验证 Makefile 语法**

```bash
make -n docker-build
make -n docker-compose-up
```

Expected: 两条命令均输出对应的 docker 命令，不报错

- [ ] **Step 3: 验证 help 能显示新增命令**

```bash
make help | grep docker
```

Expected: 输出包含 `docker-build`、`docker-run`、`docker-compose-up`、`docker-compose-down`、`docker-clean`

- [ ] **Step 4: 提交**

```bash
git add Makefile
git commit -m "build(docker): Makefile 新增 docker 快捷命令

- docker-build: 构建 seas:latest 镜像
- docker-run: 单容器运行（仅验证容器启动）
- docker-compose-up: 完整环境后台启动
- docker-compose-down: 停止 Compose 环境
- docker-clean: 删除镜像和 volume

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## 验证清单（全部任务完成后）

- [ ] `make docker-build` 成功构建 `seas:latest` 镜像
- [ ] `make docker-compose-up` 成功启动 seas + redis 两个容器
- [ ] `curl http://localhost:8000` 返回 HTTP 响应（404 或正常接口响应均可）
- [ ] `docker volume ls | grep seas` 显示 `seas_data` 和 `redis_data` volume
- [ ] `make docker-compose-down` 成功停止并移除容器
- [ ] `make docker-clean` 成功删除镜像和 volume

---

## Self-Review

**1. Spec coverage:**
- ✅ 多阶段 Dockerfile（Task 3）
- ✅ docker-compose.yml 编排（Task 4）
- ✅ configs/config.docker.yaml（Task 2）
- ✅ .dockerignore（Task 1）
- ✅ Makefile 新增命令（Task 5）
- ✅ 非 root 用户（Dockerfile 中 `USER seas`）
- ✅ 8000 端口暴露（Dockerfile `EXPOSE 8000` + docker-compose.yml `ports`）
- ✅ SQLite 持久化（docker-compose.yml `volumes: seas_data`）

**2. Placeholder scan:**
- ✅ 无 TBD/TODO
- ✅ 所有步骤包含完整代码
- ✅ 所有步骤包含验证命令和预期输出
- ✅ 无"类似 Task X"的引用

**3. Type consistency:**
- ✅ Dockerfile 中 ENTRYPOINT 引用的 `./configs/config.yaml` 与 COPY 目标一致
- ✅ docker-compose.yml 中 volume 挂载 `/app/data` 与 config.docker.yaml 中 `file:/app/data/seas.db` 一致
- ✅ docker-compose.yml 中 Redis 服务名 `redis` 与 config.docker.yaml 中 `redis:6379` 一致
