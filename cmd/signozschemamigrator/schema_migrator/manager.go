package schemamigrator

import (
	"context"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type SchemaMigrationRecord struct {
	MigrationID int
	UpItems     []Operation
	DownItems   []Operation
}

// MigrationManager is the manager for the schema migrations.
type MigrationManager struct {
	addrs              []string
	addrsMux           sync.Mutex
	clusterName        string
	replicationEnabled bool

	conn clickhouse.Conn

	logger *zap.Logger
}

type Option func(*MigrationManager)

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(opts ...Option) *MigrationManager {
	mgr := &MigrationManager{}
	for _, opt := range opts {
		opt(mgr)
	}
	if mgr.logger == nil {
		mgr.logger = zap.NewNop()
	}
	return mgr
}

func WithClusterName(clusterName string) Option {
	return func(mgr *MigrationManager) {
		mgr.clusterName = clusterName
	}
}

func WithReplicationEnabled(replicationEnabled bool) Option {
	return func(mgr *MigrationManager) {
		mgr.replicationEnabled = replicationEnabled
	}
}

func WithConn(conn clickhouse.Conn) Option {
	return func(mgr *MigrationManager) {
		mgr.conn = conn
	}
}

func WithLogger(logger *zap.Logger) Option {
	return func(mgr *MigrationManager) {
		mgr.logger = logger
	}
}

func (m *MigrationManager) createDBs() error {
	if m.clusterName != "" {
		tracesErr := m.conn.Exec(context.Background(), "CREATE DATABASE IF NOT EXISTS signoz_traces ON CLUSTER '"+m.clusterName+"'")
		metricsErr := m.conn.Exec(context.Background(), "CREATE DATABASE IF NOT EXISTS signoz_metrics ON CLUSTER '"+m.clusterName+"'")
		logsErr := m.conn.Exec(context.Background(), "CREATE DATABASE IF NOT EXISTS signoz_logs ON CLUSTER '"+m.clusterName+"'")
		return multierr.Combine(tracesErr, metricsErr, logsErr)
	} else {
		tracesErr := m.conn.Exec(context.Background(), "CREATE DATABASE IF NOT EXISTS signoz_traces")
		metricsErr := m.conn.Exec(context.Background(), "CREATE DATABASE IF NOT EXISTS signoz_metrics")
		logsErr := m.conn.Exec(context.Background(), "CREATE DATABASE IF NOT EXISTS signoz_logs")
		return multierr.Combine(tracesErr, metricsErr, logsErr)
	}
}

// Bootstrap migrates the schema up for the migrations tables
func (m *MigrationManager) Bootstrap() error {
	if err := m.createDBs(); err != nil {
		return err
	}
	// migrate up the v2 schama migrations tables
	return m.MigrateUp(context.Background(), V2Tables)
}

func (m *MigrationManager) RunSquashedMigrations(ctx context.Context) error {
	for _, migration := range SquashedLogsMigrations {
		for _, item := range migration.UpItems {
			if err := m.RunOperation(ctx, item); err != nil {
				return err
			}
		}
	}
	for _, migration := range SquashedMetricsMigrations {
		for _, item := range migration.UpItems {
			if err := m.RunOperation(ctx, item); err != nil {
				return err
			}
		}
	}
	for _, migration := range SquashedTracesMigrations {
		for _, item := range migration.UpItems {
			if err := m.RunOperation(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// HostAddrs returns the addresses of the all hosts in the cluster.
func (m *MigrationManager) HostAddrs() ([]string, error) {
	m.addrsMux.Lock()
	defer m.addrsMux.Unlock()
	if len(m.addrs) != 0 {
		return m.addrs, nil
	}

	hostAddrs := make(map[string]struct{})
	query := "SELECT DISTINCT host_address FROM system.clusters WHERE host_address NOT IN ['localhost', '127.0.0.1'] AND cluster = '" + m.clusterName + "'"
	rows, err := m.conn.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var hostAddr string
		if err := rows.Scan(&hostAddr); err != nil {
			return nil, err
		}
		hostAddrs[hostAddr] = struct{}{}
	}

	if len(hostAddrs) != 0 {
		// connect to other host and do the same thing
		for hostAddr := range hostAddrs {
			conn, err := clickhouse.Open(&clickhouse.Options{
				Addr: []string{hostAddr},
			})
			if err != nil {
				return nil, err
			}
			rows, err := conn.Query(context.Background(), query)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			for rows.Next() {
				var hostAddr string
				if err := rows.Scan(&hostAddr); err != nil {
					return nil, err
				}
				hostAddrs[hostAddr] = struct{}{}
			}
			break
		}
	}

	addrs := make([]string, 0, len(hostAddrs))
	for addr := range hostAddrs {
		addrs = append(addrs, addr)
	}
	m.addrs = addrs
	return addrs, nil
}

// WaitForRunningMutations waits for all the mutations to be completed on all the hosts in the cluster.
func (m *MigrationManager) WaitForRunningMutations(ctx context.Context) error {
	return nil
}

// WaitDistributedDDLQueue waits for all the DDLs to be completed on all the hosts in the cluster.
func (m *MigrationManager) WaitDistributedDDLQueue(ctx context.Context) error {
	return nil
}

// MigrateUp migrates the schema up.
func (m *MigrationManager) MigrateUp(ctx context.Context, migrations []SchemaMigrationRecord) error {

	for _, migration := range migrations {
		if err := m.WaitForRunningMutations(ctx); err != nil {
			return err
		}
		if err := m.WaitDistributedDDLQueue(ctx); err != nil {
			return err
		}
		for _, item := range migration.UpItems {
			if err := m.RunOperation(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// MigrateDown migrates the schema down.
func (m *MigrationManager) MigrateDown(ctx context.Context, migrations []SchemaMigrationRecord) error {
	for _, migration := range migrations {
		if err := m.WaitForRunningMutations(ctx); err != nil {
			return err
		}
		for _, item := range migration.DownItems {
			if err := m.RunOperation(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// MigrateUpSync migrates the schema up.
func (m *MigrationManager) MigrateUpSync(ctx context.Context, migrationIDS []string) error {

	return nil
}

// MigrateDownSync migrates the schema down.
func (m *MigrationManager) MigrateDownSync(ctx context.Context, migrationIDS []string) error {

	return nil
}

// MigrateUpAsync migrates the schema up.
func (m *MigrationManager) MigrateUpAsync(ctx context.Context, migrationIDS []string) error {

	return nil
}

// MigrateDownAsync migrates the schema down.
func (m *MigrationManager) MigrateDownAsync(ctx context.Context, migrationIDS []string) error {

	return nil
}

func (m *MigrationManager) RunOperation(ctx context.Context, operation Operation) error {
	var sql string
	if m.clusterName != "" {
		operation = operation.OnCluster(m.clusterName)
	}
	if m.replicationEnabled {
		operation = operation.WithReplication()
	}
	sql = operation.ToSQL()
	m.logger.Info("Running operation", zap.String("sql", sql))
	rows, err := m.conn.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	return nil
}

func (m *MigrationManager) Close() error {
	return m.conn.Close()
}
