package migrate

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
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
	query := "SELECT DISTINCT host_address, port FROM system.clusters WHERE host_address NOT IN ['localhost', '127.0.0.1', '::1'] AND cluster = ?"
	rows, err := r.conn.Query(ctx, query, r.cluster)
	if err != nil {
		return NewRetryableError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	type hostAddr struct {
		address string
		port    uint16
	}

	var hosts []hostAddr
	for rows.Next() {
		var address string
		var port uint16
		if err := rows.Scan(&address, &port); err != nil {
			return err
		}
		hosts = append(hosts, hostAddr{address: address, port: port})
	}

	for _, host := range hosts {
		if host.address != "" {
			addr, err := netip.ParseAddr(host.address)
			if err != nil {
				return err
			}

			addrPort := netip.AddrPortFrom(addr, host.port)
			conn, err := clickhouse.Open(&clickhouse.Options{
				// cannot pass all the address here as this is used for failover/ load-balancing. at any point of them one is selected and connection is established
				// ref: https://github.com/ClickHouse/clickhouse-go/blob/main/clickhouse.go#L275
				Addr: []string{addrPort.String()},
			})
			if err != nil {
				return err
			}
			defer func() {
				_ = conn.Close()
			}()

			if err := conn.Ping(ctx); err != nil {
				return NewRetryableError(fmt.Errorf("clickhouse host %s:%d not reachable: %w", host.address, host.port, err))
			}
		}
	}

	return nil
}

func (r *ready) CheckKeeper(ctx context.Context) error {
	query := "SELECT * FROM system.zookeeper_connection"
	if _, err := r.conn.Query(ctx, query); err != nil {
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

	return nil
}
