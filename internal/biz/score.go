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
}

// SubjectSummaryStats 学科统计数据
type SubjectSummaryStats struct {
	TotalParticipants int64                    // 总参考人数
	SubjectsInvolved  int32                    // 涉及学科数（仅 subjectID=0 时有意义）
	ClassesInvolved   int32                    // 涉及班级数（仅 subjectID=0 时有意义）
	Subjects          []*SubjectStats          // 学科统计详情
}

// SubjectStats 单个学科的统计信息
type SubjectStats struct {
	ID           int64
	Name         string
	FullScore    float64
	AvgScore     float64
	HighestScore float64
	LowestScore  float64
	Difficulty   float64  // 平均分/满分*100
	StudentCount int64
}

// ClassSummaryStats 班级统计数据
type ClassSummaryStats struct {
	TotalParticipants int64         // 总参考人数
	OverallGrade      *ClassStats   // 全年级统计
	ClassDetails      []*ClassStats // 各班级统计详情
}

// ClassStats 班级统计信息
type ClassStats struct {
	ClassID       int64
	ClassName     string
	TotalStudents int64
	AvgScore      float64
	HighestScore  float64
	LowestScore   float64
	ScoreDeviation float64  // 离均差：班级平均分-全年级平均分，全年级固定为0
	Difficulty    float64   // 平均分/满分*100
	StdDev        float64   // 标准差
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
