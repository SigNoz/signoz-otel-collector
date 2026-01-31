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

type asyncCheck struct {
	conn             clickhouse.Conn
	timeout          time.Duration
	migrationManager *schemamigrator.MigrationManager
	logger           *zap.Logger
}

func registerAsyncCheck(parentCmd *cobra.Command, logger *zap.Logger) {
	syncCheckCommand := &cobra.Command{
		Use:          "check",
		Short:        "Checks the status of async migrations for the store by checking the status of async migrations in the migration table.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check, err := newAsyncCheck(config.Clickhouse.DSN, config.Clickhouse.Cluster, config.Clickhouse.Replication, config.MigrateSyncCheck.Timeout, logger)
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

	config.MigrateAsyncCheck.RegisterFlags(syncCheckCommand)

	parentCmd.AddCommand(syncCheckCommand)
}

func newAsyncCheck(dsn string, cluster string, replication bool, timeout time.Duration, logger *zap.Logger) (*asyncCheck, error) {
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

	return &asyncCheck{
		conn:             conn,
		timeout:          timeout,
		migrationManager: migrationManager,
		logger:           logger,
	}, nil
}

func (cmd *asyncCheck) Run(ctx context.Context) error {
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

func (cmd *asyncCheck) Check(ctx context.Context) error {
	tracesLastMigrationID, err := cmd.getLastAsyncMigration(schemamigrator.TracesMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozTracesDB, tracesLastMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", tracesLastMigrationID, schemamigrator.SignozTracesDB)
		}
	}

	logsMigrations := schemamigrator.LogsMigrations
	if constants.EnableLogsMigrationsV2 {
		logsMigrations = schemamigrator.LogsMigrationsV2
	}

	logsLastMigrationID, err := cmd.getLastAsyncMigration(logsMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozLogsDB, logsLastMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", logsLastMigrationID, schemamigrator.SignozLogsDB)
		}
	}

	metricsLastMigrationID, err := cmd.getLastAsyncMigration(schemamigrator.MetricsMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozMetricsDB, metricsLastMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", metricsLastMigrationID, schemamigrator.SignozMetricsDB)
		}
	}

	metadataLastMigrationID, err := cmd.getLastAsyncMigration(schemamigrator.MetadataMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozMetadataDB, metadataLastMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", metadataLastMigrationID, schemamigrator.SignozMetadataDB)
		}
		return err
	}

	analyticsLastMigrationID, err := cmd.getLastAsyncMigration(schemamigrator.AnalyticsMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozAnalyticsDB, analyticsLastMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", analyticsLastMigrationID, schemamigrator.SignozAnalyticsDB)
		}
		return err
	}

	meterLastMigrationID, err := cmd.getLastAsyncMigration(schemamigrator.MeterMigrations)
	if err == nil {
		ok, err := cmd.migrationManager.CheckMigrationStatus(ctx, schemamigrator.SignozMeterDB, meterLastMigrationID, schemamigrator.FinishedStatus)
		if err != nil {
			return err
		}

		if !ok {
			return fmt.Errorf("migration with ID %d for database '%s' has not been completed", meterLastMigrationID, schemamigrator.SignozMeterDB)
		}
		return err
	}

	return nil
}

func (cmd *asyncCheck) getLastAsyncMigration(migrations []schemamigrator.SchemaMigrationRecord) (uint64, error) {
	for i := len(migrations) - 1; i >= 0; i-- {
		if cmd.migrationManager.IsAsync(migrations[i]) {
			return migrations[i].MigrationID, nil
		}
	}

	return 0, fmt.Errorf("no async migrations found")
}
