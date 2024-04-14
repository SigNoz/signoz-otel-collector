package entity

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
	"go.opentelemetry.io/collector/component"
)

// Signal is a value object built as a wrapper on top of component.DataType
// to make it work with JSON and databases.
// It supports all signals supported by opentelemetry.
type Signal struct {
	name     string
	dataType component.DataType
}

var (
	SignalTraces  = Signal{name: string(component.DataTypeTraces), dataType: component.DataTypeTraces}
	SignalLogs    = Signal{name: string(component.DataTypeLogs), dataType: component.DataTypeLogs}
	SignalMetrics = Signal{name: string(component.DataTypeMetrics), dataType: component.DataTypeMetrics}
)

func NewSignal(name string) (Signal, error) {
	switch name {
	case string(component.DataTypeTraces):
		return SignalTraces, nil
	case string(component.DataTypeLogs):
		return SignalLogs, nil
	case string(component.DataTypeMetrics):
		return SignalMetrics, nil
	default:
		return Signal{}, errors.Newf(errors.TypeUnsupported, "signal %s is not supported", name)
	}
}

func (s Signal) String() string {
	return s.name
}

func (s Signal) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Signal) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	signal, err := NewSignal(str)
	if err != nil {
		return err
	}

	*s = signal
	return nil
}

func (s Signal) Value() (driver.Value, error) {
	return s.String(), nil
}

func (s *Signal) Scan(value interface{}) error {
	if value == nil {
		return errors.New(errors.TypeInternal, "value cannot be nil")
	}

	str, ok := value.(string)
	if !ok {
		return errors.Newf(errors.TypeInternal, "failed to scan id %v", value)
	}

	signal, err := NewSignal(str)
	if err != nil {
		return err
	}

	*s = signal
	return nil
}
