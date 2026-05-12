# MySQL → SQLite 迁移实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 SEAS 的持久化层从 MySQL 完全切换到 SQLite,业务层零改动,8 处 STDDEV raw SQL 通过自定义聚合函数保留原 SQL 不变。

**Architecture:** GORM + 纯 Go SQLite 驱动(`glebarez/sqlite`)+ 在 `database/sql` 层通过 `modernc.org/sqlite` 注册自定义聚合函数 `STDDEV_POP`/`STDDEV_SAMP`;DDL 改用 `AutoMigrate` 启动时同步;DSN 启用 WAL/外键/page cache 等 pragma。

**Tech Stack:** Go 1.25 / GORM 1.25.12 / `github.com/glebarez/sqlite` v1.11.0 / `modernc.org/sqlite`(间接)/ Kratos v2.8

**Spec:** `docs/superpowers/specs/2026-05-12-mysql-to-sqlite-migration-design.md`

---

## Task 1: 创建回滚锚点(git tag)

**Files:** 无文件改动,仅 git 状态。

- [ ] **Step 1: 确认当前在 main 分支且工作区干净**

Run: `git status --short && git rev-parse HEAD`
Expected: 输出空(无未提交改动),HEAD 为最近 commit(`f863cb8` 或更新)。

- [ ] **Step 2: 打 pre-sqlite-migration tag**

Run:
```bash
git tag -a pre-sqlite-migration -m "迁移到 SQLite 前的稳定点"
git tag -l pre-sqlite-migration
```
Expected: 输出 `pre-sqlite-migration`

---

## Task 2: 添加 SQLite 驱动依赖

**Files:**
- Modify: `go.mod`、`go.sum`

- [ ] **Step 1: 添加 `github.com/glebarez/sqlite`**

Run:
```bash
go get github.com/glebarez/sqlite@v1.11.0
```
Expected: `go.mod` 中新增一行 `require github.com/glebarez/sqlite v1.11.0`,`go.sum` 自动补全。

- [ ] **Step 2: 显式 require `modernc.org/sqlite`**

`modernc.org/sqlite` 会被 glebarez 间接引入,但我们要在代码里直接 import 它(用于注册聚合函数),所以提升为直接依赖。

Run:
```bash
go get modernc.org/sqlite@latest
```
Expected: `go.mod` 中新增一行 `require modernc.org/sqlite vX.Y.Z`

- [ ] **Step 3: 暂不删除 MySQL 驱动**

`gorm.io/driver/mysql` 还在使用中,等 Task 6 切换完成后再移除。本步骤跳过删除。

- [ ] **Step 4: 验证编译仍通过**

Run: `go build ./...`
Expected: 无错误输出。

- [ ] **Step 5: 提交**

```bash
git add go.mod go.sum
git commit -m "$(cat <<'EOF'
chore(deps): 添加 glebarez/sqlite 和 modernc.org/sqlite 依赖

为 MySQL → SQLite 迁移准备驱动依赖。本提交仅引入依赖,
不修改任何业务代码,确保编译仍通过。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```
Expected: 一个新 commit。

---

## Task 3: 实现 STDDEV 聚合函数(TDD)

**Files:**
- Create: `pkg/gorm/funcs.go`
- Create: `pkg/gorm/funcs_test.go`

- [ ] **Step 1: 先写失败的测试**

写入 `/Users/kk/go/src/SEAS/pkg/gorm/funcs_test.go`:

```go
package gorm

import (
	"database/sql"
	"math"
	"testing"

	_ "github.com/glebarez/sqlite" // 注册 database/sql 驱动 "sqlite"
)

const epsilon = 1e-9

// openMem 打开一个内存 SQLite 库用于测试。
func openMem(t *testing.T) *sql.DB {
	t.Helper()
	if err := RegisterAggregates(); err != nil {
		t.Fatalf("RegisterAggregates: %v", err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE t (x REAL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

// insertValues 向 t 表插入若干浮点值。
func insertValues(t *testing.T, db *sql.DB, vs []float64) {
	t.Helper()
	for _, v := range vs {
		if _, err := db.Exec(`INSERT INTO t(x) VALUES (?)`, v); err != nil {
			t.Fatalf("insert %v: %v", v, err)
		}
	}
}

// queryStddev 执行聚合查询并返回结果指针(NULL 时为 nil)。
func queryStddev(t *testing.T, db *sql.DB, fn string) *float64 {
	t.Helper()
	var v sql.NullFloat64
	if err := db.QueryRow(`SELECT ` + fn + `(x) FROM t`).Scan(&v); err != nil {
		t.Fatalf("query %s: %v", fn, err)
	}
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func TestStddevPop_KnownSample(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{2, 4, 4, 4, 5, 5, 7, 9})
	got := queryStddev(t, db, "STDDEV_POP")
	if got == nil {
		t.Fatalf("STDDEV_POP got NULL, want ~2.0")
	}
	if math.Abs(*got-2.0) > 1e-6 {
		t.Fatalf("STDDEV_POP = %v, want ~2.0", *got)
	}
}

func TestStddevSamp_KnownSample(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{2, 4, 4, 4, 5, 5, 7, 9})
	got := queryStddev(t, db, "STDDEV_SAMP")
	if got == nil {
		t.Fatalf("STDDEV_SAMP got NULL, want ~2.138089")
	}
	if math.Abs(*got-2.138089935) > 1e-6 {
		t.Fatalf("STDDEV_SAMP = %v, want ~2.138089935", *got)
	}
}

func TestStddev_EmptySet(t *testing.T) {
	db := openMem(t)
	if got := queryStddev(t, db, "STDDEV_POP"); got != nil {
		t.Fatalf("STDDEV_POP on empty: got %v, want NULL", *got)
	}
	if got := queryStddev(t, db, "STDDEV_SAMP"); got != nil {
		t.Fatalf("STDDEV_SAMP on empty: got %v, want NULL", *got)
	}
}

func TestStddev_SingleValue(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{5})
	pop := queryStddev(t, db, "STDDEV_POP")
	if pop == nil || math.Abs(*pop) > epsilon {
		t.Fatalf("STDDEV_POP single: got %v, want 0", pop)
	}
	if samp := queryStddev(t, db, "STDDEV_SAMP"); samp != nil {
		t.Fatalf("STDDEV_SAMP single: got %v, want NULL", *samp)
	}
}

func TestStddev_AllSame(t *testing.T) {
	db := openMem(t)
	insertValues(t, db, []float64{3, 3, 3, 3})
	if pop := queryStddev(t, db, "STDDEV_POP"); pop == nil || math.Abs(*pop) > 1e-6 {
		t.Fatalf("STDDEV_POP all-same: got %v, want 0", pop)
	}
	if samp := queryStddev(t, db, "STDDEV_SAMP"); samp == nil || math.Abs(*samp) > 1e-6 {
		t.Fatalf("STDDEV_SAMP all-same: got %v, want 0", samp)
	}
}
```

- [ ] **Step 2: 运行测试,确认失败(因为还没实现)**

Run: `go test ./pkg/gorm/ -run TestStddev -v`
Expected: 编译失败,提示 `undefined: RegisterAggregates`。

- [ ] **Step 3: 实现 STDDEV 聚合函数**

写入 `/Users/kk/go/src/SEAS/pkg/gorm/funcs.go`:

```go
package gorm

import (
	"database/sql/driver"
	"fmt"
	"math"
	"sync"

	sqlite "modernc.org/sqlite"
)

// stddevAgg 同时实现 STDDEV_POP(总体)与 STDDEV_SAMP(样本)。
// 用单遍累加 sum 与 sum-of-squares,适合本项目的成绩统计精度需求。
type stddevAgg struct {
	n     int64
	sum   float64
	sumSq float64
	pop   bool // true=STDDEV_POP, false=STDDEV_SAMP
}

func coerceFloat(v driver.Value) (float64, bool, error) {
	if v == nil {
		return 0, false, nil // SQL NULL,不计入
	}
	switch x := v.(type) {
	case float64:
		return x, true, nil
	case int64:
		return float64(x), true, nil
	default:
		return 0, false, fmt.Errorf("stddev: unsupported arg type %T", x)
	}
}

func (s *stddevAgg) Step(_ *sqlite.FunctionContext, args []driver.Value) error {
	if len(args) == 0 {
		return nil
	}
	v, ok, err := coerceFloat(args[0])
	if err != nil || !ok {
		return err
	}
	s.n++
	s.sum += v
	s.sumSq += v * v
	return nil
}

// WindowInverse 仅在作为窗口函数被反向滑动时调用;本项目目前没有这种用法,
// 但接口要求实现。保持与 Step 对称即可。
func (s *stddevAgg) WindowInverse(_ *sqlite.FunctionContext, args []driver.Value) error {
	if len(args) == 0 {
		return nil
	}
	v, ok, err := coerceFloat(args[0])
	if err != nil || !ok {
		return err
	}
	s.n--
	s.sum -= v
	s.sumSq -= v * v
	return nil
}

func (s *stddevAgg) WindowValue(_ *sqlite.FunctionContext) (driver.Value, error) {
	if s.n == 0 {
		return nil, nil // 空集合 → NULL
	}
	if !s.pop && s.n == 1 {
		return nil, nil // 样本标准差单值无定义 → NULL
	}
	mean := s.sum / float64(s.n)
	variance := s.sumSq/float64(s.n) - mean*mean
	if variance < 0 {
		variance = 0 // 浮点误差保护
	}
	if s.pop {
		return math.Sqrt(variance), nil
	}
	// 样本方差 = 总体方差 × n / (n-1)
	return math.Sqrt(variance * float64(s.n) / float64(s.n-1)), nil
}

func (s *stddevAgg) Final(_ *sqlite.FunctionContext) {}

var (
	registerOnce sync.Once
	registerErr  error
)

// RegisterAggregates 把 STDDEV_POP / STDDEV_SAMP 注册到全局 sqlite 驱动,
// 必须在打开任何 SQLite 连接之前调用。多次调用是幂等的(用 sync.Once 保护)。
func RegisterAggregates() error {
	registerOnce.Do(func() {
		if err := sqlite.RegisterFunction("STDDEV_POP", &sqlite.FunctionImpl{
			NArgs:         1,
			Deterministic: true,
			MakeAggregate: func(_ sqlite.FunctionContext) (sqlite.AggregateFunction, error) {
				return &stddevAgg{pop: true}, nil
			},
		}); err != nil {
			registerErr = fmt.Errorf("register STDDEV_POP: %w", err)
			return
		}
		if err := sqlite.RegisterFunction("STDDEV_SAMP", &sqlite.FunctionImpl{
			NArgs:         1,
			Deterministic: true,
			MakeAggregate: func(_ sqlite.FunctionContext) (sqlite.AggregateFunction, error) {
				return &stddevAgg{pop: false}, nil
			},
		}); err != nil {
			registerErr = fmt.Errorf("register STDDEV_SAMP: %w", err)
			return
		}
	})
	return registerErr
}
```

- [ ] **Step 4: 运行测试,确认通过**

Run: `go test ./pkg/gorm/ -run TestStddev -v`
Expected: 5 个测试全部 PASS,无 `FAIL` 字样。

- [ ] **Step 5: 运行所有包测试,确保没有破坏其他代码**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 6: 提交**

```bash
git add pkg/gorm/funcs.go pkg/gorm/funcs_test.go go.mod go.sum
git commit -m "$(cat <<'EOF'
feat(gorm): 新增 STDDEV_POP / STDDEV_SAMP 自定义聚合函数

通过 modernc.org/sqlite 的 RegisterFunction 在 database/sql 层注册
两个 deterministic 聚合函数,让 data 层 8 处 raw SQL 中的标准差计算
在 SQLite 上零改动可用。单遍累加 sum + sumSq 算法,精度对教育成绩
场景充分。覆盖空集合、单值、样本边界等 5 个测试用例。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 补全 GORM 模型标签

**Files:**
- Modify: `internal/biz/auth.go`
- Modify: `internal/biz/subject.go`
- Modify: `internal/biz/student.go`
- Modify: `internal/biz/exam.go`
- Modify: `internal/biz/score.go`
- Modify: `internal/biz/score_item.go`

补全唯一索引、复合唯一索引、普通索引、外键约束,让 AutoMigrate 生成的表结构与原 init.sql 等价。

- [ ] **Step 1: 修改 `User` 模型(internal/biz/auth.go)补全 GORM 标签**

把 `internal/biz/auth.go` 的 `User struct` 替换为(替换整个 struct 定义,保留文件其他部分不变):

```go
// User 用户模型
type User struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement;column:id"`
	OpenID    string    `gorm:"uniqueIndex;type:varchar(64);not null;column:openid"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime;column:updated_at"`
}

// TableName 显式指定表名,与 init.sql 中的 users 表对齐
func (User) TableName() string { return "users" }
```

- [ ] **Step 2: 修改 `Subject` 模型(internal/biz/subject.go)给 Code 加唯一索引**

把 struct 替换为:

```go
type Subject struct {
	ID   int64  `gorm:"primaryKey;column:id"`
	Name string `gorm:"type:varchar(100);column:name"`
	Code string `gorm:"uniqueIndex;type:varchar(50);column:code"` // 学科编码,唯一
}
```

- [ ] **Step 3: 修改 `Student` 模型(internal/biz/student.go)添加外键约束**

把 `Student struct` 替换为(`Class` 不动):

```go
type Student struct {
	ID            int64     `gorm:"primaryKey;column:id"`
	StudentNumber string    `gorm:"uniqueIndex;type:varchar(64);column:student_number"`
	Name          string    `gorm:"type:varchar(100);column:name"`
	ClassID       int64     `gorm:"index;not null;column:class_id"`
	Class         Class     `gorm:"foreignKey:ClassID;references:ID;constraint:OnDelete:RESTRICT"`
	CreatedAt     time.Time `gorm:"autoCreateTime;column:created_at"`
}
```

- [ ] **Step 4: 修改 `ExamSubject` 模型(internal/biz/exam.go)添加复合唯一索引 + 外键**

把 `ExamSubject struct` 替换为(`Exam` 不动):

```go
// ExamSubject 考试-学科关联表
type ExamSubject struct {
	ID        int64       `gorm:"primaryKey;column:id"`
	ExamID    int64       `gorm:"uniqueIndex:idx_exam_subject;not null;column:exam_id"`
	SubjectID int64       `gorm:"uniqueIndex:idx_exam_subject;not null;column:subject_id"`
	FullScore float64     `gorm:"column:full_score;default:100"`
	CreatedAt time.Time   `gorm:"autoCreateTime;column:created_at"`
	Exam      Exam        `gorm:"foreignKey:ExamID;references:ID;constraint:OnDelete:CASCADE"`
	Subject   Subject     `gorm:"foreignKey:SubjectID;references:ID;constraint:OnDelete:CASCADE"`
}
```

- [ ] **Step 5: 修改 `Score` 模型(internal/biz/score.go)添加复合唯一索引 + 外键**

把 `Score struct` 替换为(其他 stats struct 不动):

```go
type Score struct {
	ID         int64     `gorm:"primaryKey;column:id"`
	StudentID  int64     `gorm:"uniqueIndex:idx_student_exam_subject;index;not null;column:student_id"`
	ExamID     int64     `gorm:"uniqueIndex:idx_student_exam_subject;index;not null;column:exam_id"`
	SubjectID  int64     `gorm:"uniqueIndex:idx_student_exam_subject;index;not null;column:subject_id"`
	TotalScore float64   `gorm:"not null;column:total_score"`
	CreatedAt  time.Time `gorm:"autoCreateTime;column:created_at"`
	Student    Student   `gorm:"foreignKey:StudentID;references:ID;constraint:OnDelete:CASCADE"`
	Exam       Exam      `gorm:"foreignKey:ExamID;references:ID;constraint:OnDelete:CASCADE"`
	Subject    Subject   `gorm:"foreignKey:SubjectID;references:ID;constraint:OnDelete:CASCADE"`
}
```

- [ ] **Step 6: 修改 `ScoreItem` 模型(internal/biz/score_item.go)补全索引 + 外键**

把 struct 替换为:

```go
type ScoreItem struct {
	ID             int64   `gorm:"primaryKey;column:id"`
	ScoreID        int64   `gorm:"index;not null;column:score_id"`                          // 外键关联 score 表
	QuestionNumber string  `gorm:"type:varchar(20);not null;column:question_number"`        // 小题编号
	KnowledgePoint string  `gorm:"index;type:varchar(100);column:knowledge_point"`          // 知识点
	Score          float64 `gorm:"not null;default:0;column:score"`                         // 得分
	FullScore      float64 `gorm:"not null;default:0;column:full_score"`                    // 总分
	IsCorrect      bool    `gorm:"not null;default:false;column:is_correct"`                // 是否正确
	ParentScore    Score   `gorm:"foreignKey:ScoreID;references:ID;constraint:OnDelete:CASCADE"`
}
```

注意:关联字段命名为 `ParentScore` 而不是 `Score`,以避免与已有的 `Score float64` 字段冲突。GORM 通过 `foreignKey:` 标签识别关联,字段名只用于 Go 端引用;由于 data 层从不主动 Preload 该字段,不会产生 N+1 查询。

- [ ] **Step 7: 修改 `Exam` 模型(internal/biz/exam.go)添加 exam_date 索引**

仅修改 `Exam.ExamDate` 字段,加上 `index`:

```go
type Exam struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"type:varchar(100);not null;column:name"`
	ExamDate  time.Time `gorm:"index;not null;column:exam_date"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
}
```

- [ ] **Step 8: 编译确认无语法错**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 9: 提交**

```bash
git add internal/biz/
git commit -m "$(cat <<'EOF'
refactor(biz): 补全 GORM model 标签 (索引、外键、复合唯一)

把 init.sql 中已有但 model 漏标的约束补到 GORM 标签上,为
AutoMigrate 取代手写 init.sql 做准备:

- User: 完整加 primaryKey/autoIncrement/uniqueIndex/autoUpdateTime
- Subject: Code 加 uniqueIndex
- Student: ClassID 加 index + 外键 RESTRICT
- ExamSubject: (exam_id, subject_id) 复合唯一索引 + CASCADE 外键
- Score: (student_id, exam_id, subject_id) 复合唯一索引 + 单列 index
  + CASCADE 外键
- ScoreItem: knowledge_point 加 index + CASCADE 外键
- Exam: exam_date 加 index

业务代码使用方式不变(外键关联字段不主动 Preload 不会触发 N+1)。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 实现 AutoMigrate

**Files:**
- Create: `internal/data/migrate.go`

- [ ] **Step 1: 写 migrate.go**

写入 `/Users/kk/go/src/SEAS/internal/data/migrate.go`:

```go
package data

import (
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// AutoMigrate 把所有业务 model 同步到数据库 schema。
// 启动时调用一次,幂等(GORM 仅追加缺失的表/列/索引,不删除)。
func AutoMigrate(db *gorm.DB, logger log.Logger) error {
	helper := log.NewHelper(logger)
	helper.Info("AutoMigrate: 开始同步 schema")
	if err := db.AutoMigrate(
		&biz.Class{},
		&biz.Student{},
		&biz.Subject{},
		&biz.Exam{},
		&biz.ExamSubject{},
		&biz.Score{},
		&biz.ScoreItem{},
		&biz.User{},
	); err != nil {
		helper.Errorf("AutoMigrate failed: %+v", err)
		return err
	}
	helper.Info("AutoMigrate: schema 同步完成")
	return nil
}
```

- [ ] **Step 2: 在 NewData 中调用 AutoMigrate**

修改 `/Users/kk/go/src/SEAS/internal/data/data.go` 的 `NewData` 函数。原代码:

```go
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	db, closeSQLF, err := gormsql.Init(logger, c.Database.Source)
	if err != nil {
		return nil, nil, err
	}

	rds, closeRdsF, err := redis.Init(c)
```

替换为:

```go
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	db, closeSQLF, err := gormsql.Init(logger, c.Database.Source)
	if err != nil {
		return nil, nil, err
	}

	if err := AutoMigrate(db, logger); err != nil {
		closeSQLF()
		return nil, nil, err
	}

	rds, closeRdsF, err := redis.Init(c)
```

工具调用示例:
```
Edit file_path=internal/data/data.go
  old_string="	db, closeSQLF, err := gormsql.Init(logger, c.Database.Source)\n	if err != nil {\n		return nil, nil, err\n	}\n\n	rds, closeRdsF, err := redis.Init(c)"
  new_string="	db, closeSQLF, err := gormsql.Init(logger, c.Database.Source)\n	if err != nil {\n		return nil, nil, err\n	}\n\n	if err := AutoMigrate(db, logger); err != nil {\n		closeSQLF()\n		return nil, nil, err\n	}\n\n	rds, closeRdsF, err := redis.Init(c)"
```

- [ ] **Step 3: 编译确认**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 4: 提交**

```bash
git add internal/data/migrate.go internal/data/data.go
git commit -m "$(cat <<'EOF'
feat(data): 启动时 AutoMigrate 同步 schema

新增 internal/data/migrate.go 暴露 AutoMigrate(db, logger),
在 NewData 中 gorm.Open 成功后调用一次,把 8 个 GORM model
同步成表/索引/外键。失败时关闭 db 连接并向上传播错误。

为后续替换 init.sql 做准备。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 切换 GORM 驱动到 SQLite

**Files:**
- Modify: `pkg/gorm/gorm.go`

- [ ] **Step 1: 重写 `pkg/gorm/gorm.go`**

把整个文件内容替换为:

```go
package gorm

import (
	"context"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

// Init 初始化 SQLite 连接。在 gorm.Open 前先注册自定义聚合函数
// (STDDEV_POP / STDDEV_SAMP),让 data 层已有 raw SQL 在 SQLite 上可用。
func Init(logger log.Logger, source string) (*gorm.DB, func(), error) {
	if err := RegisterAggregates(); err != nil {
		return nil, nil, err
	}

	gormLogger := NewGormLogger(logger, gormlog.Info)
	db, err := gorm.Open(sqlite.Open(source), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, nil, err
	}

	// SQLite 是单写者,连接池保持小;读不会被写阻塞(WAL 模式由 DSN PRAGMA 启用)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	closeF := func() {
		_ = sqlDB.Close()
	}
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(0) // 本地文件,无连接老化

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		closeF()
		return nil, nil, err
	}
	return db, closeF, nil
}

// 以下 GormLogger 实现不变,只是从原文件保留下来 ------------------------------

type GormLogger struct {
	logger *log.Helper
	level  gormlog.LogLevel
}

func NewGormLogger(logger log.Logger, level gormlog.LogLevel) gormlog.Interface {
	return &GormLogger{
		logger: log.NewHelper(logger),
		level:  level,
	}
}

func (l *GormLogger) LogMode(level gormlog.LogLevel) gormlog.Interface {
	return &GormLogger{
		logger: l.logger,
		level:  level,
	}
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlog.Info {
		l.logger.WithContext(ctx).Infow(append([]interface{}{"msg", msg}, data...)...)
	}
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlog.Warn {
		l.logger.WithContext(ctx).Warnw(append([]interface{}{"msg", msg}, data...)...)
	}
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlog.Error {
		l.logger.WithContext(ctx).Errorw(append([]interface{}{"msg", msg}, data...)...)
	}
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= 0 {
		return
	}
	elapsed := time.Since(begin)
	sqlStr, rows := fc()
	fields := []interface{}{
		"duration", elapsed,
		"rows", rows,
		"sql", sqlStr,
	}

	switch {
	case err != nil && l.level >= gormlog.Error:
		l.logger.WithContext(ctx).Errorw(append([]interface{}{"msg", "GORM Trace"}, append(fields, "error", err)...)...)
	case elapsed > 200*time.Millisecond && l.level >= gormlog.Warn:
		l.logger.WithContext(ctx).Warnw(append([]interface{}{"msg", "GORM Slow SQL"}, fields...)...)
	case l.level >= gormlog.Info:
		l.logger.WithContext(ctx).Infow(append([]interface{}{"msg", "GORM Info"}, fields...)...)
	}
}
```

- [ ] **Step 2: 从 go.mod 移除 MySQL 驱动**

Run:
```bash
go mod edit -droprequire gorm.io/driver/mysql
go mod tidy
```
Expected: `go.mod` 中不再有 `gorm.io/driver/mysql` 行,`go.sum` 同步清理。如果 `go mod tidy` 报错说还有引用,跳到 Step 3 检查。

- [ ] **Step 3: 确认没有残留 import**

Run: `grep -rn "gorm.io/driver/mysql\|go-sql-driver/mysql" /Users/kk/go/src/SEAS --include="*.go"`
Expected: 输出为空。

- [ ] **Step 4: 编译确认**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 5: 跑 pkg/gorm 测试,确保没破坏 STDDEV 测试**

Run: `go test ./pkg/gorm/ -v`
Expected: 所有 STDDEV 测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add pkg/gorm/gorm.go go.mod go.sum
git commit -m "$(cat <<'EOF'
refactor(gorm): 切换驱动到 SQLite (glebarez/sqlite)

- gorm.Open(mysql.Open) → gorm.Open(sqlite.Open)
- gorm.Open 前调用 RegisterAggregates() 注册 STDDEV
- 连接池:MaxOpenConns 100→1, MaxIdleConns 10→1,
  ConnMaxLifetime 1h→0(SQLite 单写者,WAL 下读不阻塞)
- 移除 gorm.io/driver/mysql 和 go-sql-driver/mysql 依赖

DSN 的 pragma 配置(WAL / synchronous / foreign_keys / cache_size)
由配置文件提供(下一步 Task 10)。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 改写 DELETE...JOIN(internal/data/exam.go)

**Files:**
- Modify: `internal/data/exam.go:78-82`

- [ ] **Step 1: 改写 delete score_items 子句**

把 `internal/data/exam.go` 第 76-86 行 `func (r *examRepo) Delete` 内的第一段:

```go
		// 1. 删除 score_items（通过 scores 关联）
		if err := tx.Exec(`
			DELETE si FROM score_items si
			JOIN scores sc ON sc.id = si.score_id
			WHERE sc.exam_id = ?
		`, id).Error; err != nil {
			log.Context(ctx).Errorf("examRepo.Delete score_items err: %+v", err)
			return err
		}
```

替换为:

```go
		// 1. 删除 score_items（通过 scores 关联）
		// SQLite 不支持 DELETE...JOIN,改用子查询
		if err := tx.Exec(`
			DELETE FROM score_items
			WHERE score_id IN (SELECT id FROM scores WHERE exam_id = ?)
		`, id).Error; err != nil {
			log.Context(ctx).Errorf("examRepo.Delete score_items err: %+v", err)
			return err
		}
```

- [ ] **Step 2: 编译确认**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 3: 提交**

```bash
git add internal/data/exam.go
git commit -m "$(cat <<'EOF'
fix(data): 改写 DELETE...JOIN 为子查询,适配 SQLite

MySQL 的多表 DELETE 语法 SQLite 不支持。改写为
DELETE FROM ... WHERE score_id IN (SELECT ...),语义等价。
scores.id 主键索引存在,子查询性能不受影响。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 改写 GREATEST(internal/data/score_item.go)

**Files:**
- Modify: `internal/data/score_item.go`(2 处,完全相同的子串)

两处字符串完全一致(`internal/data/score_item.go:41` 与 `:108`),可以用 Edit 工具的 `replace_all=true` 一次替换。

- [ ] **Step 1: 把 GREATEST 替换为 CASE WHEN(两处一起)**

把 `internal/data/score_item.go` 中所有出现的字符串:

```
GREATEST(MAX(si.full_score), MAX(si.score)) AS full_score
```

替换为:

```
CASE WHEN MAX(si.full_score) > MAX(si.score) THEN MAX(si.full_score) ELSE MAX(si.score) END AS full_score
```

工具调用示例:
```
Edit file_path=internal/data/score_item.go
  old_string="GREATEST(MAX(si.full_score), MAX(si.score)) AS full_score"
  new_string="CASE WHEN MAX(si.full_score) > MAX(si.score) THEN MAX(si.full_score) ELSE MAX(si.score) END AS full_score"
  replace_all=true
```

- [ ] **Step 2: 验证没有遗漏**

Run: `grep -n "GREATEST" /Users/kk/go/src/SEAS/internal/data/score_item.go`
Expected: 输出为空。

- [ ] **Step 3: 编译确认**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 4: 提交**

```bash
git add internal/data/score_item.go
git commit -m "$(cat <<'EOF'
fix(data): GREATEST → CASE WHEN,适配 SQLite

SQLite 没有 GREATEST 函数。score_item.go 2 处 GREATEST(MAX(a),MAX(b))
改写为 CASE WHEN MAX(a) > MAX(b) THEN MAX(a) ELSE MAX(b) END,
语义等价。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: 改写 CEIL(internal/data/score_item.go)

**Files:**
- Modify: `internal/data/score_item.go:179`

- [ ] **Step 1: 把 CEIL 替换为 CAST**

把 `internal/data/score_item.go` 中:

```
SELECT CEIL(COUNT(*) * 0.27) as k FROM student_total
```

替换为:

```
SELECT CAST((COUNT(*) * 0.27 + 0.999999) AS INTEGER) as k FROM student_total
```

工具调用示例:
```
Edit file_path=internal/data/score_item.go
  old_string="SELECT CEIL(COUNT(*) * 0.27) as k FROM student_total"
  new_string="SELECT CAST((COUNT(*) * 0.27 + 0.999999) AS INTEGER) as k FROM student_total"
```

- [ ] **Step 2: 验证没有遗漏 CEIL/FLOOR**

Run: `grep -n "CEIL\|FLOOR" /Users/kk/go/src/SEAS/internal/data/score_item.go`
Expected: 输出为空。

- [ ] **Step 3: 编译确认**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 4: 提交**

```bash
git add internal/data/score_item.go
git commit -m "$(cat <<'EOF'
fix(data): CEIL → CAST 整数转换,适配 SQLite

SQLite 没有 CEIL/CEILING 函数。区分度计算中的 CEIL(N * 0.27)
改写为 CAST((N * 0.27 + 0.999999) AS INTEGER),
0.999999 容差避免整数倍数 (如 N=100) 时多算 1。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: 更新配置文件

**Files:**
- Modify: `configs/config.yaml`
- Modify: `configs/config_example.yaml`

- [ ] **Step 1: 修改 configs/config.yaml 的 data.database 段**

把 `configs/config.yaml` 中的:

```yaml
data:
  database:
    driver: mysql
    source: root:2009829Gao@tcp(127.0.0.1:3306)/seas?parseTime=True&loc=Local
```

替换为:

```yaml
data:
  database:
    driver: sqlite
    source: ./data/seas.db?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=cache_size(-20000)
```

- [ ] **Step 2: 同步修改 configs/config_example.yaml**

把 `configs/config_example.yaml` 中的同一段做同样替换(用 `./data/seas.db?...` 同一字符串)。

- [ ] **Step 3: 验证 yaml 语法**

Run:
```bash
python3 -c "import yaml,sys; yaml.safe_load(open('/Users/kk/go/src/SEAS/configs/config.yaml')); yaml.safe_load(open('/Users/kk/go/src/SEAS/configs/config_example.yaml')); print('ok')"
```
Expected: `ok`

- [ ] **Step 4: 提交**

```bash
git add configs/config_example.yaml
git commit -m "$(cat <<'EOF'
config: 切换 database.driver 为 sqlite,配置 WAL/外键 pragma

source 改为本地文件路径 ./data/seas.db,并通过 DSN URL 参数
启用关键 PRAGMA:
- journal_mode=WAL: 读写不互相阻塞
- synchronous=NORMAL: WAL 下的安全/性能平衡
- busy_timeout=5000: 避免 database is locked 抛错
- foreign_keys=1: 启用外键约束 (SQLite 默认关)
- cache_size=-20000: 每连接 20MB page cache

注:configs/config.yaml 是本地运行时文件,已被 .gitignore 排除,
不进入版本控制;同步改动 example 文件作为参考。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

注:`configs/config.yaml` 被 `.gitignore` 排除(见 `.gitignore` 第 5 行 `/configs/config.yaml`),所以本提交只 `add` example 文件;`config.yaml` 的改动在本地生效但不入库。

---

## Task 11: 清理 init.sql、添加 data 目录、更新 .gitignore

**Files:**
- Delete: `init.sql`
- Delete: `seas.db`(根目录 0 字节占位)
- Create: `data/.gitkeep`
- Modify: `.gitignore`

- [ ] **Step 1: 删除 init.sql 和根目录的占位 seas.db**

Run:
```bash
rm /Users/kk/go/src/SEAS/init.sql
rm /Users/kk/go/src/SEAS/seas.db
```
Expected: 两个文件均不存在。

- [ ] **Step 2: 创建 data 目录与 .gitkeep**

Run:
```bash
mkdir -p /Users/kk/go/src/SEAS/data
touch /Users/kk/go/src/SEAS/data/.gitkeep
```

- [ ] **Step 3: 更新 .gitignore**

把 `/Users/kk/go/src/SEAS/.gitignore` 的内容(从 Read 时是 17 行)在末尾追加:

```

# SQLite 数据库文件(WAL/SHM 是运行时附属文件)
/data/*.db
/data/*.db-wal
/data/*.db-shm
```

- [ ] **Step 4: 提交**

```bash
git rm init.sql seas.db
git add .gitignore data/.gitkeep
git commit -m "$(cat <<'EOF'
chore: 移除 init.sql,改用 AutoMigrate;新增 data/ 目录

删除 MySQL DDL 脚本 init.sql 和根目录 0 字节占位 seas.db。
新增 data/ 目录(含 .gitkeep)用于存放 SQLite 数据库文件,
并在 .gitignore 中忽略 *.db/*.db-wal/*.db-shm 运行时文件。

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: 编译并启动 smoke

**Files:** 无文件改动,仅运行验证。

- [ ] **Step 1: 清理旧二进制**

Run: `rm -f /Users/kk/go/src/SEAS/bin/seas /Users/kk/go/src/SEAS/seas`
Expected: 无报错。

- [ ] **Step 2: 编译**

Run: `cd /Users/kk/go/src/SEAS && make build`
Expected: 二进制生成在 `./bin/seas`,无 error 输出。如果 `make build` 不可用,直接 `go build -o bin/seas ./cmd/seas`。

- [ ] **Step 3: 启动应用(后台)**

Run(用 run_in_background 或单独终端):
```bash
cd /Users/kk/go/src/SEAS && ./bin/seas -conf ./configs/config.yaml
```
Expected:
- 日志中出现 `AutoMigrate: 开始同步 schema`
- 紧接着 `AutoMigrate: schema 同步完成`
- 应用在 `0.0.0.0:8000`(HTTP)和 `0.0.0.0:9000`(gRPC)监听
- 没有 panic / fatal

- [ ] **Step 4: 验证 SQLite 文件已生成**

Run: `ls -la /Users/kk/go/src/SEAS/data/`
Expected: 至少看到 `seas.db` 和 `seas.db-wal`、`seas.db-shm`(WAL 模式产生)。

- [ ] **Step 5: 验证表结构**

Run: `sqlite3 /Users/kk/go/src/SEAS/data/seas.db ".tables"`
Expected: 输出 8 张表:
```
classes        exam_subjects  scores         students     
exams          score_items    subjects       users
```

- [ ] **Step 6: 检查表的索引**

Run: `sqlite3 /Users/kk/go/src/SEAS/data/seas.db ".indexes scores"`
Expected: 至少看到 `idx_student_exam_subject` 和单列索引(idx_scores_student_id 等;GORM 命名风格)。

Run: `sqlite3 /Users/kk/go/src/SEAS/data/seas.db ".indexes exam_subjects"`
Expected: 看到 `idx_exam_subject`。

- [ ] **Step 7: 检查外键启用**

Run: `sqlite3 /Users/kk/go/src/SEAS/data/seas.db "PRAGMA foreign_keys"`
Expected: 输出 `1`(命令行连接默认是 0,这只验证 db 文件本身没问题;实际运行时连接由 DSN 启用)。

Run: `sqlite3 /Users/kk/go/src/SEAS/data/seas.db "PRAGMA journal_mode"`
Expected: 输出 `wal`(WAL 文件存在意味着曾经以 WAL 模式打开)。

- [ ] **Step 8: 停止应用**

Run(kill 后台进程或 Ctrl+C 前台进程):
```bash
pkill -f 'bin/seas' || true
```

- [ ] **Step 9: 这一步不需要 commit**(只是验证)。

---

## Task 13: 端到端业务接口 smoke 验证

**Files:** 无文件改动,仅手工验证。

**前置:** 准备一份测试数据(可以通过 admin 后台手工导入一份小考试,或者用 SQL 直接插入几条 dummy 数据)。下方仅给出 SQL 兜底方案。

- [ ] **Step 1: 用 SQL 注入最小测试数据**

把以下 SQL 通过 `sqlite3` 注入:

```bash
sqlite3 /Users/kk/go/src/SEAS/data/seas.db <<'SQL'
INSERT INTO classes(id, name, grade) VALUES (1, '一班', '初一'), (2, '二班', '初一');
INSERT INTO subjects(id, name, code) VALUES (1, '语文', 'CHI'), (2, '数学', 'MATH');
INSERT INTO exams(id, name, exam_date) VALUES (1, '期中考试', '2026-04-01 09:00:00');
INSERT INTO exam_subjects(id, exam_id, subject_id, full_score) VALUES (1, 1, 1, 100), (2, 1, 2, 100);
INSERT INTO students(id, student_number, name, class_id) VALUES
  (1, 'S001', '张三', 1), (2, 'S002', '李四', 1),
  (3, 'S003', '王五', 2), (4, 'S004', '赵六', 2);
INSERT INTO scores(id, student_id, exam_id, subject_id, total_score) VALUES
  (1, 1, 1, 1, 85), (2, 2, 1, 1, 92),
  (3, 3, 1, 1, 78), (4, 4, 1, 1, 88),
  (5, 1, 1, 2, 95), (6, 2, 1, 2, 88),
  (7, 3, 1, 2, 70), (8, 4, 1, 2, 90);
INSERT INTO score_items(id, score_id, question_number, score, full_score, is_correct) VALUES
  (1, 1, '1', 10, 10, 1), (2, 1, '2', 8, 10, 0),
  (3, 2, '1', 10, 10, 1), (4, 2, '2', 10, 10, 1),
  (5, 3, '1', 7, 10, 0), (6, 3, '2', 8, 10, 0),
  (7, 4, '1', 9, 10, 1), (8, 4, '2', 9, 10, 1);
SELECT 'inserted ok';
SQL
```
Expected: 输出 `inserted ok`,无外键报错。

- [ ] **Step 2: 启动应用**

Run: `cd /Users/kk/go/src/SEAS && ./bin/seas -conf ./configs/config.yaml &`
Expected: 启动成功(同 Task 12 Step 3)。

- [ ] **Step 3: 调用班级单科汇总接口(命中 STDDEV_POP)**

Run:
```bash
curl -s 'http://127.0.0.1:8000/seas/api/v1/exams/1/analysis/class-summary?subject_id=1' | head -c 2000
```
Expected:
- HTTP 200
- 返回 JSON 含 `class_details`、`overall_grade`,`std_dev` 字段为非零浮点数
- 不出现 `STDDEV_POP: no such function` 之类的错

- [ ] **Step 4: 调用小题汇总接口(命中 STDDEV_SAMP + GREATEST 改写)**

Run:
```bash
curl -s 'http://127.0.0.1:8000/seas/api/v1/exams/1/subjects/1/questions' | head -c 2000
```
Expected:
- HTTP 200
- 返回的题目 `std_dev` / `discrimination` 字段为合理浮点数
- 不出现 `GREATEST: no such function`

- [ ] **Step 5: 调用删除考试接口(命中 DELETE-JOIN 改写 + 级联外键)**

Run:
```bash
curl -s -X DELETE 'http://127.0.0.1:8000/seas/api/v1/exams/1'
sqlite3 /Users/kk/go/src/SEAS/data/seas.db "SELECT COUNT(*) FROM score_items; SELECT COUNT(*) FROM scores; SELECT COUNT(*) FROM exam_subjects;"
```
Expected: 三个 count 都是 0(级联删除生效)。

- [ ] **Step 6: 关闭应用**

Run: `pkill -f 'bin/seas' || true`

- [ ] **Step 7: 提交一个 smoke 完成的标记 commit(无文件改动则跳过)**

如果整个 smoke 过程发现需要小修改(typo、漏改的 SQL),把改动提交;否则跳过。

- [ ] **Step 8: 在 git tag 上记录 smoke 通过点**

Run:
```bash
git tag -a sqlite-migration-smoke-ok -m "MySQL → SQLite 迁移 smoke 通过"
```
Expected: 标签创建成功。

---

## Task 14: 性能基线对比(可选)

**Files:** 无文件改动。

- [ ] **Step 1: 测量当前应用 RSS**

启动应用后 5 秒,运行:
```bash
ps -o pid=,rss=,comm= -p $(pgrep -f 'bin/seas')
```
记录 RSS(单位 KB,除 1024 得 MB)。

- [ ] **Step 2: 对照预期**

期望 RSS ≈ 100~150 MB。若显著超过(>300MB),复检 `cache_size` 配置和 GORM 日志级别。

- [ ] **Step 3: 把数字记在 spec 文档末尾**

把 `docs/superpowers/specs/2026-05-12-mysql-to-sqlite-migration-design.md` 末尾追加一节:

```markdown
## 11. 实测基线(2026-05-12)

- 应用 RSS:XX MB(2C2G 机器,启动后空闲态)
- MySQL 移除前对照(应用 + MySQL):约 330 MB
- 节省:约 (330-XX) MB
```

填入实际数字。

- [ ] **Step 4: 提交**

```bash
git add docs/superpowers/specs/2026-05-12-mysql-to-sqlite-migration-design.md
git commit -m "$(cat <<'EOF'
docs: 记录 SQLite 迁移后的内存基线实测

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## 完成后的状态

- ✅ MySQL 驱动已完全移除
- ✅ STDDEV_POP / STDDEV_SAMP 通过自定义聚合函数在 SQLite 上可用
- ✅ 8 张表通过 AutoMigrate 自动建立(含复合唯一索引和外键约束)
- ✅ 4 处 raw SQL (DELETE-JOIN/GREATEST/CEIL) 已改写
- ✅ 配置文件指向 `./data/seas.db`,WAL + 外键等 PRAGMA 启用
- ✅ 业务层、服务层、proto 定义零改动
- ✅ 至少 1 个 STDDEV 单测覆盖,1 次启动 smoke,3 个核心查询接口的端到端 smoke

## 回滚方法

若发现严重问题:

```bash
git reset --hard pre-sqlite-migration
go mod tidy
# 恢复 MySQL 服务,确认 init.sql 已被 reset 还原
```

由于无需迁移历史数据(用户已确认),无数据恢复步骤。

---

## 文件总览(对照 spec § 6)

| 状态 | 路径 | 任务 |
|------|------|------|
| ➕ 新增 | `pkg/gorm/funcs.go` | Task 3 |
| ➕ 新增 | `pkg/gorm/funcs_test.go` | Task 3 |
| ➕ 新增 | `internal/data/migrate.go` | Task 5 |
| ➕ 新增 | `data/.gitkeep` | Task 11 |
| ✏️ 修改 | `go.mod` / `go.sum` | Task 2 + Task 6 |
| ✏️ 修改 | `pkg/gorm/gorm.go` | Task 6 |
| ✏️ 修改 | `internal/data/data.go` | Task 5 |
| ✏️ 修改 | `internal/data/exam.go` | Task 7 |
| ✏️ 修改 | `internal/data/score_item.go` | Task 8 + 9 |
| ✏️ 修改 | `internal/biz/auth.go` | Task 4 |
| ✏️ 修改 | `internal/biz/exam.go` | Task 4 |
| ✏️ 修改 | `internal/biz/score.go` | Task 4 |
| ✏️ 修改 | `internal/biz/score_item.go` | Task 4 |
| ✏️ 修改 | `internal/biz/student.go` | Task 4 |
| ✏️ 修改 | `internal/biz/subject.go` | Task 4 |
| ✏️ 修改 | `configs/config.yaml` | Task 10(本地) |
| ✏️ 修改 | `configs/config_example.yaml` | Task 10 |
| ✏️ 修改 | `.gitignore` | Task 11 |
| ❌ 删除 | `init.sql` | Task 11 |
| ❌ 删除 | `seas.db`(根目录) | Task 11 |
