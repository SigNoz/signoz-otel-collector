package signozclickhouseauditexporter

import (
	"time"

	driver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/SigNoz/signoz-otel-collector/pkg/keycheck"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/fingerprint"
)

type typedAttributes struct {
	StringData map[string]string
	NumberData map[string]float64
	BoolData   map[string]bool
}

func convertAttributes(attributes pcommon.Map, forceStringValues bool) typedAttributes {
	result := typedAttributes{
		StringData: make(map[string]string),
		NumberData: make(map[string]float64),
		BoolData:   make(map[string]bool),
	}
	attributes.Range(func(k string, v pcommon.Value) bool {
		if forceStringValues {
			result.StringData[k] = v.AsString()
		} else {
			switch v.Type() {
			case pcommon.ValueTypeInt:
				result.NumberData[k] = float64(v.Int())
			case pcommon.ValueTypeDouble:
				result.NumberData[k] = v.Double()
			case pcommon.ValueTypeBool:
				result.BoolData[k] = v.Bool()
			default:
				result.StringData[k] = v.AsString()
			}
		}
		return true
	})
	return result
}

func (e *logsExporter) appendTagAttributes(
	tagStmt driver.Batch,
	attrKeysStmt driver.Batch,
	resourceKeysStmt driver.Batch,
	tagType utils.TagType,
	attrs typedAttributes,
) {
	unixMilli := (time.Now().UnixMilli() / 3600000) * 3600000
	now := time.Now()

	for key, val := range attrs.StringData {
		if keycheck.IsRandomKey(key) {
			continue
		}
		e.appendAttributeKey(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeString, now)

		// 7 columns: unix_milli, tag_key, tag_type, tag_data_type, string_value, int64_value, float64_value
		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeString, val, (*int64)(nil), (*float64)(nil))
	}

	for key, val := range attrs.NumberData {
		if keycheck.IsRandomKey(key) {
			continue
		}
		e.appendAttributeKey(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeNumber, now)

		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeNumber, "", (*int64)(nil), &val)
	}

	for key := range attrs.BoolData {
		if keycheck.IsRandomKey(key) {
			continue
		}
		e.appendAttributeKey(attrKeysStmt, resourceKeysStmt, key, tagType, utils.TagDataTypeBool, now)

		_ = tagStmt.Append(unixMilli, key, tagType, utils.TagDataTypeBool, "", (*int64)(nil), (*float64)(nil))
	}
}

func (e *logsExporter) appendAttributeKey(
	attrKeysStmt driver.Batch,
	resourceKeysStmt driver.Batch,
	key string,
	tagType utils.TagType,
	datatype utils.TagDataType,
	now time.Time,
) {
	if keycheck.IsRandomKey(key) {
		return
	}
	cacheKey := utils.MakeKeyForAttributeKeys(key, tagType, datatype)
	if item := e.keysCache.Get(cacheKey); item != nil {
		return
	}

	// Audit keys tables have 3 columns: name, datatype, timestamp (no DEFAULT on timestamp)
	switch tagType {
	case utils.TagTypeResource:
		_ = resourceKeysStmt.Append(key, string(datatype), now)
	case utils.TagTypeAttribute:
		_ = attrKeysStmt.Append(key, string(datatype), now)
	}
	e.keysCache.Set(cacheKey, struct{}{}, ttlcache.DefaultTTL)
}

func bucketTimestamp(ts int64, bucketSize int64) int64 {
	return (ts / bucketSize) * bucketSize
}

func resolveFingerprint(
	resourcesSeen map[int64]map[string]string,
	bucket int64,
	resourceJSON string,
	res pcommon.Resource,
) string {
	inner, ok := resourcesSeen[bucket]
	if !ok {
		inner = make(map[string]string)
		resourcesSeen[bucket] = inner
	}
	if fp, exists := inner[resourceJSON]; exists {
		return fp
	}
	fp := fingerprint.CalculateFingerprint(res.Attributes().AsRaw(), fingerprint.ResourceHierarchy())
	inner[resourceJSON] = fp
	return fp
}
