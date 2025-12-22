package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/config"
	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
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

func registerReady(parentCmd *cobra.Command, logger *zap.Logger) {
	readyCmd := &cobra.Command{
		Use:   "ready",
		Short: "Checks if the store is ready to run migrations. In cases of stores which have sharded/replicated setups, checks if all the shards/replicas are online and have the necessary permissions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ready, err := newReady(
				config.Clickhouse.DSN,
				config.Clickhouse.Shards,
				config.Clickhouse.Replicas,
				config.Clickhouse.Cluster,
				config.Clickhouse.Version,
				config.MigrateReady.Timeout,
				logger,
			)
			if err != nil {
				return err
			}

			err = ready.Run(cmd.Context())
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

func (r *ready) Run(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = r.timeout

	for {
		err := r.Ready(ctx)
		if err == nil {
			break
		}

		var error *errors.Error
		if errors.As(err, &error) {
			// exit early for non retryable errors.
			if !error.IsRetryable() {
				return fmt.Errorf("store not ready due to non-retryable error: %w", err)
			}
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

func (r *ready) Ready(ctx context.Context) error {
	if err := r.MatchVersion(ctx); err != nil {
		return err
	}

	if err := r.CheckKeeperConnection(ctx); err != nil {
		return err
	}

	if err := r.MatchReplicaAndShardCount(ctx); err != nil {
		return err
	}

	return nil
}

func (r *ready) MatchVersion(ctx context.Context) error {
	query := "SELECT version()"
	var version string
	if err := r.conn.QueryRow(ctx, query).Scan(&version); err != nil {
		return err
	}

	if !strings.HasPrefix(version, r.version) {
		return errors.NewRetryableError(fmt.Errorf("store version mismatch (%v/%v)", version, r.version))
	}

	return nil
}

func (r *ready) MatchReplicaAndShardCount(ctx context.Context) error {
	query := fmt.Sprintf("SELECT count(DISTINCT(replica_num)), count(DISTINCT(shard_num)) FROM system.clusters WHERE cluster = %s", r.cluster)
	var replicas, shards uint64
	if err := r.conn.QueryRow(ctx, query).Scan(&replicas, &shards); err != nil {
		return err
	}

	if r.replicas != replicas {
		return errors.NewRetryableError(fmt.Errorf("store replica count mismatch (%v/%v)", replicas, r.replicas))
	}

	if r.shards != shards {
		return errors.NewRetryableError(fmt.Errorf("store shard count mismatch (%v/%v)", shards, r.shards))
	}

	return nil
}

func (r *ready) CheckKeeperConnection(ctx context.Context) error {
	query := "SELECT * FROM system.zookeeper_connection"
	if _, err := r.conn.Query(ctx, query); err != nil {
		var exception *clickhouse.Exception
		if errors.As(err, &exception) {
			if exception.Code == 999 {
				if strings.Contains(exception.Error(), "No node") {
					return errors.NewRetryableError(err)
				}
			}
		}

		return errors.New(err)
	}

	return nil
}
