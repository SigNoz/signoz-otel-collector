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
	// next scrape will include events in query_log table with
	// nextScrapeMinEventTs  <= event_timestamp < (nextScrapeMinEventTs + scrapeIntervalSeconds)
	scrapeIntervalSeconds uint64

	// next scrape will happen only after timestamp at server
	// is greater than (nextScrapeMinEventTs + scrapeIntervalSeconds) + scrapeDelaySeconds
	scrapeDelaySeconds uint64

	db     driver.Conn
	logger *zap.Logger

	// members containing internal state for receivers
	nextScrapeMinEventTs uint64
	requestShutdown      context.CancelFunc
	shutdownCompleteWg   sync.WaitGroup
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
	r.nextScrapeMinEventTs = serverTsNow

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
			// TODO(Raj): Should this cause fatal errors?
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
				// TODO(Raj): Errors returned from run are
				return err
			}
			minTsBeforeNextScrapeAttempt = time.Now().Unix() + int64(secondsToWaitBeforeNextAttempt)
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

// Scrapes query_log table at clickhouse if it has been long enough since the last scrape.
// Returns the number of seconds to wait before attempting a scrape again
func (r *systemTablesReceiver) scrapeQueryLogIfReady(ctx context.Context) (uint64, error) {
	serverTsNow, err := r.unixTsNowAtClickhouse(ctx)
	if err != nil {
		return 5, fmt.Errorf("couldn't determine server ts while scraping query log: %w", err)
	}

	maxEventTsToScrape := r.nextScrapeMinEventTs + r.scrapeIntervalSeconds

	minServerTsForNextScrapeAttempt := maxEventTsToScrape + r.scrapeDelaySeconds

	if serverTsNow <= minServerTsForNextScrapeAttempt {
		waitSeconds := (minServerTsForNextScrapeAttempt - serverTsNow) + 1
		r.logger.Debug(fmt.Sprintf(
			"not ready to scrape yet. waiting %ds before next attempt. serverTsNow: %d, maxEventTsToScrape: %d",
			waitSeconds, serverTsNow, maxEventTsToScrape,
		))
		return waitSeconds, nil
	}

	queryLogRecords, err := scrapeQueryLogRecords(
		ctx, r.db, r.nextScrapeMinEventTs, maxEventTsToScrape,
	)
	if err != nil {
		return 5, fmt.Errorf(
			"couldn't scrape clickhouse query_log table: %w", err,
		)
	}

	logBodies := []string{}
	for _, lr := range queryLogRecords {
		logBodies = append(
			logBodies,
			fmt.Sprintf("%d", lr.Timestamp().AsTime().Unix()),
		)
	}

	r.logger.Debug(fmt.Sprintf(
		"scraped %d query logs. serverTsNow: %d, minTs: %d, maxTs: %d",
		len(queryLogRecords), serverTsNow, r.nextScrapeMinEventTs, maxEventTsToScrape,
	))

	r.nextScrapeMinEventTs = maxEventTsToScrape
	return r.scrapeIntervalSeconds, nil
}
