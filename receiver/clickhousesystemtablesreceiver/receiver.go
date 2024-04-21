// import "github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver"
package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type systemTablesReceiver struct {
	db driver.Conn

	// next scrape will include events in query_log table with
	// nextScrapeIntervalStartTs  <= event_timestamp < (nextScrapeIntervalStartTs + scrapeIntervalSeconds)
	scrapeIntervalSeconds     uint64
	nextScrapeIntervalStartTs uint64

	requestShutdown    context.CancelFunc
	shutdownCompleteWg sync.WaitGroup

	logger *zap.Logger
}

func (r *systemTablesReceiver) Start(ctx context.Context, host component.Host) error {
	receiverCtx, cancelReceiverCtx := context.WithCancel(context.Background())
	r.requestShutdown = cancelReceiverCtx

	// Begin scraping query_log entries that come at or after
	// the timestamp at clickhouse server when the receiver is started.
	serverTsNow, err := r.unixTsNowAtClickhouse(ctx)
	if err != nil {
		return fmt.Errorf("couldn't start clickhousesystemtablesreceiver: %w", err)
	}
	r.nextScrapeIntervalStartTs = serverTsNow

	// TODO(Raj): Add obsrecv stuff
	// obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
	// ReceiverID:             r.settings.ID,
	// Transport:              transport,
	// ReceiverCreateSettings: r.settings,
	// })
	// if err != nil {
	// return err
	// }

	go func() {
		if err := r.run(receiverCtx); err != nil {
			host.ReportFatalError(err)
		}
	}()
	return nil
}

func (r *systemTablesReceiver) Shutdown(context.Context) error {
	r.requestShutdown()
	r.shutdownCompleteWg.Wait()
	return nil
}

func (r *systemTablesReceiver) run(ctx context.Context) error {
	r.shutdownCompleteWg.Add(1)
	defer r.shutdownCompleteWg.Done()

	minTsBeforeNextScrapeAttempt := time.Now().Unix()

	for {
		// time to shut down?
		if ctx.Err() != nil {
			r.logger.Info("Stopping ClickhouseSystemTablesReceiver", zap.Error(ctx.Err()))
			return ctx.Err()
		}

		if time.Now().Unix() >= minTsBeforeNextScrapeAttempt {
			secondsToWaitBeforeNextAttempt, err := r.scrapeQueryLogIfReady(ctx)
			if err != nil {
				return err
			}
			minTsBeforeNextScrapeAttempt = time.Now().Unix() + secondsToWaitBeforeNextAttempt
		}

		time.Sleep(500 * time.Millisecond)
	}

}

func (r *systemTablesReceiver) unixTsNowAtClickhouse(ctx context.Context) (uint64, error) {
	var serverTsNow uint64
	row := r.db.QueryRow(ctx, `select toUInt64(toUnixTimestamp(now()))`)
	if err := row.Scan(&serverTsNow); err != nil {
		return 0, fmt.Errorf("couldn't query current timestamp at clickhouse server: %w", err)
	}
	return serverTsNow, nil
}

func (r *systemTablesReceiver) scrapeQueryLogIfReady(ctx context.Context) (int64, error) {
	serverTsNow, err := r.unixTsNowAtClickhouse(ctx)
	if err != nil {
		return 5, err
	}
	r.logger.Info(fmt.Sprintf("DEBUG: Pretending to scrape query log: server ts: %d", serverTsNow))
	return 5, nil
}
