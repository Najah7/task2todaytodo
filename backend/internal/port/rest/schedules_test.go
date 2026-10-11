package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	scheduledomain "github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/go-chi/chi/v5"
)

func TestScheduleHandlerRejectsUnauthenticatedBlueprintCreate(t *testing.T) {
	handler := NewScheduleHandler(scheduleusecase.ScheduleUseCases{}, nil)
	response := httptest.NewRecorder()
	handler.CreateBlueprint(response, httptest.NewRequest(http.MethodPost, "/schedule-blueprints", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create status = %d, want 401", response.Code)
	}
}

func TestScheduleHandlerRequiresCompoundOccurrenceIdentity(t *testing.T) {
	handler := NewScheduleHandler(scheduleusecase.ScheduleUseCases{}, nil)
	ctx := context.WithValue(context.Background(), UserIDContextKey, "user-1")
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("blueprint_id", "blueprint-1")
	request := httptest.NewRequest(http.MethodGet, "/schedules/blueprint-1/", nil)
	request = request.WithContext(context.WithValue(ctx, chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()
	handler.GetOccurrence(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing occurrence date status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestScheduleUseCaseErrorsMapBlueprintValidationAndRevisionConflicts(t *testing.T) {
	for _, test := range []struct {
		name  string
		err   error
		field string
		code  string
	}{
		{"title", scheduledomain.ErrBlueprintTitleEmpty, "title", "required"},
		{"timezone", scheduledomain.ErrBlueprintTimezoneEmpty, "timezone", "required"},
		{"frequency", scheduledomain.ErrBlueprintFrequency, "frequency", "invalid_frequency"},
		{"interval", scheduledomain.ErrBlueprintInterval, "interval", "invalid_interval"},
		{"weekdays", scheduledomain.ErrBlueprintWeekdays, "weekdays", "invalid_weekdays"},
		{"end date", scheduledomain.ErrBlueprintDateRange, "end_date", "invalid_date_range"},
		{"time range", scheduledomain.ErrBlueprintTimeRange, "end_at", "invalid_time_range"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, detail := scheduleUseCaseError(test.err)
			if status != http.StatusBadRequest || detail.Field != test.field || detail.Code != test.code {
				t.Fatalf("mapping = %d/%+v", status, detail)
			}
		})
	}
	status, detail := scheduleUseCaseError(scheduleusecase.ErrOccurrenceRevisionMismatch)
	if status != http.StatusPreconditionFailed || detail.Field != "If-Match" {
		t.Fatalf("revision mismatch mapping = %d/%+v", status, detail)
	}
	status, detail = scheduleUseCaseError(scheduleusecase.ErrPermissionDenied)
	if status != http.StatusForbidden || detail.Code != "permission_denied" {
		t.Fatalf("permission mapping = %d/%+v", status, detail)
	}
}
