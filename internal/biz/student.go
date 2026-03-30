package biz

import (
	"context"
	"time"
)

type Student struct {
	ID            int64     `gorm:"primaryKey;column:id"`
	StudentNumber string    `gorm:"uniqueIndex;type:varchar(64);column:student_number"`
	Name          string    `gorm:"type:varchar(100);column:name"`
	Class         string    `gorm:"type:varchar(100);column:class"`
	CreatedAt     time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (Student) TableName() string {
	return "students"
}

type StudentRepo interface {
	GetByID(ctx context.Context, id int64) (*Student, error)
	GetByStudentNumber(ctx context.Context, sn string) (*Student, error)
}
