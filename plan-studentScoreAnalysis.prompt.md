# 学生成绩分析数据库建表计划

## 📋 概述

基于现有的Go代码结构（Student、Subject、Exam、Score、ScoreItem），生成完整的SQL建表语句，包括7个表：6个核心表和1个关联表，确保支持考试、学科、学生、班级、总分和小题细粒度分析功能。

## 🎯 核心数据表设计

### 1. 班级表 (classes) ⭐
**用途**：存储班级基本信息，支持班级维度的数据分析

**字段设计**：
- `id` (BIGINT, PK) - 班级ID
- `name` (VARCHAR(100), UNIQUE) - 班级名称，唯一标识
- `grade` (VARCHAR(50)) - 年级
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 唯一索引：name

**关键意义**：
- 支持班级维度的成绩分析（平均分、最高分、最低分等）
- 灵活扩展班级属性（年级、班主任等）
- 便于按班级进行数据聚合统计

---

### 2. 学生表 (students)
**用途**：存储学生基本信息，现在关联班级

**字段设计**：
- `id` (BIGINT, PK) - 学生ID
- `student_number` (VARCHAR(64), UNIQUE) - 学号，唯一标识
- `name` (VARCHAR(100)) - 学生姓名
- `class_id` (BIGINT, FK) - 班级ID，关联到 classes 表
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 唯一索引：student_number
- 普通索引：class_id（支持按班级查询学生）
- 外键：class_id → classes(id)，ON DELETE RESTRICT

**查询场景**：
- 查询某班级的所有学生
- 查询学生信息时包含班级信息

---

### 3. 学科表 (subjects)
**用途**：存储学生基本信息

**字段设计**：
- `id` (BIGINT, PK) - 学生ID
- `student_number` (VARCHAR(64), UNIQUE) - 学号，唯一标识
- `name` (VARCHAR(100)) - 学生姓名
- `class` (VARCHAR(100)) - 班级
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 唯一索引：student_number

---

### 2. 学科表 (subjects)
**用途**：存储学科基本信息

**字段设计**：
- `id` (BIGINT, PK) - 学科ID
- `name` (VARCHAR(100)) - 学科名称
- `code` (VARCHAR(50)) - 学科编码，可选
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 唯一索引：code

---

### 3. 考试表 (exams)
**用途**：存储考试基本信息

**字段设计**：
- `id` (BIGINT, PK) - 考试ID
- `name` (VARCHAR(100)) - 考试名称
- `exam_date` (TIMESTAMP) - 考试时间
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 普通索引：exam_date（支持按考试时间查询）

---

### 4. 考试-学科关联表 (exam_subjects) ⭐
**用途**：记录每次考试包含的学科及其满分

**字段设计**：
- `id` (BIGINT, PK) - 关联ID
- `exam_id` (BIGINT, FK) - 考试ID
- `subject_id` (BIGINT, FK) - 学科ID
- `full_score` (FLOAT) - 该科满分（默认100）
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 唯一复合索引：(exam_id, subject_id) - 防止重复关联
- 外键：exam_id → exams(id)，ON DELETE CASCADE
- 外键：subject_id → subjects(id)，ON DELETE CASCADE

**关键意义**：
- 支持一个考试包含多个学科的场景
- 灵活记录每个学科的满分配置
- 通过级联删除保持数据一致性

---

### 5. 学生成绩表 (scores)
**用途**：存储学生在某次考试某学科的汇总分数

**字段设计**：
- `id` (BIGINT, PK) - 成绩ID
- `student_id` (BIGINT, FK) - 学生ID
- `exam_id` (BIGINT, FK) - 考试ID
- `subject_id` (BIGINT, FK) - 学科ID
- `total_score` (FLOAT) - 该科总分
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 唯一复合索引：(student_id, exam_id, subject_id) - 防止重复成绩记录
- 外键：student_id → students(id)，ON DELETE CASCADE
- 外键：exam_id → exams(id)，ON DELETE CASCADE
- 外键：subject_id → subjects(id)，ON DELETE CASCADE

**查询场景**：
- 查询某学生的所有成绩
- 查询某次考试的所有成绩
- 查询某学科的所有成绩
- 分析学生的科目排名

---

### 6. 学生小题成绩表 (score_items)
**用途**：细粒度记录每道小题的得分、是否正确、知识点

**字段设计**：
- `id` (BIGINT, PK) - 小题成绩ID
- `score_id` (BIGINT, FK) - 成绩ID（关联scores表）
- `question_number` (VARCHAR(20)) - 小题编号（如"1.1"、"2.3"）
- `knowledge_point` (VARCHAR(100)) - 知识点
- `score` (FLOAT) - 得分
- `full_score` (FLOAT) - 总分
- `is_correct` (TINYINT(1)) - 是否正确（0:错误, 1:正确）
- `created_at` (TIMESTAMP) - 创建时间

**索引**：
- 主键索引：id
- 普通索引：score_id（支持按成绩查询小题）
- 普通索引：knowledge_point（支持按知识点分析）
- 外键：score_id → scores(id)，ON DELETE CASCADE

**分析能力**：
- 知识点薄弱分析（知识点错误率统计）
- 题型难度分析（通过小题得分分布）
- 学生错题分析（筛选is_correct=0的题目）
- 知识点掌握度评估

---

## 🔗 关系设计图

```
班级表 (classes)
    ↓ (1:N)
学生表 (students)
    ↓ (1:N)
成绩表 (scores) ← 学科表 (subjects)
    ↓ (1:N)          ↑
小题成绩表         考试表 (exams)
(score_items)      ↑ (N:M)
                   关联表 (exam_subjects)

数据流向：
1. 创建班级，添加学生到班级
2. 创建考试 + 配置学科 (exam_subjects表记录哪些学科参加该考试及满分)
3. 学生参加考试，获得某科总分 (scores表记录)
4. 逐题记录学生的得分情况 (score_items表记录每道题的得分)
5. 支持按班级聚合分析成绩数据
```

---

## 💾 SQL 生成清单

需要执行以下SQL操作：

1. ✅ 创建 `classes` 表
2. ✅ 创建 `students` 表（关联班级表）
3. ✅ 创建 `subjects` 表
4. ✅ 创建 `exams` 表
5. ✅ 创建 `exam_subjects` 关联表
6. ✅ 创建 `scores` 表
7. ✅ 创建 `score_items` 表

---

## 🎓 典型查询场景支持

| 场景 | SQL示例 | 支持表 |
|------|--------|--------|
| 班级平均分 | SELECT c.name, AVG(scores.total_score) as平均分 FROM ... WHERE exam_id=? GROUP BY c.id | classes, students, scores |
| 班级排名 | SELECT c.name, SUM(scores.total_score) as总分 FROM ... WHERE exam_id=? GROUP BY c.id ORDER BY 总分 DESC | classes, students, scores |
| 查询学生总分排名 | SELECT s.name, c.name as班级, scores.total_score FROM ... WHERE exam_id=? ORDER BY total_score DESC | students, classes, scores |
| 知识点错误率分析 | SELECT knowledge_point, COUNT(*) as错误数 FROM score_items WHERE is_correct=0 GROUP BY knowledge_point | score_items |
| 学生科目对比 | SELECT * FROM scores WHERE student_id=? ORDER BY exam_id | scores |
| 考试难度分析 | SELECT subject_id, AVG(total_score) as平均分 FROM scores WHERE exam_id=? GROUP BY subject_id | scores |
| 某科全部学生成绩 | SELECT s.name, c.name as班级, scores.total_score FROM ... WHERE exam_id=? AND subject_id=? | students, classes, scores |
| 小题难度排序 | SELECT question_number, COUNT(*) as错误数 FROM score_items WHERE exam_id=? GROUP BY question_number | score_items |
| 班级内学生排名 | SELECT s.name, scores.total_score FROM ... WHERE exam_id=? AND class_id=? ORDER BY total_score DESC | students, scores |

---

## ⚙️ 数据库配置

- **字符集**：utf8mb4
- **排序规则**：utf8mb4_general_ci
- **存储引擎**：InnoDB
- **外键约束**：启用（ON DELETE CASCADE 保证数据一致性）

---

## ✨ 设计亮点

1. **完整的关系模型**：通过外键约束确保数据完整性
2. **灵活的学科配置**：exam_subjects 表支持每次考试不同的学科组合
3. **细粒度的分析**：score_items 表支持知识点级别的深度分析
4. **高效的查询**：合理的索引设计支持常见查询场景
5. **数据安全**：级联删除防止孤立记录，保持数据一致性

---

## 🚀 后续扩展方向

1. **班级统计缓存表**：缓存班级平均分、最高分、最低分等统计数据，加速班级分析查询
2. **知识点统计表**：缓存各知识点的掌握情况
3. **成绩变动历史表**：记录成绩修正操作（审计日志）
4. **学生能力值表**：计算学生的知识点掌握度
5. **班级对标表**：支持班级间的成绩对标分析

