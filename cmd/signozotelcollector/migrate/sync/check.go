package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/migrate/config"
	schemamigrator "github.com/SigNoz/signoz-otel-collector/cmd/signozschemamigrator/schema_migrator"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type check struct {
	conn    clickhouse.Conn
	timeout time.Duration
	logger  *zap.Logger
}

func registerCheck(parentCmd *cobra.Command, logger *zap.Logger) {
	syncCheckCommand := &cobra.Command{
		Use:   "check",
		Short: "Checks the status of migrations for the store by checking the status of migrations in the migration table.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("running migrate sync check command")
			check, err := newCheck(config.Clickhouse.DSN, config.MigrateSyncCheck.Timeout, logger)
			if err != nil {
				return err
			}

			err = check.Run()
			if err != nil {
				return err
			}

			return nil
		},
	}

	config.MigrateSyncCheck.RegisterFlags(syncCheckCommand)
	parentCmd.AddCommand(syncCheckCommand)
}

func newCheck(dsn string, timeout string, logger *zap.Logger) (*check, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dsn: %w", err)
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, err
	}

	return &check{
		conn:    conn,
		timeout: timeoutDuration,
		logger:  logger,
	}, nil
}

func (c *check) Run() error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = c.timeout

	for {
		err := c.Check()
		if err == nil {
			break
		}
		c.logger.Info("sync migrations still in progress", zap.Error(err))

		if backoff.NextBackOff() == backoff.Stop {
			return fmt.Errorf("sync migration check timed out after %s: %w", c.timeout, err)
		}
		time.Sleep(backoff.NextBackOff())
	}

	return nil
}

func (c *check) Check() error {
	err := c.shouldRunMigration(c.conn, schemamigrator.SignozTracesDB, schemamigrator.TracesMigrations[len(schemamigrator.TracesMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.shouldRunMigration(c.conn, schemamigrator.SignozLogsDB, schemamigrator.LogsMigrations[len(schemamigrator.LogsMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.shouldRunMigration(c.conn, schemamigrator.SignozMetricsDB, schemamigrator.MetricsMigrations[len(schemamigrator.MetricsMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.shouldRunMigration(c.conn, schemamigrator.SignozMetadataDB, schemamigrator.MetadataMigrations[len(schemamigrator.MetadataMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.shouldRunMigration(c.conn, schemamigrator.SignozAnalyticsDB, schemamigrator.AnalyticsMigrations[len(schemamigrator.AnalyticsMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	err = c.shouldRunMigration(c.conn, schemamigrator.SignozMeterDB, schemamigrator.MeterMigrations[len(schemamigrator.MeterMigrations)-1].MigrationID)
	if err != nil {
		return err
	}

	fmt.Println("all the migrations have been run successfully")
	return nil
}

func (c *check) shouldRunMigration(conn clickhouse.Conn, db string, migrationID uint64) error {
	query := fmt.Sprintf("SELECT * FROM %s.schema_migrations_v2 WHERE migration_id = %d SETTINGS final = 1;", db, migrationID)
	var migrationSchemaMigrationRecord schemamigrator.MigrationSchemaMigrationRecord
	if err := conn.QueryRow(context.Background(), query).ScanStruct(&migrationSchemaMigrationRecord); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		return err
	}

	fmt.Println(migrationSchemaMigrationRecord.Status, migrationSchemaMigrationRecord.MigrationID)

	if migrationSchemaMigrationRecord.Status != schemamigrator.InProgressStatus && migrationSchemaMigrationRecord.Status != schemamigrator.FinishedStatus {
		return errors.New("migration is not completed")
	}

	return nil
}
