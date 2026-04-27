package biz

import (
	"context"
	"time"
)

type Score struct {
	ID         int64     `gorm:"primaryKey;column:id"`
	StudentID  int64     `gorm:"column:student_id"`
	ExamID     int64     `gorm:"column:exam_id"`
	SubjectID  int64     `gorm:"column:subject_id"`
	TotalScore float64   `gorm:"column:total_score"`
	CreatedAt  time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (Score) TableName() string {
	return "scores"
}

type ScoreRepo interface {
	GetByExamSubjectStudent(ctx context.Context, examID, subjectID, studentID int64) (*Score, error)
	GetByStudentID(ctx context.Context, studentID int64) ([]*Score, error)
	// GetSubjectSummary 获取学科统计信息（全科或单科）
	// 如果 subjectID 为 0，则返回该考试全科的统计（多科加权平均）
	GetSubjectSummary(ctx context.Context, examID, subjectID int64) (*SubjectSummaryStats, error)
	// GetClassSummary 获取班级统计信息（全科或单科）
	GetClassSummary(ctx context.Context, examID, subjectID int64) (*ClassSummaryStats, error)
	// GetRatingDistribution 获取四率分布统计
	GetRatingDistribution(ctx context.Context, examID, subjectID int64, excellentThreshold, goodThreshold, passThreshold float64) (*RatingDistributionStats, error)
	// GetClassSubjectSummary 获取班级学科下钻汇总
	GetClassSubjectSummary(ctx context.Context, examID, classID int64) (*ClassSubjectSummaryStats, error)
	// GetSingleClassSummary 获取单科学科下班级汇总
	GetSingleClassSummary(ctx context.Context, examID, subjectID int64) (*SingleClassSummaryStats, error)
}

// SubjectSummaryStats 学科统计数据
type SubjectSummaryStats struct {
	TotalParticipants int64           // 总参考人数
	SubjectsInvolved  int32           // 涉及学科数（仅 subjectID=0 时有意义）
	ClassesInvolved   int32           // 涉及班级数（仅 subjectID=0 时有意义）
	Overall           *SubjectStats   // 新增：全年级总体
	Subjects          []*SubjectStats // 学科统计详情
}

// SubjectStats 单个学科的统计信息
type SubjectStats struct {
	ID             int64
	Name           string
	FullScore      float64
	AvgScore       float64
	HighestScore   float64
	LowestScore    float64
	Difficulty     float64 // 平均分/满分*100
	StudentCount   int64
	ScoreDeviation float64 // 新增：离均差
	StdDev         float64 // 新增：标准差
	Discrimination float64 // 新增：区分度
}

// ClassSummaryStats 班级统计数据
type ClassSummaryStats struct {
	TotalParticipants int64         // 总参考人数
	OverallGrade      *ClassStats   // 全年级统计
	ClassDetails      []*ClassStats // 各班级统计详情
}

// ClassStats 班级统计信息
type ClassStats struct {
	ClassID        int64
	ClassName      string
	TotalStudents  int64
	FullScore      float64  // 新增：满分
	AvgScore       float64
	HighestScore   float64
	LowestScore    float64
	ScoreDeviation float64 // 离均差：班级平均分-全年级平均分，全年级固定为0
	Difficulty     float64 // 平均分/满分*100
	StdDev         float64 // 标准差
	Discrimination float64  // 新增：区分度
}

// RatingDistributionStats 四率分布统计数据
type RatingDistributionStats struct {
	TotalParticipants int64               // 总参考人数
	Config            *RatingConfigStats  // 四率配置
	OverallGrade      *ClassRatingStats   // 全年级四率
	ClassDetails      []*ClassRatingStats // 各班级四率
}

// RatingConfigStats 四率配置
type RatingConfigStats struct {
	ExcellentThreshold float64
	GoodThreshold      float64
	PassThreshold      float64
}

// RatingItemStats 单个等级的统计
type RatingItemStats struct {
	Count      int64
	Percentage float64
}

// ClassRatingStats 班级的四率统计
type ClassRatingStats struct {
	ClassID       int64
	ClassName     string
	TotalStudents int64
	AvgScore      float64
	Excellent     *RatingItemStats // 优秀
	Good          *RatingItemStats // 良好
	Pass          *RatingItemStats // 合格
	Fail          *RatingItemStats // 低分
}

// ClassSubjectSummaryStats 班级学科下钻汇总
type ClassSubjectSummaryStats struct {
	ExamID    int64
	ExamName  string
	ClassID   int64
	ClassName string
	Overall   *ClassSubjectItemStats
	Subjects  []*ClassSubjectItemStats
}

// ClassSubjectItemStats 班级学科下钻项
type ClassSubjectItemStats struct {
	SubjectID     int64
	SubjectName   string
	FullScore     float64
	ClassAvgScore float64
	GradeAvgScore float64
	ScoreDiff     float64
	ClassHighest  float64
	ClassLowest   float64
	ClassRank     int32
	TotalClasses  int32
}

// SingleClassSummaryStats 单科班级汇总
type SingleClassSummaryStats struct {
	ExamID      int64
	ExamName    string
	SubjectID   int64
	SubjectName string
	Overall     *ClassStats   // 改为 *ClassStats
	Classes     []*ClassStats // 改为 []*ClassStats
}

// SingleQuestionSummaryStats 单科题目汇总
type SingleQuestionSummaryStats struct {
	ExamID      int64
	ExamName    string
	SubjectID   int64
	SubjectName string
	Questions   []*SingleQuestionSummaryItemStats
}

// SingleQuestionSummaryItemStats 单科题目汇总项
type SingleQuestionSummaryItemStats struct {
	QuestionID     string
	QuestionNumber string
	QuestionType   string
	FullScore      float64
	GradeAvgScore  float64
	ClassBreakdown []*QuestionClassBreakdownStats
	ScoreRate      float64
	Difficulty     float64 // 改为 float64
}

// QuestionClassBreakdownStats 题目按班级拆分
type QuestionClassBreakdownStats struct {
	ClassID   int64
	ClassName string
	AvgScore  float64
}
