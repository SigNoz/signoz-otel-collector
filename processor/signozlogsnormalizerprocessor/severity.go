package signozlogsnormalizerprocessor

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const maxSeverityTextLength = 50

var severityGroupNames = [...]string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

var severityByLevelName = func() map[string]plog.SeverityNumber {
	levelNames := map[plog.SeverityNumber][]string{
		plog.SeverityNumberTrace:  {"trace", "verbose", "finest", "silly"},
		plog.SeverityNumberTrace2: {"trace2", "finer"},
		plog.SeverityNumberTrace3: {"trace3"},
		plog.SeverityNumberTrace4: {"trace4"},
		plog.SeverityNumberDebug:  {"debug", "fine"},
		plog.SeverityNumberDebug2: {"debug2", "config"},
		plog.SeverityNumberDebug3: {"debug3"},
		plog.SeverityNumberDebug4: {"debug4"},
		plog.SeverityNumberInfo:   {"info", "information", "informational"},
		plog.SeverityNumberInfo2:  {"info2", "notice"},
		plog.SeverityNumberInfo3:  {"info3"},
		plog.SeverityNumberInfo4:  {"info4"},
		plog.SeverityNumberWarn:   {"warn", "warning"},
		plog.SeverityNumberWarn2:  {"warn2", "warning2"},
		plog.SeverityNumberWarn3:  {"warn3", "warning3"},
		plog.SeverityNumberWarn4:  {"warn4", "warning4"},
		plog.SeverityNumberError:  {"error", "err", "severe"},
		plog.SeverityNumberError2: {"error2", "err2", "critical", "crit"},
		plog.SeverityNumberError3: {"error3", "err3", "alert"},
		plog.SeverityNumberError4: {"error4", "err4"},
		plog.SeverityNumberFatal:  {"fatal", "panic", "emergency", "emerg"},
		plog.SeverityNumberFatal2: {"fatal2"},
		plog.SeverityNumberFatal3: {"fatal3"},
		plog.SeverityNumberFatal4: {"fatal4"},
	}

	severities := map[string]plog.SeverityNumber{}
	for severity, names := range levelNames {
		for _, name := range names {
			severities[name] = severity
		}
	}
	return severities
}()

func setSeverity(lr plog.LogRecord, results *scanResults, names *fieldNames) (derived [targetCount]bool) {
	number := lr.SeverityNumber()
	if results.found[targetSeverityNumber] {
		number = results.values[targetSeverityNumber].severity
	}
	if results.found[targetSeverityText] {
		lr.SetSeverityText(results.values[targetSeverityText].text)
	}

	if number == plog.SeverityNumberUnspecified && names.enabled(targetSeverityNumber) {
		if results.found[targetSeverityText] {
			number = results.values[targetSeverityText].severity
		} else {
			number = severityFromLevelName(lr.SeverityText())
		}
		derived[targetSeverityNumber] = number != plog.SeverityNumberUnspecified
	}
	if lr.SeverityText() == "" && names.enabled(targetSeverityText) {
		if group, ok := severityGroup(number); ok {
			lr.SetSeverityText(group)
			derived[targetSeverityText] = true
		}
	}
	lr.SetSeverityNumber(number)
	return derived
}

func severityGroup(severity plog.SeverityNumber) (string, bool) {
	if severity < plog.SeverityNumberTrace || severity > plog.SeverityNumberFatal4 {
		return "", false
	}
	return severityGroupNames[(severity-1)/4], true
}

func severityFromLevelName(name string) plog.SeverityNumber {
	return severityByLevelName[strings.ToLower(strings.TrimSpace(name))]
}

func parseSeverityText(value pcommon.Value) (inferred, bool) {
	parsed, ok := parseNonEmptyString(value)
	if !ok {
		return inferred{}, false
	}
	text := parsed.text
	if len(text) > maxSeverityTextLength {
		cut := maxSeverityTextLength
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		if cut == 0 {
			return inferred{}, false
		}
		text = text[:cut]
	}
	return inferred{text: text, severity: severityFromLevelName(parsed.text)}, true
}

func parseSeverityNumber(value pcommon.Value) (inferred, bool) {
	var number int64

	switch value.Type() {
	case pcommon.ValueTypeInt:
		number = value.Int()
	case pcommon.ValueTypeDouble:
		double := value.Double()
		if double != math.Trunc(double) {
			return inferred{}, false
		}
		number = int64(double)
	case pcommon.ValueTypeStr:
		text := strings.TrimSpace(value.Str())
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			severity := severityFromLevelName(strings.TrimPrefix(strings.ToLower(text), "severity_number_"))
			return inferred{severity: severity}, severity != plog.SeverityNumberUnspecified
		}
		number = parsed
	default:
		return inferred{}, false
	}

	if number < int64(plog.SeverityNumberTrace) || number > int64(plog.SeverityNumberFatal4) {
		return inferred{}, false
	}
	return inferred{severity: plog.SeverityNumber(number)}, true
}
