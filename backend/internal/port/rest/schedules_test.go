package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	scheduledomain "github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/go-chi/chi/v5"
)

type scheduleRevisionRestRepository struct {
	scheduleusecase.ScheduleRepository
	rows []dao.ScheduleRevision
}

func (repo scheduleRevisionRestRepository) ListScheduleRevisionsByActor(_ context.Context, _ scheduledomain.UserID, _ scheduledomain.ScheduleID, limit int, anchor *scheduleusecase.CursorAnchor) ([]dao.ScheduleRevision, error) {
	rows := append([]dao.ScheduleRevision(nil), repo.rows...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Revision > rows[j].Revision })
	if anchor != nil {
		filtered := rows[:0]
		for _, row := range rows {
			if row.Revision < anchor.Revision {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func TestScheduleResponseUsesStableVirtualIDAndOccurrenceTimezone(t *testing.T) {
	start := time.Date(2026, time.October, 12, 9, 30, 0, 0, time.FixedZone("JST", 9*60*60))
	response := scheduleResponse(dao.Schedule{
		ID: "generated-id", UserID: "owner-1", ProjectID: "project-1", AssigneeID: "member-1",
		SeriesID: "series-1", OccurrenceDate: "2026-10-12", StartAt: start.Unix(), EndAt: start.Add(time.Hour).Unix(),
		Timezone: "Asia/Tokyo", IntervalWeeks: 2, RepeatState: "active", IsException: false,
		Frequencies: []dao.Frequency{{Value: "mon", Label: "Monday", LabelJp: "月"}},
	})
	if response.ID != scheduleVirtualOccurrenceID || response.UserID != "owner-1" || response.AssigneeID != "member-1" || response.ProjectID != "project-1" {
		t.Fatalf("schedule response identity/ownership = %+v", response)
	}
	if response.StartAt != "2026-10-12T09:30:00+09:00" || response.EndAt != "2026-10-12T10:30:00+09:00" {
		t.Fatalf("start/end = %q/%q, want local Schedule timezone", response.StartAt, response.EndAt)
	}
	if len(response.Frequencies) != 1 || response.Frequencies[0].Value != "mon" {
		t.Fatalf("frequencies = %+v", response.Frequencies)
	}
}

func TestScheduleHandlerRejectsUnauthenticatedAndInvalidCreate(t *testing.T) {
	handler := NewScheduleHandler(scheduleusecase.ScheduleUseCases{}, nil)
	unauthenticated := httptest.NewRecorder()
	handler.Create(unauthenticated, httptest.NewRequest(http.MethodPost, "/schedules", strings.NewReader(`{}`)))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated create status = %d, want 401", unauthenticated.Code)
	}

	ctx := context.WithValue(context.Background(), UserIDContextKey, "user-1")
	request := httptest.NewRequest(http.MethodPost, "/schedules", strings.NewReader(`{"title":"Focus","start_at":"bad","end_at":"2026-10-12T10:00:00Z"}`)).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.Create(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid create timestamp status = %d, want 400; body=%s", response.Code, response.Body.String())
	}
}

func TestScheduleHandlerRequiresSchedulePathID(t *testing.T) {
	handler := NewScheduleHandler(scheduleusecase.ScheduleUseCases{}, nil)
	ctx := context.WithValue(context.Background(), UserIDContextKey, "user-1")
	request := httptest.NewRequest(http.MethodGet, "/schedules/", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.Get(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing path ID status = %d, want 400", response.Code)
	}
}

func TestScheduleUseCaseErrorsMapPermissionsAndOccurrenceConflicts(t *testing.T) {
	status, detail := scheduleUseCaseError(scheduleusecase.ErrPermissionDenied)
	if status != http.StatusForbidden || detail.Code != "permission_denied" {
		t.Fatalf("permission error mapping = %d/%q", status, detail.Code)
	}
	status, detail = scheduleUseCaseError(scheduleusecase.ErrOccurrenceCompleted)
	if status != http.StatusConflict || detail.Field != "occurrence_date" {
		t.Fatalf("completed occurrence mapping = %d/%+v", status, detail)
	}
	status, detail = scheduleUseCaseError(scheduleusecase.ErrScheduleProjectChanged)
	if status != http.StatusConflict || detail.Code != "schedule_changed" || detail.Field != "project_id" {
		t.Fatalf("stale Project association mapping = %d/%+v", status, detail)
	}
	status, detail = scheduleUseCaseError(scheduledomain.ErrScheduleTitleEmpty)
	if status != http.StatusBadRequest || detail.Field != "title" {
		t.Fatalf("invalid title mapping = %d/%+v", status, detail)
	}
}

func TestScheduleCursorRequestCarriesOccurrenceStableAnchor(t *testing.T) {
	request := listRequest{Size: 25, FromDate: "2026-10-12", AsOf: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Anchor: &listAnchor{
		At: "2026-10-12T09:00:00+09:00", SeriesID: "series-1", OccurrenceDate: "2026-10-19", AsOf: "2026-10-12T00:00:00Z",
	}}
	got := scheduleCursorRequest(request)
	if got.Size != 25 || got.FromDate != request.FromDate || got.Anchor == nil || got.Anchor.SeriesID != "series-1" || got.Anchor.OccurrenceDate != "2026-10-19" || got.Anchor.AsOf != request.Anchor.AsOf {
		t.Fatalf("cursor request = %+v", got)
	}
}

func TestScheduleRevisionHandlerPaginatesFullRecurrenceSnapshots(t *testing.T) {
	repository := scheduleRevisionRestRepository{rows: []dao.ScheduleRevision{
		{ID: "series-1", Revision: 2, UserID: "user-1", AssigneeID: "user-1", Title: "Updated", Timezone: "UTC", StartAt: 1_800_000_000, EndAt: 1_800_003_600, RepeatState: "active", IntervalWeeks: 2, Frequencies: []dao.Frequency{{Value: "mon", Label: "Monday", LabelJp: "月"}}, ChangedBy: "user-1", ChangedAt: 10},
		{ID: "series-1", Revision: 1, UserID: "user-1", AssigneeID: "user-1", Title: "Initial", Timezone: "UTC", StartAt: 1_700_000_000, EndAt: 1_700_003_600, RepeatState: "active", IntervalWeeks: 1, ChangedBy: "user-1", ChangedAt: 5},
	}}
	group := scheduleusecase.ScheduleUseCases{Revisions: scheduleusecase.NewListScheduleRevisionsUseCase(repository, nil)}
	handler := NewScheduleHandler(group, nil, listTestCodec())
	ctx := context.WithValue(context.Background(), UserIDContextKey, "user-1")
	first := httptest.NewRequest(http.MethodGet, "/schedules/series-1/revisions?page_size=1", nil).WithContext(schedulePathContext(ctx, "series-1"))
	response := httptest.NewRecorder()
	handler.ListRevisions(response, first)
	if response.Code != http.StatusOK {
		t.Fatalf("first page status = %d body=%s", response.Code, response.Body.String())
	}
	var page ScheduleRevisionListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Revision != 2 || page.Items[0].IntervalWeeks != 2 || len(page.Items[0].Frequencies) != 1 || page.NextPageToken == "" {
		t.Fatalf("first revision page = %+v", page)
	}
	second := httptest.NewRequest(http.MethodGet, "/schedules/series-1/revisions?page_size=1&page_token="+page.NextPageToken, nil).WithContext(schedulePathContext(ctx, "series-1"))
	response = httptest.NewRecorder()
	handler.ListRevisions(response, second)
	if response.Code != http.StatusOK {
		t.Fatalf("second page status = %d body=%s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Revision != 1 || page.NextPageToken != "" {
		t.Fatalf("second revision page = %+v", page)
	}
}

func schedulePathContext(ctx context.Context, id string) context.Context {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", id)
	return context.WithValue(ctx, chi.RouteCtxKey, routeContext)
}
