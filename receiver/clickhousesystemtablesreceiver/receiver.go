package clickhousesystemtablesreceiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

type systemTablesReceiver struct {
	config *Config
	// next scrape will include events in query_log table with
	// nextScrapeIntervalStartTs  <= event_timestamp < (nextScrapeIntervalStartTs + scrapeIntervalSeconds)
	scrapeIntervalSeconds uint32

	// next scrape will happen only after timestamp at server is at least
	// (nextScrapeIntervalStartTs + scrapeIntervalSeconds) + scrapeDelaySeconds
	scrapeDelaySeconds uint32

	clickhouse   clickhouseQuerier
	nextConsumer consumer.Logs

	logger  *zap.Logger
	obsrecv *receiverhelper.ObsReport

	nextScrapeIntervalStartTs uint32

	requestShutdown    context.CancelFunc
	shutdownCompleteWg sync.WaitGroup
}

func (r *systemTablesReceiver) Start(context.Context, component.Host) error {
	receiverCtx, cancelReceiverCtx := context.WithCancel(context.Background())
	r.requestShutdown = cancelReceiverCtx
	r.shutdownCompleteWg.Add(1)
	go r.run(receiverCtx)

	return nil
}

func (r *systemTablesReceiver) Shutdown(context.Context) error {
	r.requestShutdown()
	r.shutdownCompleteWg.Wait()
	return nil
}

func (r *systemTablesReceiver) run(ctx context.Context) {
	defer r.shutdownCompleteWg.Done()

	ticker := time.NewTicker(time.Second * time.Duration(r.scrapeIntervalSeconds+r.scrapeDelaySeconds))

	r.logger.Info("starting system.query_log table scrape", zap.Uint32("scrape_interval", r.scrapeIntervalSeconds), zap.Uint32("min_scrape_delay", r.scrapeDelaySeconds))

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("stopping receiver run loop")
			return
		case <-ticker.C:
			secondsToWaitBeforeNextAttempt, err := r.scrapeQueryLogIfReady(ctx)
			if err != nil {
				r.logger.Error("scrape attempt failed", zap.Error(err))
			}
			nextWait := time.Duration(secondsToWaitBeforeNextAttempt) * time.Second
			ticker.Reset(max(nextWait, 1*time.Millisecond))
		}
	}

}

// Scrapes query_log table at clickhouse if the next set of query_log rows to be scraped are ready for scraping
// Returns the number of seconds to wait before attempting a scrape again
func (r *systemTablesReceiver) scrapeQueryLogIfReady(ctx context.Context) (uint32, error) {
	if r.clickhouse == nil {
		db, err := newClickhouseClient(ctx, r.config.DSN)
		if err != nil {
			return r.scrapeIntervalSeconds, fmt.Errorf("couldn't create clickhouse client: %w", err)
		}
		r.clickhouse = newClickhouseQuerrier(db, r.config.ClusterName)
	}

	serverTsNow, err := r.clickhouse.unixTsNow(ctx)
	if err != nil {
		return r.scrapeIntervalSeconds, fmt.Errorf("couldn't determine server timestamp: %w", err)
	}

	if r.nextScrapeIntervalStartTs == 0 {
		r.nextScrapeIntervalStartTs = serverTsNow
	}

	// Events with ts in [r.nextScrapedMinEventTs, scrapeIntervalEndTs) are to be scraped next
	scrapeIntervalEndTs := r.nextScrapeIntervalStartTs + r.scrapeIntervalSeconds

	// not late enough to reliably scrape the next batch of query_logs to be scraped?
	minServerTsForNextScrapeAttempt := scrapeIntervalEndTs + r.scrapeDelaySeconds
	if serverTsNow < minServerTsForNextScrapeAttempt {
		waitSeconds := minServerTsForNextScrapeAttempt - serverTsNow
		r.logger.Debug("not ready to scrape yet", zap.Uint32("wait_seconds", waitSeconds), zap.Uint32("server_ts_now", serverTsNow), zap.Uint32("scrape_interval_end_ts", scrapeIntervalEndTs))
		return waitSeconds, nil
	}

	// scrape the next batch of query_log rows to be scraped.
	if r.obsrecv != nil {
		ctx = r.obsrecv.StartLogsOp(ctx)
	}

	pushedCount, scrapeErr := r.scrapeAndPushQueryLogs(
		ctx, r.nextScrapeIntervalStartTs, scrapeIntervalEndTs,
	)

	if r.obsrecv != nil {
		r.obsrecv.EndLogsOp(ctx, "clickhouse.system.query_log", pushedCount, scrapeErr)
	}

	if scrapeErr != nil {
		return r.scrapeIntervalSeconds, fmt.Errorf("couldn't scrape and push query logs: %w", scrapeErr)
	}

	r.logger.Debug("scraped query logs", zap.Int("pushed_count", pushedCount), zap.Uint32("server_ts_now", serverTsNow), zap.Uint32("min_ts", r.nextScrapeIntervalStartTs), zap.Uint32("max_ts", scrapeIntervalEndTs))

	r.nextScrapeIntervalStartTs = scrapeIntervalEndTs

	// Go for the next scrape immediately if next interval to be scraped is already far enough in the past.
	// Allows the receiver to catch up if it is running behind
	// For example, this can happen if this was the first successful scrape
	// after several failed attempts and subsequent waits for r.ScrapeIntervalSeconds
	nextScrapeMinServerTs := r.nextScrapeIntervalStartTs + r.scrapeIntervalSeconds + r.scrapeDelaySeconds

	nextWaitSeconds := uint32(0)
	if nextScrapeMinServerTs > serverTsNow {
		// Do the subtraction only if it will not lead to an overflow/wrap around
		nextWaitSeconds = nextScrapeMinServerTs - serverTsNow
	}

	return nextWaitSeconds, nil
}

func (r *systemTablesReceiver) scrapeAndPushQueryLogs(
	ctx context.Context, minEventTs uint32, maxEventTs uint32,
) (int, error) {
	queryLogs, err := r.clickhouse.scrapeQueryLog(
		ctx, minEventTs, maxEventTs,
	)
	if err != nil {
		return 0, fmt.Errorf("couldn't scrape clickhouse query_log table: %w", err)
	}

	pl, err := r.queryLogsToPlogs(queryLogs)
	if err != nil {
		return 0, fmt.Errorf("couldn't create plogs.Logs for scraped query logs: %w", err)
	}

	err = r.nextConsumer.ConsumeLogs(ctx, pl)
	if err != nil {
		return 0, fmt.Errorf("couldn't push logs to next consumer: %w", err)
	}

	return len(queryLogs), nil
}

func (r *systemTablesReceiver) queryLogsToPlogs(queryLogs []QueryLog) (
	plog.Logs, error,
) {
	pl := plog.NewLogs()

	logsByQueryLogHostName := map[string]plog.LogRecordSlice{}

	for _, ql := range queryLogs {
		lr, err := ql.toLogRecord()
		if err != nil {
			return pl, err
		}

		if _, exists := logsByQueryLogHostName[ql.Hostname]; !exists {
			rl := pl.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("hostname", ql.Hostname)
			sl := rl.ScopeLogs().AppendEmpty()
			logsByQueryLogHostName[ql.Hostname] = sl.LogRecords()
		}

		lr.MoveTo(logsByQueryLogHostName[ql.Hostname].AppendEmpty())
	}

	return pl, nil
}
