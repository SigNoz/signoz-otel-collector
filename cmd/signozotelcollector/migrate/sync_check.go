package migrate

import (
	"context"
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

type syncCheck struct {
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

func newSyncCheck(dsn string, cluster string, replication bool, timeout time.Duration, logger *zap.Logger) (*syncCheck, error) {
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

	return &syncCheck{
		conn:             conn,
		timeout:          timeout,
		migrationManager: migrationManager,
		logger:           logger,
	}, nil
}

func (cmd *syncCheck) Run(ctx context.Context) error {
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

func (cmd *syncCheck) Check(ctx context.Context) error {
	tracesLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.TracesMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozTracesDB, tracesLastSyncMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", tracesLastSyncMigrationID, schemamigrator.SignozTracesDB)
		}
	}

	logsMigrations := schemamigrator.LogsMigrations
	if constants.EnableLogsMigrationsV2 {
		logsMigrations = schemamigrator.LogsMigrationsV2
	}

	logsLastSyncMigrationID, err := cmd.getLastSyncMigration(logsMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozLogsDB, logsLastSyncMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", logsLastSyncMigrationID, schemamigrator.SignozLogsDB)
		}
	}

	metricsLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.MetricsMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozMetricsDB, metricsLastSyncMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", metricsLastSyncMigrationID, schemamigrator.SignozMetricsDB)
		}
	}

	metadataLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.MetadataMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozMetadataDB, metadataLastSyncMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", metadataLastSyncMigrationID, schemamigrator.SignozMetadataDB)
		}
	}

	analyticsLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.AnalyticsMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozAnalyticsDB, analyticsLastSyncMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", analyticsLastSyncMigrationID, schemamigrator.SignozAnalyticsDB)
		}
	}

	meterLastSyncMigrationID, err := cmd.getLastSyncMigration(schemamigrator.MeterMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozMeterDB, meterLastSyncMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", meterLastSyncMigrationID, schemamigrator.SignozMeterDB)
		}
	}

	return nil
}

func (cmd *syncCheck) getLastSyncMigration(migrations []schemamigrator.SchemaMigrationRecord) (uint64, error) {
	for i := len(migrations) - 1; i >= 0; i-- {
		if cmd.migrationManager.IsSync(migrations[i]) {
			return migrations[i].MigrationID, nil
		}
	}

	return 0, fmt.Errorf("no sync migration found")
}
