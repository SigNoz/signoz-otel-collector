package clickhousesystemtablesreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

func TestReceiverStartAndShutdown(t *testing.T) {
	require := require.New(t)

	r := &systemTablesReceiver{
		scrapeIntervalSeconds: 2,
		scrapeDelaySeconds:    8,
		clickhouse:            &mockClickhouseQuerrier{},
		nextConsumer:          consumertest.NewNop(),
		logger:                zap.NewNop(),
	}

	require.NoError(r.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(r.Shutdown(context.Background()))
}

func TestReceiver(t *testing.T) {
	require := require.New(t)

	testScrapeIntervalSeconds := uint32(2)
	testScrapeDelaySeconds := uint32(8)
	mockQuerrier := &mockClickhouseQuerrier{}
	logsSink := consumertest.LogsSink{}
	testReceiver, err := newTestReceiver(
		mockQuerrier,
		testScrapeIntervalSeconds,
		testScrapeDelaySeconds,
		&logsSink,
	)
	require.Nil(err)

	// Receiver should start scraping query_log rows that come after the
	// clickhouse ts it observes when starting up
	t0 := uint32(time.Now().Unix())
	mockQuerrier.tsNow = t0

	// The receiver should wait long enough to respect
	// min scrape delay - ensuring query_log table has been flushed

	testQl1 := makeTestQueryLog("test-host1", time.Now(), "test query 1")
	testQl2 := makeTestQueryLog("test-host2", time.Now(), "test query 2")
	mockQuerrier.nextScrapeResult = []QueryLog{testQl1, testQl2}

	require.Equal(len(logsSink.AllLogs()), 0)
	waitSeconds, err := testReceiver.scrapeQueryLogIfReady(context.Background())
	require.Nil(err)
	require.GreaterOrEqual(waitSeconds, testScrapeDelaySeconds+testScrapeIntervalSeconds)
	require.Equal(len(logsSink.AllLogs()), 0)

	mockQuerrier.tsNow = t0 + testScrapeIntervalSeconds
	waitSeconds, err = testReceiver.scrapeQueryLogIfReady(context.Background())
	require.Nil(err)
	require.GreaterOrEqual(waitSeconds, testScrapeDelaySeconds)
	require.Less(waitSeconds, testScrapeIntervalSeconds+testScrapeDelaySeconds)
	require.Equal(len(logsSink.AllLogs()), 0)

	// receiver should be able to start scraping after waiting long enough
	mockQuerrier.tsNow = t0 + testScrapeIntervalSeconds + testScrapeDelaySeconds
	waitSeconds, err = testReceiver.scrapeQueryLogIfReady(context.Background())
	require.Nil(err)
	require.GreaterOrEqual(waitSeconds, testScrapeIntervalSeconds)
	require.Equal(len(logsSink.AllLogs()), 1)

	plogProduced := logsSink.AllLogs()[0]
	require.Equal(plogProduced.ResourceLogs().Len(), 2)
	producedHostNames := []string{}
	for i := 0; i < plogProduced.ResourceLogs().Len(); i++ {
		rl := plogProduced.ResourceLogs().At(i)
		hn, exists := rl.Resource().Attributes().Get("hostname")
		require.True(exists, "scrape query logs are expected to have hostname in resource attribs")
		producedHostNames = append(producedHostNames, hn.Str())
	}
	require.ElementsMatch(producedHostNames, []string{"test-host1", "test-host2"})

	// attempting a scrape before scrape interval has passed should not fetch more logs
	mockQuerrier.tsNow += testScrapeIntervalSeconds - 1
	waitSeconds, err = testReceiver.scrapeQueryLogIfReady(context.Background())
	require.Nil(err)
	require.GreaterOrEqual(waitSeconds, uint32(1))
	require.Equal(len(logsSink.AllLogs()), 1)

	// should scrape again after specified interval has passed
	mockQuerrier.tsNow += testScrapeIntervalSeconds + 1

	testHost := "host-3"
	testQlEventTime := time.Now()
	testQuery := "test query"
	testQl3 := makeTestQueryLog(testHost, testQlEventTime, testQuery)
	mockQuerrier.nextScrapeResult = []QueryLog{testQl3}

	_, err = testReceiver.scrapeQueryLogIfReady(context.Background())
	require.Nil(err)
	require.Equal(len(logsSink.AllLogs()), 2)
	plogProduced = logsSink.AllLogs()[1]

	// log data should be as expected.
	require.Equal(1, plogProduced.ResourceLogs().Len())
	rl := plogProduced.ResourceLogs().At(0)

	hostProduced, _ := rl.Resource().Attributes().Get("hostname")
	require.Equal(hostProduced.Str(), testHost)

	require.Equal(rl.ScopeLogs().Len(), 1)
	sl := rl.ScopeLogs().At(0)

	require.Equal(sl.LogRecords().Len(), 1)
	lr := sl.LogRecords().At(0)

	require.Equal(lr.Body().Str(), testQuery)
	require.Equal(lr.Timestamp().AsTime().Unix(), testQlEventTime.Unix())
	et, exists := lr.Attributes().Get("clickhouse.query_log.event_time")
	require.True(exists)
	require.Equal(et.Str(), testQlEventTime.Format(time.RFC3339))

	// should scrape again immediately if scrape is too far behind the server ts
	// for example: this can happen if clickhouse goes down for some time
	mockQuerrier.tsNow += 10 * testScrapeIntervalSeconds
	testQl4 := makeTestQueryLog("host-4", time.Now(), "test query 4")
	mockQuerrier.nextScrapeResult = []QueryLog{testQl4}

	waitSeconds, err = testReceiver.scrapeQueryLogIfReady(context.Background())
	require.Nil(err)
	require.Equal(uint32(0), waitSeconds)
}
