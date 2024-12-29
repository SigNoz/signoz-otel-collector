package metering

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/SigNoz/signoz-otel-collector/pkg/schema/traces"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type jsonSizer struct {
	Logger *zap.Logger
}

func NewJSONSizer(logger *zap.Logger) Sizer {
	return &jsonSizer{
		Logger: logger,
	}
}

func (sizer *jsonSizer) SizeOfMapStringAny(input map[string]any) int {
	bytes, err := json.Marshal(input)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", input))
		return 0
	}

	return len(bytes)
}

func (sizer *jsonSizer) SizeOfFlatPcommonMapInMapStringString(input pcommon.Map) int {
	output := map[string]string{}

	input.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeMap:
			flattened := flatten.FlattenJSON(v.Map().AsRaw(), k)
			for kf, vf := range flattened {
				output[kf] = fmt.Sprintf("%v", vf)
			}
		default:
			output[k] = v.AsString()
		}
		return true
	})

	bytes, err := json.Marshal(output)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", input))
		return 0
	}

	return len(bytes)
}

func (sizer *jsonSizer) SizeOfFlatPcommonMapInNumberStringBool(input pcommon.Map) (int, int, int) {
	n := map[string]float64{}
	s := map[string]string{}
	b := map[string]bool{}

	input.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeDouble:
			if utils.IsValidFloat(v.Double()) {
				n[k] = v.Double()
			}
		case pcommon.ValueTypeInt:
			n[k] = float64(v.Int())
		case pcommon.ValueTypeBool:
			b[k] = v.Bool()
		case pcommon.ValueTypeMap:
			flattened := flatten.FlattenJSON(v.Map().AsRaw(), k)
			for kf, vf := range flattened {
				switch tvf := vf.(type) {
				case string:
					s[kf] = tvf
				case float64:
					n[kf] = tvf
				case bool:
					b[kf] = tvf
				}
			}
		default:
			s[k] = v.AsString()
		}

		return true
	})

	nbytes, err := json.Marshal(n)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", input))
		nbytes = []byte(nil)
	}

	sbytes, err := json.Marshal(s)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", input))
		sbytes = []byte(nil)
	}

	bbytes, err := json.Marshal(b)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", input))
		bbytes = []byte(nil)
	}

	return len(nbytes), len(sbytes), len(bbytes)
}

func (sizer *jsonSizer) SizeOfInt(input int) int {
	return len(strconv.Itoa(input))
}

func (sizer *jsonSizer) SizeOfFloat64(input float64) int {
	return len(strconv.FormatFloat(input, 'f', -1, 64))
}

func (sizer *jsonSizer) SizeOfTraceID(input pcommon.TraceID) int {
	if input.IsEmpty() {
		return 0
	}

	// Since we encode to hex, the original 16 bytes are stored in 32 bytes
	return 32
}

func (sizer *jsonSizer) SizeOfSpanID(input pcommon.SpanID) int {
	if input.IsEmpty() {
		return 0
	}

	// Since we encode to hex, the original 8 bytes are stored in 16 bytes
	return 16
}

func (sizer *jsonSizer) SizeOfStringSlice(input []string) int {
	bytes, err := json.Marshal(input)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", input))
		return 0
	}

	return len(bytes)
}

func (sizer *jsonSizer) SizeOfOtelSpanRefs(input []traces.OtelSpanRef) int {
	if input == nil {
		return 0
	}

	bytes, err := json.Marshal(input)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", input))
		return 0
	}

	return len(bytes)
}
