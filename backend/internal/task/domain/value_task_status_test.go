package domain

import (
	"errors"
	"testing"
)

func TestNewTaskStatus(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    TaskStatus
		wantErr error
	}{
		{name: "open", value: "open", want: TaskStatus{Value: "open", Label: "Open", LabelJp: "オープン"}},
		{name: "pending", value: "pending", want: TaskStatus{Value: "pending", Label: "Pending", LabelJp: "保留"}},
		{name: "waiting on others", value: "waiting_on_others", want: TaskStatus{Value: "waiting_on_others", Label: "Waiting on others", LabelJp: "他者待ち"}},
		{name: "in progress", value: "in_progress", want: TaskStatus{Value: "in_progress", Label: "In progress", LabelJp: "進行中"}},
		{name: "done", value: "done", want: TaskStatus{Value: "done", Label: "Done", LabelJp: "完了"}},
		{name: "empty", wantErr: ErrTaskStatusEmpty},
		{name: "invalid", value: "archived", wantErr: ErrTaskStatusInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTaskStatus(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewTaskStatus() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("task status = %+v, want %+v", got, tt.want)
			}
			if err == nil && got.String() != tt.want.Value {
				t.Errorf("task status string = %q, want %q", got.String(), tt.want.Value)
			}
		})
	}
}

func TestTaskStatusPredicates(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		wantOpen       bool
		wantPending    bool
		wantWaiting    bool
		wantInProgress bool
		wantDone       bool
	}{
		{name: "open", value: "open", wantOpen: true},
		{name: "pending", value: "pending", wantPending: true},
		{name: "waiting on others", value: "waiting_on_others", wantWaiting: true},
		{name: "in progress", value: "in_progress", wantInProgress: true},
		{name: "done", value: "done", wantDone: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := NewTaskStatus(tt.value)
			if err != nil {
				t.Fatalf("NewTaskStatus() error = %v", err)
			}

			if status.IsOpen() != tt.wantOpen ||
				status.IsPending() != tt.wantPending ||
				status.IsWaitingOnOthers() != tt.wantWaiting ||
				status.IsInProgress() != tt.wantInProgress ||
				status.IsDone() != tt.wantDone {
				t.Fatalf("status predicates for %q = open:%t pending:%t waiting:%t in_progress:%t done:%t",
					tt.value,
					status.IsOpen(),
					status.IsPending(),
					status.IsWaitingOnOthers(),
					status.IsInProgress(),
					status.IsDone(),
				)
			}
		})
	}
}
