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

// ListAll 获取所有考试，支持分页和关键词搜索
func (r *examRepo) ListAll(ctx context.Context, pageIndex, pageSize int32, keyword string) ([]*biz.Exam, int64, error) {
	var exams []*biz.Exam
	var total int64

	query := r.data.db.WithContext(ctx).Model(&biz.Exam{})

	// 关键词搜索：按考试名称模糊匹配
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

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

// Create 创建考试记录
func (r *examRepo) Create(ctx context.Context, exam *biz.Exam) error {
	return r.data.db.WithContext(ctx).Create(exam).Error
}

// GetExamStudentCounts 批量获取考试的独立学生人数
func (r *examRepo) GetExamStudentCounts(ctx context.Context, examIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64)
	if len(examIDs) == 0 {
		return counts, nil
	}

	var results []struct {
		ExamID       int64 `gorm:"column:exam_id"`
		StudentCount int64 `gorm:"column:student_count"`
	}

	err := r.data.db.WithContext(ctx).Raw(`
		SELECT exam_id, COUNT(DISTINCT student_id) as student_count
		FROM scores
		WHERE exam_id IN ?
		GROUP BY exam_id
	`, examIDs).Scan(&results).Error

	if err != nil {
		log.Context(ctx).Errorf("examRepo.GetExamStudentCounts err: %+v", err)
		return nil, err
	}

	for _, r := range results {
		counts[r.ExamID] = r.StudentCount
	}
	return counts, nil
}
