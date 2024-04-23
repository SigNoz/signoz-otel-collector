// import "github.com/SigNoz/signoz-otel-collector/receiver/clickhousesystemtablesreceiver"
package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type systemTablesReceiver struct {
	// next scrape will include events in query_log table with
	// nextScrapeIntervalStartTs  <= event_timestamp < (nextScrapeIntervalStartTs + scrapeIntervalSeconds)
	scrapeIntervalSeconds uint32

	// next scrape will happen only after timestamp at server
	// is greater than (nextScrapeIntervalStartTs + scrapeIntervalSeconds) + scrapeDelaySeconds
	scrapeDelaySeconds uint32

	clickhouse   clickhouseQuerrier
	nextConsumer consumer.Logs
	logger       *zap.Logger

	// members containing internal state for receivers
	nextScrapeIntervalStartTs uint32
	requestShutdown           context.CancelFunc
	shutdownCompleteWg        sync.WaitGroup
}

func (r *systemTablesReceiver) Start(ctx context.Context, host component.Host) error {
	receiverCtx, cancelReceiverCtx := context.WithCancel(context.Background())
	r.requestShutdown = cancelReceiverCtx

	// TODO(Raj): Add obsrecv stuff
	// obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
	// ReceiverID:             r.settings.ID,
	// Transport:              transport,
	// ReceiverCreateSettings: r.settings,
	// })
	// if err != nil {
	// return err
	// }

	err := r.init(ctx)
	if err != nil {
		return err
	}

	go func() {
		if err := r.run(receiverCtx); err != nil {
			// TODO(Raj): Should this cause fatal errors?
			host.ReportFatalError(err)
		}
	}()
	return nil
}

func (r *systemTablesReceiver) init(ctx context.Context) error {
	// Begin scraping query_log entries that come at or after
	// the timestamp at clickhouse server when the receiver is started.
	serverTsNow, err := r.clickhouse.unixTsNow(ctx)
	if err != nil {
		return fmt.Errorf("couldn't start clickhousesystemtablesreceiver: %w", err)
	}
	r.nextScrapeIntervalStartTs = serverTsNow
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

// Scrapes query_log table at clickhouse if it has been long enough since the last scrape.
// Returns the number of seconds to wait before attempting a scrape again
func (r *systemTablesReceiver) scrapeQueryLogIfReady(ctx context.Context) (uint32, error) {
	serverTsNow, err := r.clickhouse.unixTsNow(ctx)
	if err != nil {
		return 0, fmt.Errorf("couldn't determine server ts while scraping query log: %w", err)
	}

	// Events with ts in [r.nextScrapedMinEventTs, scrapeIntervalEndTs) are to be scraped next
	scrapeIntervalEndTs := r.nextScrapeIntervalStartTs + r.scrapeIntervalSeconds

	minServerTsForNextScrapeAttempt := scrapeIntervalEndTs + r.scrapeDelaySeconds

	if serverTsNow < minServerTsForNextScrapeAttempt {
		waitSeconds := minServerTsForNextScrapeAttempt - serverTsNow + 1
		r.logger.Debug(fmt.Sprintf(
			"not ready to scrape yet. waiting %ds before next attempt. serverTsNow: %d, scrapeIntervalEndTs: %d",
			waitSeconds, serverTsNow, scrapeIntervalEndTs,
		))
		return waitSeconds, nil
	}

	queryLogs, err := r.clickhouse.scrapeQueryLog(
		ctx, r.nextScrapeIntervalStartTs, scrapeIntervalEndTs,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"couldn't scrape clickhouse query_log table: %w", err,
		)
	}

	pl := plog.NewLogs()

	logsByQueryLogHostName := map[string]plog.LogRecordSlice{}
	for _, ql := range queryLogs {
		if _, exists := logsByQueryLogHostName[ql.Hostname]; !exists {
			rl := pl.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			logsByQueryLogHostName[ql.Hostname] = sl.LogRecords()
		}

		lr, err := ql.toLogRecord()
		if err != nil {
			return 0, fmt.Errorf("couldn't convert to query_log to plog record: %w", err)
		}
		lr.MoveTo(logsByQueryLogHostName[ql.Hostname].AppendEmpty())
	}

	r.logger.Debug(fmt.Sprintf(
		"scraped %d query logs. serverTsNow: %d, minTs: %d, maxTs: %d",
		len(queryLogs), serverTsNow, r.nextScrapeIntervalStartTs, scrapeIntervalEndTs,
	))

	r.nextConsumer.ConsumeLogs(ctx, pl)

	r.nextScrapeIntervalStartTs = scrapeIntervalEndTs
	return r.scrapeIntervalSeconds, nil
}
