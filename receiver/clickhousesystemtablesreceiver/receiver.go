package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

type systemTablesReceiver struct {
	settings receiver.CreateSettings

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
	obsrecv                   *receiverhelper.ObsReport
	nextScrapeIntervalStartTs uint32
	requestShutdown           context.CancelFunc
	shutdownCompleteWg        sync.WaitGroup
}

func (r *systemTablesReceiver) Start(ctx context.Context, host component.Host) error {
	receiverCtx, cancelReceiverCtx := context.WithCancel(context.Background())
	r.requestShutdown = cancelReceiverCtx

	err := r.init(ctx)
	if err != nil {
		return err
	}

	go func() {
		if err := r.run(receiverCtx); err != nil {
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
				// errors returned from run will cause collector shutdown
				r.logger.Error("scrape attempt failed", zap.Error(err))
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

	if r.obsrecv != nil {
		ctx = r.obsrecv.StartLogsOp(ctx)
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
			rl.Resource().Attributes().PutStr("hostname", ql.Hostname)
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

	err = r.nextConsumer.ConsumeLogs(ctx, pl)

	if r.obsrecv != nil {
		r.obsrecv.EndLogsOp(ctx, "clickhouse.system.query_log", 1, err)
	}

	if err != nil {
		return 0, fmt.Errorf("couldn't push logs to next consumer: %w", err)
	}

	r.nextScrapeIntervalStartTs = scrapeIntervalEndTs
	return r.scrapeIntervalSeconds, nil
}
