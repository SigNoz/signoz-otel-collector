package bodyparser

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/plog"
)

type Default struct {
}

func (l *Default) Parse(body []byte) (plog.Logs, int) {
	// split by newline and return
	// TODO: add configuration for multiline
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	data := string(body)
	loglines := strings.Split(data, "\n")
	for _, log := range loglines {
		sl.LogRecords().AppendEmpty().Body().SetStr(log)
	}
	return ld, len(loglines)
}
