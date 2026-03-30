package data

import (
	"context"
	"errors"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type scoreRepo struct {
	data *Data
}

func NewScoreRepo(data *Data) biz.ScoreRepo {
	return &scoreRepo{
		data: data,
	}
}

func (r *scoreRepo) GetByExamSubjectStudent(ctx context.Context, examID, subjectID, studentID int64) (*biz.Score, error) {
	var s biz.Score
	err := r.data.db.WithContext(ctx).Where("exam_id = ? AND subject_id = ? AND student_id = ?", examID, subjectID, studentID).First(&s).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("scoreRepo.GetByExamSubjectStudent err: %+v", err)
	}
	return &s, err
}

func (r *scoreRepo) GetByStudentID(ctx context.Context, studentID int64) ([]*biz.Score, error) {
	var scores []*biz.Score
	err := r.data.db.WithContext(ctx).Where("student_id = ?", studentID).Find(&scores).Error
	if err != nil {
		log.Context(ctx).Errorf("scoreRepo.GetByStudentID err: %+v", err)
		return nil, err
	}
	return scores, nil
}
