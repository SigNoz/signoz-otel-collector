package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/config"
	schemamigrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
	"github.com/SigNoz/signoz-otel-collector/constants"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type check struct {
	conn             clickhouse.Conn
	timeout          time.Duration
	migrationManager *schemamigrator.MigrationManager
	logger           *zap.Logger
}

func registerSyncCheck(parentCmd *cobra.Command, logger *zap.Logger) {
	syncCheckCommand := &cobra.Command{
		Use:          "check",
		Short:        "Checks the status of migrations for the store by checking the status of migrations in the migration table.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check, err := newSyncCheck(config.Clickhouse.DSN, config.Clickhouse.Cluster, config.Clickhouse.Replication, config.MigrateSyncCheck.Timeout, logger)
			if err != nil {
				return err
			}

			err = check.Run(cmd.Context())
			if err != nil {
				return err
			}

			return nil
		},
	}

	config.MigrateSyncCheck.RegisterFlags(syncCheckCommand)

	parentCmd.AddCommand(syncCheckCommand)
}

func newSyncCheck(dsn string, cluster string, replication bool, timeout time.Duration, logger *zap.Logger) (*check, error) {
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

	return &check{
		conn:             conn,
		timeout:          timeout,
		migrationManager: migrationManager,
		logger:           logger,
	}, nil
}

func (cmd *check) Run(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = cmd.timeout

	for {
		err := cmd.Check(ctx)
		if err == nil {
			break
		}

		cmd.logger.Info("Error occurred while checking for sync migrations to complete, retrying", zap.Error(err))
		nextBackOff := backoff.NextBackOff()
		if nextBackOff == backoff.Stop {
			return errors.New("timed out waiting for sync migrations to complete within the configured timeout")
		}
		time.Sleep(nextBackOff)
	}

	return nil
}

func (cmd *check) Check(ctx context.Context) error {
	tracesLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.TracesMigrations)
	if err != nil {
		return err
	}

	err = cmd.isMigrationSuccess(ctx, schemamigrator.SignozTracesDB, tracesLastSyncMigrationID)
	if err != nil {
		return err
	}

	logsMigrations := schemamigrator.LogsMigrations
	if constants.EnableLogsMigrationsV2 {
		logsMigrations = schemamigrator.LogsMigrationsV2
	}

	logsLastSyncMigrationID, err := cmd.getLastSyncMigration(logsMigrations)
	if err != nil {
		return err
	}

	err = cmd.isMigrationSuccess(ctx, schemamigrator.SignozLogsDB, logsLastSyncMigrationID)
	if err != nil {
		return err
	}

	metricsLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.MetricsMigrations)
	if err != nil {
		return err
	}

	err = cmd.isMigrationSuccess(ctx, schemamigrator.SignozMetricsDB, metricsLastSyncMigrationID)
	if err != nil {
		return err
	}

	metadataLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.MetadataMigrations)
	if err != nil {
		return err
	}

	err = cmd.isMigrationSuccess(ctx, schemamigrator.SignozMetadataDB, metadataLastSyncMigrationID)
	if err != nil {
		return err
	}

	analyticsLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.AnalyticsMigrations)
	if err != nil {
		return err
	}

	err = cmd.isMigrationSuccess(ctx, schemamigrator.SignozAnalyticsDB, analyticsLastSyncMigrationID)
	if err != nil {
		return err
	}

	meterLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.MeterMigrations)
	if err != nil {
		return err
	}

	err = cmd.isMigrationSuccess(ctx, schemamigrator.SignozMeterDB, meterLastSyncMigrationID)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *check) isMigrationSuccess(ctx context.Context, db string, migrationID uint64) error {
	cmd.logger.Info("Checking migration completion", zap.String("database", db), zap.Uint64("migration_id", migrationID))
	query := fmt.Sprintf("SELECT * FROM %s.schema_migrations_v2 WHERE migration_id = %d SETTINGS final = 1;", db, migrationID)

	var migrationRecord schemamigrator.MigrationSchemaMigrationRecord
	if err := cmd.conn.QueryRow(ctx, query).ScanStruct(&migrationRecord); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("migration with ID %d for database '%s' has not run yet", migrationID, db)
		}

		return err
	}

	if migrationRecord.Status == schemamigrator.FinishedStatus {
		return nil
	}

	return fmt.Errorf("migration with ID %d for database '%s' has not been completed (current status: %s)", migrationID, db, migrationRecord.Status)
}

func (cmd *check) getLastSyncMigration(migrations []schemamigrator.SchemaMigrationRecord) (uint64, error) {
	for i := len(migrations) - 1; i >= 0; i-- {
		if cmd.migrationManager.IsSync(migrations[i]) {
			return migrations[i].MigrationID, nil
		}
	}

	return 0, fmt.Errorf("no sync migration found")
}
