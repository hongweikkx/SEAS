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
}
