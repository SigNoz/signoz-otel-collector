package opamp

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/collector"
)

type simpleClient struct {
	coll collector.WrappedCollector
}

func NewSimpleClient(coll collector.WrappedCollector) *simpleClient {
	return &simpleClient{coll: coll}
}

func (c simpleClient) Start(ctx context.Context) error {
	return c.coll.Run(ctx)
}

func (c simpleClient) Stop(ctx context.Context) error {
	c.coll.Shutdown()
	return nil
}
