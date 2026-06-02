package biz

import (
	"context"
	"time"
)

type Exam struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	Name      string    `gorm:"type:varchar(100);not null;column:name"`
	ExamDate  time.Time `gorm:"index;not null;column:exam_date"`
	UserID    uint64    `gorm:"index;column:user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (Exam) TableName() string {
	return "exams"
}

// ExamSubject 考试-学科关联表
type ExamSubject struct {
	ID        int64     `gorm:"primaryKey;column:id"`
	ExamID    int64     `gorm:"uniqueIndex:idx_exam_subject;not null;column:exam_id"`
	SubjectID int64     `gorm:"uniqueIndex:idx_exam_subject;not null;column:subject_id"`
	FullScore float64   `gorm:"column:full_score;default:100"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
	Exam      Exam      `gorm:"foreignKey:ExamID;references:ID;constraint:OnDelete:CASCADE"`
	Subject   Subject   `gorm:"foreignKey:SubjectID;references:ID;constraint:OnDelete:CASCADE"`
}

func (ExamSubject) TableName() string {
	return "exam_subjects"
}

type ExamRepo interface {
	GetByID(ctx context.Context, id int64) (*Exam, error)
	// ListAll 获取所有考试，支持分页和关键词搜索
	ListAll(ctx context.Context, pageIndex, pageSize int32, keyword string) ([]*Exam, int64, error)
	// ListByUserID 按用户 ID 查询考试列表
	ListByUserID(ctx context.Context, userID uint64, pageIndex, pageSize int32, keyword string) ([]*Exam, int64, error)
	// GetExamName 获取考试名称
	GetExamName(ctx context.Context, id int64) (string, error)
	// Create 创建考试记录
	Create(ctx context.Context, exam *Exam) error
	// GetExamStudentCounts 批量获取考试的独立学生人数（从 scores 表统计）
	GetExamStudentCounts(ctx context.Context, examIDs []int64) (map[int64]int64, error)
	// Delete 删除考试及其关联数据
	Delete(ctx context.Context, id int64) error
	// GetUserIDByExamID 获取考试的创建者 user_id
	GetUserIDByExamID(ctx context.Context, examID int64) (uint64, error)
}
