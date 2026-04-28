package data

import (
	"context"
	"errors"
	"fmt"
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

type studentRepo struct {
	data *Data
	log  *log.Helper
}

func NewStudentRepo(data *Data, logger log.Logger) biz.StudentRepo {
	return &studentRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *studentRepo) GetByID(ctx context.Context, id int64) (*biz.Student, error) {
	var student biz.Student
	err := r.data.db.WithContext(ctx).First(&student, id).Error
	if err != nil {
		return nil, err
	}
	return &student, nil
}

func (r *studentRepo) GetByStudentNumber(ctx context.Context, sn string) (*biz.Student, error) {
	var student biz.Student
	err := r.data.db.WithContext(ctx).Where("student_number = ?", sn).First(&student).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Context(ctx).Errorf("studentRepo.GetByStudentNumber err: %+v", err)
	}
	return &student, err
}

// FindOrCreateByNameClass 按姓名+班级查找或创建学生
func (r *studentRepo) FindOrCreateByNameClass(ctx context.Context, name string, classID int64) (*biz.Student, error) {
	var student biz.Student
	// 按姓名+班级查找（假设同一班级内姓名唯一）
	err := r.data.db.WithContext(ctx).Where("name = ? AND class_id = ?", name, classID).First(&student).Error
	if err == nil {
		return &student, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		r.log.Errorf("studentRepo.FindOrCreateByNameClass find err: %+v", err)
		return nil, err
	}
	// 不存在则创建，自动生成学号
	sn := fmt.Sprintf("TEMP_%d_%s", classID, name)
	student = biz.Student{
		StudentNumber: sn,
		Name:          name,
		ClassID:       classID,
	}
	if err := r.data.db.WithContext(ctx).Create(&student).Error; err != nil {
		r.log.Errorf("studentRepo.FindOrCreateByNameClass create err: %+v", err)
		return nil, err
	}
	return &student, nil
}
