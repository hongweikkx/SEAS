# SEAS 学生成绩分析 - 测试数据快速指南

## ✅ 数据生成完成

已为您的系统生成了完整的测试数据集。以下是生成的内容：

### 📊 测试数据规模

```
✓ 3个班级（高一3个班）
✓ 20个学生（班级分布：7+7+6）
✓ 4个学科（语文、数学、英语、物理）
✓ 10场考试（从2026年3月到2027年1月）
✓ 27个考试-学科关联配置
✓ 540条学生成绩记录
✓ 574+条小题成绩记录
```

### 📁 生成的文件

| 文件 | 用途 |
|------|------|
| `seed-data.sql` | 包含所有测试数据的SQL脚本（已执行） |
| `test-data-summary.md` | 详细的数据统计和查询示例 |
| 本文件 | 快速参考指南 |

---

## 🚀 开始使用

### 1. 数据已在数据库中

所有数据已通过 `seed-data.sql` 脚本导入到MySQL数据库 `seas`。

### 2. 快速验证数据

```bash
# 查看数据统计
mysql -u root -p seas < test-data.sql

# 或直接运行查询
mysql -u root -p -e "SELECT COUNT(*) as '学生成绩' FROM seas.scores;"
```

### 3. 常用查询

#### 查看所有班级及人数
```sql
SELECT c.name, COUNT(s.id) as 人数 
FROM classes c 
LEFT JOIN students s ON c.id = s.class_id 
GROUP BY c.id;
```

#### 查看学生在某次考试中的成绩
```sql
SELECT s.name, sub.name, sc.total_score 
FROM scores sc 
JOIN students s ON sc.student_id = s.id 
JOIN subjects sub ON sc.subject_id = sub.id 
WHERE sc.exam_id = 1;  -- exam_id 改为具体的考试ID
```

#### 班级成绩平均分对比
```sql
SELECT c.name as 班级, ROUND(AVG(sc.total_score), 2) as 平均分
FROM classes c 
JOIN students s ON c.id = s.class_id 
JOIN scores sc ON s.id = sc.student_id 
GROUP BY c.id;
```

#### 知识点错误率分析
```sql
SELECT si.knowledge_point, 
       COUNT(*) as 总数,
       SUM(CASE WHEN si.is_correct=0 THEN 1 ELSE 0 END) as 错误数,
       ROUND(SUM(CASE WHEN si.is_correct=0 THEN 1 ELSE 0 END)/COUNT(*)*100,2) as 错误率
FROM score_items si 
GROUP BY si.knowledge_point 
ORDER BY 错误率 DESC;
```

---

## 📚 学生列表

### 高一(1)班（7人）
- 张三 (S001)、李四 (S002)、王五 (S003)、赵六 (S004)
- 孙七 (S005)、周八 (S006)、吴九 (S007)

### 高一(2)班（7人）
- 郑十 (S008)、陈十一 (S009)、何十二 (S010)、林十三 (S011)
- 冯十四 (S012)、曾十五 (S013)、曹十六 (S014)

### 高一(3)班（6人）
- 韦十七 (S015)、韩十八 (S016)、唐十九 (S017)、许二十 (S018)
- 邓二一 (S019)、傅二二 (S020)

---

## 📋 考试列表

| ID | 考试名称 | 日期 | 科目 |
|----|---------|------|------|
| 1 | 2026年3月月考 | 2026-03-01 | 语、数、英 |
| 2 | 2026年4月月考 | 2026-04-01 | 数、英、物 |
| 3 | 2026年5月月考 | 2026-05-01 | 语、数 |
| 4 | 2026年6月期中考试 | 2026-06-01 | 语、数、英、物 |
| 5 | 2026年7月月考 | 2026-07-01 | 英、物 |
| 6 | 2026年9月月考 | 2026-09-01 | 语、英 |
| 7 | 2026年10月月考 | 2026-10-01 | 数、物 |
| 8 | 2026年11月月考 | 2026-11-01 | 语、数、英 |
| 9 | 2026年12月期末考试 | 2026-12-01 | 语、数、英、物 |
| 10 | 2027年1月月考 | 2027-01-01 | 语、数 |

---

## 🎯 测试场景示例

### 场景1：班级成绩分析
> 想要分析高一(1)班在2026年6月期中考试中各科目的平均分

```sql
SELECT 
  c.name as 班级,
  sub.name as 学科,
  ROUND(AVG(sc.total_score), 2) as 平均分
FROM classes c 
JOIN students s ON c.id = s.class_id 
JOIN scores sc ON s.id = sc.student_id 
JOIN subjects sub ON sc.subject_id = sub.id 
WHERE c.id = 1 AND sc.exam_id = 4
GROUP BY sub.id;
```

### 场景2：学生薄弱知识点
> 找出张三在所有考试中错误率最高的知识点

```sql
SELECT 
  si.knowledge_point,
  COUNT(*) as 总数,
  SUM(CASE WHEN si.is_correct=0 THEN 1 ELSE 0 END) as 错误数,
  ROUND(SUM(CASE WHEN si.is_correct=0 THEN 1 ELSE 0 END)/COUNT(*)*100,2) as 错误率
FROM score_items si 
JOIN scores sc ON si.score_id = sc.id 
JOIN students s ON sc.student_id = s.id 
WHERE s.student_number = 'S001'
GROUP BY si.knowledge_point 
ORDER BY 错误率 DESC;
```

### 场景3：班级排名
> 查看三个班级在2026年12月期末考试中的排名（按平均分）

```sql
SELECT 
  c.name as 班级,
  COUNT(DISTINCT s.id) as 学生数,
  ROUND(AVG(sc.total_score), 2) as 平均分
FROM classes c 
JOIN students s ON c.id = s.class_id 
JOIN scores sc ON s.id = sc.student_id 
WHERE sc.exam_id = 9
GROUP BY c.id 
ORDER BY 平均分 DESC;
```

---

## 🔄 数据特点

✅ **完整的班级维度** - 支持班级级的数据聚合分析  
✅ **细粒度小题数据** - 每科10-15个小题，支持知识点分析  
✅ **多次考试数据** - 10次考试覆盖整个学年  
✅ **真实成绩分布** - 平均分86左右，模拟真实考试  
✅ **混合学科配置** - 不同考试科目数不同，贴近实际  
✅ **知识点标记完整** - 每道小题都有知识点标签  

---

## 📝 数据修改

### 删除所有测试数据（重新开始）
```sql
DELETE FROM score_items;
DELETE FROM scores;
DELETE FROM exam_subjects;
DELETE FROM exams;
DELETE FROM subjects;
DELETE FROM students;
DELETE FROM classes;
```

### 添加新的考试
```sql
INSERT INTO exams (name, exam_date) VALUES ('2027年2月月考', '2027-02-01');

-- 配置该考试的科目
INSERT INTO exam_subjects (exam_id, subject_id, full_score) VALUES 
(11, 1, 100),  -- 语文
(11, 2, 100);  -- 数学
```

### 批量添加学生成绩
参考 `seed-data.sql` 中的 INSERT 语句格式进行扩展。

---

## 💻 开发集成

### Go代码中使用
当您的API接口就绪时，可以直接查询这些测试数据：

```go
// 查询班级信息
var classes []biz.Class
db.Where("id = ?", 1).Find(&classes)

// 查询学生
var students []biz.Student
db.Where("class_id = ?", 1).Find(&students)

// 查询成绩
var scores []biz.Score
db.Where("exam_id = ? AND subject_id = ?", 1, 1).Find(&scores)
```

---

## ❓ 常见问题

### Q: 数据在哪个数据库中？
A: 所有数据都在 MySQL 数据库 `seas` 中。

### Q: 如何清空重新生成？
A: 执行删除语句后，重新运行 `seed-data.sql`。

### Q: 能否修改数据？
A: 可以，直接修改 `seed-data.sql` 中的 INSERT 语句后重新执行。

### Q: 需要更多学生/考试吗？
A: 可以复制现有的 INSERT 语句并修改ID继续添加。

---

## 📞 后续需求

如果您需要：
- ✓ 添加更多数据量
- ✓ 生成不同成绩分布的数据
- ✓ 添加其他字段或表
- ✓ 生成特定场景的测试数据

请直接告诉我，我会为您扩展脚本！

---

**测试数据生成时间**: $(date)  
**数据覆盖周期**: 2026年3月 - 2027年1月  
**系统就绪**: ✓ 可开始开发和测试

