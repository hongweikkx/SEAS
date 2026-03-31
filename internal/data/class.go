package data

import (
	"context"
	"errors"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type classRepo struct {
	data *Data
	log  *log.Helper
}

func NewClassRepo(data *Data, logger log.Logger) biz.ClassRepo {
	return &classRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *classRepo) GetByID(ctx context.Context, id int64) (*biz.Class, error) {
	var class biz.Class
	err := r.data.db.WithContext(ctx).First(&class, id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("classRepo.GetByID err: %+v", err)
	}
	return &class, err
}

func (r *classRepo) GetByName(ctx context.Context, name string) (*biz.Class, error) {
	var class biz.Class
	err := r.data.db.WithContext(ctx).Where("name = ?", name).First(&class).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("classRepo.GetByName err: %+v", err)
	}
	return &class, err
}

