package domain

import (
	"errors"
	"testing"
)

func TestNewTaskFrequency(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    TaskFrequency
		wantErr error
	}{
		{name: "monday", value: "mon", want: TaskFrequency{Value: "mon", Label: "Monday", LabelJp: "月曜日"}},
		{name: "tuesday", value: "tue", want: TaskFrequency{Value: "tue", Label: "Tuesday", LabelJp: "火曜日"}},
		{name: "wednesday", value: "wed", want: TaskFrequency{Value: "wed", Label: "Wednesday", LabelJp: "水曜日"}},
		{name: "thursday", value: "thu", want: TaskFrequency{Value: "thu", Label: "Thursday", LabelJp: "木曜日"}},
		{name: "friday", value: "fri", want: TaskFrequency{Value: "fri", Label: "Friday", LabelJp: "金曜日"}},
		{name: "saturday", value: "sat", want: TaskFrequency{Value: "sat", Label: "Saturday", LabelJp: "土曜日"}},
		{name: "sunday", value: "sun", want: TaskFrequency{Value: "sun", Label: "Sunday", LabelJp: "日曜日"}},
		{name: "empty", wantErr: ErrTaskFrequencyEmpty},
		{name: "invalid", value: "daily", wantErr: ErrTaskFrequencyInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTaskFrequency(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewTaskFrequency() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("task frequency = %+v, want %+v", got, tt.want)
			}
			if err == nil && got.String() != tt.want.Value {
				t.Errorf("task frequency string = %q, want %q", got.String(), tt.want.Value)
			}
		})
	}
}

func mustTaskFrequency(t *testing.T, value string) TaskFrequency {
	t.Helper()
	frequency, err := NewTaskFrequency(value)
	if err != nil {
		t.Fatalf("NewTaskFrequency() error = %v", err)
	}
	return frequency
}
