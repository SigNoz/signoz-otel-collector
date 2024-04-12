package log

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type zapLogger struct {
	l *zap.SugaredLogger
}

func NewZapLogger(level string) Logger {
	// Get atomic level from string level
	parsedLevel, err := zap.ParseAtomicLevel(level)
	if err != nil {
		panic(err)
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = parsedLevel
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.TimeKey = "timestamp"

	return &zapLogger{
		l: zap.Must(cfg.Build()).Sugar(),
	}
}

func (l *zapLogger) Debugctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Debugw(msg, fields...)
}

func (l *zapLogger) Infoctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Infow(msg, fields...)
}

func (l *zapLogger) Warnctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Warnw(msg, fields...)
}

func (l *zapLogger) Errorctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Errorw(msg, fields...)
}

func (l *zapLogger) Panicctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Panicw(msg, fields...)
}

func (l *zapLogger) Fatalctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Fatalw(msg, fields...)
}

func (l *zapLogger) Debug(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Debugw(msg, fields...)
}

func (l *zapLogger) Info(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Infow(msg, fields...)
}

func (l *zapLogger) Warn(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Warnw(msg, fields...)
}

func (l *zapLogger) Error(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Errorw(msg, fields...)
}

func (l *zapLogger) Panic(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Panicw(msg, fields...)
}

func (l *zapLogger) Fatal(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Fatalw(msg, fields...)
}

func (l *zapLogger) Flush() error {
	return l.l.Sync()
}

func (l *zapLogger) setl(newl *zap.SugaredLogger) {
	l.l = newl
}
