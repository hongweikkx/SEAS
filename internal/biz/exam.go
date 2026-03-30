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

type ExamRepo interface {
	GetByID(ctx context.Context, id int64) (*Exam, error)
}
