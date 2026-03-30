package biz

import "context"

type Subject struct {
	ID   int64  `gorm:"primaryKey;column:id"`
	Name string `gorm:"type:varchar(100);column:name"`
	Code string `gorm:"type:varchar(50);column:code"` // 可选字段，用于标识学科编码
}

func (Subject) TableName() string {
	return "subjects"
}

type SubjectRepo interface {
	GetByID(ctx context.Context, id int64) (*Subject, error)
}
