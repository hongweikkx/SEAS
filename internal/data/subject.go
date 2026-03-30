package data

import (
	"context"
	"errors"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type subjectRepo struct {
	data *Data
}

func NewSubjectRepo(data *Data) biz.SubjectRepo {
	return &subjectRepo{
		data: data,
	}
}

func (r *subjectRepo) GetByID(ctx context.Context, id int64) (*biz.Subject, error) {
	var subject biz.Subject
	err := r.data.db.WithContext(ctx).First(&subject, id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("GetByID err: %+v", err)
	}
	return &subject, nil
}
