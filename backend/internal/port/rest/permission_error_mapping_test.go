package rest

import (
	"net/http"
	"testing"

	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
)

func TestSharedResourcePermissionDenialsMapToForbidden(t *testing.T) {
	tests := []struct {
		name     string
		mapError func(error) (int, ErrDetail)
	}{
		{name: "task", mapError: taskErrorResponse},
		{name: "action item", mapError: actionItemErrorResponse},
		{name: "schedule", mapError: scheduleUseCaseError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			permissionDenied := error(taskusecase.ErrPermissionDenied)
			if test.name == "schedule" {
				permissionDenied = scheduleusecase.ErrPermissionDenied
			}
			status, detail := test.mapError(permissionDenied)
			if status != http.StatusForbidden || detail.Code != "permission_denied" {
				t.Fatalf("permission error maps to HTTP %d/code %q; want 403/permission_denied", status, detail.Code)
			}
		})
	}
}
