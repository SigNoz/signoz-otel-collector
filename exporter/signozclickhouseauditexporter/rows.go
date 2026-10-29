package signozclickhouseauditexporter

import (
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"github.com/segmentio/ksuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
)

const bucketSeconds = 1800

type logRow struct {
	tsBucketStart       uint64
	resourceFingerprint string
	timestamp           uint64
	observedTimestamp   uint64
	id                  string
	traceID             string
	spanID              string
	traceFlags          uint32
	severityText        string
	severityNumber      uint8
	body                string
	scope               string
	attributes          string
	resource            string
	eventName           string
}

type resourceRow struct {
	labels              string
	fingerprint         string
	seenAtTSBucketStart int64
}

type batch struct {
	logs      []logRow
	resources []resourceRow
}

// batchBuilder converts plog.Logs into rows. Resources already in the exporter cache are skipped.
type batchBuilder struct {
	exp          *auditExporter
	batch        batch
	fingerprints map[int64]map[string]string
}

func (e *auditExporter) buildBatch(ld plog.Logs, now time.Time) (*batch, error) {
	b := &batchBuilder{
		exp:          e,
		fingerprints: map[int64]map[string]string{},
	}

	resourceLogs := ld.ResourceLogs()
	for i := range resourceLogs.Len() {
		if err := b.addResourceLogs(resourceLogs.At(i), now); err != nil {
			return nil, err
		}
	}

	return &b.batch, nil
}

func (b *batchBuilder) addResourceLogs(rl plog.ResourceLogs, now time.Time) error {
	res := rl.Resource()
	resourceJSON, err := marshalJSON(map[string]any{"attributes": res.Attributes().AsRaw()})
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}
	labels, err := resourceLabels(res.Attributes())
	if err != nil {
		return fmt.Errorf("failed to marshal resource labels: %w", err)
	}

	scopeLogs := rl.ScopeLogs()
	for i := range scopeLogs.Len() {
		sl := scopeLogs.At(i)
		scope := sl.Scope()
		scopeJSON, err := marshalJSON(map[string]any{
			"name":       scope.Name(),
			"version":    scope.Version(),
			"attributes": scope.Attributes().AsRaw(),
		})
		if err != nil {
			return fmt.Errorf("failed to marshal scope: %w", err)
		}

		records := sl.LogRecords()
		for j := range records.Len() {
			record := records.At(j)

			ts := uint64(record.Timestamp())
			ots := uint64(record.ObservedTimestamp())
			if ots == 0 {
				ots = uint64(now.UnixNano())
			}
			if ts == 0 {
				ts = ots
			}

			id, err := ksuid.NewRandomWithTime(time.Unix(0, int64(ts)))
			if err != nil {
				return fmt.Errorf("failed to generate id: %w", err)
			}

			bucket := int64(ts/uint64(time.Second)) / bucketSeconds * bucketSeconds
			fp := b.fingerprint(bucket, labels, res.Attributes())

			body, err := bodyJSON(record.Body())
			if err != nil {
				return fmt.Errorf("failed to marshal body: %w", err)
			}
			attributes, err := marshalJSON(record.Attributes().AsRaw())
			if err != nil {
				return fmt.Errorf("failed to marshal attributes: %w", err)
			}

			b.batch.logs = append(b.batch.logs, logRow{
				tsBucketStart:       uint64(bucket),
				resourceFingerprint: fp,
				timestamp:           ts,
				observedTimestamp:   ots,
				id:                  id.String(),
				traceID:             utils.TraceIDToHexOrEmptyString(record.TraceID()),
				spanID:              utils.SpanIDToHexOrEmptyString(record.SpanID()),
				traceFlags:          uint32(record.Flags()),
				severityText:        record.SeverityText(),
				severityNumber:      uint8(record.SeverityNumber()),
				body:                body,
				scope:               scopeJSON,
				attributes:          attributes,
				resource:            resourceJSON,
				eventName:           record.EventName(),
			})
		}
	}

	return nil
}

func (b *batchBuilder) fingerprint(bucket int64, labels string, attrs pcommon.Map) string {
	byLabels, ok := b.fingerprints[bucket]
	if !ok {
		byLabels = map[string]string{}
		b.fingerprints[bucket] = byLabels
	}
	if fp, ok := byLabels[labels]; ok {
		return fp
	}

	fp := fingerprint.CalculateFingerprint(attrs.AsRaw(), fingerprint.AuditResourceHierarchy())
	byLabels[labels] = fp
	if b.exp.rfCache.Get(utils.MakeKeyForRFCache(bucket, fp)) == nil {
		b.batch.resources = append(b.batch.resources, resourceRow{labels: labels, fingerprint: fp, seenAtTSBucketStart: bucket})
	}

	return fp
}

// bodyJSON keeps a map body as is and wraps any other body as {"message": <body as string>}.
func bodyJSON(body pcommon.Value) (string, error) {
	if body.Type() == pcommon.ValueTypeMap {
		return marshalJSON(body.Map().AsRaw())
	}
	return marshalJSON(map[string]any{"message": body.AsString()})
}

func resourceLabels(attrs pcommon.Map) (string, error) {
	labels := make(map[string]string, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})
	b, err := json.Marshal(labels)
	return string(b), err
}

func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(utils.SanitizeJSON(v))
	return string(b), err
}
