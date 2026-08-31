package json

import (
	"strings"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func (f fieldConfig) setScope(ent *entry.Entry, results scanResults) {
	if name, ok := f.take(results, targetScopeName); ok {
		ent.ScopeName = name.(string)
	}

	if version, ok := f.take(results, targetScopeVersion); ok {
		ent.AddAttribute(signozstanzaentry.InternalTempScopeVersionAttribute, version.(string))
	}
}

func parseNonEmptyString(value any) (any, bool) {
	str, ok := value.(string)
	if !ok {
		return nil, false
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return nil, false
	}
	return str, true
}
