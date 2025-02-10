package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultMaxSize    = 100 // MB
	defaultMaxBackups = 5
	defaultMaxAge     = 30 // days
)

var (
	// 单例实例
	instance *zap.Logger
	once     sync.Once
	mu       sync.RWMutex
)

// Options 配置日志选项
type Options struct {
	// Level 日志级别
	Level string
	// Filename 日志文件路径
	Filename string
	// MaxSize 每个日志文件的最大大小(MB)
	MaxSize int
	// MaxBackups 保留的旧日志文件的最大数量
	MaxBackups int
	// MaxAge 保留旧日志文件的最大天数
	MaxAge int
	// Console 是否输出到控制台
	Console bool
}

// Init 初始化全局日志实例
func Init(opts Options) error {
	var err error
	once.Do(func() {
		instance, err = newLogger(opts)
	})
	return err
}

// 创建新的日志记录器
func newLogger(opts Options) (*zap.Logger, error) {
	if err := os.MkdirAll(filepath.Dir(opts.Filename), 0755); err != nil {
		return nil, fmt.Errorf("create log directory failed: %w", err)
	}

	level := zap.InfoLevel
	if opts.Level != "" {
		if err := level.UnmarshalText([]byte(opts.Level)); err != nil {
			return nil, fmt.Errorf("parse log level failed: %w", err)
		}
	}

	if opts.MaxSize == 0 {
		opts.MaxSize = defaultMaxSize
	}
	if opts.MaxBackups == 0 {
		opts.MaxBackups = defaultMaxBackups
	}
	if opts.MaxAge == 0 {
		opts.MaxAge = defaultMaxAge
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	writer := &lumberjack.Logger{
		Filename:   opts.Filename,
		MaxSize:    opts.MaxSize,
		MaxBackups: opts.MaxBackups,
		MaxAge:     opts.MaxAge,
		LocalTime:  true,
		Compress:   true,
	}

	var cores []zapcore.Core

	cores = append(cores, zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(writer),
		level,
	))

	if opts.Console {
		cores = append(cores, zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			level,
		))
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return logger, nil
}

// Default 获取默认日志实例
func Default() *zap.Logger {
	mu.RLock()
	if instance != nil {
		defer mu.RUnlock()
		return instance
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if instance == nil {
		_ = Init(Options{
			Level:      "info",
			Filename:   "app.log",
			MaxSize:    defaultMaxSize,
			MaxBackups: defaultMaxBackups,
			MaxAge:     defaultMaxAge,
			Console:    true,
		})
	}
	return instance
}

// Debug 输出Debug级别日志
func Debug(msg string, fields ...zap.Field) {
	Default().Debug(msg, fields...)
}

// Info 输出Info级别日志
func Info(msg string, fields ...zap.Field) {
	Default().Info(msg, fields...)
}

// Warn 输出Warn级别日志
func Warn(msg string, fields ...zap.Field) {
	Default().Warn(msg, fields...)
}

// Error 输出Error级别日志
func Error(msg string, fields ...zap.Field) {
	Default().Error(msg, fields...)
}

// Fatal 输出Fatal级别日志
func Fatal(msg string, fields ...zap.Field) {
	Default().Fatal(msg, fields...)
}

// WithContext 为日志添加上下文信息
func WithContext(fields ...zap.Field) *zap.Logger {
	return Default().With(fields...)
}

// Sync 同步日志缓冲区到磁盘
func Sync() error {
	if instance != nil {
		return instance.Sync()
	}
	return nil
}
