package signozlogspipelineprocessor

import (
	"testing"

	signozstanzaentry "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestConvertEntriesToPlogsGroupsByScope(t *testing.T) {
	newEntry := func(scopeName, scopeVersion string, resource map[string]any) *entry.Entry {
		e := entry.New()
		e.Body = "hello"
		e.ScopeName = scopeName
		e.Resource = resource
		if scopeVersion != "" {
			e.AddAttribute(signozstanzaentry.InternalTempScopeVersionAttribute, scopeVersion)
		}
		return e
	}

	resource := map[string]any{"service.name": "checkout"}
	pLogs := convertEntriesToPlogs([]*entry.Entry{
		newEntry("checkout", "1.4.0", resource),
		newEntry("checkout", "1.4.0", resource),
		newEntry("checkout", "2.0.0", resource),
		newEntry("checkout", "", resource),
		newEntry("payments", "1.4.0", resource),
	})

	require.Equal(t, 1, pLogs.ResourceLogs().Len())
	scopeLogs := pLogs.ResourceLogs().At(0).ScopeLogs()

	require.Equal(t, 4, scopeLogs.Len())

	type scope struct {
		name    string
		version string
		records int
	}
	scopes := []scope{}
	for i := 0; i < scopeLogs.Len(); i++ {
		sl := scopeLogs.At(i)
		scopes = append(scopes, scope{
			name:    sl.Scope().Name(),
			version: sl.Scope().Version(),
			records: sl.LogRecords().Len(),
		})
	}
	require.Equal(t, []scope{
		{name: "checkout", version: "1.4.0", records: 2},
		{name: "checkout", version: "2.0.0", records: 1},
		{name: "checkout", version: "", records: 1},
		{name: "payments", version: "1.4.0", records: 1},
	}, scopes)

	firstRecord := scopeLogs.At(0).LogRecords().At(0)
	_, exists := firstRecord.Attributes().Get(signozstanzaentry.InternalTempScopeVersionAttribute)
	require.False(t, exists)
	require.Equal(t, 0, firstRecord.Attributes().Len())
}

func TestStashUnmappedFieldsRoundTrip(t *testing.T) {
	ld := plog.NewLogs()
	sl := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	sl.Scope().SetName("checkout")
	sl.Scope().SetVersion("1.4.0")
	sl.Scope().Attributes().PutStr("scope.attr", "value")
	first := sl.LogRecords().AppendEmpty()
	first.Body().SetStr("hello")
	first.SetEventName("checkout.completed")
	sl.LogRecords().AppendEmpty().Body().SetStr("hello again")

	stashUnmappedFields(ld)

	entries := []*entry.Entry{}
	for i := 0; i < sl.LogRecords().Len(); i++ {
		record := sl.LogRecords().At(i)
		e := entry.New()
		e.Body = record.Body().AsRaw()
		e.ScopeName = sl.Scope().Name()
		e.Attributes = record.Attributes().AsRaw()
		entries = append(entries, e)
	}
	require.Equal(t, "1.4.0", entries[0].Attributes[signozstanzaentry.InternalTempScopeVersionAttribute])
	require.Equal(t, map[string]any{"scope.attr": "value"},
		entries[0].Attributes[signozstanzaentry.InternalTempScopeAttributesAttribute])
	require.Equal(t, "checkout.completed", entries[0].Attributes[signozstanzaentry.InternalTempEventNameAttribute])
	require.NotContains(t, entries[1].Attributes, signozstanzaentry.InternalTempEventNameAttribute)

	out := convertEntriesToPlogs(entries)

	require.Equal(t, 1, out.ResourceLogs().Len())
	require.Equal(t, 1, out.ResourceLogs().At(0).ScopeLogs().Len())
	outScope := out.ResourceLogs().At(0).ScopeLogs().At(0)
	require.Equal(t, "checkout", outScope.Scope().Name())
	require.Equal(t, "1.4.0", outScope.Scope().Version())
	require.Equal(t, map[string]any{"scope.attr": "value"}, outScope.Scope().Attributes().AsRaw())
	require.Equal(t, 2, outScope.LogRecords().Len())
	require.Equal(t, 0, outScope.LogRecords().At(0).Attributes().Len())
	require.Equal(t, "checkout.completed", outScope.LogRecords().At(0).EventName())
	require.Empty(t, outScope.LogRecords().At(1).EventName())
}

func TestStashUnmappedFieldsSkipsWhenThereIsNothingToCarry(t *testing.T) {
	ld := plog.NewLogs()
	sl := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	sl.Scope().SetName("checkout")
	sl.LogRecords().AppendEmpty().Body().SetStr("hello")

	stashUnmappedFields(ld)

	require.Equal(t, 0, sl.LogRecords().At(0).Attributes().Len())
}

func TestConvertEntriesToPlogsGroupsByScopeAttributes(t *testing.T) {
	newEntry := func(scopeAttributes map[string]any) *entry.Entry {
		e := entry.New()
		e.Body = "hello"
		e.ScopeName = "checkout"
		e.Attributes = map[string]any{
			signozstanzaentry.InternalTempScopeAttributesAttribute: scopeAttributes,
		}
		return e
	}

	pLogs := convertEntriesToPlogs([]*entry.Entry{
		newEntry(map[string]any{"a": "1"}),
		newEntry(map[string]any{"a": "1"}),
		newEntry(map[string]any{"a": "2"}),
	})

	scopeLogs := pLogs.ResourceLogs().At(0).ScopeLogs()
	require.Equal(t, 2, scopeLogs.Len())
	require.Equal(t, 2, scopeLogs.At(0).LogRecords().Len())
	require.Equal(t, map[string]any{"a": "1"}, scopeLogs.At(0).Scope().Attributes().AsRaw())
	require.Equal(t, 1, scopeLogs.At(1).LogRecords().Len())
	require.Equal(t, map[string]any{"a": "2"}, scopeLogs.At(1).Scope().Attributes().AsRaw())
}
