// Package recurrence contains recurrence cadence and weekday policy shared by
// application contexts.
package recurrence

import "errors"

var (
	ErrFrequencyEmpty   = errors.New("frequency cannot be empty")
	ErrFrequencyInvalid = errors.New("invalid task frequency")
)

const (
	OnceIntervalWeeks       = 0
	WeeklyIntervalWeeks     = 1
	BiWeeklyIntervalWeeks   = 2
	FourWeekIntervalWeeks   = 4
	QuarterlyIntervalWeeks  = 13
	SemiAnnualIntervalWeeks = 26
	AnnualIntervalWeeks     = 52
)

type Frequency struct {
	Value   string
	Label   string
	LabelJp string
}

var frequencies = map[string]Frequency{
	"mon": {Value: "mon", Label: "Monday", LabelJp: "月曜日"},
	"tue": {Value: "tue", Label: "Tuesday", LabelJp: "火曜日"},
	"wed": {Value: "wed", Label: "Wednesday", LabelJp: "水曜日"},
	"thu": {Value: "thu", Label: "Thursday", LabelJp: "木曜日"},
	"fri": {Value: "fri", Label: "Friday", LabelJp: "金曜日"},
	"sat": {Value: "sat", Label: "Saturday", LabelJp: "土曜日"},
	"sun": {Value: "sun", Label: "Sunday", LabelJp: "日曜日"},
}

func NewFrequency(value string) (Frequency, error) {
	frequency, ok := frequencies[value]
	if !ok {
		frequency = Frequency{Value: value}
	}
	if err := frequency.validate(); err != nil {
		return Frequency{}, err
	}
	return frequency, nil
}

func (frequency Frequency) validate() error {
	if frequency.String() == "" {
		return ErrFrequencyEmpty
	}
	if _, ok := frequencies[frequency.Value]; !ok {
		return ErrFrequencyInvalid
	}
	return nil
}

func (frequency Frequency) String() string { return frequency.Value }

type Frequencies []Frequency

func (frequencies Frequencies) IsWeekday() bool {
	for _, frequency := range frequencies {
		switch frequency.Value {
		case "mon", "tue", "wed", "thu", "fri":
			return true
		}
	}
	return false
}

func (frequencies Frequencies) IsWeekend() bool {
	for _, frequency := range frequencies {
		switch frequency.Value {
		case "sat", "sun":
			return true
		}
	}
	return false
}
