package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const scheduleHandlerUser = "user-1"

type scheduleHandlerIDGen struct{}

func (scheduleHandlerIDGen) Generate() string { return "schedule-new" }

type scheduleHandlerTaskRepo struct{ taskusecase.TaskRepository }

func (scheduleHandlerTaskRepo) LockByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return dao.Task{ID: string(taskID), UserID: string(userID), Status: dao.TaskStatus{Value: "open"}}, nil
}
func (repo scheduleHandlerTaskRepo) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}
func (scheduleHandlerTaskRepo) SetStatusByUserID(context.Context, domain.UserID, domain.TaskID, domain.TaskStatus) error {
	return nil
}
func (repo scheduleHandlerTaskRepo) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, _ int32, _ shared.Capability) error {
	return repo.SetStatusByUserID(ctx, userID, taskID, status)
}
func (scheduleHandlerTaskRepo) ReadTaskProgressSources(_ context.Context, _ []string, _ []string, _ time.Time) (dao.TaskProgressSources, error) {
	return dao.TaskProgressSources{Counts: map[string]dao.TaskProgressCounts{}, Statuses: map[string]dao.TaskStatus{}}, nil
}

func (scheduleHandlerTaskRepo) GetByUserID(_ context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error) {
	if userID != scheduleHandlerUser || id != "task-1" {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	return dao.Task{ID: "task-1", UserID: scheduleHandlerUser}, nil
}

func (repo scheduleHandlerTaskRepo) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, id)
}

type scheduleHandlerScheduleRepo struct {
	taskusecase.TaskScheduleRepository
	row                dao.TaskSchedule
	overrides          []dao.TaskSchedule
	createErr          error
	getErr             error
	updateErr          error
	listErr            error
	created            domain.TaskSchedule
	updated            domain.TaskSchedule
	frequencyInterval  int
	frequencyValues    []string
	seriesUpdateCalls  int
	skippedOccurrences map[string][]int64
	completed          bool
	completionCalls    int
}

func (repo *scheduleHandlerScheduleRepo) GetByTaskAndUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID) (dao.TaskSchedule, error) {
	if repo.getErr != nil {
		return dao.TaskSchedule{}, repo.getErr
	}
	if userID != scheduleHandlerUser || taskID != "task-1" || repo.row.ID != string(id) {
		return dao.TaskSchedule{}, domain.ErrTaskScheduleNotFound
	}
	return repo.row, nil
}
func (repo *scheduleHandlerScheduleRepo) ListByTaskAndUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TaskSchedule, error) {
	if repo.listErr != nil {
		return nil, repo.listErr
	}
	if userID != scheduleHandlerUser || taskID != "task-1" {
		return nil, domain.ErrTaskScheduleNotFound
	}
	return append([]dao.TaskSchedule{repo.row}, repo.overrides...), nil
}

func (repo *scheduleHandlerScheduleRepo) ListByTaskAndUserIDCursor(_ context.Context, userID domain.UserID, taskID domain.TaskID, limit int, _ *taskusecase.CursorAnchor) ([]dao.TaskSchedule, error) {
	if repo.listErr != nil {
		return nil, repo.listErr
	}
	if userID != scheduleHandlerUser || taskID != "task-1" {
		return nil, domain.ErrTaskScheduleNotFound
	}
	if limit < 1 {
		return []dao.TaskSchedule{}, nil
	}
	return append([]dao.TaskSchedule{repo.row}, repo.overrides...), nil
}

func (repo *scheduleHandlerScheduleRepo) UpsertTaskScheduleOverride(_ context.Context, _ domain.UserID, schedule domain.TaskSchedule) (string, error) {
	repo.updated = schedule
	row := scheduleHandlerDAO(schedule)
	row.ID = "override-" + row.SeriesID + "-" + row.OccurrenceDate
	row.IsException = true
	for index := range repo.overrides {
		if repo.overrides[index].SeriesID == row.SeriesID && repo.overrides[index].OccurrenceDate == row.OccurrenceDate {
			repo.overrides[index] = row
			return row.ID, nil
		}
	}
	repo.overrides = append(repo.overrides, row)
	return row.ID, nil
}

func (repo *scheduleHandlerScheduleRepo) ListTaskScheduleSkippedOccurrences(_ context.Context, _ domain.UserID, _ domain.TaskID, seriesID domain.TaskScheduleID) ([]int64, error) {
	return append([]int64(nil), repo.skippedOccurrences[string(seriesID)]...), nil
}
func (repo *scheduleHandlerScheduleRepo) ListTaskScheduleSkippedOccurrencesForCapability(ctx context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, _ shared.Capability) ([]int64, error) {
	return repo.ListTaskScheduleSkippedOccurrences(ctx, userID, taskID, seriesID)
}
func (repo *scheduleHandlerScheduleRepo) SetTaskScheduleSkippedOccurrence(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, date time.Time, skipped bool) error {
	_ = taskID
	value := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).Unix()
	if repo.skippedOccurrences == nil {
		repo.skippedOccurrences = make(map[string][]int64)
	}
	series := string(seriesID)
	if skipped {
		if !slices.Contains(repo.skippedOccurrences[series], value) {
			repo.skippedOccurrences[series] = append(repo.skippedOccurrences[series], value)
		}
	} else {
		repo.skippedOccurrences[series] = slices.DeleteFunc(repo.skippedOccurrences[series], func(existing int64) bool { return existing == value })
	}
	return nil
}
func (repo *scheduleHandlerScheduleRepo) CreateByTaskAndUserID(_ context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	repo.created = schedule
	if repo.createErr != nil {
		return dao.TaskSchedule{}, repo.createErr
	}
	return scheduleHandlerDAO(schedule), nil
}
func (repo *scheduleHandlerScheduleRepo) UpdateByTaskAndUserID(_ context.Context, userID domain.UserID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	repo.updated = schedule
	if repo.updateErr != nil {
		return dao.TaskSchedule{}, repo.updateErr
	}
	repo.row = scheduleHandlerDAO(schedule)
	return repo.row, nil
}
func (repo *scheduleHandlerScheduleRepo) ReplaceFrequenciesForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID, interval int, frequencies domain.TaskFrequencies) error {
	repo.frequencyInterval = interval
	repo.frequencyValues = nil
	for _, value := range frequencies {
		repo.frequencyValues = append(repo.frequencyValues, value.Value)
	}
	repo.row.IntervalWeeks = interval
	return nil
}
func (repo *scheduleHandlerScheduleRepo) SetCompletedForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, id domain.TaskScheduleID, completed bool) error {
	repo.completionCalls++
	if userID != scheduleHandlerUser || taskID != "task-1" {
		return domain.ErrTaskScheduleNotFound
	}
	if repo.row.ID == string(id) {
		repo.completed = completed
		return nil
	}
	for index := range repo.overrides {
		if repo.overrides[index].ID == string(id) {
			repo.overrides[index].Completed = completed
			return nil
		}
	}
	return domain.ErrTaskScheduleNotFound
}
func (repo *scheduleHandlerScheduleRepo) UpdateSeriesByTaskAndUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, seriesID domain.TaskScheduleID, _ time.Time, _ domain.TaskSchedule) error {
	repo.seriesUpdateCalls++
	return nil
}

func (repo *scheduleHandlerScheduleRepo) UpdateTaskScheduleSeriesTemplate(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID, _ domain.TaskScheduleID, schedule domain.TaskSchedule) (dao.TaskSchedule, error) {
	repo.seriesUpdateCalls++
	repo.row = scheduleHandlerDAO(schedule)
	return repo.row, nil
}

func (repo *scheduleHandlerScheduleRepo) SetTaskScheduleRecurrence(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TaskScheduleID, _ time.Time, interval int, frequencies []dao.TaskFrequency) error {
	repo.frequencyInterval = interval
	repo.frequencyValues = nil
	for _, frequency := range frequencies {
		repo.frequencyValues = append(repo.frequencyValues, frequency.Value)
	}
	repo.row.IntervalWeeks = interval
	repo.row.Frequencies = frequencies
	repo.row.RepeatState = "active"
	return nil
}

func (repo *scheduleHandlerScheduleRepo) StopTaskScheduleRecurrence(_ context.Context, _ domain.UserID, _ domain.TaskID, _ domain.TaskScheduleID) error {
	repo.frequencyInterval = 0
	repo.frequencyValues = nil
	repo.row.IntervalWeeks = 0
	repo.row.Frequencies = nil
	repo.row.RepeatState = "stopped"
	return nil
}
func (repo *scheduleHandlerScheduleRepo) DeleteUneditedFutureBySeries(context.Context, domain.UserID, domain.TaskID, domain.TaskScheduleID, time.Time) error {
	return nil
}

type scheduleHandlerRepos struct {
	taskusecase.Repositories
	tasks     *scheduleHandlerTaskRepo
	schedules *scheduleHandlerScheduleRepo
}

func (r scheduleHandlerRepos) Tasks() taskusecase.TaskRepository                 { return r.tasks }
func (r scheduleHandlerRepos) TaskSchedules() taskusecase.TaskScheduleRepository { return r.schedules }

type scheduleHandlerUOW struct {
	repos taskusecase.Repositories
	err   error
}

func (u scheduleHandlerUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	if u.err != nil {
		return u.err
	}
	return fn(ctx, u.repos)
}

type scheduleHandlerTimezone struct{}

func (scheduleHandlerTimezone) GetTimezone(context.Context, domain.UserID) (string, error) {
	return "Asia/Tokyo", nil
}

func newScheduleHandler(t *testing.T) (*TaskScheduleHandler, *scheduleHandlerScheduleRepo) {
	t.Helper()
	row := dao.TaskSchedule{
		ID: "schedule-1", TaskID: "task-1", Title: "Focus", Description: "Details", Location: "Desk",
		StartAt: time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC).Unix(), EndAt: time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC).Unix(),
		IntervalWeeks: 1, Frequencies: []dao.TaskFrequency{{Value: "sun", Label: "Sunday", LabelJp: "日曜日"}},
		SeriesID: "schedule-1", OccurrenceDate: "2026-10-11", Timezone: "Asia/Tokyo", RepeatState: "active",
		FrequencyAnchorDate: time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC).Unix(),
	}
	schedules := &scheduleHandlerScheduleRepo{row: row}
	repos := scheduleHandlerRepos{tasks: &scheduleHandlerTaskRepo{}, schedules: schedules}
	uow := scheduleHandlerUOW{repos: repos}
	usecases := taskusecase.TaskScheduleUseCases{
		Create:          taskusecase.NewCreateTaskScheduleUseCase(uow, scheduleHandlerTimezone{}, nil),
		List:            taskusecase.NewListTaskSchedulesUseCase(uow, nil),
		Update:          taskusecase.NewUpdateTaskScheduleUseCase(uow, nil, scheduleHandlerIDGen{}),
		Delete:          taskusecase.NewDeleteTaskScheduleUseCase(uow, nil),
		Complete:        taskusecase.NewCompleteTaskScheduleUseCase(uow, nil, scheduleHandlerIDGen{}),
		Reopen:          taskusecase.NewReopenTaskScheduleUseCase(uow, nil),
		Skip:            taskusecase.NewSkipTaskScheduleUseCase(uow, nil),
		Restore:         taskusecase.NewRestoreTaskScheduleUseCase(uow, nil),
		Reschedule:      taskusecase.NewRescheduleTaskScheduleUseCase(uow, nil, scheduleHandlerIDGen{}),
		UpdateFrequency: taskusecase.NewUpdateTaskScheduleFrequencyUseCase(uow, nil),
	}
	return NewTaskScheduleHandler(usecases, scheduleHandlerIDGen{}, listTestCodec()), schedules
}

func scheduleHandlerDAO(s domain.TaskSchedule) dao.TaskSchedule {
	freqs := make([]dao.TaskFrequency, 0, len(s.Frequencies))
	for _, f := range s.Frequencies {
		freqs = append(freqs, dao.TaskFrequency{Value: f.Value, Label: f.Label, LabelJp: f.LabelJp})
	}
	return dao.TaskSchedule{ID: string(s.ID), TaskID: string(s.TaskID), Title: s.Title, Description: s.Description, Location: s.Location,
		StartAt: s.StartAt.Unix(), EndAt: s.EndAt.Unix(), IntervalWeeks: s.IntervalWeeks, Frequencies: freqs,
		SeriesID: string(s.SeriesID), OccurrenceDate: s.OccurrenceDate.Format("2006-01-02"), Timezone: s.Timezone,
		IsException: s.IsException, Completed: s.Completed, CreatedAt: s.CreatedAt.Unix(), UpdatedAt: s.UpdatedAt.Unix()}
}

func scheduleHandlerRequest(method, path, body string, authenticated bool) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if authenticated {
		r = r.WithContext(context.WithValue(r.Context(), UserIDContextKey, scheduleHandlerUser))
	}
	return r
}

func TestTaskScheduleCreateThroughChi(t *testing.T) {
	h, repo := newScheduleHandler(t)
	router := chi.NewRouter()
	router.Post("/tasks/{taskId}/schedules", h.Create)
	req := scheduleHandlerRequest(http.MethodPost, "/tasks/task-1/schedules", `{"title":"Plan","start_at":"2026-10-04T09:00:00Z","end_at":"2026-10-04T10:00:00Z","interval_weeks":0,"frequencies":[]}`, true)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var got TaskScheduleResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "schedule-new" || got.TaskID != "task-1" || got.Title != "Plan" {
		t.Fatalf("response = %+v", got)
	}
	if repo.created.TaskID != "task-1" || repo.created.ID != "schedule-new" {
		t.Fatalf("usecase input = %+v", repo.created)
	}
}

func TestTaskScheduleListAuthAndParentScope(t *testing.T) {
	h, _ := newScheduleHandler(t)
	for _, tc := range []struct {
		name   string
		auth   bool
		path   string
		status int
	}{
		{"unauthorized", false, "/tasks/task-1/schedules", http.StatusUnauthorized},
		{"wrong parent", true, "/tasks/task-other/schedules", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Get("/tasks/{taskId}/schedules", h.List)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, scheduleHandlerRequest(http.MethodGet, tc.path, "", tc.auth))
			if w.Code != tc.status {
				t.Fatalf("status = %d want %d body=%s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestTaskScheduleCompleteAndReopenHandlers(t *testing.T) {
	h, repo := newScheduleHandler(t)
	router := chi.NewRouter()
	router.Post("/tasks/{taskId}/schedules/{id}:complete", h.Complete)
	router.Post("/tasks/{taskId}/schedules/{id}:reopen", h.Reopen)
	router.Post("/tasks/{taskId}/schedules/{id}:skip", h.Skip)
	router.Post("/tasks/{taskId}/schedules/{id}:restore", h.Restore)

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "complete", path: "/tasks/task-1/schedules/schedule-1:complete"},
		{name: "reopen", path: "/tasks/task-1/schedules/schedule-1:reopen"},
		{name: "skip", path: "/tasks/task-1/schedules/schedule-1:skip"},
		{name: "restore", path: "/tasks/task-1/schedules/schedule-1:restore"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 2 {
				res := httptest.NewRecorder()
				router.ServeHTTP(res, scheduleHandlerRequest(http.MethodPost, tc.path, `{"occurrence_date":"2026-10-11"}`, true))
				if res.Code != http.StatusOK {
					t.Fatalf("status = %d body=%s; want 200", res.Code, res.Body.String())
				}
			}
			if tc.name == "skip" && len(repo.skippedOccurrences["schedule-1"]) != 1 {
				t.Fatalf("skip markers = %v; want one separate skipped-occurrence marker without deleting the schedule", repo.skippedOccurrences)
			}
			if tc.name == "restore" && len(repo.skippedOccurrences["schedule-1"]) != 0 {
				t.Fatalf("restore markers = %v; want the skip marker removed without resurrecting a deleted row", repo.skippedOccurrences)
			}
		})
	}
}

func TestTaskScheduleCompleteHandlerRequiresOwnershipAndAuthentication(t *testing.T) {
	h, _ := newScheduleHandler(t)
	router := chi.NewRouter()
	router.Post("/tasks/{taskId}/schedules/{id}:complete", h.Complete)
	for _, tc := range []struct {
		name   string
		path   string
		auth   bool
		status int
	}{
		{name: "missing owner scope", path: "/tasks/other-task/schedules/schedule-1:complete", auth: true, status: http.StatusNotFound},
		{name: "missing authentication", path: "/tasks/task-1/schedules/schedule-1:complete", auth: false, status: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			router.ServeHTTP(res, scheduleHandlerRequest(http.MethodPost, tc.path, "", tc.auth))
			if res.Code != tc.status {
				t.Fatalf("status = %d body=%s; want %d", res.Code, res.Body.String(), tc.status)
			}
		})
	}
}

func TestTaskScheduleCompleteRequiresOccurrenceDateForRecurringSeries(t *testing.T) {
	h, _ := newScheduleHandler(t)
	req := scheduleHandlerRequest(http.MethodPost, "/tasks/task-1/schedules/schedule-1:complete", "", true)
	req.SetPathValue("taskId", "task-1")
	req.SetPathValue("id", "schedule-1")
	response := httptest.NewRecorder()
	h.Complete(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s; want missing occurrence_date rejected", response.Code, response.Body.String())
	}
}

func TestTaskScheduleUpdateRequiresScopeAndAllowsNullOptionalFields(t *testing.T) {
	h, repo := newScheduleHandler(t)
	for _, tc := range []struct {
		name, query, body string
		status            int
	}{
		{"missing scope", "", `{"occurrence_date":"2026-10-11","description":null,"location":"Home"}`, http.StatusBadRequest},
		{"query only", "?scope=current", `{"occurrence_date":"2026-10-11","description":null,"location":"Home"}`, http.StatusBadRequest},
		{"invalid scope", "", `{"scope":"all","occurrence_date":"2026-10-11","description":null,"location":"Home"}`, http.StatusBadRequest},
		{"current", "", `{"scope":"current","occurrence_date":"2026-10-11","description":null,"location":"Home"}`, http.StatusOK},
		{"future", "", `{"scope":"future","description":null,"location":"Home"}`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := scheduleHandlerRequest(http.MethodPatch, "/tasks/task-1/schedules/schedule-1"+tc.query, tc.body, true)
			req.SetPathValue("taskId", "task-1")
			req.SetPathValue("id", "schedule-1")
			w := httptest.NewRecorder()
			h.Update(w, req)
			if w.Code != tc.status {
				t.Fatalf("status = %d want %d body=%s", w.Code, tc.status, w.Body.String())
			}
		})
	}
	if repo.updated.TaskID != "task-1" || repo.updated.Location != "Home" || repo.updated.Description != "" {
		t.Fatalf("patched schedule = %+v", repo.updated)
	}
}

func TestTaskScheduleRescheduleRejectsReversedTimes(t *testing.T) {
	h, repo := newScheduleHandler(t)
	path := "/tasks/task-1/schedules/schedule-1:reschedule"
	req := scheduleHandlerRequest(http.MethodPost, path, `{"scope":"current","occurrence_date":"2026-10-11","start_at":"2026-10-11T11:00:00Z","end_at":"2026-10-11T10:00:00Z"}`, true)
	req.SetPathValue("taskId", "task-1")
	req.SetPathValue("id", "schedule-1")
	w := httptest.NewRecorder()
	h.Reschedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if repo.updated.ID != "" {
		t.Fatalf("unexpected update: %+v", repo.updated)
	}
}

func TestTaskScheduleFutureRescheduleRejectsDifferentLocalDate(t *testing.T) {
	h, _ := newScheduleHandler(t)
	req := scheduleHandlerRequest(http.MethodPost, "/tasks/task-1/schedules/schedule-1:reschedule", `{"scope":"future","occurrence_date":"2026-10-11","start_at":"2026-10-12T11:00:00Z","end_at":"2026-10-12T12:00:00Z"}`, true)
	req.SetPathValue("taskId", "task-1")
	req.SetPathValue("id", "schedule-1")
	w := httptest.NewRecorder()
	h.Reschedule(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s; future scope must preserve the occurrence's local date", w.Code, w.Body.String())
	}
}

func TestTaskScheduleRescheduleRequiresBodyScope(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
	}{
		{"missing", "/tasks/task-1/schedules/schedule-1:reschedule", `{"occurrence_date":"2026-10-11","start_at":"2026-10-11T11:00:00Z","end_at":"2026-10-11T12:00:00Z"}`},
		{"query only", "/tasks/task-1/schedules/schedule-1:reschedule?scope=current", `{"occurrence_date":"2026-10-11","start_at":"2026-10-11T11:00:00Z","end_at":"2026-10-11T12:00:00Z"}`},
		{"invalid", "/tasks/task-1/schedules/schedule-1:reschedule", `{"scope":"all","occurrence_date":"2026-10-11","start_at":"2026-10-11T11:00:00Z","end_at":"2026-10-11T12:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newScheduleHandler(t)
			r := scheduleHandlerRequest(http.MethodPost, tc.path, tc.body, true)
			r.SetPathValue("taskId", "task-1")
			r.SetPathValue("id", "schedule-1")
			w := httptest.NewRecorder()
			h.Reschedule(w, r)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestTaskScheduleFrequencyCanStopRecurrence(t *testing.T) {
	h, repo := newScheduleHandler(t)
	req := scheduleHandlerRequest(http.MethodPut, "/tasks/task-1/schedules/schedule-1/frequency", `{"interval_weeks":0,"frequencies":[]}`, true)
	req.SetPathValue("taskId", "task-1")
	req.SetPathValue("id", "schedule-1")
	w := httptest.NewRecorder()
	h.UpdateFrequency(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if repo.frequencyInterval != 0 || len(repo.frequencyValues) != 0 {
		t.Fatalf("frequency update interval=%d values=%v", repo.frequencyInterval, repo.frequencyValues)
	}
}

func TestTaskScheduleFrequencyRejectsOccurrenceDate(t *testing.T) {
	h, repo := newScheduleHandler(t)
	req := scheduleHandlerRequest(http.MethodPut, "/tasks/task-1/schedules/schedule-1/frequency", `{"occurrence_date":"2026-10-11","interval_weeks":1,"frequencies":["sun"]}`, true)
	req.SetPathValue("taskId", "task-1")
	req.SetPathValue("id", "schedule-1")
	w := httptest.NewRecorder()
	h.UpdateFrequency(w, req)
	if w.Code != http.StatusBadRequest || repo.frequencyInterval != 0 {
		t.Fatalf("status=%d frequency writes=%d; occurrence_date must not scope frequency updates", w.Code, repo.frequencyInterval)
	}
}

func TestTaskScheduleRescheduleUsesCurrentOrFutureScope(t *testing.T) {
	for _, scope := range []string{"current", "future"} {
		t.Run(scope, func(t *testing.T) {
			h, repo := newScheduleHandler(t)
			r := scheduleHandlerRequest(http.MethodPost, "/tasks/task-1/schedules/schedule-1:reschedule",
				`{"scope":"`+scope+`","occurrence_date":"2026-10-11","start_at":"2026-10-11T11:00:00Z","end_at":"2026-10-11T12:00:00Z"}`, true)
			r.SetPathValue("taskId", "task-1")
			r.SetPathValue("id", "schedule-1")
			w := httptest.NewRecorder()
			h.Reschedule(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			if scope == "current" && repo.updated.StartAt.Hour() != 11 {
				t.Fatalf("current update = %+v", repo.updated)
			}
			if scope == "future" && repo.seriesUpdateCalls != 1 {
				t.Fatalf("future template writes = %d, want 1", repo.seriesUpdateCalls)
			}
		})
	}
}

func TestTaskScheduleCreateMapsValidationConflictAndInternalErrors(t *testing.T) {
	for _, tc := range []struct {
		name      string
		body      string
		createErr error
		status    int
	}{
		{"invalid body", `{"title":"x","start_at":"bad","end_at":"2026-10-04T10:00:00Z"}`, nil, http.StatusBadRequest},
		{"conflict", `{"title":"x","start_at":"2026-10-04T09:00:00Z","end_at":"2026-10-04T10:00:00Z"}`, &pgconn.PgError{Code: "23505", ConstraintName: "task_schedules_pkey"}, http.StatusConflict},
		{"internal", `{"title":"x","start_at":"2026-10-04T09:00:00Z","end_at":"2026-10-04T10:00:00Z"}`, errors.New("db unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, repo := newScheduleHandler(t)
			repo.createErr = tc.createErr
			r := scheduleHandlerRequest(http.MethodPost, "/tasks/task-1/schedules", tc.body, true)
			r.SetPathValue("taskId", "task-1")
			w := httptest.NewRecorder()
			h.Create(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d want %d body=%s", w.Code, tc.status, w.Body.String())
			}
		})
	}
}

func TestTaskScheduleDeleteUsesTaskAndScheduleIDs(t *testing.T) {
	h, _ := newScheduleHandler(t)
	r := scheduleHandlerRequest(http.MethodDelete, "/tasks/task-1/schedules/schedule-other", "", true)
	r.SetPathValue("taskId", "task-1")
	r.SetPathValue("id", "schedule-other")
	w := httptest.NewRecorder()
	h.Delete(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}
