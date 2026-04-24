# SEAS 后端下钻接口 P0 设计文档

## 1. 概述

本设计文档仅覆盖前端重构后需要后端补齐的 `P0` 范围，即 5 个下钻分析接口：

- `GET /exams/{examId}/classes/{classId}/subjects`
- `GET /exams/{examId}/subjects/{subjectId}/classes`
- `GET /exams/{examId}/subjects/{subjectId}/classes/{classId}/questions`
- `GET /exams/{examId}/subjects/{subjectId}/questions`
- `GET /exams/{examId}/subjects/{subjectId}/classes/{classId}/questions/{questionId}`

本次不包含 AI 分析接口，不包含 PDF/共享报告能力，也不改动前端交互结构。

## 2. 目标

- 让前端 [API_DOCUMENT_FOR_BACKEND.md](/Users/kk/go/src/SEAS/API_DOCUMENT_FOR_BACKEND.md) 中定义的 5 个下钻接口全部切换到真实后端
- 复用现有 Kratos + protobuf + service/usecase/repo 分层，不引入旁路 HTTP handler
- 基于现有数据表完成查询聚合，不新增表、不调整现有前端协议
- 保持与现有分析接口同一风格的路径、返回结构、错误传播方式

## 3. 非目标

- 不实现 `POST /ai/analysis`
- 不重构现有 5 个已上线分析接口
- 不修改前端返回字段命名
- 不引入新的题库表、作答内容表或题目元数据表

## 4. 现状约束

### 4.1 已存在能力

当前后端已经具备：

- `Analysis` service 与统一前缀 `/seas/api/v1`
- 已实现的考试列表、学科列表、学科汇总、班级汇总、四率分析
- 数据表与实体：
  - `scores`：学生在考试某学科的总分
  - `score_items`：某次学科成绩下的小题得分
  - `students` / `classes`
  - `subjects` / `exam_subjects`

### 4.2 关键限制

当前 `score_items` 仅暴露以下稳定字段：

- `score_id`
- `question_number`
- `knowledge_point`
- `score`
- `full_score`
- `is_correct`

因此题目层设计必须接受以下限制：

- 后端内部以 `question_number` 作为题目标识
- 返回给前端的 `questionId` 先与 `questionNumber` 使用同一值
- `questionType` 当前无稳定来源，统一返回空字符串
- `questionContent` 当前无稳定来源，详情接口省略或返回空字符串
- `answerContent` 当前无稳定来源，详情接口省略该字段

这不是临时 TODO，而是本期明确边界。若未来需要真实题型、题干、作答内容，必须补充独立数据源。

## 5. 方案选择

### 方案 A：继续扩展现有 `Analysis` 服务

做法：

- 在 `api/seas/v1/analysis.proto` 中新增 5 个 rpc 和 message
- 生成对应 pb 文件
- 在 `internal/service/analysis.go` 中新增 5 个 handler
- 在 `internal/biz` 和 `internal/data` 中补齐查询与映射

优点：

- 与现有接口体系一致
- 前端无需适配新的 base path 或服务名
- 复用现有依赖注入、server 注册和错误处理

缺点：

- `analysis.proto` 会继续增长

### 方案 B：新增独立 `Drilldown` 服务

优点：

- 逻辑边界更独立

缺点：

- 当前需求只有 5 个接口，工程成本明显高于收益
- 需要新增 proto、service、wire、server 注册与生成物

### 结论

采用方案 A。当前仓库已经把分析能力集中在 `Analysis` 服务内，P0 下钻接口继续沿用这一模式最稳。

## 6. 接口设计

### 6.1 班级学科汇总

路由：

`GET /seas/api/v1/exams/{exam_id}/classes/{class_id}/subjects`

用途：

- 从全科班级汇总视图点击班级后，查看该班级各学科表现

返回语义：

- `overall` 表示该班级全科汇总行
- `subjects` 表示各学科明细
- `classRank` 为该班在该学科下按平均分排序后的名次
- `scoreDiff = classAvgScore - gradeAvgScore`

数据来源：

- `scores` 聚合班级和年级均分、最高分、最低分
- `subjects` 与 `exam_subjects` 提供学科名称与满分

### 6.2 单科班级汇总

路由：

`GET /seas/api/v1/exams/{exam_id}/subjects/{subject_id}/classes`

用途：

- 单科模式顶层视图，展示各班在该学科下的表现

返回语义：

- `overall` 表示全年级汇总
- `classes` 表示各班明细
- `passRate`、`excellentRate` 使用百分比数值，范围 `0-100`
- 排名按 `subjectAvgScore` 从高到低

数据来源：

- `scores` 聚合单科各班平均分
- `students/classes` 统计班级人数
- 阈值暂固定：
  - 优秀线 `90`
  - 及格线 `60`

### 6.3 单科班级题目

路由：

`GET /seas/api/v1/exams/{exam_id}/subjects/{subject_id}/classes/{class_id}/questions`

用途：

- 查看某班某学科每道题的表现

返回语义：

- 每一行对应一个 `question_number`
- `questionId = questionNumber`
- `scoreRate = classAvgScore / fullScore * 100`
- `difficulty` 由年级得分率推导：
  - `>= 80` -> `easy`
  - `>= 60 && < 80` -> `medium`
  - `< 60` -> `hard`

数据来源：

- `score_items` 按题号聚合
- `scores -> students` 关联到班级
- 同时计算班级均分与年级均分

### 6.4 单科题目汇总

路由：

`GET /seas/api/v1/exams/{exam_id}/subjects/{subject_id}/questions`

用途：

- 查看某学科所有题目的年级表现与班级拆解

返回语义：

- `questions` 按 `question_number` 排序
- `classBreakdown` 列出该题各班平均分
- `scoreRate = gradeAvgScore / fullScore * 100`
- `difficulty` 规则与 6.3 保持一致

数据来源：

- `score_items` 按题号聚合
- `students/classes` 提供班级拆分

### 6.5 单科班级题目详情

路由：

`GET /seas/api/v1/exams/{exam_id}/subjects/{subject_id}/classes/{class_id}/questions/{question_id}`

用途：

- 查看某班某题的学生得分详情

返回语义：

- `question_id` 按 `question_number` 解释
- `students` 仅返回该班参与该学科考试的学生
- `classRank` 为该班该题得分排名
- `gradeRank` 为全年级该题得分排名
- 不返回 `answerContent`，除非未来有新数据源

数据来源：

- `score_items` + `scores` + `students`

## 7. 后端结构设计

### 7.1 API 层

修改文件：

- `api/seas/v1/analysis.proto`

新增内容：

- 5 个 rpc
- 5 组 request/reply message
- 题目维度与学生维度的嵌套 message

约定：

- proto 字段继续采用 snake_case 命名
- HTTP path 参数命名与现有风格保持一致：`exam_id`、`subject_id`、`class_id`、`question_id`
- `analysis.proto` 与运行时 HTTP 绑定必须保留上述 snake_case 路径占位符；若 `openapi.yaml` 由于代码生成器规范化而显示为 camelCase path placeholders，可接受，不视为实现偏差

### 7.2 Service 层

修改文件：

- `internal/service/analysis.go`

职责：

- 读取路径参数
- 调用 `ExamAnalysisUseCase`
- 查询 `examName`、`className`、`subjectName` 等展示字段
- 将 biz 结构映射到 pb reply

不在 service 层做复杂统计或排序逻辑。

### 7.3 Biz 层

主要修改文件：

- `internal/biz/exam_analysis.go`
- `internal/biz/score.go`
- `internal/biz/score_item.go`

设计原则：

- 继续由 `ExamAnalysisUseCase` 承担考试分析聚合能力
- 在 biz 层定义 5 组明确的返回结构，避免 service 层直接感知 SQL 查询形状
- 排名、百分比、难度标签等规则放在 biz/usecase 层或 repo 内统一处理，不能散落在 service

建议新增领域结构：

- `ClassSubjectSummaryStats`
- `ClassSubjectItemStats`
- `SingleClassSummaryStats`
- `SingleClassSummaryItemStats`
- `SingleClassQuestionStats`
- `ClassQuestionItemStats`
- `SingleQuestionSummaryStats`
- `SingleQuestionSummaryItemStats`
- `QuestionClassBreakdownStats`
- `SingleQuestionDetailStats`
- `StudentQuestionDetailStats`

### 7.4 Data 层

主要修改文件：

- `internal/data/score.go`
- `internal/data/score_item.go`
- 如缺少基础查询，则补充：
  - `internal/data/class.go`
  - `internal/data/subject.go`
  - `internal/data/exam.go`

原则：

- 总分维度统计继续优先放在 `scoreRepo`
- 小题维度统计优先放在 `scoreItemRepo`
- 若一个接口同时涉及总分和小题，可由 usecase 组合多个 repo 调用，避免单 repo 职责失控

## 8. 查询与计算规则

### 8.1 排名规则

- 班级排名：同考试、同学科下按班级平均分降序排列，名次从 `1` 开始
- 学生题目排名：
  - `classRank`：同班级、同题按得分降序
  - `gradeRank`：同考试同学科全年级、同题按得分降序

本期不处理并列名次跳号，统一按排序后数组位置生成连续名次。

### 8.2 百分比规则

- `scoreRate`、`passRate`、`excellentRate` 保留两位小数
- 前端类型是 `number`，后端直接返回数值百分比，如 `78.35`

### 8.3 题目排序规则

- 优先按 `question_number` 的数值语义排序
- 若无法可靠解析为纯数字，则退化为字符串升序

本期先保证常见的 `"1"`, `"2"`, `"10"` 正确排序。

### 8.4 难度规则

统一根据题目年级得分率推导：

- `easy`: `scoreRate >= 80`
- `medium`: `60 <= scoreRate < 80`
- `hard`: `scoreRate < 60`

## 9. 错误处理

- 路径参数非法时返回参数错误
- 目标考试、学科、班级不存在时返回业务错误
- 题号不存在或该题在目标班级/学科下无数据时返回空结果或业务错误，具体实现统一为“无数据错误”
- 不因部分字段缺失伪造题型、题干、作答内容

## 10. 测试策略

至少覆盖：

- usecase / repo 层聚合逻辑测试
- 题目排序测试
- 排名计算测试
- `questionId = questionNumber` 的兼容测试
- 空数据与非法参数测试
- `go test ./...` 全量通过

如果 proto 有改动，还需要：

- `make api`
- 确认生成物无脏差异

## 11. 实施拆分建议

建议后续 implementation plan 按以下顺序拆分：

1. 扩 proto 与生成代码
2. 增加 biz 领域结构与 usecase 接口
3. 实现班级/学科两个汇总型接口
4. 实现两个题目汇总接口
5. 实现题目详情接口
6. 补测试与回归验证

## 12. 风险

- `score_items.question_number` 如果历史数据格式不统一，题目排序与详情定位会出现边角问题
- 缺失题型、题干、作答内容会导致部分前端字段只能为空，这属于本期已接受限制
- 若部分考试缺少完整 `score_items` 数据，则题目层接口会比总分层接口更容易出现无数据场景

## 13. 结论

本期采用“扩展现有 `Analysis` 服务”的方案，在不改表、不改前端协议的前提下补齐 5 个下钻接口。题目层统一基于 `score_items.question_number` 建模，明确接受题型、题干、作答内容缺失的现实约束，以保证 P0 能稳定落地。
