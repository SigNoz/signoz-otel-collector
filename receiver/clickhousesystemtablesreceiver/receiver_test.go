package clickhousesystemtablesreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestReceiverStartAndShutdown(t *testing.T) {

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

	// Receiver should start scraping from the server ts
	// it observes when it starts
	t0 := uint32(time.Now().Unix())
	mockQuerrier.tsNow = t0

	testQl1 := makeTestQueryLog("test-host1", time.Now(), "test query 1")
	testQl2 := makeTestQueryLog("test-host2", time.Now(), "test query 2")
	mockQuerrier.scrapeResult = []QueryLog{testQl1, testQl2}

	err = testReceiver.init(context.Background())
	require.Nil(err)
	require.Equal(testReceiver.nextScrapeIntervalStartTs, t0)

	// The receiver should wait long enough to respect
	// min scrape delay - ensuring query_log table has been flushed
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

	mockQuerrier.tsNow = t0 + testScrapeIntervalSeconds + testScrapeDelaySeconds
	waitSeconds, err = testReceiver.scrapeQueryLogIfReady(context.Background())
	require.Nil(err)
	require.GreaterOrEqual(waitSeconds, testScrapeIntervalSeconds)
	require.Equal(len(logsSink.AllLogs()), 1)
	logProduced := logsSink.AllLogs()[0]

	require.Equal(logProduced.ResourceLogs().Len(), 2)
	producedHostNames := []string{}
	for i := 0; i < logProduced.ResourceLogs().Len(); i++ {
		rl := logProduced.ResourceLogs().At(i)
		hn, exists := rl.Resource().Attributes().Get("hostname")
		require.True(exists, "scrape query logs are expected to have hostname in resource attribs")
		producedHostNames = append(producedHostNames, hn.Str())
	}
	require.ElementsMatch(producedHostNames, []string{"test-host1", "test-host2"})

	// The receiver should then continue to scrape periodically
	// as per specified interval

	// require.Equal(1, 2)
}
