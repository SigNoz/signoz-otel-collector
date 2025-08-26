package metering

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-json"

	"github.com/SigNoz/signoz-otel-collector/pkg/schema/traces"
	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/SigNoz/signoz-otel-collector/utils/flatten"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

type jsonSizer struct {
	Logger         *zap.Logger
	ExcludePattern *regexp.Regexp
}

type jsonSizerOptions struct {
	ExcludePattern *regexp.Regexp
}

type jsonSizerOption func(*jsonSizerOptions)

func WithExcludePattern(pattern *regexp.Regexp) jsonSizerOption {
	return func(opts *jsonSizerOptions) {
		opts.ExcludePattern = pattern
	}
}

func NewJSONSizer(logger *zap.Logger, options ...jsonSizerOption) Sizer {
	opts := &jsonSizerOptions{}
	for _, opt := range options {
		opt(opts)
	}
	return &jsonSizer{
		Logger:         logger,
		ExcludePattern: opts.ExcludePattern,
	}
}

func (sizer *jsonSizer) SizeOfMapStringAny(input map[string]any) int {
	output := map[string]any{}

	if sizer.ExcludePattern != nil {
		for key, value := range input {
			if sizer.ExcludePattern.MatchString(key) {
				continue
			}
			output[key] = value
		}
	} else {
		// skip the looping through if the exclude pattern isn't configured
		output = input
	}

	bytes, err := json.Marshal(output)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Error(err), zap.Any("obj", output))
		return 0
	}

	return len(bytes)
}

func (sizer *jsonSizer) SizeOfFlatPcommonMapInMapStringString(input pcommon.Map) int {
	output := map[string]string{}

	input.Range(func(k string, v pcommon.Value) bool {
		if sizer.ExcludePattern != nil && sizer.ExcludePattern.MatchString(k) {
			return true
		}
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

func (sizer *jsonSizer) SizeOfEvents(input []string) int {
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

	escapeCharacters := strings.Count(string(bytes), "\"")
	return len(bytes) + escapeCharacters
}

func (size *jsonSizer) TotalSizeIfKeyExists(key int, value int, extra int) int {
	if value == 0 {
		return 0
	}

	return key + value + extra
}

func (size *jsonSizer) TotalSizeIfKeyExistsAndValueIsMapOrSlice(key int, value int, extra int) int {
	if value <= 2 {
		return 0
	}

	return key + value + extra
}
