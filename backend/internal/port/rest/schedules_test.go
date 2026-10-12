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

func TestScheduleHandlerRejectsUnauthenticatedRuleCreate(t *testing.T) {
	handler := NewScheduleHandler(scheduleusecase.ScheduleUseCases{}, nil)
	response := httptest.NewRecorder()
	handler.CreateRule(response, httptest.NewRequest(http.MethodPost, "/schedule-rules", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create status = %d, want 401", response.Code)
	}
}

func TestScheduleHandlerRequiresCompoundOccurrenceIdentity(t *testing.T) {
	handler := NewScheduleHandler(scheduleusecase.ScheduleUseCases{}, nil)
	ctx := context.WithValue(context.Background(), UserIDContextKey, "user-1")
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("rule_id", "rule-1")
	request := httptest.NewRequest(http.MethodGet, "/schedules/rule-1/", nil)
	request = request.WithContext(context.WithValue(ctx, chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()
	handler.GetOccurrence(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing occurrence date status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestScheduleUseCaseErrorsMapRuleValidationAndRevisionConflicts(t *testing.T) {
	for _, test := range []struct {
		name  string
		err   error
		field string
		code  string
	}{
		{"title", scheduledomain.ErrRuleTitleEmpty, "title", "required"},
		{"timezone", scheduledomain.ErrRuleTimezoneEmpty, "timezone", "required"},
		{"frequency", scheduledomain.ErrRuleFrequency, "frequency", "invalid_frequency"},
		{"interval", scheduledomain.ErrRuleInterval, "interval", "invalid_interval"},
		{"weekdays", scheduledomain.ErrRuleWeekdays, "weekdays", "invalid_weekdays"},
		{"end date", scheduledomain.ErrRuleDateRange, "end_date", "invalid_date_range"},
		{"time range", scheduledomain.ErrRuleTimeRange, "end_at", "invalid_time_range"},
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
