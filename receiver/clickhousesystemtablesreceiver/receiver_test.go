package clickhousesystemtablesreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReceiverStartAndShutdown(t *testing.T) {

}

func TestReceiver(t *testing.T) {
	require := require.New(t)

	mockCh, err := newMockClickhouse()
	require.Nil(err)

	testScrapeIntervalSeconds := 2
	testScrapeDelaySeconds := 8
	testReceiver := newTestReceiver(
		mockCh.mockDB, testScrapeIntervalSeconds, testScrapeDelaySeconds,
	)

	t0 := uint32(time.Now().Unix())
	mockCh.mockServerTsNow(t0)
	testReceiver.init()

	// The receiver should wait long enough to respect
	// min scrape delay - ensuring query_log table has been flushed

	// The receiver should then continue to scrape periodically
	// as per specified interval

	require.Equal(1, 2)
}
