package recurrence

import (
	"errors"
	"testing"
)

func TestNewFrequency(t *testing.T) {
	got, err := NewFrequency("mon")
	if err != nil {
		t.Fatal(err)
	}
	if got != (Frequency{Value: "mon", Label: "Monday", LabelJp: "月曜日"}) {
		t.Fatalf("NewFrequency() = %+v, want Monday labels", got)
	}
	if got, err := NewFrequency(""); !errors.Is(err, ErrFrequencyEmpty) || got != (Frequency{}) {
		t.Fatalf("NewFrequency(empty) = %+v, %v; want zero and ErrFrequencyEmpty", got, err)
	}
	if got, err := NewFrequency("daily"); !errors.Is(err, ErrFrequencyInvalid) || got != (Frequency{}) {
		t.Fatalf("NewFrequency(daily) = %+v, %v; want zero and ErrFrequencyInvalid", got, err)
	}
}

func TestFrequenciesWeekdayAndWeekend(t *testing.T) {
	if !(Frequencies{{Value: "mon"}, {Value: "fri"}}).IsWeekday() {
		t.Fatal("weekday frequencies not recognized")
	}
	if !(Frequencies{{Value: "sat"}}).IsWeekend() {
		t.Fatal("weekend frequency not recognized")
	}
}
