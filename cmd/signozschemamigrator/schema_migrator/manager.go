package schemamigrator

import (
	"context"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// SQLSchemaMigration is the interface that wraps the methods for a schema migration.
// It is used to represent the schema migration in the SQL.
type SQLSchemaMigration interface {
	Version() int
	// Apply applies the migration to the database
	UpItems(ctx context.Context) []Operation
	// Rollback rolls back the migration from the database
	DownItems(ctx context.Context) []Operation
}

type SchemaMigrationRecord struct {
	MigrationID int
	UpItems     []Operation
	DownItems   []Operation
}

// MigrationManager is the manager for the schema migrations.
type MigrationManager struct {
	dsn                string
	addrs              []string
	addrsMux           sync.Mutex
	clusterName        string
	replicationEnabled bool

	conn clickhouse.Conn
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(dsn string, clusterName string, replicationEnabled bool) *MigrationManager {
	return &MigrationManager{dsn: dsn, clusterName: clusterName, replicationEnabled: replicationEnabled}
}

// Init initializes the migration manager.
func (m *MigrationManager) Init() error {
	options := &clickhouse.Options{
		Addr: []string{m.dsn},
	}
	conn, err := clickhouse.Open(options)
	if err != nil {
		return err
	}
	m.conn = conn
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
func (m *MigrationManager) MigrateUp(ctx context.Context, migrations []SQLSchemaMigration) error {

	for _, migration := range migrations {
		if err := m.WaitForRunningMutations(ctx); err != nil {
			return err
		}
		if err := m.WaitDistributedDDLQueue(ctx); err != nil {
			return err
		}
		items := migration.UpItems(ctx)
		for _, item := range items {
			if err := m.RunOperation(ctx, item); err != nil {
				return err
			}
		}
	}
	return nil
}

// MigrateDown migrates the schema down.
func (m *MigrationManager) MigrateDown(ctx context.Context, migrations []SQLSchemaMigration) error {
	for _, migration := range migrations {
		if err := m.WaitForRunningMutations(ctx); err != nil {
			return err
		}
		items := migration.DownItems(ctx)
		for _, item := range items {
			if err := m.RunOperation(ctx, item); err != nil {
				return err
			}
		}
	}
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
