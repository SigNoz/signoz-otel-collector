package bodyparser

import "go.opentelemetry.io/collector/pdata/plog"

type Default struct {
}

func (l *Default) Parse(body []byte, records plog.LogRecordSlice) {
}
