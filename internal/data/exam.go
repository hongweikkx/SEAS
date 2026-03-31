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

// ListAll 获取所有考试，支持分页
func (r *examRepo) ListAll(ctx context.Context, pageIndex, pageSize int32) ([]*biz.Exam, int64, error) {
	var exams []*biz.Exam
	var total int64

	query := r.data.db.WithContext(ctx).Model(&biz.Exam{})

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		log.Context(ctx).Errorf("examRepo.ListAll count err: %+v", err)
		return nil, 0, err
	}

	// 分页查询，按考试时间倒序
	offset := int((pageIndex - 1) * pageSize)
	if err := query.Order("exam_date DESC").Offset(offset).Limit(int(pageSize)).Find(&exams).Error; err != nil {
		log.Context(ctx).Errorf("examRepo.ListAll find err: %+v", err)
		return nil, 0, err
	}

	return exams, total, nil
}

// GetExamName 获取考试名称
func (r *examRepo) GetExamName(ctx context.Context, id int64) (string, error) {
	var exam biz.Exam
	err := r.data.db.WithContext(ctx).Select("name").First(&exam, id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("examRepo.GetExamName err: %+v", err)
		return "", err
	}
	return exam.Name, err
}
