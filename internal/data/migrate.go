package data

import (
	"seas/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

// AutoMigrate 把所有业务 model 同步到数据库 schema。
// 启动时调用一次,幂等(GORM 仅追加缺失的表/列/索引,不删除)。
func AutoMigrate(db *gorm.DB, logger log.Logger) error {
	helper := log.NewHelper(logger)
	helper.Info("AutoMigrate: 开始同步 schema")
	if err := db.AutoMigrate(
		&biz.Class{},
		&biz.Student{},
		&biz.Subject{},
		&biz.Exam{},
		&biz.ExamSubject{},
		&biz.Score{},
		&biz.ScoreItem{},
		&biz.User{},
	); err != nil {
		helper.Errorf("AutoMigrate failed: %+v", err)
		return err
	}
	helper.Info("AutoMigrate: schema 同步完成")
	return nil
}
