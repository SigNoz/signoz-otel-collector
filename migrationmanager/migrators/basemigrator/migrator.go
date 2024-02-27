package basemigrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/SigNoz/signoz-otel-collector/migrationmanager/migrators"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

type BaseMigrator struct {
	Cfg    migrators.MigratorConfig
	Logger *zap.Logger
	DB     driver.Conn
}

func New(cfg migrators.MigratorConfig, logger *zap.Logger) (*BaseMigrator, error) {
	dbConn, err := createClickhouseConnection(cfg.DSN)
	if err != nil {
		logger.Error("Failed to create clickhouse connection", zap.Error(err))
		return nil, err
	}

	return &BaseMigrator{
		Cfg:    cfg,
		Logger: logger,
		DB:     dbConn,
	}, nil
}

func (m *BaseMigrator) Migrate(ctx context.Context, database string, migrationFolder string) error {
	err := m.createDB(ctx, database)
	if err != nil {
		return err
	}

	return m.runSqlMigrations(ctx, migrationFolder, database)
}

func (m *BaseMigrator) Close() error {
	return m.DB.Close()
}

func (m *BaseMigrator) createDB(ctx context.Context, database string) error {
	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s ON CLUSTER %s;", database, m.Cfg.ClusterName)
	err := m.DB.Exec(ctx, q)
	if err != nil {
		return fmt.Errorf("failed to create database, err: %s", err)
	}
	return nil
}

func (m *BaseMigrator) dropSchemaMigrationsTable(ctx context.Context, database string) error {
	err := m.DB.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s.%s ON CLUSTER %s;`, database, "schema_migrations", m.Cfg.ClusterName))
	if err != nil {
		return fmt.Errorf("error dropping schema_migrations table: %v", err)
	}
	return nil
}

func (m *BaseMigrator) runSqlMigrations(ctx context.Context, migrationFolder, database string) error {
	clickhouseUrl, err := m.buildClickhouseMigrateURL(database)
	if err != nil {
		return fmt.Errorf("failed to build clickhouse migrate url, err: %s", err)
	}
	migrator, err := migrate.New("file://"+migrationFolder, clickhouseUrl)
	if err != nil {
		return fmt.Errorf("failed to create migrator, err: %s", err)
	}
	migrator.Log = newZapLoggerAdapter(m.Logger, m.Cfg.VerboseLoggingEnabled)
	migrator.EnableTemplating = true

	err = migrator.Up()
	if err != nil && !strings.HasSuffix(err.Error(), "no change") {
		return fmt.Errorf("clickhouse migrate failed to run, error: %s", err)
	}
	return nil
}

func (m *BaseMigrator) buildClickhouseMigrateURL(database string) (string, error) {
	var clickhouseUrl string
	options, err := clickhouse.ParseDSN(m.Cfg.DSN)
	if err != nil {
		return "", err
	}

	if len(options.Auth.Username) > 0 && len(options.Auth.Password) > 0 {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s:%s@%s/%s?x-multi-statement=true&x-cluster-name=%s&x-migrations-table=schema_migrations&x-migrations-table-engine=MergeTree", options.Auth.Username, options.Auth.Password, options.Addr[0], database, m.Cfg.ClusterName)
	} else {
		clickhouseUrl = fmt.Sprintf("clickhouse://%s/%s?x-multi-statement=true&x-cluster-name=%s&x-migrations-table=schema_migrations&x-migrations-table-engine=MergeTree", options.Addr[0], database, m.Cfg.ClusterName)
	}
	return clickhouseUrl, nil
}

func createClickhouseConnection(dsn string) (driver.Conn, error) {
	options, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dsn: %w", err)
	}

	db, err := clickhouse.Open(options)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse connection: %w", err)
	}

	if err := db.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping clickhouse: %w", err)
	}

	return db, nil
}
