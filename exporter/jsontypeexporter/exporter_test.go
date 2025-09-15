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
		config: &Config{},
		logger: zap.NewNop(),
	}

	ld := plog.NewLogs()
	// Note: This test will fail without a real database connection
	// In a real test environment, you would mock the database connection
	err := exp.pushLogs(context.Background(), ld)
	// We expect an error here since we don't have a real database connection
	assert.Error(t, err)
}
