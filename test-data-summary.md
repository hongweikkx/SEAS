# SEAS 学生成绩分析系统 - 测试数据统计

## 📊 数据概览

### 数据规模统计
| 表名 | 数量 | 说明 |
|------|------|------|
| 班级 (classes) | 3 | 高一(1)班、高一(2)班、高一(3)班 |
| 学科 (subjects) | 4 | 语文、数学、英语、物理 |
| 学生 (students) | 20 | 班级1: 7人，班级2: 7人，班级3: 6人 |
| 考试 (exams) | 10 | 时间跨度：2026年3月-2027年1月 |
| 考试-学科 (exam_subjects) | 27 | 平均每场考试2.7科 |
| **学生成绩 (scores)** | **540** | 20学生 × 10考试 × 2.7科平均 |
| **小题成绩 (score_items)** | **574+** | 每科10-15个小题 |

### 班级分布
```
高一(1)班  ├─ 张三、李四、王五、赵六、孙七、周八、吴九 (7人)
高一(2)班  ├─ 郑十、陈十一、何十二、林十三、冯十四、曾十五、曹十六 (7人)
高一(3)班  └─ 韦十七、韩十八、唐十九、许二十、邓二一、傅二二 (6人)
```

### 考试配置
| 考试 | 时间 | 科目 | 备注 |
|------|------|------|------|
| 2026年3月月考 | 2026-03-01 | 语、数、英 | 常规月考 |
| 2026年4月月考 | 2026-04-01 | 数、英、物 | 常规月考 |
| 2026年5月月考 | 2026-05-01 | 语、数 | 常规月考 |
| 2026年6月期中考试 | 2026-06-01 | 语、数、英、物 | 期中全科 |
| 2026年7月月考 | 2026-07-01 | 英、物 | 常规月考 |
| 2026年9月月考 | 2026-09-01 | 语、英 | 常规月考 |
| 2026年10月月考 | 2026-10-01 | 数、物 | 常规月考 |
| 2026年11月月考 | 2026-11-01 | 语、数、英 | 常规月考 |
| 2026年12月期末考试 | 2026-12-01 | 语、数、英、物 | 期末全科 |
| 2027年1月月考 | 2027-01-01 | 语、数 | 常规月考 |

---

## 🔍 查询示例

### 1. 班级成绩分析

**查询每个班级在各次考试中的平均分：**

```sql
SELECT 
  c.name as '班级',
  e.name as '考试',
  sub.name as '学科',
  COUNT(sc.id) as '学生数',
  AVG(sc.total_score) as '平均分',
  MAX(sc.total_score) as '最高分',
  MIN(sc.total_score) as '最低分'
FROM classes c
JOIN students s ON c.id = s.class_id
JOIN scores sc ON s.id = sc.student_id
JOIN exams e ON sc.exam_id = e.id
JOIN subjects sub ON sc.subject_id = sub.id
GROUP BY c.id, e.id, sub.id
ORDER BY e.id, c.id, sub.id;
```

### 2. 学生成绩排名

**查询每场考试中学生的总分排名（按班级）：**

```sql
SELECT 
  e.name as '考试',
  c.name as '班级',
  s.name as '学生',
  s.student_number as '学号',
  ROUND(SUM(sc.total_score), 2) as '总分',
  RANK() OVER (PARTITION BY e.id, c.id ORDER BY SUM(sc.total_score) DESC) as '班内排名'
FROM students s
JOIN classes c ON s.class_id = c.id
JOIN scores sc ON s.id = sc.student_id
JOIN exams e ON sc.exam_id = e.id
GROUP BY e.id, c.id, s.id
ORDER BY e.id, c.id, SUM(sc.total_score) DESC;
```

### 3. 知识点掌握分析

**分析学生在各知识点上的掌握情况（错误率）：**

```sql
SELECT 
  si.knowledge_point as '知识点',
  sub.name as '学科',
  COUNT(*) as '总题数',
  SUM(CASE WHEN si.is_correct = 1 THEN 1 ELSE 0 END) as '正确数',
  SUM(CASE WHEN si.is_correct = 0 THEN 1 ELSE 0 END) as '错误数',
  ROUND(SUM(CASE WHEN si.is_correct = 0 THEN 1 ELSE 0 END) / COUNT(*) * 100, 2) as '错误率%',
  ROUND(AVG(si.score), 2) as '平均得分'
FROM score_items si
JOIN scores sc ON si.score_id = sc.id
JOIN subjects sub ON sc.subject_id = sub.id
GROUP BY si.knowledge_point, sub.id
ORDER BY sub.id, 错误率% DESC;
```

### 4. 学生个人分析

**某学生的成绩变化趋势：**

```sql
SELECT 
  e.name as '考试',
  sub.name as '学科',
  sc.total_score as '成绩'
FROM students s
JOIN scores sc ON s.id = sc.student_id
JOIN exams e ON sc.exam_id = e.id
JOIN subjects sub ON sc.subject_id = sub.id
WHERE s.student_number = 'S001'
ORDER BY e.id, sub.id;
```

### 5. 科目难度分析

**分析各科目在不同考试中的难度（平均分）：**

```sql
SELECT 
  sub.name as '学科',
  e.name as '考试',
  COUNT(DISTINCT sc.student_id) as '参加人数',
  ROUND(AVG(sc.total_score), 2) as '平均分',
  ROUND(STDDEV(sc.total_score), 2) as '标准差'
FROM scores sc
JOIN exams e ON sc.exam_id = e.id
JOIN subjects sub ON sc.subject_id = sub.id
GROUP BY sub.id, e.id
ORDER BY sub.id, AVG(sc.total_score) DESC;
```

### 6. 小题难度排序

**找出最难的小题（错误率最高）：**

```sql
SELECT 
  si.question_number as '小题号',
  si.knowledge_point as '知识点',
  sub.name as '学科',
  COUNT(*) as '总数',
  SUM(CASE WHEN si.is_correct = 0 THEN 1 ELSE 0 END) as '错误数',
  ROUND(SUM(CASE WHEN si.is_correct = 0 THEN 1 ELSE 0 END) / COUNT(*) * 100, 2) as '错误率%'
FROM score_items si
JOIN scores sc ON si.score_id = sc.id
JOIN subjects sub ON sc.subject_id = sub.id
GROUP BY si.question_number, si.knowledge_point, sub.id
ORDER BY 错误率% DESC
LIMIT 20;
```

### 7. 班级对比

**比较三个班级在期末考试中的表现：**

```sql
SELECT 
  c.name as '班级',
  COUNT(DISTINCT s.id) as '学生数',
  ROUND(AVG(sc.total_score), 2) as '平均分',
  MAX(sc.total_score) as '最高分',
  MIN(sc.total_score) as '最低分',
  ROUND(STDDEV(sc.total_score), 2) as '成绩差异'
FROM classes c
JOIN students s ON c.id = s.class_id
JOIN scores sc ON s.id = sc.student_id
JOIN exams e ON sc.exam_id = e.id
WHERE e.id = 9  -- 期末考试
GROUP BY c.id;
```

---

## 📈 主要统计指标

### 全校概况
- **学生总数**：20人
- **班级数**：3个
- **学科数**：4个
- **考试次数**：10次
- **总成绩记录**：540条
- **总小题记录**：574+条

### 成绩分布（基于样本）
- **平均成绩范围**：75.5 - 95.5 分
- **总体平均分**：~86 分
- **最高单科成绩**：95.5 分
- **最低单科成绩**：75.5 分

### 小题数据
- **每科小题数**：10-15题
- **每科满分分**：100 分
- **小题满分配置**：单选5分，大题50分（混合）
- **正确率分布**：约80-85% 正确率

---

## 💡 使用建议

### 数据分析方向
1. **学生学情诊断** - 基于知识点错误率识别薄弱环节
2. **班级对标** - 班级间成绩差异分析
3. **科目特征** - 不同科目的难度特征分析
4. **学生进度** - 追踪学生的成绩变化趋势
5. **教学反馈** - 通过小题错误率反馈教学效果

### 数据特点
- ✅ 完整的班级维度数据，支持班级级分析
- ✅ 细粒度的小题成绩，支持知识点分析
- ✅ 多次考试数据，支持趋势分析
- ✅ 混合的学科配置，贴近真实场景
- ✅ 合理的成绩分布，模拟真实成绩曲线

---

## 🔧 数据更新

若需要添加更多测试数据，可以：

1. **添加更多学生**：修改 `students` 表插入语句
2. **添加更多考试**：修改 `exams` 和 `exam_subjects` 表
3. **批量生成成绩**：使用存储过程或 SQL 循环生成

示例存储过程框架已在注释中提供，可直接拿去扩展使用。

