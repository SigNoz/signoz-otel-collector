package migrator

import (
	"context"
	"fmt"

	"github.com/SigNoz/signoz-otel-collector/migrationManager/migrators"
	basemigrator "github.com/SigNoz/signoz-otel-collector/migrationManager/migrators/baseMigrator"
	"github.com/SigNoz/signoz-otel-collector/migrationManager/migrators/logs"
	"github.com/SigNoz/signoz-otel-collector/migrationManager/migrators/metrics"
	"github.com/SigNoz/signoz-otel-collector/migrationManager/migrators/traces"
	"go.uber.org/zap"
)

type MigrationManager struct {
	Migrators []migrators.Migrator
	logger    *zap.Logger
}

type Config struct {
	migrators.MigratorConfig
}

func NewConfig(
	dsn string,
	clusterName string,
	isMultiNodeCluster bool,
	isDurationSortFeatureDisabled bool,
	isTimestampSortFeatureDisabled bool,
) Config {
	return Config{
		MigratorConfig: migrators.MigratorConfig{
			DSN:                            dsn,
			ClusterName:                    clusterName,
			IsMultiNodeCluster:             isMultiNodeCluster,
			IsDurationSortFeatureDisabled:  isDurationSortFeatureDisabled,
			IsTimestampSortFeatureDisabled: isTimestampSortFeatureDisabled,
		},
	}
}

func New(cfg Config) (*MigrationManager, error) {
	logger := zap.L().With(zap.String("component", "migrationManager"))

	logsMigrator, err := createNewMigrator("logs", cfg.MigratorConfig)
	if err != nil {
		logger.Error("Failed to create logs migrator", zap.Error(err))
		return nil, err
	}
	metricsMigrator, err := createNewMigrator("metrics", cfg.MigratorConfig)
	if err != nil {
		logger.Error("Failed to create metrics migrator", zap.Error(err))
		return nil, err
	}
	tracesMigrator, err := createNewMigrator("traces", cfg.MigratorConfig)
	if err != nil {
		logger.Error("Failed to create traces migrator", zap.Error(err))
		return nil, err
	}
	return &MigrationManager{
		Migrators: []migrators.Migrator{
			logsMigrator,
			metricsMigrator,
			tracesMigrator,
		},
		logger: logger,
	}, nil
}

func createNewMigrator(migratorType string, cfg migrators.MigratorConfig) (migrators.Migrator, error) {
	logger := zap.L().With(zap.String("migrator", migratorType))
	b, err := basemigrator.New(cfg, logger)
	if err != nil {
		logger.Error("Failed to create base migrator", zap.Error(err))
		return nil, err
	}
	switch migratorType {
	case "traces":
		return &traces.TracesMigrator{BaseMigrator: b}, nil
	case "metrics":
		return &metrics.MetricsMigrator{BaseMigrator: b}, nil
	case "logs":
		return &logs.LogsMigrator{BaseMigrator: b}, nil
	default:
		return nil, fmt.Errorf("invalid migrator type: %s", migratorType)
	}
}

func (m *MigrationManager) Migrate(ctx context.Context) error {
	m.logger.Info("Running migrations for all migrators")
	for _, migrator := range m.Migrators {
		m.logger.Info(fmt.Sprintf("Running migrations for %s", migrator.Name()), zap.String("migrator", migrator.Name()))
		err := migrator.Migrate(ctx)
		if err != nil {
			m.logger.Error("Failed to run migrations for migrator", zap.String("migrator", migrator.Name()), zap.Error(err))
			return err
		}
	}
	m.logger.Info("Finished running migrations for all migrators")
	return nil
}

func (m *MigrationManager) Close() error {
	for _, migrator := range m.Migrators {
		if err := migrator.Close(); err != nil {
			return err
		}
	}
	return nil
}