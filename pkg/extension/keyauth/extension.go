package keyauth

import (
	"context"
	"errors"
	"net/http"

	"github.com/SigNoz/signoz-otel-collector/pkg/env"
	"github.com/SigNoz/signoz-otel-collector/pkg/storage"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/auth"
	"go.uber.org/zap"
)

var _ auth.Server = (*KeyAuth)(nil)

// KeyAuth is an implementation of auth.Server.
type KeyAuth struct {
	storage *storage.Storage
	headers []string
	logger  *zap.Logger
}

func newKeyAuth(cfg *Config, logger *zap.Logger) *KeyAuth {
	return &KeyAuth{
		headers: cfg.Headers,
		storage: env.G().Storage(),
		logger:  logger,
	}
}

// Start initializes the logger and the storage interface
func (auth *KeyAuth) Start(ctx context.Context, _ component.Host) error {
	return nil
}

// Shutdown does nothing
func (auth *KeyAuth) Shutdown(_ context.Context) error {
	return nil
}

// Authenticate checks whether the given context contains valid auth data.
func (auth *KeyAuth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	var name string
	var values []string

	for _, header := range auth.headers {
		// Go converts all headers to canonical headers
		auth, ok := headers[http.CanonicalHeaderKey(header)]
		if ok {
			name = header
			values = auth
			break
		}
	}

	if name == "" || len(values) == 0 {
		return ctx, errors.New("missing header or key")
	}

	authData, err := auth.storage.DAO.Auth().SelectByKeyValue(ctx, values[0])
	if err != nil {
		return nil, err
	}

	//Set authdata into context
	cl := client.FromContext(ctx)
	cl.Auth = authData
	return client.NewContext(ctx, cl), nil
}
