package signozclickhouseauditexporter

import (
	"math"
	"testing"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
)

func resourceFingerprint(attrs map[string]any) string {
	return fingerprint.CalculateFingerprint(attrs, fingerprint.ResourceHierarchy())
}

func TestBuildBatch(t *testing.T) {
	testCases := []struct {
		name          string
		logs          func() plog.Logs
		now           time.Time
		setup         func(e *auditExporter)
		wantLogs      []logRow
		wantResources []resourceRow
		wantFieldKeys []fieldKeyRow
		wantFields    []fieldRow
	}{
		{
			name: "StringBody_ScalarAttributes",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("service.name", "signoz")
				sl := rl.ScopeLogs().AppendEmpty()
				sl.Scope().SetName("signoz.audit")
				sl.Scope().SetVersion("1.0.0")
				sl.Scope().Attributes().PutStr("signoz.audit.schema", "v1")
				record := sl.LogRecords().AppendEmpty()
				record.SetTimestamp(1790677977519692024)
				record.SetObservedTimestamp(1790677977522805958)
				record.SetTraceID(pcommon.TraceID{0xa4, 0xa3, 0x15, 0x37, 0x07, 0xd9, 0x13, 0x82, 0xff, 0x51, 0x06, 0xeb, 0xaf, 0x0c, 0x7a, 0x66})
				record.SetSpanID(pcommon.SpanID{0xbf, 0x16, 0x07, 0x71, 0xcd, 0xb3, 0x61, 0x42})
				record.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
				record.SetSeverityText("INFO")
				record.SetSeverityNumber(plog.SeverityNumberInfo)
				record.SetEventName("dashboard.updated")
				record.Body().SetStr("user23 updated dashboard")
				record.Attributes().PutStr("signoz.audit.action", "update")
				record.Attributes().PutInt("http.response.status_code", 200)
				record.Attributes().PutDouble("signoz.audit.score", 0.5)
				record.Attributes().PutBool("signoz.audit.dry_run", false)
				record.Attributes().PutEmptySlice("signoz.audit.tags").AppendEmpty().SetStr("a")
				return ld
			},
			now: time.Unix(0, 1790677980000000000),
			wantLogs: []logRow{{
				tsBucketStart:       1790677800,
				resourceFingerprint: resourceFingerprint(map[string]any{"service.name": "signoz"}),
				timestamp:           1790677977519692024,
				observedTimestamp:   1790677977522805958,
				traceID:             "a4a3153707d91382ff5106ebaf0c7a66",
				spanID:              "bf160771cdb36142",
				traceFlags:          1,
				severityText:        "INFO",
				severityNumber:      9,
				body:                `{"message":"user23 updated dashboard"}`,
				scope:               `{"attributes":{"signoz.audit.schema":"v1"},"name":"signoz.audit","version":"1.0.0"}`,
				attributes:          `{"http.response.status_code":200,"signoz.audit.action":"update","signoz.audit.dry_run":false,"signoz.audit.score":0.5,"signoz.audit.tags":["a"]}`,
				resource:            `{"attributes":{"service.name":"signoz"}}`,
				eventName:           "dashboard.updated",
			}},
			wantResources: []resourceRow{{
				labels:              `{"service.name":"signoz"}`,
				fingerprint:         resourceFingerprint(map[string]any{"service.name": "signoz"}),
				seenAtTSBucketStart: 1790677800,
			}},
			wantFieldKeys: []fieldKeyRow{
				{name: "service.name", fieldContext: "resource", fieldDataType: "string"},
				{name: "signoz.audit.schema", fieldContext: "scope", fieldDataType: "string"},
				{name: "signoz.audit.action", fieldContext: "attribute", fieldDataType: "string"},
				{name: "http.response.status_code", fieldContext: "attribute", fieldDataType: "int64"},
				{name: "signoz.audit.score", fieldContext: "attribute", fieldDataType: "float64"},
				{name: "signoz.audit.dry_run", fieldContext: "attribute", fieldDataType: "bool"},
			},
			wantFields: []fieldRow{
				{name: "service.name", fieldContext: "resource", fieldDataType: "string", value: "signoz"},
				{name: "signoz.audit.schema", fieldContext: "scope", fieldDataType: "string", value: "v1"},
				{name: "signoz.audit.action", fieldContext: "attribute", fieldDataType: "string", value: "update"},
				{name: "http.response.status_code", fieldContext: "attribute", fieldDataType: "int64", value: int64(200)},
				{name: "signoz.audit.score", fieldContext: "attribute", fieldDataType: "float64", value: 0.5},
				{name: "signoz.audit.dry_run", fieldContext: "attribute", fieldDataType: "bool", value: false},
			},
		},
		{
			name: "MapBody_KeptAsIs",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				record := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				record.SetTimestamp(1790681234000000000)
				body := record.Body().SetEmptyMap()
				body.PutStr("message", "role revoked")
				body.PutEmptyMap("detail").PutStr("role", "admin")
				return ld
			},
			now: time.Unix(0, 1790681240000000000),
			wantLogs: []logRow{{
				tsBucketStart:       1790679600,
				resourceFingerprint: resourceFingerprint(map[string]any{}),
				timestamp:           1790681234000000000,
				observedTimestamp:   1790681240000000000,
				body:                `{"detail":{"role":"admin"},"message":"role revoked"}`,
				scope:               `{"attributes":{},"name":"","version":""}`,
				attributes:          `{}`,
				resource:            `{"attributes":{}}`,
			}},
			wantResources: []resourceRow{{
				labels:              `{}`,
				fingerprint:         resourceFingerprint(map[string]any{}),
				seenAtTSBucketStart: 1790679600,
			}},
		},
		{
			name: "ZeroTimestamp_UsesObserved",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				record := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				record.SetObservedTimestamp(1790681000000000000)
				record.Body().SetStr("session started")
				return ld
			},
			now: time.Unix(0, 1790682000000000000),
			wantLogs: []logRow{{
				tsBucketStart:       1790679600,
				resourceFingerprint: resourceFingerprint(map[string]any{}),
				timestamp:           1790681000000000000,
				observedTimestamp:   1790681000000000000,
				body:                `{"message":"session started"}`,
				scope:               `{"attributes":{},"name":"","version":""}`,
				attributes:          `{}`,
				resource:            `{"attributes":{}}`,
			}},
			wantResources: []resourceRow{{
				labels:              `{}`,
				fingerprint:         resourceFingerprint(map[string]any{}),
				seenAtTSBucketStart: 1790679600,
			}},
		},
		{
			name: "ZeroTimestamps_UseNow",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				record := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				record.Body().SetStr("api key created")
				return ld
			},
			now: time.Unix(0, 1790690000000000000),
			wantLogs: []logRow{{
				tsBucketStart:       1790688600,
				resourceFingerprint: resourceFingerprint(map[string]any{}),
				timestamp:           1790690000000000000,
				observedTimestamp:   1790690000000000000,
				body:                `{"message":"api key created"}`,
				scope:               `{"attributes":{},"name":"","version":""}`,
				attributes:          `{}`,
				resource:            `{"attributes":{}}`,
			}},
			wantResources: []resourceRow{{
				labels:              `{}`,
				fingerprint:         resourceFingerprint(map[string]any{}),
				seenAtTSBucketStart: 1790688600,
			}},
		},
		{
			name: "RepeatedAttributes_MetadataDeduped",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				records := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
				for _, ts := range []pcommon.Timestamp{1790700000000000000, 1790700001000000000} {
					record := records.AppendEmpty()
					record.SetTimestamp(ts)
					record.Body().SetStr("dashboard deleted")
					record.Attributes().PutStr("signoz.audit.action", "delete")
				}
				return ld
			},
			now: time.Unix(0, 1790700002000000000),
			wantLogs: []logRow{
				{
					tsBucketStart:       1790699400,
					resourceFingerprint: resourceFingerprint(map[string]any{}),
					timestamp:           1790700000000000000,
					observedTimestamp:   1790700002000000000,
					body:                `{"message":"dashboard deleted"}`,
					scope:               `{"attributes":{},"name":"","version":""}`,
					attributes:          `{"signoz.audit.action":"delete"}`,
					resource:            `{"attributes":{}}`,
				},
				{
					tsBucketStart:       1790699400,
					resourceFingerprint: resourceFingerprint(map[string]any{}),
					timestamp:           1790700001000000000,
					observedTimestamp:   1790700002000000000,
					body:                `{"message":"dashboard deleted"}`,
					scope:               `{"attributes":{},"name":"","version":""}`,
					attributes:          `{"signoz.audit.action":"delete"}`,
					resource:            `{"attributes":{}}`,
				},
			},
			wantResources: []resourceRow{{
				labels:              `{}`,
				fingerprint:         resourceFingerprint(map[string]any{}),
				seenAtTSBucketStart: 1790699400,
			}},
			wantFieldKeys: []fieldKeyRow{{name: "signoz.audit.action", fieldContext: "attribute", fieldDataType: "string"}},
			wantFields:    []fieldRow{{name: "signoz.audit.action", fieldContext: "attribute", fieldDataType: "string", value: "delete"}},
		},
		{
			name: "CachedKeysAndResource_Skipped",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("service.name", "signoz-api")
				record := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				record.SetTimestamp(1790710000000000000)
				record.Body().SetStr("user invited")
				record.Attributes().PutStr("signoz.audit.outcome", "success")
				return ld
			},
			now: time.Unix(0, 1790710001000000000),
			setup: func(e *auditExporter) {
				e.keysCache.Set("service.name:resource:string", struct{}{}, ttlcache.DefaultTTL)
				e.keysCache.Set("signoz.audit.outcome:attribute:string", struct{}{}, ttlcache.DefaultTTL)
				fp := resourceFingerprint(map[string]any{"service.name": "signoz-api"})
				e.rfCache.Set(utils.MakeKeyForRFCache(1790708400, fp), struct{}{}, ttlcache.DefaultTTL)
			},
			wantLogs: []logRow{{
				tsBucketStart:       1790708400,
				resourceFingerprint: resourceFingerprint(map[string]any{"service.name": "signoz-api"}),
				timestamp:           1790710000000000000,
				observedTimestamp:   1790710001000000000,
				body:                `{"message":"user invited"}`,
				scope:               `{"attributes":{},"name":"","version":""}`,
				attributes:          `{"signoz.audit.outcome":"success"}`,
				resource:            `{"attributes":{"service.name":"signoz-api"}}`,
			}},
			wantFields: []fieldRow{
				{name: "service.name", fieldContext: "resource", fieldDataType: "string", value: "signoz-api"},
				{name: "signoz.audit.outcome", fieldContext: "attribute", fieldDataType: "string", value: "success"},
			},
		},
		{
			name: "NonFiniteFloat_NotReported",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				record := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				record.SetTimestamp(1790720000000000000)
				record.Body().SetStr("quota updated")
				record.Attributes().PutDouble("signoz.audit.ratio", math.NaN())
				return ld
			},
			now: time.Unix(0, 1790720001000000000),
			wantLogs: []logRow{{
				tsBucketStart:       1790719200,
				resourceFingerprint: resourceFingerprint(map[string]any{}),
				timestamp:           1790720000000000000,
				observedTimestamp:   1790720001000000000,
				body:                `{"message":"quota updated"}`,
				scope:               `{"attributes":{},"name":"","version":""}`,
				attributes:          `{"signoz.audit.ratio":null}`,
				resource:            `{"attributes":{}}`,
			}},
			wantResources: []resourceRow{{
				labels:              `{}`,
				fingerprint:         resourceFingerprint(map[string]any{}),
				seenAtTSBucketStart: 1790719200,
			}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exp := newTestExporter(t)
			if testCase.setup != nil {
				testCase.setup(exp)
			}

			b, err := exp.buildBatch(testCase.logs(), testCase.now)
			require.NoError(t, err)

			require.Len(t, b.logs, len(testCase.wantLogs))
			for i := range b.logs {
				id, err := ksuid.Parse(b.logs[i].id)
				require.NoError(t, err)
				assert.Equal(t, int64(b.logs[i].timestamp/uint64(time.Second)), id.Time().Unix())
				b.logs[i].id = ""
			}
			assert.Equal(t, testCase.wantLogs, b.logs)
			assert.Equal(t, testCase.wantResources, b.resources)
			assert.Equal(t, testCase.wantFieldKeys, b.fieldKeys)
			assert.Equal(t, testCase.wantFields, b.fields)
		})
	}
}
