package ready

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/config"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type ready struct {
	conn     clickhouse.Conn
	shards   uint64
	replicas uint64
	cluster  string
	version  string
	timeout  time.Duration
	logger   *zap.Logger
}

func Register(parentCmd *cobra.Command, logger *zap.Logger) {
	readyCmd := &cobra.Command{
		Use:   "ready",
		Short: "Checks if the store is ready to run migrations. In cases of stores which have sharded/replicated setups, checks if all the shards/replicas are online and have the necessary permissions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ready, err := newReady(
				config.Clickhouse.DSN,
				config.Clickhouse.Replicas,
				config.Clickhouse.Shards,
				config.Clickhouse.Cluster,
				config.Clickhouse.Version,
				config.MigrateReady.Timeout,
				logger,
			)
			if err != nil {
				return err
			}

			err = ready.Run()
			if err != nil {
				return err
			}

			return nil
		},
	}

	config.MigrateReady.RegisterFlags(readyCmd)
	parentCmd.AddCommand(readyCmd)
}

func newReady(dsn string, shards, replicas uint64, cluster, version, timeout string, logger *zap.Logger) (*ready, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, err
	}

	return &ready{
		conn:     conn,
		shards:   shards,
		replicas: replicas,
		cluster:  cluster,
		version:  version,
		timeout:  timeoutDuration,
		logger:   logger,
	}, nil
}

func (r *ready) Run() error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = r.timeout

	for {
		err := r.Ready()
		if err == nil {
			break
		}

		r.logger.Info("Waiting for store to be in ready state", zap.Error(err))
		nextBackOff := backoff.NextBackOff()
		if nextBackOff == backoff.Stop {
			return fmt.Errorf("timed out waiting for store readiness checks to pass within the configured timeout of %s", r.timeout)
		}
		time.Sleep(nextBackOff)
	}

	return nil
}

func (r *ready) Ready() error {
	if err := r.MatchVersion(); err != nil {
		return err
	}

	if err := r.MatchReplicaCount(); err != nil {
		return err
	}

	if err := r.MatchShardCount(); err != nil {
		return err
	}

	return nil
}

func (r *ready) MatchVersion() error {
	query := "SELECT version()"
	var version string
	if err := r.conn.QueryRow(context.Background(), query).Scan(&version); err != nil {
		return err
	}

	if r.version != version {
		return fmt.Errorf("store version mismatch (%v/%v)", version, r.version)
	}

	return nil
}

func (r *ready) MatchReplicaCount() error {
	query := fmt.Sprintf("SELECT count(DISTINCT(shard_num)) FROM system.clusters WHERE cluster = %s", r.cluster)
	var replicas uint64
	if err := r.conn.QueryRow(context.Background(), query).Scan(&replicas); err != nil {
		return err
	}

	if r.replicas != replicas {
		return fmt.Errorf("store replica count mismatch (%v/%v)", replicas, r.replicas)
	}

	return nil
}

func (r *ready) MatchShardCount() error {
	query := fmt.Sprintf("SELECT count(DISTINCT(replica_num)) FROM system.clusters WHERE cluster = %s", r.cluster)
	var shards uint64
	if err := r.conn.QueryRow(context.Background(), query).Scan(&shards); err != nil {
		return err
	}

	if r.shards != shards {
		return fmt.Errorf("store shard count mismatch (%v/%v)", shards, r.shards)
	}

	return nil
}
