package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/config"
	schemamigrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type bootstrap struct {
	conn             clickhouse.Conn
	cluster          string
	migrationManager *schemamigrator.MigrationManager
	timeout          time.Duration
	logger           *zap.Logger
}

func registerBootstrap(parentCmd *cobra.Command, logger *zap.Logger) {
	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Creates the necessary tables to track status of migrations. A migration table is typically created to track the status of migrations. This command creates the necessary tables to track the status of migrations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			bootstrap, err := newBootstrap(config.Clickhouse.DSN, config.Clickhouse.Cluster, config.Clickhouse.Replication, config.MigrateBootstrap.Timeout, logger)
			if err != nil {
				return err
			}

			if err := bootstrap.Run(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	config.MigrateBootstrap.RegisterFlags(cmd)
	parentCmd.AddCommand(cmd)
}

func newBootstrap(dsn string, cluster string, replication bool, timeout time.Duration, logger *zap.Logger) (*bootstrap, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	migrationManager, err := schemamigrator.NewMigrationManager(
		schemamigrator.WithClusterName(cluster),
		schemamigrator.WithReplicationEnabled(replication),
		schemamigrator.WithConn(conn),
		schemamigrator.WithConnOptions(*opts),
		schemamigrator.WithLogger(logger),
	)
	if err != nil {
		return nil, err
	}

	return &bootstrap{
		conn:             conn,
		cluster:          cluster,
		migrationManager: migrationManager,
		timeout:          timeout,
		logger:           logger,
	}, nil
}

func (cmd *bootstrap) Run(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = cmd.timeout

	for {
		err := cmd.Bootstrap(ctx)
		if err == nil {
			break
		}

		migrateErr := Unwrapb(err)
		// exit early for non-retryable errors.
		if !migrateErr.IsRetryable() {
			return fmt.Errorf("failed to bootstrap store for migrations: %w", err)
		}

		cmd.logger.Info("retrying store bootstrap for migrations", zap.Error(err))
		nextBackOff := backoff.NextBackOff()
		if nextBackOff == backoff.Stop {
			return fmt.Errorf("failed to bootstrap store for migrations within the configured timeout of %s", cmd.timeout)
		}
		time.Sleep(nextBackOff)
	}

	return nil
}

func (cmd *bootstrap) Bootstrap(ctx context.Context) error {
	err := cmd.CreateDatabases(ctx)
	if err != nil {
		return err
	}

	err = cmd.CreateMigrationTables(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *bootstrap) CreateDatabases(ctx context.Context) error {
	cmd.logger.Info("Creating databases")

	query := `SELECT name FROM system.databases`
	rows, err := cmd.conn.Query(ctx, query)
	if err != nil {
		return NewRetryableError(err)
	}

	databases := map[string]struct{}{}
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return err
		}

		databases[database] = struct{}{}
	}

	for _, database := range schemamigrator.Databases {
		if _, ok := databases[database]; !ok {
			command := fmt.Sprintf("CREATE DATABASE %s ON CLUSTER %s", database, cmd.cluster)
			if err := cmd.conn.Exec(ctx, command); err != nil {
				return err
			}
		}
	}

	return nil
}

func (cmd *bootstrap) CreateMigrationTables(ctx context.Context) error {
	cmd.logger.Info("Creating migration tables")

	for _, migration := range schemamigrator.V2MigrationTablesLogs {
		for _, operation := range migration.UpItems {
			shouldRun, err := cmd.ShouldRunOperation(ctx, operation)
			if err != nil {
				return err
			}

			if !shouldRun {
				continue
			}

			err = cmd.migrationManager.RunOperation(ctx, operation, migration.MigrationID, schemamigrator.SignozLogsDB, true)
			if err != nil {
				return err
			}
		}
	}

	for _, migration := range schemamigrator.V2MigrationTablesMetrics {
		for _, operation := range migration.UpItems {
			shouldRun, err := cmd.ShouldRunOperation(ctx, operation)
			if err != nil {
				return err
			}

			if !shouldRun {
				continue
			}

			err = cmd.migrationManager.RunOperation(ctx, operation, migration.MigrationID, schemamigrator.SignozMetricsDB, true)
			if err != nil {
				return err
			}
		}
	}

	for _, migration := range schemamigrator.V2MigrationTablesTraces {
		for _, operation := range migration.UpItems {
			shouldRun, err := cmd.ShouldRunOperation(ctx, operation)
			if err != nil {
				return err
			}

			if !shouldRun {
				continue
			}

			err = cmd.migrationManager.RunOperation(ctx, operation, migration.MigrationID, schemamigrator.SignozMetricsDB, true)
			if err != nil {
				return err
			}
		}
	}

	for _, migration := range schemamigrator.V2MigrationTablesMetadata {
		for _, operation := range migration.UpItems {
			shouldRun, err := cmd.ShouldRunOperation(ctx, operation)
			if err != nil {
				return err
			}

			if !shouldRun {
				continue
			}

			err = cmd.migrationManager.RunOperation(ctx, operation, migration.MigrationID, schemamigrator.SignozMetricsDB, true)
			if err != nil {
				return err
			}
		}
	}

	for _, migration := range schemamigrator.V2MigrationTablesAnalytics {
		for _, operation := range migration.UpItems {
			shouldRun, err := cmd.ShouldRunOperation(ctx, operation)
			if err != nil {
				return err
			}

			if !shouldRun {
				continue
			}

			err = cmd.migrationManager.RunOperation(ctx, operation, migration.MigrationID, schemamigrator.SignozMetricsDB, true)
			if err != nil {
				return err
			}
		}
	}

	for _, migration := range schemamigrator.V2MigrationTablesMeter {
		for _, operation := range migration.UpItems {
			shouldRun, err := cmd.ShouldRunOperation(ctx, operation)
			if err != nil {
				return err
			}

			if !shouldRun {
				continue
			}

			err = cmd.migrationManager.RunOperation(ctx, operation, migration.MigrationID, schemamigrator.SignozMetricsDB, true)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cmd *bootstrap) ShouldRunOperation(ctx context.Context, operation schemamigrator.Operation) (bool, error) {
	tableOp, ok := operation.(schemamigrator.CreateTableOperation)
	if ok {
		query := fmt.Sprintf(`SELECT count() FROM system.tables WHERE database = '%s' AND name = '%s'`, tableOp.Database, tableOp.Table)

		cmd.logger.Info("Checking if the operation should run", zap.String("sql", query))
		var count uint64
		err := cmd.conn.QueryRow(ctx, query).Scan(&count)
		if err != nil {
			return false, NewRetryableError(err)
		}

		if count > 0 {
			return false, nil
		}
	}

	return true, nil
}
