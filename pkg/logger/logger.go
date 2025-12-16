package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Fatal(msg string, fields ...interface{})
	With(fields ...interface{}) Logger
}

type zapLogger struct {
	logger *zap.SugaredLogger
}

type Config struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	AddCaller  bool   `mapstructure:"add_caller"`
	Stacktrace bool   `mapstructure:"stacktrace"`
}

func New(cfg Config) Logger {
	config := zap.NewProductionConfig()

	// Set log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(level)

	// Set output format
	if cfg.Format == "console" {
		config.Encoding = "console"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config.Encoding = "json"
	}

	// Set output
	if cfg.Output == "stdout" {
		config.OutputPaths = []string{"stdout"}
		config.ErrorOutputPaths = []string{"stderr"}
	} else {
		config.OutputPaths = []string{cfg.Output}
		config.ErrorOutputPaths = []string{cfg.Output}
	}

	// Add caller info
	if cfg.AddCaller {
		config.EncoderConfig.CallerKey = "caller"
		config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	}

	// Add stacktrace
	if cfg.Stacktrace {
		config.Development = true
	}

	// Build logger
	logger, err := config.Build()
	if err != nil {
		// Fallback to default logger
		logger = zap.NewExample()
	}

	return &zapLogger{
		logger: logger.Sugar(),
	}
}

func NewDefault() Logger {
	return New(Config{
		Level:     "info",
		Format:    "json",
		Output:    "stdout",
		AddCaller: true,
	})
}

func NewNop() Logger {
	return &zapLogger{
		logger: zap.NewNop().Sugar(),
	}
}

func (l *zapLogger) Debug(msg string, fields ...interface{}) {
	l.logger.Debugw(msg, fields...)
}

func (l *zapLogger) Info(msg string, fields ...interface{}) {
	l.logger.Infow(msg, fields...)
}

func (l *zapLogger) Warn(msg string, fields ...interface{}) {
	l.logger.Warnw(msg, fields...)
}

func (l *zapLogger) Error(msg string, fields ...interface{}) {
	l.logger.Errorw(msg, fields...)
}

func (l *zapLogger) Fatal(msg string, fields ...interface{}) {
	l.logger.Fatalw(msg, fields...)
	os.Exit(1)
}

func (l *zapLogger) With(fields ...interface{}) Logger {
	return &zapLogger{
		logger: l.logger.With(fields...),
	}
}

// Helper functions for structured logging
func Field(key string, value interface{}) interface{} {
	return []interface{}{key, value}
}

func Fields(fields map[string]interface{}) []interface{} {
	result := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		result = append(result, k, v)
	}
	return result
}
