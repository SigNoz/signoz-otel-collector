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

const (
	bucketSeconds = 1800

	fieldContextAttribute = "attribute"
	fieldContextResource  = "resource"
	fieldContextScope     = "scope"
)

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

type fieldKeyRow struct {
	name          string
	fieldContext  string
	fieldDataType string
}

// fieldRow.value holds a string, int64, float64 or bool so the Dynamic column keeps the type.
type fieldRow struct {
	name          string
	fieldContext  string
	fieldDataType string
	value         any
}

type batch struct {
	logs      []logRow
	resources []resourceRow
	fieldKeys []fieldKeyRow
	fields    []fieldRow
}

// batchBuilder converts plog.Logs into rows. Rows already present in the exporter caches are skipped.
type batchBuilder struct {
	exp          *auditExporter
	batch        batch
	fingerprints map[int64]map[string]string
	fieldKeys    map[fieldKeyRow]struct{}
	fields       map[fieldRow]struct{}
}

func (e *auditExporter) buildBatch(ld plog.Logs, now time.Time) (*batch, error) {
	b := &batchBuilder{
		exp:          e,
		fingerprints: map[int64]map[string]string{},
		fieldKeys:    map[fieldKeyRow]struct{}{},
		fields:       map[fieldRow]struct{}{},
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
	b.addFields(fieldContextResource, res.Attributes())

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
		b.addFields(fieldContextScope, scope.Attributes())

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
			b.addFields(fieldContextAttribute, record.Attributes())

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

	fp := fingerprint.CalculateFingerprint(attrs.AsRaw(), fingerprint.ResourceHierarchy())
	byLabels[labels] = fp
	if b.exp.rfCache.Get(utils.MakeKeyForRFCache(bucket, fp)) == nil {
		b.batch.resources = append(b.batch.resources, resourceRow{labels: labels, fingerprint: fp, seenAtTSBucketStart: bucket})
	}

	return fp
}

func (b *batchBuilder) addFields(fieldContext string, attrs pcommon.Map) {
	attrs.Range(func(name string, v pcommon.Value) bool {
		dataType, value, ok := scalar(v)
		if !ok {
			return true
		}

		key := fieldKeyRow{name: name, fieldContext: fieldContext, fieldDataType: dataType}
		if _, seen := b.fieldKeys[key]; !seen {
			b.fieldKeys[key] = struct{}{}
			if b.exp.keysCache.Get(fieldKeyCacheKey(key)) == nil {
				b.batch.fieldKeys = append(b.batch.fieldKeys, key)
			}
		}

		field := fieldRow{name: name, fieldContext: fieldContext, fieldDataType: dataType, value: value}
		if _, seen := b.fields[field]; !seen {
			b.fields[field] = struct{}{}
			b.batch.fields = append(b.batch.fields, field)
		}

		return true
	})
}

// scalar returns the field data type and Go value of v. Slices, maps, bytes and non-finite floats are not reported.
func scalar(v pcommon.Value) (string, any, bool) {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return string(utils.FieldDataTypeString), v.Str(), true
	case pcommon.ValueTypeInt:
		return string(utils.FieldDataTypeInt64), v.Int(), true
	case pcommon.ValueTypeDouble:
		if !utils.IsValidFloat(v.Double()) {
			return "", nil, false
		}
		return string(utils.FieldDataTypeFloat64), v.Double(), true
	case pcommon.ValueTypeBool:
		return string(utils.FieldDataTypeBool), v.Bool(), true
	default:
		return "", nil, false
	}
}

func fieldKeyCacheKey(key fieldKeyRow) string {
	return key.name + ":" + key.fieldContext + ":" + key.fieldDataType
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
