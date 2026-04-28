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

// ListByExamID 获取某次考试关联的学科列表，支持分页
func (r *subjectRepo) ListByExamID(ctx context.Context, examID int64, pageIndex, pageSize int32) ([]*biz.Subject, int64, error) {
	var subjects []*biz.Subject
	var total int64

	// 通过 exam_subjects 关联表查询
	query := r.data.db.WithContext(ctx).Model(&biz.Subject{}).
		Joins("JOIN exam_subjects es ON es.subject_id = subjects.id").
		Where("es.exam_id = ?", examID)

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		log.Context(ctx).Errorf("subjectRepo.ListByExamID count err: %+v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := int((pageIndex - 1) * pageSize)
	if err := query.Select("subjects.*").Offset(offset).Limit(int(pageSize)).Find(&subjects).Error; err != nil {
		log.Context(ctx).Errorf("subjectRepo.ListByExamID find err: %+v", err)
		return nil, 0, err
	}

	return subjects, total, nil
}

// GetFullScoreByExamSubject 获取考试中该学科的满分
func (r *subjectRepo) GetFullScoreByExamSubject(ctx context.Context, examID, subjectID int64) (float64, error) {
	var fullScore float64
	err := r.data.db.WithContext(ctx).Model(&biz.ExamSubject{}).
		Where("exam_id = ? AND subject_id = ?", examID, subjectID).
		Pluck("full_score", &fullScore).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("subjectRepo.GetFullScoreByExamSubject err: %+v", err)
		return 0, err
	}
	return fullScore, err
}

// FindOrCreateByName 按名称查找或创建学科
func (r *subjectRepo) FindOrCreateByName(ctx context.Context, name string) (*biz.Subject, error) {
	var subject biz.Subject
	err := r.data.db.WithContext(ctx).Where("name = ?", name).First(&subject).Error
	if err == nil {
		return &subject, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("subjectRepo.FindOrCreateByName find err: %+v", err)
		return nil, err
	}
	subject = biz.Subject{Name: name}
	if err := r.data.db.WithContext(ctx).Create(&subject).Error; err != nil {
		log.Context(ctx).Errorf("subjectRepo.FindOrCreateByName create err: %+v", err)
		return nil, err
	}
	return &subject, nil
}
