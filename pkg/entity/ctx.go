package entity

import (
	"context"
)

// All context keys for entities should be defined here and should be unexported
type keyIdContextKey struct{}

// Put the keyId into context to be used by subsequent processors or exporters
func CtxWithKeyId(ctx context.Context, keyId Id) context.Context {
	return context.WithValue(ctx, keyIdContextKey{}, keyId)
}

func KeyIdFromCtx(ctx context.Context) (Id, bool) {
	value, ok := ctx.Value(keyIdContextKey{}).(Id)
	return value, ok
}
