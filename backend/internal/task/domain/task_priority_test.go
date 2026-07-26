package domain

import (
	"errors"
	"testing"
)

func TestNewTaskPriority(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    TaskPriority
		wantErr error
	}{
		{name: "urgent", value: "urgent", want: TaskPriority{Value: "urgent", Label: "Urgent", LabelJp: "緊急", Weight: 100}},
		{name: "high", value: "high", want: TaskPriority{Value: "high", Label: "High", LabelJp: "高", Weight: 50}},
		{name: "medium", value: "medium", want: TaskPriority{Value: "medium", Label: "Medium", LabelJp: "中", Weight: 25}},
		{name: "low", value: "low", want: TaskPriority{Value: "low", Label: "Low", LabelJp: "低", Weight: 10}},
		{name: "someday", value: "someday", want: TaskPriority{Value: "someday", Label: "Someday", LabelJp: "いつか", Weight: 0}},
		{name: "empty", wantErr: ErrTaskPriorityEmpty},
		{name: "invalid", value: "blocked", wantErr: ErrTaskPriorityInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTaskPriority(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewTaskPriority() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("task priority = %+v, want %+v", got, tt.want)
			}
			if err == nil && got.String() != tt.want.Value {
				t.Errorf("task priority string = %q, want %q", got.String(), tt.want.Value)
			}
		})
	}
}
