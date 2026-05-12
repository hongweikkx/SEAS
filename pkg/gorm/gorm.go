package gorm

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

// Init 初始化 SQLite 连接。在 gorm.Open 前先注册自定义聚合函数
// (STDDEV_POP / STDDEV_SAMP),让 data 层已有 raw SQL 在 SQLite 上可用。
// 通过 sqlite.Dialector{DriverName: SeasDriverName} 让 GORM 使用本服务
// 注册的自定义 driver(由 RegisterAggregates 设置)。
func Init(logger log.Logger, source string) (*gorm.DB, func(), error) {
	if err := RegisterAggregates(); err != nil {
		return nil, nil, err
	}

	gormLogger := NewGormLogger(logger, gormlog.Info)
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: SeasDriverName,
		DSN:        source,
	}, &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, nil, err
	}

	// SQLite 是单写者,连接池保持小;读不会被写阻塞(WAL 模式由 DSN PRAGMA 启用)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	closeF := func() {
		_ = sqlDB.Close()
	}
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(0) // 本地文件,无连接老化

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		closeF()
		return nil, nil, err
	}
	return db, closeF, nil
}

// 以下 GormLogger 实现不变,只是从原文件保留下来 ------------------------------

type GormLogger struct {
	logger *log.Helper
	level  gormlog.LogLevel
}

func NewGormLogger(logger log.Logger, level gormlog.LogLevel) gormlog.Interface {
	return &GormLogger{
		logger: log.NewHelper(logger),
		level:  level,
	}
}

func (l *GormLogger) LogMode(level gormlog.LogLevel) gormlog.Interface {
	return &GormLogger{
		logger: l.logger,
		level:  level,
	}
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlog.Info {
		l.logger.WithContext(ctx).Infow(append([]interface{}{"msg", msg}, data...)...)
	}
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlog.Warn {
		l.logger.WithContext(ctx).Warnw(append([]interface{}{"msg", msg}, data...)...)
	}
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.level >= gormlog.Error {
		l.logger.WithContext(ctx).Errorw(append([]interface{}{"msg", msg}, data...)...)
	}
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= 0 {
		return
	}
	elapsed := time.Since(begin)
	sqlStr, rows := fc()
	fields := []interface{}{
		"duration", elapsed,
		"rows", rows,
		"sql", sqlStr,
	}

	switch {
	case err != nil && l.level >= gormlog.Error:
		l.logger.WithContext(ctx).Errorw(append([]interface{}{"msg", "GORM Trace"}, append(fields, "error", err)...)...)
	case elapsed > 200*time.Millisecond && l.level >= gormlog.Warn:
		l.logger.WithContext(ctx).Warnw(append([]interface{}{"msg", "GORM Slow SQL"}, fields...)...)
	case l.level >= gormlog.Info:
		l.logger.WithContext(ctx).Infow(append([]interface{}{"msg", "GORM Info"}, fields...)...)
	}
}
