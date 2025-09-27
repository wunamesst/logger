package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/local-log-viewer/internal/config"
)

var (
	// Global logger instance
	globalLogger *zap.Logger
	sugar        *zap.SugaredLogger
)

// Initialize 初始化日志系统
func Initialize(logConfig config.LogConfig) error {
	// 解析日志级别
	level, err := parseLogLevel(logConfig.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	// 配置编码器
	var encoderConfig zapcore.EncoderConfig
	var encoder zapcore.Encoder

	if strings.ToLower(logConfig.Format) == "json" {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 配置输出
	var writeSyncer zapcore.WriteSyncer
	switch strings.ToLower(logConfig.OutputPath) {
	case "", "stdout":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		// 文件输出
		file, err := os.OpenFile(logConfig.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		writeSyncer = zapcore.AddSync(file)
	}

	// 创建核心
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// 创建logger
	globalLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	sugar = globalLogger.Sugar()

	return nil
}

// parseLogLevel 解析日志级别
func parseLogLevel(levelStr string) (zapcore.Level, error) {
	switch strings.ToLower(levelStr) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unknown log level: %s", levelStr)
	}
}

// GetLogger 获取结构化logger
func GetLogger() *zap.Logger {
	if globalLogger == nil {
		// 如果没有初始化，使用默认配置
		config := zap.NewDevelopmentConfig()
		globalLogger, _ = config.Build()
		sugar = globalLogger.Sugar()
	}
	return globalLogger
}

// GetSugar 获取sugar logger (更简单的API)
func GetSugar() *zap.SugaredLogger {
	if sugar == nil {
		GetLogger() // 这会初始化sugar
	}
	return sugar
}

// Sync 同步日志缓冲区
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// 便捷方法
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// Sugar便捷方法
func Debugf(template string, args ...interface{}) {
	GetSugar().Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	GetSugar().Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	GetSugar().Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	GetSugar().Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	GetSugar().Fatalf(template, args...)
}
