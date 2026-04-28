package biz

import (
	"context"
	"time"
)

type Exam struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"type:varchar(100);column:name"`
	ExamDate  time.Time `gorm:"column:exam_date"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (Exam) TableName() string {
	return "exams"
}

// ExamSubject 考试-学科关联表
type ExamSubject struct {
	ID         int64     `gorm:"primaryKey;column:id"`
	ExamID     int64     `gorm:"column:exam_id"`
	SubjectID  int64     `gorm:"column:subject_id"`
	FullScore  float64   `gorm:"column:full_score"`
	CreatedAt  time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (ExamSubject) TableName() string {
	return "exam_subjects"
}

type ExamRepo interface {
	GetByID(ctx context.Context, id int64) (*Exam, error)
	// ListAll 获取所有考试，支持分页和关键词搜索
	ListAll(ctx context.Context, pageIndex, pageSize int32, keyword string) ([]*Exam, int64, error)
	// GetExamName 获取考试名称
	GetExamName(ctx context.Context, id int64) (string, error)
	// Create 创建考试记录
	Create(ctx context.Context, exam *Exam) error
	// GetExamStudentCounts 批量获取考试的独立学生人数（从 scores 表统计）
	GetExamStudentCounts(ctx context.Context, examIDs []int64) (map[int64]int64, error)
}
