package signozclickhouseauditexporter

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

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

func bucketTimestamp(ts int64, bucketSize int64) int64 {
	return (ts / bucketSize) * bucketSize
}

func resolveFingerprint(resourcesSeen map[int64]map[string]string, bucket int64, resourceJSON string, res pcommon.Resource) string {
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
