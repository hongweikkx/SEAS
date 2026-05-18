# SEAS — Smart-Edu Analysis System

基于 Kratos 的考试成绩多维分析后端服务，支持 Excel 成绩导入、多维度统计分析、AI 智能解读与微信公众号扫码登录。

---

## 核心功能

### 1. 考试与成绩管理

- **创建考试**：仅创建考试记录（名称、日期）。
- **Excel 导入成绩**：`multipart/form-data` 上传，自动识别「简单模式」（仅总分）与「完整模式」（含小题分）。自动创建学生、班级、学科关联。
- **更新学科满分**：为已导入的考试批量设置各学科满分。
- **删除考试**：级联删除关联的成绩、小题、学生等数据。

### 2. 多维成绩分析（13 个 REST API）

分析维度覆盖「考试 → 学科 → 班级 → 题目 → 学生」五级下钻：

| 维度 | 接口 |
|---|---|
| 考试概览 | `GET /exams` 考试列表；`GET /exams/{id}/subjects` 学科列表 |
| 学科汇总 | `GET /exams/{id}/analysis/subject-summary`（全年级 / 单科） |
| 班级汇总 | `GET /exams/{id}/analysis/class-summary`（含离均差、标准差、区分度） |
| 班级学科下钻 | `GET /exams/{id}/classes/{cid}/subjects` |
| 单科班级汇总 | `GET /exams/{id}/subjects/{sid}/classes` |
| 题目维度 | `GET .../subjects/{sid}/classes/{cid}/questions` 单科班级题目汇总 |
| 题目详情 | `GET .../questions/{qid}` 含学生得分明细、班排 / 年排 |
| 试题班级对比 | `GET .../questions/{qid}/class-compare` 各班级同一试题得分横向对比 |
| 四率分析 | `GET /exams/{id}/analysis/rating-distribution`（优秀/良好/中等/及格/低分，阈值可配） |
| 分数段分析 | `POST /exams/{id}/analysis/score-segment`（自定义区间与步长） |
| 名次段分析 | `POST /exams/{id}/analysis/rank-segment`（自定义名次区间） |

所有分析接口均返回**难度、区分度、标准差**等教育测量学指标。

### 3. AI 智能解读

`POST /seas/api/v1/ai/analysis`

调用豆包（ARK）大模型，基于各分析视图的实时数据生成 80-120 字的学业诊断文本。AI 输出支持**内联链接**语法（如 `[[班级名称|single-class-summary|{"classId":"1"}]]`），前端可解析为可跳转的富文本片段。

当前支持的分析视图：`class-summary`、`subject-summary`、`rating-analysis`、`class-subject-summary`、`single-class-summary`、`single-class-question`、`single-question-summary`、`single-question-detail`。

### 4. 认证

微信公众号扫码登录：后端生成 5 位数字验证码，用户关注公众号后回复验证码完成登录。使用 SSE 长连接向浏览器推送登录状态，成功后下发 JWT Token。

---

## 技术栈

- **Go 1.25** + **Kratos v2**（gRPC + HTTP 双协议）
- **Protobuf 3** 定义 API，`protoc-gen-go-http` 自动生成 HTTP 路由
- **SQLite**（WAL 模式）+ **GORM** 作为持久层
- **豆包 ARK**（`eino` + `eino-ext/components/model/ark`）接入大模型
- **Wire** 依赖注入
- **OpenTelemetry** + **Prometheus** 可观测性
- **Zap** + **lumberjack** 结构化日志与轮转
- **Redis**（可选缓存层）

---

## 项目结构

```
SEAS/
├── api/seas/v1/          # Protobuf API 定义（analysis / exam_import / auth）
├── cmd/seas/             # 入口 main.go、Wire 注入（wire.go / wire_gen.go）
├── internal/
│   ├── biz/              # 业务实体与用例（ExamAnalysisUseCase、ExamImportUseCase、AuthUsecase）
│   ├── data/             # 数据访问层（GORM Repo、AutoMigrate）
│   ├── service/          # gRPC/HTTP Service 实现，负责 pb ↔ biz 转换
│   ├── server/           # HTTP/GRPC Server 装配、自定义 Handler（AI分析、微信回调、SSE）
│   └── conf/             # 配置结构（protobuf 定义）
├── pkg/                  # 公共包：jwt、gorm、redis、zaplog、prometheus、helper
├── configs/              # 配置文件（config.yaml / config_example.yaml）
├── data/                 # SQLite 数据库文件目录（.gitkeep）
├── third_party/          # Protobuf 外部依赖（google/api、validate 等）
└── Makefile
```

---

## 快速开始

### 前置条件

- Go 1.25+
- `protoc`、`protoc-gen-go`、`protoc-gen-go-grpc`、`protoc-gen-go-http`、`protoc-gen-openapi`、`wire`
- SQLite（Go 驱动 `mattn/go-sqlite3`，需 CGO）

### 安装工具链

```bash
make init
```

### 编译运行

```bash
# 构建
make build

# 热重载开发（kratos run）
make run

# 默认配置文件 configs/config.yaml
# 首次启动会自动执行 AutoMigrate 创建表结构
cd cmd/seas && go run . -conf ../../configs/config.yaml
```

### 生成代码（修改 .proto 后）

```bash
make api      # 生成 api 层 proto
generate      # go generate + go mod tidy
make all      # api + config + generate
```

---

## 配置说明

`configs/config.yaml` 关键项：

```yaml
server:
  http:
    addr: 0.0.0.0:8000
  grpc:
    addr: 0.0.0.0:9000
data:
  database:
    driver: sqlite
    source: "file:./data/seas.db?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON&_synchronous=NORMAL"
  redis:
    addr: localhost:6379
llm:
  provider: ark
  model: doubao-seed-1.6
  api_key: your-ark-api-key
  api_base: https://ark.cn-beijing.volces.com/api/v3
  temperature: 0.2
auth:
  jwt_secret: your-jwt-secret-key
  wechat_token: your-wechat-token        # 微信公众号消息验证 Token
  wechat_qr_url: https://mp.weixin.qq.com/ # 公众号二维码页面
```

---

## 数据模型

启动时通过 `AutoMigrate` 自动同步：

- `Class` / `Student` / `Subject` — 基础维度
- `Exam` / `ExamSubject` — 考试与学科关联
- `Score` — 学生总分记录
- `ScoreItem` — 小题分记录（完整模式导入时生成）
- `User` — 登录用户（OpenID 关联）

---

## 开发路线

- [x] 考试成绩多维分析（学科/班级/题目/学生）
- [x] Excel 成绩导入（简单/完整模式）
- [x] AI 智能解读（豆包大模型）
- [x] 微信公众号扫码登录
- [x] 四率分析 / 分数段分析 / 名次段分析
- [x] 试题班级对比
