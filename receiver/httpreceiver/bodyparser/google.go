package bodyparser

import "go.opentelemetry.io/collector/pdata/plog"

type GCloud struct {
}

func (l *GCloud) Parse(body []byte) (plog.Logs, int) {
	return plog.Logs{}, 0
}
