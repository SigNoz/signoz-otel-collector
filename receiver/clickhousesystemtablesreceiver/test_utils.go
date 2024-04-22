package clickhousesystemtablesreceiver

import (
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/DATA-DOG/go-sqlmock"
	mockhouse "github.com/srikanthccv/ClickHouse-go-mock"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

type mockClickhouse struct {
	mockDB mockhouse.ClickConnMockCommon
}

func newMockClickhouse() (*mockClickhouse, error) {
	mockCh, err := mockhouse.NewClickHouseWithQueryMatcher(
		nil, sqlmock.QueryMatcherRegexp,
	)
	if err != nil {
		return nil, err
	}

	mockCh.MatchExpectationsInOrder(false)

	return &mockClickhouse{
		mockDB: mockCh,
	}, nil

}

func (mch *mockClickhouse) mockServerTsNow(ts uint32) {
	cols := []mockhouse.ColumnType{}
	cols = append(cols, mockhouse.ColumnType{Type: "UInt32", Name: "toUnixTimestamp(now())"})

	values := [][]any{
		{
			uint32(ts),
		},
	}

	mch.mockDB.ExpectQuery(
		`select toUnixTimestamp(now())`,
	).WillReturnRows(mockhouse.NewRows(cols, values))
}

func newTestReceiver(
	db driver.Conn,
	scrapeIntervalSeconds uint64,
	scrapeDelaySeconds uint64,
) *systemTablesReceiver {
	return &systemTablesReceiver{
		scrapeIntervalSeconds: scrapeIntervalSeconds,
		scrapeDelaySeconds:    scrapeDelaySeconds,
		nextConsumer:          consumertest.NewNop(),
		db:                    db,
		logger:                zap.NewNop(),
	}
}
