package sync

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
	conn    clickhouse.Conn
	timeout time.Duration
	logger  *zap.Logger
}

func RegisterCheck(parentCmd *cobra.Command, logger *zap.Logger) {
	syncCheckCommand := &cobra.Command{
		Use:          "check",
		Short:        "Checks the status of migrations for the store by checking the status of migrations in the migration table.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			check, err := newCheck(config.Clickhouse.DSN, config.MigrateSyncCheck.Timeout, logger)
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

func newCheck(dsn string, timeout time.Duration, logger *zap.Logger) (*check, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	return &check{
		conn:    conn,
		timeout: timeout,
		logger:  logger,
	}, nil
}

func (c *check) Run(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = c.timeout

	for {
		err := c.Check(ctx)
		if err == nil {
			break
		}

		c.logger.Info("Waiting for sync migrations to complete...", zap.Error(err))
		nextBackOff := backoff.NextBackOff()
		if nextBackOff == backoff.Stop {
			return errors.New("timed out waiting for sync migrations to complete within the configured timeout")
		}
		time.Sleep(nextBackOff)
	}

	return nil
}

func (c *check) Check(ctx context.Context) error {
	err := c.isMigrationSuccess(ctx, schemamigrator.SignozTracesDB, schemamigrator.TracesMigrations[len(schemamigrator.TracesMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	logsMigrations := schemamigrator.LogsMigrations
	if constants.EnableLogsMigrationsV2 {
		logsMigrations = schemamigrator.LogsMigrationsV2
	}

	err = c.isMigrationSuccess(ctx, schemamigrator.SignozLogsDB, logsMigrations[len(logsMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.isMigrationSuccess(ctx, schemamigrator.SignozMetricsDB, schemamigrator.MetricsMigrations[len(schemamigrator.MetricsMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.isMigrationSuccess(ctx, schemamigrator.SignozMetadataDB, schemamigrator.MetadataMigrations[len(schemamigrator.MetadataMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.isMigrationSuccess(ctx, schemamigrator.SignozAnalyticsDB, schemamigrator.AnalyticsMigrations[len(schemamigrator.AnalyticsMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.isMigrationSuccess(ctx, schemamigrator.SignozMeterDB, schemamigrator.MeterMigrations[len(schemamigrator.MeterMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	return nil
}

func (c *check) isMigrationSuccess(ctx context.Context, db string, migrationID uint64) error {
	c.logger.Info("Checking migration completion", zap.String("database", db), zap.Uint64("migrationID", migrationID))
	query := fmt.Sprintf("SELECT * FROM %s.schema_migrations_v2 WHERE migration_id = %d SETTINGS final = 1;", db, migrationID)

	var migrationRecord schemamigrator.MigrationSchemaMigrationRecord
	if err := c.conn.QueryRow(ctx, query).ScanStruct(&migrationRecord); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		return err
	}

	if migrationRecord.Status != schemamigrator.InProgressStatus && migrationRecord.Status != schemamigrator.FinishedStatus {
		return fmt.Errorf("migration with ID %d for database '%s' has not been completed (current status: %s)", migrationID, db, migrationRecord.Status)
	}

	return nil
}
