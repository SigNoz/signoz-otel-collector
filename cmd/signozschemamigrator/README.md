# Signoz Schema Migrator

This is a tool to manage the ClickHouse schema migrations.

## Why we wrote this?

We initially adopted https://github.com/golang-migrate/migrate to manage the ClickHouse schema migrations. However, we faced the following issues:

1. No support for Clickhouse cluster mode.
2. Schema migrations that trigger the mutations on the tables would cause the migrations to fail.
3. Race condition when running the migrations in parallel from multiple collector instances.

### Mutations in Clickhouse

From the Clickhouse docs https://clickhouse.com/docs/optimize/avoid-mutations:

>Mutations refers to ALTER queries that manipulate table data through deletion or updates. Most notably they are queries like ALTER TABLE … DELETE, UPDATE, etc. Performing such queries will produce new mutated versions of the data parts. This means that such statements would trigger a rewrite of whole data parts for all data that was inserted before the mutation, translating to a large amount of write requests.

Read more about the mutations from ALTER commands here https://clickhouse.com/docs/sql-reference/statements/alter#mutations

When a mutation is performed, it triggers a rewrite of the data parts. This means that the data parts are rewritten to disk and the old data parts are deleted. This is a resource intensive operation and should be avoided. However, there are some cases where mutations are necessary and we need to run them. When they are run, there is no way to know when the mutation is going to complete. In our migration workflow, this created a problematic sequence.

1. Run the migrations
2. One of the migrations is implemented as a mutation on the table.
3. It triggers a rewrite of the data parts.
4. golang-migrate library awaits query completion.
5. CH either completes it in 300 seconds or times out (moves the work to background mode).
6. When the mutation is moved to background mode, the migration fails with a timeout error.
7. golang-migrate library does not handle this and fails the migration.
8. Subsequent migrations fail with "dirty database" error.

Such failures could leave the DB in a inconsistent state and lead to ingestion failures. If a migration attempted both a mutation and a new column addition, and the migration failed at the mutation step, the DB would be left in a state where the mutation is running in the background but the new columns are not added. Meanwhile, updated collectors expecting the new columns would fail to ingest data.

### Distributed DDL Complications

Every schema migration using ON CLUSTER creates entries in the `system.distributed_ddl_queue` table. New DDL operations cannot proceed while the existing DDL operation is in pending state. The DDL entry corresponding to the mutation would be in pending state till the mutation is completed. This means that the other migrations have to wait for the mutation to complete. Since the migrations are written using .sql files, it's not possible to know when the DDL operation is complete. The golang-migrate library would run the migration as multi-statement DDL operation, queuing up the DDL operations and waiting for them to complete. This would fail with a timeout error.

### Materialization Migrations

When we run materialization migrations on the tables, intra-shard insert would fail because the insert is performed on the old table schema. See more here https://github.com/SigNoz/signoz/issues/4566.


These challenges necessitated a more mutation-aware migration approach with better detection and handling capabilities for ClickHouse's specific behavior.

## What does this tool do?

Every operation implments the following interface:

```go
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
}
```

The migrator divides the operation into 2 phases based on the type of the operation:

1. Synchronous operations: These operations are expected to complete quickly. They are executed in the foreground and block the migration.
2. Asynchronous operations: These operations are expected to complete in the background. They are executed in the background and do not block the migration.

The migrator first runs the synchronous operations and then the asynchronous operations. The upgrade would wait for the synchronous operations to complete before proceeding to the next step. This would ensure that the upgrade does not proceed until the synchronous operations are complete and the DB is in a consistent state.

## Adding a new operation

To add a new operation, you need to implement the `Operation` interface. You need to make sure that the operation returns appropriate values for the `IsMutation`, `IsIdempotent`, `IsLightweight` and `ShouldWaitForDistributionQueue` methods.


## Adding a new migration

To add a new migration, you need to find the migration file for the data source you want to migrate. The migration files are named as `{datasource}_migrations.go`. Find the last entry in the migrations array and add the new migration after that. Browse the existing migrations to understand the pattern.


## Usage

Migrations run through the `migrate` command of `signoz-otel-collector`.

### Commands

- `migrate ready`: checks if the store is ready to run migrations.
- `migrate bootstrap`: creates the migration tables if they do not exist.
- `migrate sync up`: runs sync migrations; these block the upgrade until complete.
- `migrate sync check`: checks that all sync migrations have finished.
- `migrate async up`: runs async migrations; these do not block the upgrade.
- `migrate async check`: checks that all async migrations have finished.

### Flags

- `--clickhouse-dsn`: ClickHouse connection string, e.g. `tcp://localhost:9000`.
- `--clickhouse-cluster`: cluster name (default `cluster`).
- `--clickhouse-replication`: whether replication is enabled (default `true`).
- `--timeout`: per-command timeout.

### Running the migrator

```bash
go run ./cmd/signozotelcollector migrate bootstrap --clickhouse-dsn="tcp://localhost:9000"
go run ./cmd/signozotelcollector migrate sync up --clickhouse-dsn="tcp://localhost:9000"
go run ./cmd/signozotelcollector migrate async up --clickhouse-dsn="tcp://localhost:9000"
```

For a local single-node ClickHouse, run `make migrator`.
