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

func (r *systemTablesReceiver) Start(
	ctx context.Context, host component.Host,
) error {
	err := r.init(ctx)
	if err != nil {
		return err
	}

	receiverCtx, cancelReceiverCtx := context.WithCancel(context.Background())
	r.requestShutdown = cancelReceiverCtx

	go func() {
		if err := r.run(receiverCtx); err != nil {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (r *systemTablesReceiver) init(ctx context.Context) error {
	// The receiver starts scraping query_log entries that come at or after
	// the timestamp at clickhouse server when the receiver is started.
	serverTsNow, err := r.clickhouse.unixTsNow(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine ts at clickhouse at receiver startup: %w", err)
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
				// errors returned from run() will cause collector shutdown
				r.logger.Error("scrape attempt failed", zap.Error(err))
			}
			minTsBeforeNextScrapeAttempt = time.Now().Unix() + int64(secondsToWaitBeforeNextAttempt)
		}

		time.Sleep(500 * time.Millisecond)
	}

}

// Scrapes query_log table at clickhouse if the next set of query_log rows to be scraped are ready for scraping
// Returns the number of seconds to wait before attempting a scrape again
func (r *systemTablesReceiver) scrapeQueryLogIfReady(ctx context.Context) (
	uint32, error,
) {
	serverTsNow, err := r.clickhouse.unixTsNow(ctx)
	if err != nil {
		return r.scrapeIntervalSeconds, fmt.Errorf(
			"couldn't determine server ts while scraping query log: %w", err,
		)
	}

	// Events with ts in [r.nextScrapedMinEventTs, scrapeIntervalEndTs) are to be scraped next
	scrapeIntervalEndTs := r.nextScrapeIntervalStartTs + r.scrapeIntervalSeconds

	// not late enough to reliably scrape the next batch of query_logs to be scraped?
	minServerTsForNextScrapeAttempt := scrapeIntervalEndTs + r.scrapeDelaySeconds
	if serverTsNow < minServerTsForNextScrapeAttempt {
		waitSeconds := minServerTsForNextScrapeAttempt - serverTsNow
		r.logger.Debug(fmt.Sprintf(
			"not ready to scrape yet. waiting %ds before next attempt. serverTsNow: %d, scrapeIntervalEndTs: %d",
			waitSeconds, serverTsNow, scrapeIntervalEndTs,
		))
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
		return r.scrapeIntervalSeconds, fmt.Errorf(
			"couldn't scrape and push query logs: %w", scrapeErr,
		)
	}

	r.logger.Debug(fmt.Sprintf(
		"scraped %d query logs. serverTsNow: %d, minTs: %d, maxTs: %d",
		pushedCount, serverTsNow, r.nextScrapeIntervalStartTs, scrapeIntervalEndTs,
	))

	r.nextScrapeIntervalStartTs = scrapeIntervalEndTs
	return r.scrapeIntervalSeconds, nil

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
