package schemamigrator

// Operation is the interface that all operations must implement.
// An Operation that is not mutation, idempotent and lightweight is expected
// to complete almost immediately given there are no blocking items in the
// distributed_ddl_queue.
// Such operations are completed synchronously and allow the release upgrade
// to proceed.
// All other operations are run asynchronously in the background and do not
// block the release upgrade.
type Operation interface {
	// ToSQL returns the SQL for the alter operation
	ToSQL() string
	// IsMutation returns true if the operation is a mutation
	IsMutation() bool
	// IsIdempotent returns true if the operation is idempotent
	// This is used to determine if the operation can be retried in case of a
	// failure.
	IsIdempotent() bool
	// IsLightweight returns true if the operation is lightweight
	// The lightweight operations are the ones that either modify the metadata or
	// drop the delete from disk as opposed to the ones that re-write the whole
	// data parts.
	IsLightweight() bool

	// OnCluster returns a new operation with the cluster name set
	// This is used when the operation is run on a specific cluster
	OnCluster(string) Operation

	// WithReplication returns a new operation with the replication set
	WithReplication() Operation

	// ShouldWaitForDistributionQueue returns true if the operation should wait for the distribution queue to be empty
	ShouldWaitForDistributionQueue() (bool, string, string)

	// ForceMigrate returns true if the operation should run in sync mode
	// the operation will run synchronously regardless of IsIdempotent and IsLightweight and IsMutation attributes
	// If false, the operation will follow the default sync/async logic based on IsMutation, IsIdempotent, and IsLightweight values
	ForceMigrate() bool
}
