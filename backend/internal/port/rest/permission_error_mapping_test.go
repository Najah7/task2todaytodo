package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
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

func TestTeamSubscriptionErrorsMapToClearForbidden(t *testing.T) {
	for _, test := range []struct {
		name     string
		mapError func(error) (int, ErrDetail)
	}{
		{name: "project", mapError: projectUseCaseError},
		{name: "task", mapError: taskErrorResponse},
		{name: "action item", mapError: actionItemErrorResponse},
		{name: "schedule", mapError: scheduleUseCaseError},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, detail := test.mapError(projectusecase.ErrTeamSubscriptionRequired)
			if status != http.StatusForbidden || detail.Code != "team_required" {
				t.Fatalf("Team requirement maps to HTTP %d/code %q; want 403/team_required", status, detail.Code)
			}
		})
	}
}

func TestTodoTeamSubscriptionErrorMapsToClearForbidden(t *testing.T) {
	response := httptest.NewRecorder()
	writeTodoError(response, todoFailure, shared.ErrTeamSubscriptionRequired)
	var body ErrResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusForbidden || len(body.Error.Details) != 1 || body.Error.Details[0].Code != "team_required" {
		t.Fatalf("Todo Team requirement maps to HTTP %d/body %#v; want 403/team_required", response.Code, body.Error)
	}
}

func TestFreePlanLimitErrorsMapToConflictWithStableCodes(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		code     string
		mapError func(error) (int, ErrDetail)
	}{
		{name: "project", err: projectusecase.ErrFreeProjectLimitReached, code: "free_project_limit_reached", mapError: projectUseCaseError},
		{name: "task move", err: taskusecase.ErrFreeActionItemLimitReached, code: "free_action_item_limit_reached", mapError: taskErrorResponse},
		{name: "action item", err: taskusecase.ErrFreeActionItemLimitReached, code: "free_action_item_limit_reached", mapError: actionItemErrorResponse},
		{name: "project task", err: taskusecase.ErrFreeActionItemLimitReached, code: "free_action_item_limit_reached", mapError: projectUseCaseError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, detail := test.mapError(test.err)
			if status != http.StatusConflict || detail.Code != test.code {
				t.Fatalf("limit error maps to HTTP %d/code %q; want 409/%s", status, detail.Code, test.code)
			}
		})
	}
}
