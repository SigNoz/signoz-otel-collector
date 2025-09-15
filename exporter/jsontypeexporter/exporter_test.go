package jsontypeexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestPushLogs(t *testing.T) {
	exp := &jsonTypeExporter{
		config: &Config{OutputPath: "./test.json"},
		logger: zap.NewNop(),
	}

	ld := plog.NewLogs()
	err := exp.pushLogs(context.Background(), ld)
	assert.NoError(t, err)
}
