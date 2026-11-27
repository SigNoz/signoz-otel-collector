package migrate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/SigNoz/signoz-otel-collector/cmd/signozotelcollector/config"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type ready struct {
	conn    clickhouse.Conn
	cluster string
	timeout time.Duration
	logger  *zap.Logger
}

func registerReady(parentCmd *cobra.Command, logger *zap.Logger) {
	readyCmd := &cobra.Command{
		Use:   "ready",
		Short: "Checks if the store is ready to run migrations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ready, err := newReady(
				config.Clickhouse.DSN,
				config.Clickhouse.Cluster,
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

func newReady(dsn string, cluster string, timeout time.Duration, logger *zap.Logger) (*ready, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}

	return &ready{
		conn:    conn,
		cluster: cluster,
		timeout: timeout,
		logger:  logger,
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

		migrateErr := Unwrapb(err)
		// exit early for non-retryable errors.
		if !migrateErr.IsRetryable() {
			return fmt.Errorf("store not ready due to non-retryable error: %w", err)
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
	if err := r.CheckClickhouse(ctx); err != nil {
		return err
	}

	if err := r.CheckKeeper(ctx); err != nil {
		return err
	}

	return nil
}

func (r *ready) CheckClickhouse(ctx context.Context) error {
	query := "SELECT hostName() FROM clusterAllReplicas(?, system.one) SETTINGS skip_unavailable_shards = 0"
	rows, err := r.conn.Query(ctx, query, r.cluster)
	if err != nil {
		return NewRetryableError(fmt.Errorf("clickhouse cluster %s not ready: %w", r.cluster, err))
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var host string
		if err := rows.Scan(&host); err != nil {
			return err
		}
		r.logger.Info("clickhouse is ready", zap.String("host", host))
	}

	if err := rows.Err(); err != nil {
		return NewRetryableError(fmt.Errorf("clickhouse cluster %s not ready: %w", r.cluster, err))
	}

	return nil
}

func (r *ready) CheckKeeper(ctx context.Context) error {
	query := "SELECT host, port FROM system.zookeeper_connection"
	rows, err := r.conn.Query(ctx, query)
	if err != nil {
		var exception *clickhouse.Exception
		if errors.As(err, &exception) {
			if exception.Code == 999 {
				if strings.Contains(exception.Error(), "No node") {
					return NewRetryableError(err)
				}
			}
		}

		return err
	}

	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var host string
		var port uint16
		if err := rows.Scan(&host, &port); err != nil {
			return err
		}
		r.logger.Info("keeper is ready", zap.String("host", host), zap.Uint16("port", port))
	}

	return nil
}
