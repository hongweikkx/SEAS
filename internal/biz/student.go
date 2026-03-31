package biz

import (
	"context"
	"time"
)

type Class struct {
	ID   int64  `gorm:"primaryKey;column:id"`
	Name string `gorm:"type:varchar(100);uniqueIndex;column:name"`
	Grade string `gorm:"type:varchar(50);column:grade"`
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (Class) TableName() string {
	return "classes"
}

type Student struct {
	ID            int64     `gorm:"primaryKey;column:id"`
	StudentNumber string    `gorm:"uniqueIndex;type:varchar(64);column:student_number"`
	Name          string    `gorm:"type:varchar(100);column:name"`
	ClassID       int64     `gorm:"column:class_id"`
	CreatedAt     time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (Student) TableName() string {
	return "students"
}

type StudentRepo interface {
	GetByID(ctx context.Context, id int64) (*Student, error)
	GetByStudentNumber(ctx context.Context, sn string) (*Student, error)
}

type ClassRepo interface {
	GetByID(ctx context.Context, id int64) (*Class, error)
	GetByName(ctx context.Context, name string) (*Class, error)
}

