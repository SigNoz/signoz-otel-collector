package json

import (
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

var severityByLevelName = func() map[string]entry.Severity {
	levelNames := map[entry.Severity][]string{
		entry.Trace:  {"trace", "verbose", "finest", "silly"},
		entry.Trace2: {"trace2", "finer"},
		entry.Trace3: {"trace3"},
		entry.Trace4: {"trace4"},
		entry.Debug:  {"debug", "fine"},
		entry.Debug2: {"debug2", "config"},
		entry.Debug3: {"debug3"},
		entry.Debug4: {"debug4"},
		entry.Info:   {"info", "information", "informational"},
		entry.Info2:  {"info2", "notice"},
		entry.Info3:  {"info3"},
		entry.Info4:  {"info4"},
		entry.Warn:   {"warn", "warning"},
		entry.Warn2:  {"warn2", "warning2"},
		entry.Warn3:  {"warn3", "warning3"},
		entry.Warn4:  {"warn4", "warning4"},
		entry.Error:  {"error", "err", "severe"},
		entry.Error2: {"error2", "err2", "critical", "crit"},
		entry.Error3: {"error3", "err3", "alert"},
		entry.Error4: {"error4", "err4"},
		entry.Fatal:  {"fatal", "panic", "emergency", "emerg"},
		entry.Fatal2: {"fatal2"},
		entry.Fatal3: {"fatal3"},
		entry.Fatal4: {"fatal4"},
	}

	severities := map[string]entry.Severity{}
	for severity, names := range levelNames {
		for _, name := range names {
			severities[name] = severity
		}
	}
	return severities
}()

func (f fieldConfig) setSeverity(ent *entry.Entry, results scanResults) {
	if severity, ok := f.take(results, targetSeverityNumber); ok {
		ent.Severity = severity.(entry.Severity)
	}

	if text, ok := f.take(results, targetSeverityText); ok {
		name := text.(string)
		if severity, known := severityByLevelName[strings.ToLower(name)]; known {
			ent.SeverityText = severityText(severity)
			if ent.Severity == entry.Default {
				ent.Severity = severity
			}
		} else {
			ent.SeverityText = name
		}
	}

	if ent.Severity == entry.Default {
		if severity, ok := severityFromLevelName(ent.SeverityText); ok {
			ent.Severity = severity
		}
	}
	if ent.SeverityText == "" && ent.Severity != entry.Default {
		ent.SeverityText = severityText(ent.Severity)
	}
}

func severityText(severity entry.Severity) string {
	if severity == entry.Default {
		return ""
	}
	group := entry.Severity((int(severity)-1)/4*4 + 1)
	return group.String()
}

func severityFromLevelName(value any) (entry.Severity, bool) {
	name, ok := value.(string)
	if !ok {
		return entry.Default, false
	}
	severity, ok := severityByLevelName[strings.ToLower(strings.TrimSpace(name))]
	return severity, ok
}

func parseSeverityNumber(value any) (any, bool) {
	severity, ok := severityFromNumber(value)
	if !ok {
		return nil, false
	}
	return severity, true
}

func severityFromNumber(value any) (entry.Severity, bool) {
	var number int64

	switch v := value.(type) {
	case int64:
		number = v
	case int:
		number = int64(v)
	case float64:
		if v < float64(entry.Trace) || v > float64(entry.Fatal4) || v != float64(int64(v)) {
			return entry.Default, false
		}
		number = int64(v)
	case string:
		text := strings.TrimSpace(v)
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return severityFromLevelName(strings.TrimPrefix(strings.ToLower(text), "severity_number_"))
		}
		number = parsed
	default:
		return entry.Default, false
	}

	if number < int64(entry.Trace) || number > int64(entry.Fatal4) {
		return entry.Default, false
	}
	return entry.Severity(number), true
}
