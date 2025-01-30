package ch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLogComment(t *testing.T) {
	ctx := context.WithValue(context.Background(), LogCommentKey, map[string]interface{}{
		"exporter": "clickhouse_traces_exporter",
		"count":    10,
	})
	c := clickhouseConnWrapper{
		conn: nil,
	}
	assert.Equal(t, "{\"count\":10,\"exporter\":\"clickhouse_traces_exporter\"}", c.getLogComment(ctx))
}
