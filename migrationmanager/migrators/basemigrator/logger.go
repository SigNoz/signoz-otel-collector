package basemigrator

import (
	"fmt"

	"go.uber.org/zap"
)

type zapLoggerAdapter struct {
	*zap.Logger
	verboseLoggingEnabled bool
}

func newZapLoggerAdapter(logger *zap.Logger, verboseLoggingEnabled bool) *zapLoggerAdapter {
	return &zapLoggerAdapter{
		Logger:                logger,
		verboseLoggingEnabled: verboseLoggingEnabled,
	}
}

func (l *zapLoggerAdapter) Printf(format string, v ...interface{}) {
	l.Logger.Info(fmt.Sprintf(format, v...))
}

func (l *zapLoggerAdapter) Verbose() bool {
	return l.verboseLoggingEnabled
}
