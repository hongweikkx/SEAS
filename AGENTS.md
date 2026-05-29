# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

---

## 项目概述

SEAS（Smart-Edu Analysis System）是一个基于 **Kratos v2** 框架的考试成绩多维分析后端服务。使用 **Go 1.25**，数据库为 **SQLite**（WAL 模式），接入 **豆包（ARK）大模型** 做 AI 智能解读，认证采用 **微信公众号扫码登录**（5 位验证码 + SSE 推送）。

---

## 常用命令

```bash
# 开发工具链安装（protoc 插件、wire、kratos CLI）
make init

# 构建二进制到 bin/
make build

# 热重载开发
make run

# 直接运行（非 kratos run）
cd cmd/seas && go run . -conf ../../configs/config.yaml

# 修改 .proto 后重新生成代码
make api          # 生成 api/seas/v1/*
make config       # 生成 internal/conf/*
make generate     # go generate ./... + go mod tidy
make all          # 以上三条全部执行

# 测试
go test ./...
go test ./internal/biz/...
go test -run TestQuestionNumberLess ./internal/biz
go test -v ./pkg/gorm/...

# Wire 依赖注入（修改 wire.go 后）
cd cmd/seas && wire
```

CI（GitHub Actions）在 `push` / `pull_request` 到 `main` 时执行 `go build -v ./...` + `go test -v ./...`。

---

## 高层架构

### Kratos 四层分层

```
api/seas/v1/          → Protobuf API 定义 + 生成的 pb / grpc / http 代码
internal/service/     → gRPC/HTTP Service 实现，负责 pb 消息结构 ↔ biz 实体之间的转换
internal/biz/         → 业务用例（UseCase）与领域实体，纯业务逻辑
internal/data/        → GORM Repo 实现，封装 SQL 查询
internal/server/      → HTTP/GRPC Server 装配，以及自定义 Handler（AI分析、微信回调、SSE）
```

依赖注入通过 **Wire** 管理，ProviderSet 分散在各层：
- `biz.ProviderSet` — `NewAnalysisUseCase`、`NewExamAnalysisUseCaseWithScoreItem`、`NewExamImportUseCase`、`NewAuthUsecase`
- `data.ProviderSet` — 各 Repo 构造函数 + `NewData`
- `service.ProviderSet` — 各 Service 构造函数
- `server.ProviderSet` — `NewHTTPServer`、`NewGRPCServer`、自定义 Handler

入口 `cmd/seas/wire.go` 将以上 ProviderSet 与 `newApp`、`NewTraceProvider` 组装为 `kratos.App`。

### 关键设计决策

**1. ScoreItemRepo 的显式注入**

`ExamAnalysisUseCase` 的构造函数 `NewExamAnalysisUseCase` 只接收 `ExamRepo`、`SubjectRepo`、`ScoreRepo`。题目维度接口（如 `GetSingleClassQuestions`、`GetSingleQuestionSummary`、`GetSingleQuestionDetail`、`GetSingleQuestionClassCompare`）需要 `ScoreItemRepo`，必须通过 `WithScoreItemRepo` 方法注入。若未注入就调用这些接口，会返回 `"score item repo is not configured"` 错误。

**2. 自定义 HTTP 路由覆盖 protobuf 生成路由**

Excel 成绩导入使用 `multipart/form-data`，protobuf 无法原生定义文件字段。因此在 `internal/server/http.go` 中，通过 `srv.Route("/").POST(...)` 在 `RegisterExamImportHTTPServer` **之前** 注册自定义 handler，覆盖 protobuf 生成的 JSON handler。顺序不可颠倒。

**3. SQLite 相对路径转绝对路径**

`cmd/seas/main.go` 启动时会检测 `data.database.source`：若以 `file:` 开头且包含 `./`，则基于配置文件路径计算项目根目录，将相对路径转为绝对路径。这是为了避免 `kratos run` 时工作目录变化导致找不到数据库文件。

**4. AI 分析是独立 HTTP Handler，非 protobuf 接口**

`POST /seas/api/v1/ai/analysis` 由 `AIAnalysisHandler`（`internal/server/ai_analysis.go`）直接处理，不走 protobuf 生成的路由。该 handler 接收 `{view, examId, params}`，根据 `view` 字段调用对应 `AnalysisService` 方法获取数据，组装 prompt 后调用豆包大模型生成分析文本，并解析内联链接语法返回结构化 segment 数组。

**5. 微信公众号登录流程**

- `LoginRequest` 生成 5 位数字验证码 + 公众号二维码 URL
- 浏览器通过 SSE (`/seas/api/v1/auth/login-sse?code=xxxxx`) 长连接轮询登录状态
- 用户关注公众号后回复验证码，微信推送 XML 消息到 `/seas/api/v1/auth/wechat/callback`
- 后端验证验证码，更新登录状态，SSE 推送 `success` 并下发 JWT Token
- `AuthHandler` 和 `LoginSSEHandler` 均为自定义 HTTP Handler，在 `NewHTTPServer` 中通过 `srv.Handle` 注册

**6. 数据模型自动迁移**

启动时调用 `data.AutoMigrate`，同步以下表结构（仅追加，不删除）：`Class`、`Student`、`Subject`、`Exam`、`ExamSubject`、`Score`、`ScoreItem`、`User`。

---

## 配置要点

配置文件 `configs/config.yaml`，关键字段：

- `data.database.source` — SQLite 文件路径，支持查询参数控制 WAL / 外键 / 超时等
- `llm.*` — 豆包 ARK 模型配置（`api_key`、`model`、`api_base`、`temperature`）
- `auth.jwt_secret` — JWT 签名密钥
- `auth.wechat_token` / `auth.wechat_qr_url` — 微信公众号消息验证 Token 和二维码页面

开发时复制 `configs/config_example.yaml` 修改即可。
