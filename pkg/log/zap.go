package log

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapLogger struct {
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

	return &ZapLogger{
		l: zap.Must(cfg.Build()).Sugar(),
	}
}

func (l *ZapLogger) Debugctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Debugw(msg, fields...)
}

func (l *ZapLogger) Infoctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Infow(msg, fields...)
}

func (l *ZapLogger) Warnctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Warnw(msg, fields...)
}

func (l *ZapLogger) Errorctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Errorw(msg, fields...)
}

func (l *ZapLogger) Panicctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Panicw(msg, fields...)
}

func (l *ZapLogger) Fatalctx(ctx context.Context, msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Fatalw(msg, fields...)
}

func (l *ZapLogger) Debug(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Debugw(msg, fields...)
}

func (l *ZapLogger) Info(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Infow(msg, fields...)
}

func (l *ZapLogger) Warn(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Warnw(msg, fields...)
}

func (l *ZapLogger) Error(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Errorw(msg, fields...)
}

func (l *ZapLogger) Panic(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Panicw(msg, fields...)
}

func (l *ZapLogger) Fatal(msg string, fields ...interface{}) {
	l.l.WithOptions(zap.AddCallerSkip(1)).Fatalw(msg, fields...)
}

func (l *ZapLogger) Flush() error {
	return l.l.Sync()
}

func (l *ZapLogger) setl(newl *zap.SugaredLogger) {
	l.l = newl
}

func (l *ZapLogger) Getl() *zap.SugaredLogger {
	return l.l
}
