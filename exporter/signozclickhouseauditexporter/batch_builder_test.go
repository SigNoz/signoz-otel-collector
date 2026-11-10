package signozclickhouseauditexporter

import (
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/SigNoz/signoz-otel-collector/pkg/timebucketedset"
)

func TestBuildBatch(t *testing.T) {
	testCases := []struct {
		name          string
		logs          func() plog.Logs
		now           time.Time
		setup         func(t *testing.T, e *auditLogsExporter)
		wantLogs      []logRow
		wantResources []resourceRow
	}{
		{
			name: "StringBody_ScalarAttributes",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("service.name", "signoz")
				rl.Resource().Attributes().PutStr("signoz.resource.kind", "dashboard")
				rl.Resource().Attributes().PutStr("signoz.resource.id", "019a4f2e-7c1d-7b3a-9e5f-2d8c6a0b4e17")
				sl := rl.ScopeLogs().AppendEmpty()
				sl.Scope().SetName("signoz.audit")
				sl.Scope().SetVersion("1.0.0")
				record := sl.LogRecords().AppendEmpty()
				record.SetTimestamp(1790677977519692024)
				record.SetObservedTimestamp(1790677977522805958)
				record.SetTraceID(pcommon.TraceID{0xa4, 0xa3, 0x15, 0x37, 0x07, 0xd9, 0x13, 0x82, 0xff, 0x51, 0x06, 0xeb, 0xaf, 0x0c, 0x7a, 0x66})
				record.SetSpanID(pcommon.SpanID{0xbf, 0x16, 0x07, 0x71, 0xcd, 0xb3, 0x61, 0x42})
				record.SetFlags(plog.DefaultLogRecordFlags.WithIsSampled(true))
				record.SetSeverityText("INFO")
				record.SetSeverityNumber(plog.SeverityNumberInfo)
				record.SetEventName("signoz.audit.update")
				record.Body().SetStr("jane.doe@example.com updated dashboard")
				record.Attributes().PutStr("signoz.audit.verb", "update")
				record.Attributes().PutStr("signoz.audit.action_category", "configuration_changes")
				record.Attributes().PutStr("signoz.audit.outcome", "success")
				record.Attributes().PutStr("signoz.principal.type", "user")
				record.Attributes().PutInt("http.response.status_code", 200)
				record.Attributes().PutInt("client.port", 51234)
				return ld
			},
			now: time.Unix(0, 1790677980000000000),
			wantLogs: []logRow{{
				tsBucketStart:       1790677800,
				resourceFingerprint: resourceFingerprint(map[string]any{"service.name": "signoz", "signoz.resource.kind": "dashboard", "signoz.resource.id": "019a4f2e-7c1d-7b3a-9e5f-2d8c6a0b4e17"}),
				timestamp:           1790677977519692024,
				observedTimestamp:   1790677977522805958,
				traceID:             "a4a3153707d91382ff5106ebaf0c7a66",
				spanID:              "bf160771cdb36142",
				traceFlags:          1,
				severityText:        "INFO",
				severityNumber:      9,
				body:                `{"message":"jane.doe@example.com updated dashboard"}`,
				scope:               `{"attributes":{},"name":"signoz.audit","version":"1.0.0"}`,
				attributes:          `{"client.port":51234,"http.response.status_code":200,"signoz.audit.action_category":"configuration_changes","signoz.audit.outcome":"success","signoz.audit.verb":"update","signoz.principal.type":"user"}`,
				resource:            `{"attributes":{"service.name":"signoz","signoz.resource.id":"019a4f2e-7c1d-7b3a-9e5f-2d8c6a0b4e17","signoz.resource.kind":"dashboard"}}`,
				eventName:           "signoz.audit.update",
			}},
			wantResources: []resourceRow{{
				labels:              `{"service.name":"signoz","signoz.resource.id":"019a4f2e-7c1d-7b3a-9e5f-2d8c6a0b4e17","signoz.resource.kind":"dashboard"}`,
				fingerprint:         resourceFingerprint(map[string]any{"service.name": "signoz", "signoz.resource.kind": "dashboard", "signoz.resource.id": "019a4f2e-7c1d-7b3a-9e5f-2d8c6a0b4e17"}),
				seenAtTSBucketStart: 1790677800,
			}},
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
			name: "SameResource_OneResourceRow",
			logs: func() plog.Logs {
				ld := plog.NewLogs()
				records := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
				for _, ts := range []pcommon.Timestamp{1790700000000000000, 1790700001000000000} {
					record := records.AppendEmpty()
					record.SetTimestamp(ts)
					record.Body().SetStr("dashboard deleted")
					record.Attributes().PutStr("signoz.audit.verb", "delete")
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
					attributes:          `{"signoz.audit.verb":"delete"}`,
					resource:            `{"attributes":{}}`,
				},
				{
					tsBucketStart:       1790699400,
					resourceFingerprint: resourceFingerprint(map[string]any{}),
					timestamp:           1790700001000000000,
					observedTimestamp:   1790700002000000000,
					body:                `{"message":"dashboard deleted"}`,
					scope:               `{"attributes":{},"name":"","version":""}`,
					attributes:          `{"signoz.audit.verb":"delete"}`,
					resource:            `{"attributes":{}}`,
				},
			},
			wantResources: []resourceRow{{
				labels:              `{}`,
				fingerprint:         resourceFingerprint(map[string]any{}),
				seenAtTSBucketStart: 1790699400,
			}},
		},
		{
			name: "CachedResource_Skipped",
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
			setup: func(t *testing.T, e *auditLogsExporter) {
				id := []byte(resourceFingerprint(map[string]any{"service.name": "signoz-api"}))
				require.NoError(t, timebucketedset.RunSteps(e.resourceSet, []timebucketedset.Step{
					timebucketedset.PlanStep(id, 1790708400000, 1790710001000, nil, nil),
					timebucketedset.ApplyStep(id, 1790708400000),
				}))
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
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exp := newTestExporter(t)
			if testCase.setup != nil {
				testCase.setup(t, exp)
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
		})
	}
}
