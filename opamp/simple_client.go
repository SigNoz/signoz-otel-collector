package opamp

import (
	"context"

	"github.com/SigNoz/signoz-otel-collector/signozcol"
)

type simpleClient struct {
	coll *signozcol.WrappedCollector
}

func NewSimpleClient(coll *signozcol.WrappedCollector) *simpleClient {
	return &simpleClient{coll: coll}
}

func (c simpleClient) Start(ctx context.Context) error {
	return c.coll.Run(ctx)
}

func (c simpleClient) Stop(ctx context.Context) error {
	c.coll.Shutdown()
	return c.Error()
}

func (c simpleClient) Error() error {
	var err error
	select {
	case err = <-c.coll.ErrorChan():
	default:
	}
	return err
}
