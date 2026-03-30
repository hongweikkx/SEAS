package data

import (
	"context"
	"errors"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type examRepo struct {
	data *Data
}

func NewExamRepo(data *Data) biz.ExamRepo {
	return &examRepo{
		data: data,
	}
}
func (r *examRepo) GetByID(ctx context.Context, id int64) (*biz.Exam, error) {
	var exam biz.Exam
	err := r.data.db.WithContext(ctx).First(&exam, id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("examRepo.GetByID err: %+v", err)
	}
	return &exam, err
}
