package zaplog

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitLogger() *Logger {
	fileWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "../../logs/app.log", // 日志文件路径
		MaxSize:    100,                  // 每个日志文件最大尺寸 (MB)
		MaxBackups: 10,                   // 保留的旧文件最大个数
		MaxAge:     30,                   // 保留的最大天数
		Compress:   true,                 // 是否压缩/归档
	})
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder(), fileWriteSyncer, zapcore.DebugLevel),
		zapcore.NewCore(consoleEncoder(), zapcore.Lock(os.Stdout), zapcore.DebugLevel),
	)
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(3),
		zap.AddStacktrace(zap.WarnLevel),
	)
	return NewLogger(logger)
}

// consoleEncoder 设置日志存储格式
func consoleEncoder() zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder, // 终端输出的关键词高亮
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	// 本地设置内置的 Console 解码器（支持 stacktrace 换行）
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// fileEncoder 设置日志存储格式
func fileEncoder() zapcore.Encoder {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	return zapcore.NewJSONEncoder(encoderConfig)
}
