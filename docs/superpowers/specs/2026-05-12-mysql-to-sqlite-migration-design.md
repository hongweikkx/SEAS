# 设计文档:MySQL → SQLite 迁移

- 日期: 2026-05-12
- 状态: 草案,待用户审查
- 负责: SEAS

## 1. 背景与目标

### 1.1 背景

SEAS 是一个基于 Go + Kratos + GORM 的学校考试报告服务,当前使用 MySQL 作为持久化存储。
部署目标机器规格为 2 核 2 GB 内存,MySQL 守护进程会常驻 200~400 MB 内存,
占用比例高、运维成本不必要。

业务特征:

- 单机、单租户(一所学校)
- 数据量小:`scores` 表 10~50 万行/年,`score_items` 表数百万行/年级
- 工作负载:**写少读多**——考完一次集中导入成绩,平时是大量统计查询
- 已使用 Redis 做热数据缓存

这种规模与负载,SQLite 完全胜任,且在 WAL 模式下复杂统计查询的延迟通常优于
通过 TCP 访问的 MySQL(无网络往返)。

### 1.2 目标

- **完全替换 MySQL**,不保留双驱动切换能力
- 业务层(`internal/biz/`、`internal/service/`)**零改动**
- 8 处使用 `STDDEV_POP`/`STDDEV_SAMP` 的 raw SQL **一字不动**
- 释放至少 200 MB 内存给应用进程和 OS page cache
- 部署简化为单二进制 + 一个 .db 文件

### 1.3 非目标

- 不做现有 MySQL 数据迁移(用户确认无需保留)
- 不做读写分离的连接池(初版单写连接已够用,需要时再升级)
- 不引入 SQLite 外部扩展(.so/.dylib),保持单二进制可部署

---

## 2. 架构

### 2.1 总体结构

```
┌─────────────────────────────────────────────────────────────┐
│ configs/config.yaml                                          │
│  driver: sqlite                                              │
│  source: ./data/seas.db?_pragma=...                          │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│ pkg/gorm/gorm.go (修改)                                       │
│  ① 调用 funcs.RegisterAggregates()                            │
│  ② sqlite.Open(source) → GORM 打开连接                        │
│  ③ SetMaxOpenConns(1) / SetMaxIdleConns(1)                   │
└──────────────────────────┬───────────────────────────────────┘
                           │
        ┌──────────────────┼───────────────────┐
        ▼                  ▼                   ▼
┌──────────────┐  ┌──────────────────┐  ┌─────────────────┐
│ pkg/gorm/    │  │ internal/data/   │  │ internal/data/  │
│  funcs.go    │  │  migrate.go (新) │  │  data.go (改)   │
│  (新)        │  │  AutoMigrate     │  │  调用 migrate   │
│ STDDEV_POP   │  │  8 张表          │  └─────────────────┘
│ STDDEV_SAMP  │  └──────────────────┘
└──────────────┘
        ▲
        │ 注册 (deterministic)
        │
┌──────────────────────────────────────────────────────┐
│ internal/data/{score,score_item,exam,...}.go (改)     │
│  4 处 raw SQL 改写(GREATEST/CEIL/DELETE-JOIN)         │
│  其余 raw SQL 不变(窗口函数、CTE、IFNULL 等)           │
└──────────────────────────────────────────────────────┘
```

### 2.2 模块边界

| 模块 | 职责 | 对外接口 |
|------|------|---------|
| `pkg/gorm` | 数据库连接 + SQLite 自定义函数注册 | `Init(logger, dsn) (*gorm.DB, closeFn, error)` |
| `internal/data/migrate.go`(新) | 启动时把所有 model 同步成表/索引 | `AutoMigrate(db *gorm.DB) error` |
| `internal/biz` | model 定义(GORM tag) | 与数据库无关的业务接口 |
| `internal/data` | repo 实现,raw SQL 调用 | repo 接口,由 wire 注入 |

`pkg/gorm` 内部不依赖业务包;`internal/data` 依赖 `pkg/gorm` 和 `internal/biz`。

---

## 3. 关键技术决策

### 3.1 驱动选择

**选用 `github.com/glebarez/sqlite`(基于 `modernc.org/sqlite`,纯 Go)**。

| 维度 | `glebarez/sqlite`(纯 Go) | `gorm.io/driver/sqlite`(CGO) |
|------|--------------------------|------------------------------|
| CGO | 不需要 | 需要 |
| 交叉编译 | 直接 `GOOS=linux go build` | 需 gcc 工具链 |
| 性能 | 略低(<10%) | 略高 |
| 部署体积 | 单二进制 | 单二进制(需链接 sqlite3) |
| 自定义函数注册 | 支持 (`sqlite.MustRegisterDeterministicScalarFunction` 等) | 支持 |

2C2G 部署机器上避免装 gcc/glibc-devel,**纯 Go 驱动更省事**。性能差距对本项目可忽略。

### 3.2 DSN(连接字符串)

```
./data/seas.db?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=cache_size(-20000)
```

| PRAGMA | 值 | 作用 |
|--------|---|------|
| `journal_mode` | `WAL` | 读写不互相阻塞,WAL 日志支持崩溃恢复 |
| `synchronous` | `NORMAL` | WAL 下的安全/性能平衡(配合 WAL 日志,断电仅可能丢失最后一个未提交事务) |
| `busy_timeout` | `5000` | 写锁竞争时最多等 5s,避免 `database is locked` |
| `foreign_keys` | `1` | 启用外键约束(SQLite 默认关) |
| `cache_size` | `-20000` | 每连接 20 MB page cache(单位 KB,负数=KB) |

### 3.3 连接池

| 参数 | MySQL 当前 | SQLite 新值 | 理由 |
|------|-----------|-------------|------|
| `MaxOpenConns` | 100 | **1** | SQLite 单写者;统一池保证写串行 |
| `MaxIdleConns` | 10 | **1** | 同上 |
| `ConnMaxLifetime` | 1h | **0**(永久) | 本地文件,无连接老化 |

WAL 模式下读取走快照,**读取不会被写锁住**;且业务模式是集中写 + 大量读,
单连接性能足够。如果将来读 QPS 成为瓶颈,可改用 `gorm.io/plugin/dbresolver`
的读写分离池(读池可放 N 个连接),但**初版不引入**。

### 3.4 STDDEV 自定义聚合函数

`modernc.org/sqlite` 暴露 `sqlite.MustRegisterAggregateFunction(name string, nArg int, makeAggregate func() function.AggregateFunction)`,
在 `database/sql` 打开连接前**全局**注册一次,所有连接立即可用。

实现位置:`pkg/gorm/funcs.go`

```go
// 伪代码(实际接口以 modernc.org/sqlite 当前版本为准)
type stddevAgg struct {
    n      int
    sum    float64
    sumSq  float64
    pop    bool
}
func (s *stddevAgg) Step(v float64) {
    s.n++
    s.sum   += v
    s.sumSq += v * v
}
func (s *stddevAgg) Final() (any, error) {
    if s.n == 0 { return nil, nil }              // NULL
    if !s.pop && s.n == 1 { return nil, nil }    // SAMP 单值 → NULL
    mean := s.sum / float64(s.n)
    variance := (s.sumSq / float64(s.n)) - mean*mean
    if variance < 0 { variance = 0 }             // 浮点误差保护
    if s.pop {
        return math.Sqrt(variance), nil
    }
    // 样本方差 = 总体方差 * n / (n-1)
    return math.Sqrt(variance * float64(s.n) / float64(s.n-1)), nil
}
```

注册:`pkg/gorm.Init` 在 `gorm.Open` 之前调用 `RegisterAggregates()`,
内部为 `STDDEV_POP`、`STDDEV_SAMP` 各注册一次(deterministic=true,单参数)。

**实现说明:**
- 单次扫描算法(Welford 算法的简化变体);精度对教育场景完全够用
- `deterministic=true` 允许 SQLite 把它用在表达式索引等优化路径
- 不处理 NULL 输入(SQL 标准:NULL 不计入统计),由 SQLite 在 Step 调用层过滤

### 3.5 DDL 同步:GORM AutoMigrate

废弃 `init.sql`(MySQL 方言无法兼容 SQLite),改用 GORM `AutoMigrate`。

**实现位置:** `internal/data/migrate.go`

```go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &biz.Class{},
        &biz.Student{},
        &biz.Subject{},
        &biz.Exam{},
        &biz.ExamSubject{},
        &biz.Score{},
        &biz.ScoreItem{},
        &biz.User{},
    )
}
```

调用时机:`internal/data/data.go` 的 `NewData` 中,在 `gormsql.Init` 成功之后、
任何 repo 操作之前。

**前置条件:**所有 model 的 GORM 标签必须完整声明:
- 主键(已有)
- 单列唯一索引(已有)
- 复合唯一索引(**需补全**——见 § 4.3)
- 普通索引(**部分需补全**)

**关于外键约束:**GORM 在 model 字段上声明 `gorm:"foreignKey:ClassID;references:ID"`
可生成 SQLite 的 `FOREIGN KEY` 子句。**本次按 init.sql 中的级联策略补齐外键**,
以保留考试删除时自动清理 scores/score_items 的行为(`ON DELETE CASCADE`),以及
学生删除受限于班级存在(`ON DELETE RESTRICT`)。

### 3.6 其他 4 处 raw SQL 改写

#### 3.6.1 GREATEST → CASE WHEN(2 处)

`internal/data/score_item.go:41, :108`

```sql
-- 改写前(MySQL)
GREATEST(MAX(si.full_score), MAX(si.score)) AS full_score

-- 改写后(SQLite,标量 MAX 不能直接套聚合 MAX 结果,需用 CASE)
CASE WHEN MAX(si.full_score) > MAX(si.score)
     THEN MAX(si.full_score)
     ELSE MAX(si.score)
END AS full_score
```

**注:** 严格来说,SQLite 通过参数数量区分 `MAX(col)` 聚合与 `MAX(a, b)` 标量,
`MAX(MAX(a), MAX(b))` 这种嵌套语法在 3.x 主流版本上能正常工作。但**用 CASE 表达更清晰**,
避免读者(和未来扩展为多列时)误读,因此本次按 CASE 改写。

#### 3.6.2 CEIL → 整数转换(1 处)

`internal/data/score_item.go:179`

```sql
-- 改写前
SELECT CEIL(COUNT(*) * 0.27) as k FROM student_total

-- 改写后
SELECT CAST((COUNT(*) * 0.27 + 0.999999) AS INTEGER) as k FROM student_total
```

**注:** SQLite 没有 `CEIL`/`CEILING` 内置函数。`+ 0.999999` 比 `+ 1` 安全
(避免整数倍数情况下多 1)。学生数 N 极少为 0(查询前提是有学生),不处理 0 情况。

#### 3.6.3 DELETE...JOIN(1 处)

`internal/data/exam.go:78-82`

```sql
-- 改写前
DELETE si FROM score_items si
JOIN scores sc ON sc.id = si.score_id
WHERE sc.exam_id = ?

-- 改写后
DELETE FROM score_items
WHERE score_id IN (SELECT id FROM scores WHERE exam_id = ?)
```

语义等价。`scores.id` 主键索引存在,子查询性能 OK。

### 3.7 SUM() 返回类型差异处理

`internal/data/score.go:945-959` 中有 string 回退分支(MySQL `SUM()` 返回
`DECIMAL` → GORM 扫描为 string)。SQLite 的 `SUM()` 在整数列上返回 INTEGER,
实数列上返回 REAL,**不会**返回 string,该分支在 SQLite 下不会命中。

**处理:保留**(零成本,且若将来再切换驱动也能兼容)。

---

## 4. 数据模型变更

### 4.1 现有 model 完整度梳理

✅ 完整:`Class`、`Student`、`Subject`、`Exam`、`ScoreItem` 主体字段
⚠️ 不完整:见下方 § 4.2 / 4.3 / 4.4

### 4.2 `User`(internal/biz/auth.go)— 完整补全 GORM 标签

```go
type User struct {
    ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
    OpenID    string    `gorm:"uniqueIndex;type:varchar(64);not null;column:openid"`
    CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
    UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at"`
}

func (User) TableName() string { return "users" }
```

### 4.3 唯一索引补全

| Model | 索引列 | 类型 | 注解 |
|-------|--------|------|------|
| `Subject`(subject.go) | `code` | 单列唯一 | `gorm:"uniqueIndex;type:varchar(50);column:code"` —— init.sql 有,model 漏了 |
| `ExamSubject`(exam.go) | `(exam_id, subject_id)` | 复合唯一 | `gorm:"uniqueIndex:idx_exam_subject;column:exam_id"` 同 `column:subject_id` |
| `Score`(score.go) | `(student_id, exam_id, subject_id)` | 复合唯一 | `gorm:"uniqueIndex:idx_student_exam_subject;..."` 三字段同名 |

### 4.4 普通索引补全

| Model | 索引列 |
|-------|--------|
| `Score` | `student_id`、`exam_id`、`subject_id` |
| `ScoreItem` | `score_id`(已有 `index`)、`knowledge_point` |
| `ExamSubject` | `exam_id`(由复合索引覆盖)、`subject_id` |
| `Exam` | `exam_date` |

### 4.5 外键约束声明

| Model | 字段 | 引用 | OnDelete |
|-------|------|------|----------|
| `Student.ClassID` | `Class.ID` | RESTRICT |
| `ExamSubject.ExamID` | `Exam.ID` | CASCADE |
| `ExamSubject.SubjectID` | `Subject.ID` | CASCADE |
| `Score.StudentID` | `Student.ID` | CASCADE |
| `Score.ExamID` | `Exam.ID` | CASCADE |
| `Score.SubjectID` | `Subject.ID` | CASCADE |
| `ScoreItem.ScoreID` | `Score.ID` | CASCADE |

通过 GORM 关联字段 + `constraint:OnDelete:CASCADE`(或 `RESTRICT`)声明。

### 4.6 类型映射注意点

| MySQL 类型 | SQLite 实际存储 | GORM 标签建议 |
|-----------|----------------|---------------|
| `BIGINT UNSIGNED` | INTEGER(无 unsigned 概念) | 保持 `uint64`,SQLite 存为 INTEGER |
| `TINYINT(1)` for bool | INTEGER 0/1 | `bool` 即可 |
| `FLOAT` | REAL | `float64` |
| `VARCHAR(N)` | TEXT(忽略长度限制) | 保留 `type:varchar(N)`,SQLite 接受但不强制 |
| `TIMESTAMP` | TEXT(GORM 自动 RFC3339) | `time.Time` |

---

## 5. 配置变更

### 5.1 `configs/config.yaml`

```yaml
data:
  database:
    driver: sqlite
    source: ./data/seas.db?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=cache_size(-20000)
```

### 5.2 `configs/config_example.yaml`

同上(`driver: sqlite`,`source` 用模板路径 `./data/seas.db?...`)。

### 5.3 `internal/conf/conf.proto`

**不变**(`Database.driver` / `source` 字段保留,driver 字段值不再被代码使用)。

### 5.4 `.gitignore`

新增:
```
/data/*.db
/data/*.db-wal
/data/*.db-shm
```

`data/` 目录添加 `.gitkeep`。

---

## 6. 文件变更清单

### 新增

- `pkg/gorm/funcs.go` —— STDDEV 自定义聚合
- `pkg/gorm/funcs_test.go` —— STDDEV 单元测试
- `internal/data/migrate.go` —— AutoMigrate 入口
- `data/.gitkeep`
- `docs/superpowers/specs/2026-05-12-mysql-to-sqlite-migration-design.md`(本文档)

### 修改

- `go.mod` / `go.sum` —— 添加 `github.com/glebarez/sqlite`,移除 mysql 驱动直接依赖
- `pkg/gorm/gorm.go` —— 切换驱动、调整连接池、注册函数
- `internal/data/data.go` —— `NewData` 调用 `migrate.AutoMigrate`
- `internal/data/exam.go` —— DELETE-JOIN 改写
- `internal/data/score_item.go` —— GREATEST(2 处)+ CEIL(1 处)改写
- `internal/biz/auth.go` —— User 补全 GORM 标签
- `internal/biz/exam.go` —— ExamSubject 索引/外键标签
- `internal/biz/score.go` —— Score 复合索引/外键
- `internal/biz/score_item.go` —— knowledge_point 索引
- `internal/biz/student.go` —— Student 外键
- `internal/biz/subject.go` —— Subject.Code 唯一索引
- `configs/config.yaml` / `configs/config_example.yaml` —— driver/source
- `.gitignore` —— 忽略 `data/*.db*`

### 删除

- `init.sql`
- `seas.db`(根目录 0 字节占位文件)

---

## 7. 测试与验证

### 7.1 单元测试

`pkg/gorm/funcs_test.go`:

| 场景 | STDDEV_POP 期望 | STDDEV_SAMP 期望 |
|------|----------------|------------------|
| `[2,4,4,4,5,5,7,9]` | ≈ 2.0 | ≈ 2.138089 |
| 空集合 | NULL | NULL |
| 单值 `[5]` | 0 | NULL |
| 全相同 `[3,3,3,3]` | 0 | 0 |

测试通过 `database/sql` 打开内存库 `:memory:`,执行 SQL,断言结果。

### 7.2 集成 smoke

1. `make build` 通过
2. 启动应用,日志出现 `AutoMigrate completed`
3. `sqlite3 ./data/seas.db ".schema"` 确认 8 张表存在
4. 对接 HTTP 客户端跑以下接口,确认 200 + 数据合理:
   - 班级汇总单科 (`STDDEV_POP`)
   - 班级汇总全科(CTE + `STDDEV_POP`)
   - 小题汇总(`STDDEV_SAMP` + `IFNULL` + GREATEST 改写)
   - 区分度分析(CEIL 改写 + CTE + 窗口函数)
   - 名次段查询(`RANK() OVER` + 动态 SQL)
   - 删除考试(DELETE-JOIN 改写 + 级联外键)

### 7.3 性能基线

不做正式 benchmark。启动后 `ps -o rss= -p <pid>` 比较 RSS 与改造前的应用 + MySQL 之和,
预期应用单进程 RSS ≈ 100~150 MB(此前应用 ~80 MB + MySQL ~250 MB 约 330 MB)。

---

## 8. 风险与回滚

### 8.1 已识别风险

| 风险 | 严重性 | 缓解 |
|------|--------|------|
| SQLite 单写连接遇到长事务阻塞读 | 中 | WAL 模式下读不阻塞;长事务需在业务层避免(目前看 raw SQL 都是只读统计或单语句 DML) |
| 自定义 STDDEV 函数浮点误差 | 低 | 用 (variance < 0 ? 0) 处理负方差;教育数据精度 ±0.01 完全可接受 |
| AutoMigrate 添加复合索引时表已有重复数据 | 低 | 新部署从空库开始,本次迁移确认无现有数据要保留 |
| SQLite 文件被并发误删/锁定 | 低 | 单进程持有;无 daemon |

### 8.2 回滚路径

迁移前在 git 上打 tag `pre-sqlite-migration`。如发现重大问题:

1. `git checkout pre-sqlite-migration -- pkg/gorm/ internal/data/ internal/biz/ configs/ init.sql go.mod go.sum`
2. `go mod tidy`
3. 恢复 MySQL 实例,重建表

由于本次确认**无需迁移现有数据**,回滚不涉及数据恢复。

---

## 9. 不在本期范围

- 读写分离连接池(`gorm.io/plugin/dbresolver`)
- 数据库备份脚本(后续可加 cron 复制 `seas.db` 到备份目录)
- SQLite 在线 schema 演进策略(目前依赖 AutoMigrate,后续如表结构频繁变更可引入 goose/atlas)
- 性能压测与 benchmark

---

## 10. 实施步骤大纲(留给 writing-plans 详化)

1. 补全 model GORM 标签(internal/biz/*.go)
2. 新增 `pkg/gorm/funcs.go` 及单测
3. 修改 `pkg/gorm/gorm.go` 切驱动 + 注册函数 + 池参数
4. 新增 `internal/data/migrate.go`,在 `NewData` 中调用
5. 改写 4 处 raw SQL(exam.go 1 处,score_item.go 3 处)
6. 配置文件改 driver/source
7. 更新 `.gitignore`,新增 `data/.gitkeep`
8. 删除 `init.sql` 和根目录 `seas.db`
9. `go mod tidy`,本地 `make build` 通过
10. 手工 smoke(§ 7.2)

完成上述后,代码处于"可工作"状态。
