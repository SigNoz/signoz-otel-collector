package basemigrator

import (
	"fmt"

	"go.uber.org/zap"
)

type zapLoggerAdapter struct {
	*zap.Logger
}

func newZapLoggerAdapter(logger *zap.Logger) *zapLoggerAdapter {
	return &zapLoggerAdapter{
		Logger:                logger,
	}
}

func (l *zapLoggerAdapter) Printf(format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

func (l *zapLoggerAdapter) Verbose() bool {
	return true
}
