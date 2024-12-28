package payloadmeter

import (
	"encoding/json"

	"go.uber.org/zap"
)

type jsonSizer struct {
	Logger *zap.Logger
}

func NewJSONSizer(logger *zap.Logger) *jsonSizer {
	return &jsonSizer{
		Logger: logger,
	}
}

func (sizer *jsonSizer) SizeOfMapStringAny(input map[string]any) int {
	bytes, err := json.Marshal(input)
	if err != nil {
		sizer.Logger.Error("cannot marshal object, setting size to 0", zap.Any("obj", input))
		return 0
	}

	return len(bytes)
}
