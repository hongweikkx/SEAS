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
	// ListByExamID 获取某次考试关联的学科列表，支持分页
	ListByExamID(ctx context.Context, examID int64, pageIndex, pageSize int32) ([]*Subject, int64, error)
	// GetFullScoreByExamSubject 获取考试中该学科的满分
	GetFullScoreByExamSubject(ctx context.Context, examID, subjectID int64) (float64, error)
}
