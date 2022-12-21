package opamp

import "context"

type Client interface {
	Start(ctx context.Context) error

	Stop(ctx context.Context) error
}
