package application

import (
	"errors"
	"testing"

	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	todousecase "github.com/Najah7/task2todaytodo/internal/application/todo/usecase"
)

func TestMapTodoTaskOwnerError(t *testing.T) {
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"source missing", taskusecase.ErrActionItemNotFound, todousecase.ErrActionItemNotFound},
		{"task missing", taskusecase.ErrTaskNotFound, todousecase.ErrActionItemNotFound},
		{"permission", taskusecase.ErrPermissionDenied, todousecase.ErrPermissionDenied},
		{"revision", taskusecase.ErrRevisionConflict, todousecase.ErrRevisionConflict},
		{"started", taskusecase.ErrActionItemStarted, todousecase.ErrActionItemStarted},
		{"unexpected", errors.New("database unavailable"), errors.New("database unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := mapTodoTaskOwnerError(test.in)
			if test.name == "unexpected" {
				if got == nil || got.Error() != test.want.Error() {
					t.Fatalf("got %v, want unexpected error %q", got, test.want)
				}
			} else if got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestMapTodoScheduleOwnerError(t *testing.T) {
	tests := []struct {
		name string
		in   error
		want error
	}{
		{"missing", scheduleusecase.ErrOccurrenceNotFound, todousecase.ErrFrameNotFound},
		{"inactive", scheduleusecase.ErrOccurrenceInactive, todousecase.ErrFrameNotFound},
		{"permission", scheduleusecase.ErrPermissionDenied, todousecase.ErrPermissionDenied},
		{"revision", scheduleusecase.ErrOccurrenceRevisionMismatch, todousecase.ErrRevisionConflict},
		{"completed", scheduleusecase.ErrOccurrenceCompleted, todousecase.ErrScheduleCompleted},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := mapTodoScheduleOwnerError(test.in); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}
