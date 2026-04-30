// Package data
package data

import (
	"seas/internal/conf"
	gormsql "seas/pkg/gorm"

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
)

// Data 是对所有数据库资源的统一封装
type Data struct {
	db *gorm.DB
}

// NewData 创建 Data 并注入所有 repo 所需依赖
func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	db, closeSQLF, err := gormsql.Init(logger, c.Database.Source)
	if err != nil {
		return nil, nil, err
	}
	d := &Data{db: db}
	closeF := func() {
		closeSQLF()
	}
	return d, closeF, nil
}
