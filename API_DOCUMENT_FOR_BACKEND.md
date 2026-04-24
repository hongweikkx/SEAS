# SEAS 前端 API 接口文档（供后端参考）

> 本文档由前端代码梳理生成，列出了前端当前已定义的所有接口，
> 区分「已接入真实后端」和「仍使用 Mock 数据、需要后端实现」两类。

---

## 基础信息

- **Base URL**: `/seas/api/v1`（前端通过 `NEXT_PUBLIC_API_URL` 配置，如 `http://localhost:8000/seas/api/v1`）
- **响应格式**: 前端 Axios 拦截器已配置为直接返回 `response.data`，请后端统一返回 JSON 对象

---

## 一、已接入真实后端的接口（已有实现）

以下 5 个接口前端已调用真实后端，请后端确保其稳定性。

### 1.1 考试列表

```
GET /exams?pageIndex={pageIndex}&pageSize={pageSize}
```

**响应类型**:

```typescript
interface Exam {
  id: string
  name: string
  examDate: string
  createdAt: string
}

interface ExamsResponse {
  exams: Exam[]
  totalCount: number
  pageIndex: number
  pageSize: number
}
```

### 1.2 学科列表

```
GET /exams/{examId}/subjects?pageIndex={pageIndex}&pageSize={pageSize}
```

**响应类型**:

```typescript
interface Subject {
  id: string
  name: string
}

interface SubjectsResponse {
  subjects: Subject[]
  total: number
  pageIndex: number
  pageSize: number
}
```

### 1.3 学科汇总（视图1：学科情况汇总）

```
GET /exams/{examId}/analysis/subject-summary?scope={scope}&subjectId={subjectId}
```

**参数说明**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `scope` | `'all_subjects' \| 'single_subject'` | 是 | 分析范围 |
| `subjectId` | `string` | 否 | 单科分析时必填 |

**响应类型**:

```typescript
interface SubjectSummary {
  id: string          // 学科ID
  name: string        // 学科名称
  fullScore: number   // 满分
  avgScore: number    // 平均分
  highestScore: number// 最高分
  lowestScore: number // 最低分
  difficulty: number  // 难度系数
  studentCount: number// 参考人数
}

interface SubjectSummaryResponse {
  examId: string
  examName: string
  scope: 'all_subjects' | 'single_subject'
  totalParticipants: number   // 总参考人数
  subjectsInvolved?: number   // 涉及学科数（全科时）
  classesInvolved?: number    // 涉及班级数（全科时）
  subjects: SubjectSummary[]
}
```

### 1.4 班级汇总（视图2：班级情况汇总）

```
GET /exams/{examId}/analysis/class-summary?scope={scope}&subjectId={subjectId}
```

**参数说明**: 同 1.3

**响应类型**:

```typescript
interface ClassSummary {
  classId: number      // 班级ID
  className: string     // 班级名称
  totalStudents: number // 班级人数
  avgScore: number      // 平均分
  highestScore: number  // 最高分
  lowestScore: number   // 最低分
  scoreDeviation: number// 与年级均分偏差
  difficulty: number    // 难度系数
  stdDev: number        // 标准差
}

interface ClassSummaryResponse {
  examId: string
  examName: string
  scope: 'all_subjects' | 'single_subject'
  totalParticipants: number
  overallGrade: ClassSummary   // 年级总体数据
  classDetails: ClassSummary[] // 各班明细
}
```

### 1.5 四率分析（视图3：四率分析）

```
GET /exams/{examId}/analysis/rating-distribution?scope={scope}&subjectId={subjectId}
```

**参数说明**:

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `scope` | `'all_subjects' \| 'single_subject'` | 是 | 分析范围 |
| `subjectId` | `string` | 否 | 单科分析时必填 |

**请求体**（前端目前作为第二个参数传入，但 axios GET 可能将其序列化到 query 或 body，建议后端支持灵活读取；若不便，可改为 POST 或从 query 读取）：

```typescript
interface RatingConfig {
  excellent_threshold: number  // 优秀线，默认 90
  good_threshold: number       // 良好线，默认 70
  pass_threshold: number       // 及格线，默认 60
}
```

**响应类型**:

```typescript
interface RatingItem {
  count: number      // 人数
  percentage: number // 百分比
}

interface ClassRatingDistribution {
  classId: number
  className: string
  totalStudents: number
  avgScore: number
  excellent: RatingItem  // 优秀
  good: RatingItem       // 良好
  pass: RatingItem       // 及格
  fail: RatingItem       // 不及格
}

interface RatingDistributionResponse {
  examId: string
  examName: string
  scope: 'all_subjects' | 'single_subject'
  totalParticipants: number
  config: RatingConfig
  overallGrade: ClassRatingDistribution   // 年级总体
  classDetails: ClassRatingDistribution[] // 各班明细
}
```

---

## 二、需要后端实现的接口（当前为 Mock 数据）

以下 5 个下钻接口 + 1 个 AI 分析接口，前端当前使用 Mock 数据，**需要后端提供真实实现**。

### 2.1 班级学科汇总（视图4：班级学科汇总）

```
GET /exams/{examId}/classes/{classId}/subjects
```

**响应类型**:

```typescript
interface ClassSubjectItem {
  subjectId: string      // 学科ID
  subjectName: string     // 学科名称
  fullScore: number       // 满分
  classAvgScore: number   // 班级平均分
  gradeAvgScore: number   // 年级平均分
  scoreDiff: number       // 分差（班级-年级）
  classHighest: number    // 班级最高分
  classLowest: number     // 班级最低分
  classRank: number       // 班级排名
  totalClasses: number    // 总班级数
}

interface ClassSubjectSummaryResponse {
  examId: string
  examName: string
  classId: number
  className: string
  overall: ClassSubjectItem   // 总体数据
  subjects: ClassSubjectItem[] // 各学科明细
}
```

### 2.2 单科班级汇总（视图5：单科班级汇总）

```
GET /exams/{examId}/subjects/{subjectId}/classes
```

**响应类型**:

```typescript
interface SingleClassSummaryItem {
  classId: number         // 班级ID
  className: string        // 班级名称
  totalStudents: number    // 班级人数
  subjectAvgScore: number  // 学科平均分
  gradeAvgScore: number    // 年级平均分
  scoreDiff: number        // 分差
  classRank: number        // 班级排名
  totalClasses: number     // 总班级数
  passRate: number         // 及格率
  excellentRate: number    // 优秀率
}

interface SingleClassSummaryResponse {
  examId: string
  examName: string
  subjectId: string
  subjectName: string
  overall: SingleClassSummaryItem   // 年级总体
  classes: SingleClassSummaryItem[] // 各班明细
}
```

### 2.3 单科班级题目（视图6：单科班级题目）

```
GET /exams/{examId}/subjects/{subjectId}/classes/{classId}/questions
```

**响应类型**:

```typescript
interface ClassQuestionItem {
  questionId: string      // 题目ID
  questionNumber: string   // 题号（如 "1"、"2"、"三、1"）
  questionType: string     // 题型（如 "选择题"、"填空题"、"解答题"）
  fullScore: number        // 满分
  classAvgScore: number    // 班级均分
  scoreRate: number        // 得分率
  gradeAvgScore: number    // 年级均分
  difficulty: 'easy' | 'medium' | 'hard'  // 难度等级
}

interface SingleClassQuestionResponse {
  examId: string
  examName: string
  subjectId: string
  subjectName: string
  classId: number
  className: string
  questions: ClassQuestionItem[]
}
```

### 2.4 单科题目汇总（视图7：单科题目汇总）

```
GET /exams/{examId}/subjects/{subjectId}/questions
```

**响应类型**:

```typescript
interface QuestionClassBreakdown {
  classId: number    // 班级ID
  className: string   // 班级名称
  avgScore: number    // 该班此题均分
}

interface SingleQuestionSummaryItem {
  questionId: string              // 题目ID
  questionNumber: string           // 题号
  questionType: string             // 题型
  fullScore: number                // 满分
  gradeAvgScore: number            // 年级均分
  classBreakdown: QuestionClassBreakdown[]  // 各班得分明细
  scoreRate: number                // 得分率
  difficulty: 'easy' | 'medium' | 'hard'
}

interface SingleQuestionSummaryResponse {
  examId: string
  examName: string
  subjectId: string
  subjectName: string
  questions: SingleQuestionSummaryItem[]
}
```

### 2.5 单科班级题目详情（视图8：单科班级题目详情）

```
GET /exams/{examId}/subjects/{subjectId}/classes/{classId}/questions/{questionId}
```

**响应类型**:

```typescript
interface StudentQuestionDetail {
  studentId: string     // 学生ID
  studentName: string    // 学生姓名
  score: number          // 得分
  fullScore: number      // 满分
  scoreRate: number      // 得分率
  classRank: number      // 班级排名
  gradeRank: number      // 年级排名
  answerContent?: string // 作答内容（可选，非客观题时）
}

interface SingleQuestionDetailResponse {
  examId: string
  examName: string
  subjectId: string
  subjectName: string
  classId: number
  className: string
  questionId: string
  questionNumber: string
  questionType: string
  fullScore: number
  questionContent?: string  // 题目内容（可选）
  students: StudentQuestionDetail[]
}
```

---

## 三、AI 智能分析接口（新增需求）

前端已在 8 个分析视图中集成了「AI 智能分析」功能，当前为 Mock 实现，**需要后端提供真实的 AI 分析接口**。

### 建议接口设计

```
POST /ai/analysis
```

**请求体**:

```typescript
interface AIAnalysisRequest {
  view: 'class-summary' | 'subject-summary' | 'rating-analysis' |
        'class-subject-summary' | 'single-class-summary' |
        'single-class-question' | 'single-question-summary' |
        'single-question-detail'  // 当前视图类型
  examId: string       // 考试ID
  params?: {
    classId?: string
    subjectId?: string
    questionId?: string
  }
  // 可选：前端可将当前视图的关键数据作为上下文传入，
  // 以减少后端重复查询数据库
  context?: Record<string, unknown>
}
```

**响应类型**:

```typescript
interface AILink {
  label: string        // 显示文字
  targetView: 'class-summary' | 'subject-summary' | 'rating-analysis' |
              'class-subject-summary' | 'single-class-summary' |
              'single-class-question' | 'single-question-summary' |
              'single-question-detail'
  params?: {
    classId?: string
    subjectId?: string
    questionId?: string
  }
}

interface AIAnalysisResult {
  segments: Array<
    | { type: 'text'; content: string }
    | { type: 'link'; content: string; link: AILink }
  >
  generatedAt: number  // 生成时间戳（毫秒）
}
```

**说明**:

- `segments` 是一个富文本片段数组，每个片段要么是纯文本（`type: 'text'`），要么是带跳转链接的文本（`type: 'link'`）。
- 链接片段被用户点击后，前端会自动跳转到对应视图并携带参数。
- 后端可以接入大模型（如 OpenAI、Claude、通义千问等）生成分析文本，并在关键名词处插入 `link` 片段。
- `generatedAt` 用于前端缓存和显示生成时间。

---

## 四、接口优先级建议

| 优先级 | 接口 | 说明 |
|--------|------|------|
| P0 | 5 个下钻接口（2.1 ~ 2.5） | 核心分析功能，当前全 Mock，必须尽快接入真实数据 |
| P1 | AI 分析接口（3） | 增值功能，可先用 Mock 过渡，建议在下钻接口完成后实现 |
| P2 | PDF 导出 / 共享报告 | 前端已有 UI 占位，后端可后续按需支持 |

---

## 五、视图与接口对照表

| 前端视图名称 | 视图标识 | 对应接口 |
|-------------|---------|---------|
| 班级情况汇总 | `class-summary` | 1.4 班级汇总 |
| 学科情况汇总 | `subject-summary` | 1.3 学科汇总 |
| 四率分析 | `rating-analysis` | 1.5 四率分析 |
| 班级学科汇总 | `class-subject-summary` | 2.1 班级学科汇总 |
| 单科班级汇总 | `single-class-summary` | 2.2 单科班级汇总 |
| 单科班级题目 | `single-class-question` | 2.3 单科班级题目 |
| 单科题目汇总 | `single-question-summary` | 2.4 单科题目汇总 |
| 单科班级题目详情 | `single-question-detail` | 2.5 单科班级题目详情 |

所有 8 个视图均支持 AI 智能分析（接口 3）。

---

## 六、前端关键代码索引

如需对照前端实现，请参考以下文件：

| 文件路径 | 说明 |
|---------|------|
| `src/services/analysis.ts` | 已接入真实后端的 3 个分析接口 |
| `src/services/drilldown.ts` | 仍为 Mock 的 5 个下钻接口 |
| `src/types/analysis.ts` | 分析相关类型定义 |
| `src/types/drilldown.ts` | 下钻相关类型定义 |
| `src/types/ai.ts` | AI 分析相关类型定义 |
| `src/store/analysisStore.ts` | 全局状态管理（含视图路由、下钻路径、AI 结果缓存） |

---

*文档生成时间: 2026-04-24*
