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

// FindOrCreateByName 按名称查找或创建班级
func (r *classRepo) FindOrCreateByName(ctx context.Context, name string) (*biz.Class, error) {
	var class biz.Class
	err := r.data.db.WithContext(ctx).Where("name = ?", name).First(&class).Error
	if err == nil {
		return &class, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("classRepo.FindOrCreateByName find err: %+v", err)
		return nil, err
	}
	// 不存在则创建
	class = biz.Class{Name: name}
	if err := r.data.db.WithContext(ctx).Create(&class).Error; err != nil {
		log.Context(ctx).Errorf("classRepo.FindOrCreateByName create err: %+v", err)
		return nil, err
	}
	return &class, nil
}

