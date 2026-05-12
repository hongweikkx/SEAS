// Package data
package data

import (
	"seas/internal/conf"
	gormsql "seas/pkg/gorm"
	"seas/pkg/redis"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	"gorm.io/gorm"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewClassRepo,
	NewExamRepo,
	NewScoreRepo,
	NewScoreItemRepo,
	NewStudentRepo,
	NewSubjectRepo,
	NewAuthRepo,
)

// Data 是对所有数据库资源的统一封装
type Data struct {
	db  *gorm.DB
	rds *redis.Client
}

// NewData 创建 Data 并注入所有 repo 所需依赖
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	db, closeSQLF, err := gormsql.Init(logger, c.Database.Source)
	if err != nil {
		return nil, nil, err
	}

	if err := AutoMigrate(db, logger); err != nil {
		closeSQLF()
		return nil, nil, err
	}

	rds, closeRdsF, err := redis.Init(c)
	if err != nil {
		closeSQLF()
		return nil, nil, err
	}

	d := &Data{db: db, rds: rds}
	closeF := func() {
		closeSQLF()
		closeRdsF()
	}
	return d, closeF, nil
}

// DB 获取 GORM 数据库连接
func (d *Data) DB() *gorm.DB {
	return d.db
}

// Redis 获取 Redis 客户端
func (d *Data) Redis() *redis.Client {
	return d.rds
}
