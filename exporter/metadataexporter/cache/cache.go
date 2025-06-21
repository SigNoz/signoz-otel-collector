package metadataexporter

import (
	"context"

	"go.opentelemetry.io/collector/pipeline"
)

type KeyCache interface {
	// AddAttrsToResource adds attribute fingerprints for a specific resource fingerprint,
	// respecting the maximum cardinalities for resource count and attributes per resource.
	AddAttrsToResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) error

	// AttrsExistForResource checks which of the given attrFps exist for the given resourceFp.
	// Returns a parallel slice of booleans for each attrFp.
	AttrsExistForResource(ctx context.Context, resourceFp uint64, attrFps []uint64, ds pipeline.Signal) ([]bool, error)

	// Debug logs internal state (for debugging).
	Debug(ctx context.Context)

	// ResourcesLimitExceeded returns true if the resource limit has been exceeded.
	ResourcesLimitExceeded(ctx context.Context, ds pipeline.Signal) bool

	// CardinalityLimitExceeded returns true if the cardinality limit has been exceeded for the given resourceFp.
	CardinalityLimitExceeded(ctx context.Context, resourceFp uint64, ds pipeline.Signal) bool

	// CardinalityLimitExceededMulti returns true if the cardinality limit has been exceeded for given resourceFps.
	// Returns a parallel slice of booleans for each resourceFp.
	CardinalityLimitExceededMulti(ctx context.Context, resourceFps []uint64, ds pipeline.Signal) ([]bool, error)

	// TotalCardinalityLimitExceeded returns true if the total cardinality limit has been exceeded.
	TotalCardinalityLimitExceeded(ctx context.Context, ds pipeline.Signal) bool

	Close(ctx context.Context) error
}
