package rest

import (
	"net/http"
	"testing"

	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
)

func TestSharedResourcePermissionDenialsMapToForbidden(t *testing.T) {
	tests := []struct {
		name     string
		mapError func(error) (int, ErrDetail)
	}{
		{name: "task", mapError: taskErrorResponse},
		{name: "todo item", mapError: todoItemErrorResponse},
		{name: "task schedule", mapError: taskScheduleUseCaseError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, detail := test.mapError(taskusecase.ErrPermissionDenied)
			if status != http.StatusForbidden || detail.Code != "permission_denied" {
				t.Fatalf("permission error maps to HTTP %d/code %q; want 403/permission_denied", status, detail.Code)
			}
		})
	}
}
