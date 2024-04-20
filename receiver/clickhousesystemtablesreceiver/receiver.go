// import "github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver"
package clickhousesystemtablesreceiver

import (
	"context"
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

func (r *systemTablesReceiver) Start(_ context.Context, host component.Host) error {
	receiverCtx, cancelReceiverCtx := context.WithCancel(context.Background())
	r.requestShutdown = cancelReceiverCtx

	// Begin scraping query_log entries that come at or after
	// the timestamp at clickhouse server when the receiver is started.
	r.nextScrapeIntervalStartTs = r.unixTsNowAtClickhouse()

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
			secondsToWaitBeforeNextAttempt, err := r.scrapeQueryLogIfReady()
			if err != nil {
				return err
			}
			minTsBeforeNextScrapeAttempt = time.Now().Unix() + secondsToWaitBeforeNextAttempt
		}

		time.Sleep(500 * time.Millisecond)
	}

}

func (r *systemTablesReceiver) unixTsNowAtClickhouse() uint64 {
	return 0
}

func (r *systemTablesReceiver) scrapeQueryLogIfReady() (int64, error) {
	r.logger.Info("DEBUG: Pretending to scrape query log")
	return 5, nil
}
