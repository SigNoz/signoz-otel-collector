package entity

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/SigNoz/signoz-otel-collector/pkg/errors"
)

// Period represents a value object representing specific time periods,
// such as second or minute.
// It implements the driver.Valuer and sql.Scanner interfaces for
// working with databases,and the json.Marshaler and json.Unmarshaler
// interfaces for JSON serialization and deserialization.
type Period struct {
	name string
}

var (
	PeriodSecond = Period{name: "second"}
	PeriodMinute = Period{name: "minute"}
	PeriodHour   = Period{name: "hour"}
	PeriodDay    = Period{name: "day"}
	PeriodMonth  = Period{name: "month"}
	PeriodYear   = Period{name: "year"}
)

func NewPeriod(name string) (Period, error) {
	switch name {
	case "second":
		return PeriodSecond, nil
	case "minute":
		return PeriodMinute, nil
	case "hour":
		return PeriodHour, nil
	case "day":
		return PeriodDay, nil
	case "month":
		return PeriodMonth, nil
	case "year":
		return PeriodYear, nil
	default:
		return Period{}, errors.Newf(errors.TypeUnsupported, "period %s is not supported", name)
	}
}

func (p Period) String() string {
	return p.name
}

func SupportedPeriods() map[Period]bool {
	return map[Period]bool{
		PeriodSecond: true,
		PeriodMinute: true,
		PeriodHour:   true,
		PeriodDay:    true,
		PeriodMonth:  true,
		PeriodYear:   true,
	}
}

func (p Period) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *Period) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	period, err := NewPeriod(str)
	if err != nil {
		return err
	}

	*p = period
	return nil
}

func (p Period) Value() (driver.Value, error) {
	return p.String(), nil
}

func (p *Period) Scan(value interface{}) error {
	if value == nil {
		return errors.New(errors.TypeInternal, "value cannot be nil")
	}

	str, ok := value.(string)
	if !ok {
		return errors.Newf(errors.TypeInternal, "failed to scan id %v", value)
	}

	period, err := NewPeriod(str)
	if err != nil {
		return err
	}

	*p = period
	return nil
}

// Returns a map containing time at different precision levels.
// For example, consider the value for PeriodMinute. At an input time t,
// PeriodMinute denotes the time with the second precision removed.
func PeriodAt(t time.Time) map[Period]time.Time {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()

	return map[Period]time.Time{
		PeriodSecond: time.Date(year, month, day, hour, min, sec, 0, time.Local),
		PeriodMinute: time.Date(year, month, day, hour, min, 0, 0, time.Local),
		PeriodHour:   time.Date(year, month, day, hour, 0, 0, 0, time.Local),
		PeriodDay:    time.Date(year, month, day, 0, 0, 0, 0, time.Local),
		PeriodMonth:  time.Date(year, month, 1, 0, 0, 0, 0, time.Local),
		PeriodYear:   time.Date(year, 1, 1, 0, 0, 0, 0, time.Local),
	}
}
