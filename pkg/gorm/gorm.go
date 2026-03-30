package gorm

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

func Init(logger log.Logger, source string) (*gorm.DB, func(), error) {
	gormLogger := NewGormLogger(logger, gormlog.Info)
	db, err := gorm.Open(mysql.Open(source), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, nil, err
	}
	// 设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	closeF := func() {
		_ = sqlDB.Close()
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	err = sqlDB.PingContext(ctx)
	if err != nil {
		closeF()
		return nil, nil, err
	}
	return db, closeF, err
}

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
