package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/shirenchuang/bilibili-mcp/pkg/config"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

// Init 初始化日志系统
func Init(cfg *config.Config) error {
	log = logrus.New()

	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// 设置日志格式
	if cfg.Logging.Format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// 设置输出
	if cfg.Logging.Output != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.Logging.Output)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// 打开日志文件
		file, err := os.OpenFile(cfg.Logging.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		// 同时输出到文件和控制台
		log.SetOutput(io.MultiWriter(os.Stdout, file))
	}

	return nil
}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	if log == nil {
		log = logrus.New()
	}
	return log
}

// Info 记录信息日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 格式化记录信息日志
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Error 记录错误日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 格式化记录错误日志
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Debug 记录调试日志
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf 格式化记录调试日志
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Warn 记录警告日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 格式化记录警告日志
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}
