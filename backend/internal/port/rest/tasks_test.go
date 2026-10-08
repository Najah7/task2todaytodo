package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type taskHandlerID struct{ value string }

func (generator taskHandlerID) Generate() string { return generator.value }

type taskHandlerTaskRepository struct {
	taskusecase.TaskRepository
	task        dao.Task
	rows        []dao.Task
	getErr      error
	createErr   error
	updateErr   error
	deleteErr   error
	pageErr     error
	created     domain.Task
	updated     domain.Task
	pageLimit   int
	pageOffset  int
	pageUserID  domain.UserID
	getCalls    int
	createCalls int
	updateCalls int
	deleteCalls int
	listCalls   int
}

func (repo *taskHandlerTaskRepository) ReadTaskProgressSources(_ context.Context, taskIDs []string, _ time.Time) (dao.TaskProgressSources, error) {
	requested := make(map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		requested[id] = true
	}
	rows := append([]dao.Task(nil), repo.rows...)
	if repo.task.ID != "" {
		rows = append(rows, repo.task)
	}
	sources := dao.TaskProgressSources{Counts: make(map[string]dao.TaskProgressCounts), Statuses: make(map[string]dao.TaskStatus)}
	seen := make(map[string]bool)
	for _, row := range rows {
		if seen[row.ID] || !requested[row.ID] {
			continue
		}
		seen[row.ID] = true
		total, completed := 100, row.Progress
		sources.Counts[row.ID] = dao.TaskProgressCounts{Total: total, Completed: completed}
		sources.Statuses[row.ID] = row.Status
	}
	return sources, nil
}

func (repo *taskHandlerTaskRepository) LockByUserID(ctx context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, id)
}

func (repo *taskHandlerTaskRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, id)
}

func (repo *taskHandlerTaskRepository) SetStatusByUserID(_ context.Context, userID domain.UserID, id domain.TaskID, status domain.TaskStatus) error {
	if repo.task.ID != string(id) || repo.task.UserID != string(userID) {
		return taskusecase.ErrTaskNotFound
	}
	repo.task.Status = dao.TaskStatus{Value: status.Value}
	repo.task.Revision++
	repo.updated.ID = domain.TaskID(repo.task.ID)
	repo.updated.UserID = domain.UserID(repo.task.UserID)
	repo.updated.Status = status
	return nil
}

func (repo *taskHandlerTaskRepository) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, status domain.TaskStatus, expectedRevision int32, _ shared.Capability) error {
	if repo.task.Revision != expectedRevision {
		return taskusecase.ErrRevisionConflict
	}
	return repo.SetStatusByUserID(ctx, userID, id, status)
}

func (repo *taskHandlerTaskRepository) GetByUserID(_ context.Context, userID domain.UserID, id domain.TaskID) (dao.Task, error) {
	repo.getCalls++
	if repo.getErr != nil {
		return dao.Task{}, repo.getErr
	}
	if repo.task.ID != string(id) || repo.task.UserID != string(userID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	return repo.task, nil
}

func (repo *taskHandlerTaskRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, id domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repo.GetByUserID(ctx, userID, id)
}

func (repo *taskHandlerTaskRepository) ListByUserIDCursor(_ context.Context, userID domain.UserID, limit int, _ *taskusecase.CursorAnchor) ([]dao.Task, error) {
	repo.listCalls++
	repo.pageUserID, repo.pageLimit, repo.pageOffset = userID, limit, 0
	if repo.pageErr != nil {
		return nil, repo.pageErr
	}
	return repo.rows, nil
}

func (repo *taskHandlerTaskRepository) Create(_ context.Context, task domain.Task) (dao.Task, error) {
	repo.createCalls++
	repo.created = task
	if repo.createErr != nil {
		return dao.Task{}, repo.createErr
	}
	repo.task = taskHandlerDAO(task)
	return repo.task, nil
}

func (repo *taskHandlerTaskRepository) UpdateByUserID(_ context.Context, userID domain.UserID, task domain.Task, expectedRevision int32) (dao.Task, error) {
	repo.updateCalls++
	if repo.updateErr != nil {
		return dao.Task{}, repo.updateErr
	}
	if userID != domain.UserID(repo.task.UserID) || task.UserID != userID || task.ID != domain.TaskID(repo.task.ID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	if repo.task.Revision != expectedRevision {
		return dao.Task{}, taskusecase.ErrRevisionConflict
	}
	repo.updated = task
	repo.task = taskHandlerDAO(task)
	repo.task.Revision = expectedRevision + 1
	return repo.task, nil
}

func (repo *taskHandlerTaskRepository) DeleteByUserID(_ context.Context, userID domain.UserID, id domain.TaskID, expectedRevision int32) error {
	repo.deleteCalls++
	if repo.deleteErr != nil {
		return repo.deleteErr
	}
	if userID != domain.UserID(repo.task.UserID) || id != domain.TaskID(repo.task.ID) {
		return taskusecase.ErrTaskNotFound
	}
	if repo.task.Revision != expectedRevision {
		return taskusecase.ErrRevisionConflict
	}
	return nil
}

func taskHandlerDAO(task domain.Task) dao.Task {
	var dueDate int64
	if !task.DueDate.IsZero() {
		dueDate = task.DueDate.Unix()
	}
	return dao.Task{
		ID:               string(task.ID),
		UserID:           string(task.UserID),
		AssigneeID:       string(task.AssigneeID),
		ProjectID:        string(task.ProjectID),
		Title:            task.Title,
		Description:      task.Description,
		DueDate:          dueDate,
		EstimatedMinutes: cloneInt(task.EstimatedMinutes),
		ActualMinutes:    cloneInt(task.ActualMinutes),
		Progress:         task.Progress,
		Priority:         dao.Priority{Value: task.Priority.String()},
		Status:           dao.TaskStatus{Value: task.Status.String()},
		CreatedAt:        task.CreatedAt.Unix(),
		UpdatedAt:        task.UpdatedAt.Unix(),
		Revision:         1,
	}
}

type taskHandlerTagRepository struct {
	taskusecase.TaskTagRepository
	tags []dao.TaskTag
	err  error
}

func (repo *taskHandlerTagRepository) ListByTaskAndUserID(context.Context, domain.UserID, domain.TaskID) ([]dao.TaskTag, error) {
	return repo.tags, repo.err
}

type taskHandlerTodoRepository struct{ taskusecase.TodoItemRepository }

func (taskHandlerTodoRepository) DeleteUneditedFutureByTask(context.Context, domain.UserID, domain.TaskID, time.Time) (int64, error) {
	return 0, nil
}

type taskHandlerRepositories struct {
	taskusecase.Repositories
	tasks     taskusecase.TaskRepository
	tags      taskusecase.TaskTagRepository
	todoItems taskusecase.TodoItemRepository
}

func (repos taskHandlerRepositories) Tasks() taskusecase.TaskRepository       { return repos.tasks }
func (repos taskHandlerRepositories) TaskTags() taskusecase.TaskTagRepository { return repos.tags }
func (repos taskHandlerRepositories) TodoItems() taskusecase.TodoItemRepository {
	return repos.todoItems
}

type taskHandlerUOW struct {
	repos taskusecase.Repositories
	calls int
}

func (uow *taskHandlerUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	uow.calls++
	return fn(ctx, uow.repos)
}

func newTaskHandlerFixture() (*TaskHandler, *taskHandlerTaskRepository, *taskHandlerUOW) {
	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	taskRepo := &taskHandlerTaskRepository{task: dao.Task{
		ID: "task-1", UserID: "user-1", AssigneeID: "user-1", Title: "Plan release", Description: "Release notes",
		DueDate:  time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC).Unix(),
		Progress: 20, Priority: dao.Priority{Value: "high"}, Status: dao.TaskStatus{Value: "open"},
		CreatedAt: createdAt.Unix(), UpdatedAt: createdAt.Unix(), Revision: 1,
	}}
	tagRepo := &taskHandlerTagRepository{tags: []dao.TaskTag{{ID: "tag-1", UserID: "user-1", Name: "work"}}}
	repos := taskHandlerRepositories{
		tasks: taskRepo, tags: tagRepo, todoItems: taskHandlerTodoRepository{},
	}
	uow := &taskHandlerUOW{repos: repos}
	tasks := taskusecase.TaskUseCases{
		Create:   taskusecase.NewCreateTaskUseCase(taskRepo, nil),
		List:     taskusecase.NewListTasksUseCase(taskRepo, nil),
		Get:      taskusecase.NewGetTaskUseCase(uow, nil),
		Update:   taskusecase.NewUpdateTaskUseCase(uow, taskRepo, nil),
		Delete:   taskusecase.NewDeleteTaskUseCase(uow, nil),
		Start:    taskusecase.NewStartTaskUseCase(uow, nil),
		Hold:     taskusecase.NewHoldTaskUseCase(uow, nil),
		Wait:     taskusecase.NewWaitTaskUseCase(uow, nil),
		Complete: taskusecase.NewCompleteTaskUseCase(uow, func() time.Time { return createdAt }, nil),
		Reopen:   taskusecase.NewReopenTaskUseCase(uow, nil),
	}
	return NewTaskHandler(tasks, taskHandlerID{value: "task-new"}, listTestCodec()), taskRepo, uow
}

func taskRequest(method, path, body string, authenticated bool) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if authenticated && method != http.MethodGet && method != http.MethodHead {
		request.Header.Set(ifMatchHeader, `"1"`)
	}
	pathID := strings.TrimPrefix(path, "/tasks/")
	pathID = strings.SplitN(pathID, "/", 2)[0]
	pathID = strings.SplitN(pathID, ":", 2)[0]
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", pathID)
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeContext)
	if authenticated {
		ctx = context.WithValue(ctx, UserIDContextKey, "user-1")
	}
	return request.WithContext(ctx)
}

func decodeTaskError(t *testing.T, recorder *httptest.ResponseRecorder) ErrResponse {
	t.Helper()
	var response ErrResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("error response JSON: %v", err)
	}
	return response
}

func TestTaskHandlerListPagesOwnedTasks(t *testing.T) {
	handler, repo, _ := newTaskHandlerFixture()
	repo.rows = []dao.Task{
		{ID: "one", UserID: "user-1", Title: "One", Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}},
		{ID: "two", UserID: "user-1", Title: "Two", Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}},
		{ID: "three", UserID: "user-1", Title: "Three", Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}},
	}
	recorder := httptest.NewRecorder()
	handler.List(recorder, taskRequest(http.MethodGet, "/tasks?page_size=2", "", true))
	if recorder.Code != http.StatusOK {
		t.Fatalf("List status = %d, body %s", recorder.Code, recorder.Body.String())
	}
	var response TaskListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 2 {
		t.Fatalf("page = %#v, want two items", response)
	}
	if repo.pageUserID != "user-1" || repo.pageLimit != 3 {
		t.Errorf("ListByUserIDCursor args = %q/%d", repo.pageUserID, repo.pageLimit)
	}
}

func TestTaskHandlerListRejectsInvalidPaginationAndMissingAuth(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		auth       bool
		wantStatus int
	}{
		{name: "invalid limit", path: "/tasks?limit=101", auth: true, wantStatus: http.StatusBadRequest},
		{name: "invalid integer", path: "/tasks?offset=x", auth: true, wantStatus: http.StatusBadRequest},
		{name: "missing identity", path: "/tasks", auth: false, wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, repo, _ := newTaskHandlerFixture()
			recorder := httptest.NewRecorder()
			handler.List(recorder, taskRequest(http.MethodGet, test.path, "", test.auth))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if repo.listCalls != 0 {
				t.Errorf("list calls = %d, want 0", repo.listCalls)
			}
		})
	}
}

func TestTaskHandlerCreateValidatesInputAndMapsConflictAndInternalErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
	}{
		{name: "malformed json", body: "{", wantStatus: http.StatusBadRequest},
		{name: "forbidden project field", body: `{"title":"Work","project_id":"p1"}`, wantStatus: http.StatusBadRequest},
		{name: "invalid title", body: `{"title":"  "}`, wantStatus: http.StatusBadRequest},
		{name: "id conflict", body: `{"title":"Work"}`, createErr: &pgconn.PgError{Code: "23505", ConstraintName: "tasks_pkey"}, wantStatus: http.StatusConflict},
		{name: "unknown failure", body: `{"title":"Work"}`, createErr: errors.New("database secret"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, repo, _ := newTaskHandlerFixture()
			repo.createErr = test.createErr
			recorder := httptest.NewRecorder()
			handler.Create(recorder, taskRequest(http.MethodPost, "/tasks", test.body, true))
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if test.wantStatus == http.StatusInternalServerError && strings.Contains(recorder.Body.String(), "database secret") {
				t.Error("response exposes internal error")
			}
			if test.createErr == nil && test.wantStatus == http.StatusCreated {
				t.Errorf("unexpected success case in test table")
			}
		})
	}

	handler, repo, _ := newTaskHandlerFixture()
	recorder := httptest.NewRecorder()
	handler.Create(recorder, taskRequest(http.MethodPost, "/tasks", `{"title":"New task","due_date":"2026-12-31","estimated_minutes":25}`, true))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("Create success status = %d, body %s", recorder.Code, recorder.Body.String())
	}
	if repo.created.ID != "task-new" || repo.created.UserID != "user-1" || repo.created.ProjectID != "" || repo.created.Priority.String() != "low" || repo.created.Status.String() != "open" || repo.created.DueDate.Format("2006-01-02") != "2026-12-31" {
		t.Errorf("created task = %#v, expected user, generated id, standalone/defaults, and due date", repo.created)
	}
}

func TestTaskHandlerGetHidesMissingOrUnownedTaskAndReturnsTags(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler, _, _ := newTaskHandlerFixture()
		recorder := httptest.NewRecorder()
		request := taskRequest(http.MethodGet, "/tasks/task-1", "", true)
		handler.Get(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("Get status = %d, body %s", recorder.Code, recorder.Body.String())
		}
		var response TaskDetailsResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Task.ID != "task-1" || len(response.Tags) != 1 || response.Tags[0].Name != "work" {
			t.Errorf("Get response = %#v, want task and assigned tag", response)
		}
	})

	tests := []struct {
		name string
		task dao.Task
	}{
		{name: "missing task"},
		{name: "other owner", task: dao.Task{ID: "task-1", UserID: "someone-else", Title: "Private", Priority: dao.Priority{Value: "low"}, Status: dao.TaskStatus{Value: "open"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, repo, _ := newTaskHandlerFixture()
			repo.task = test.task
			recorder := httptest.NewRecorder()
			request := taskRequest(http.MethodGet, "/tasks/task-1", "", true)
			handler.Get(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Errorf("Get status = %d, want 404: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTaskHandlerUpdateOnlyAcceptsBasicFieldsAndPreservesPatchSemantics(t *testing.T) {
	for _, field := range []string{"project_id", "status", "progress"} {
		t.Run("reject "+field, func(t *testing.T) {
			handler, repo, _ := newTaskHandlerFixture()
			recorder := httptest.NewRecorder()
			request := taskRequest(http.MethodPatch, "/tasks/task-1", fmt.Sprintf(`{"%s":"invalid"}`, field), true)
			handler.Update(recorder, request)
			if recorder.Code != http.StatusBadRequest || repo.updateCalls != 0 {
				t.Errorf("PATCH %s = status %d update calls %d, want 400 and no update", field, recorder.Code, repo.updateCalls)
			}
		})
	}

	t.Run("omitted fields remain unchanged and null clears nullable fields", func(t *testing.T) {
		handler, repo, _ := newTaskHandlerFixture()
		recorder := httptest.NewRecorder()
		request := taskRequest(http.MethodPatch, "/tasks/task-1", `{"description":null,"due_date":null,"estimated_minutes":null,"actual_minutes":15}`, true)
		handler.Update(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("Update status = %d, body %s", recorder.Code, recorder.Body.String())
		}
		if repo.updated.Title != "Plan release" || repo.updated.Description != "" || !repo.updated.DueDate.IsZero() || repo.updated.EstimatedMinutes != nil || repo.updated.ActualMinutes == nil || *repo.updated.ActualMinutes != 15 {
			t.Errorf("updated task fields = %#v, want omitted title retained and explicit nulls cleared", repo.updated)
		}
	})

	t.Run("null title rejected", func(t *testing.T) {
		handler, repo, _ := newTaskHandlerFixture()
		recorder := httptest.NewRecorder()
		request := taskRequest(http.MethodPatch, "/tasks/task-1", `{"title":null}`, true)
		handler.Update(recorder, request)
		if recorder.Code != http.StatusBadRequest || repo.updateCalls != 0 {
			t.Errorf("PATCH null title = status %d update calls %d, want 400 and no save", recorder.Code, repo.updateCalls)
		}
	})
}

func TestTaskHandlerMapsUpdateDeleteAndAuthErrors(t *testing.T) {
	t.Run("update missing task", func(t *testing.T) {
		handler, repo, _ := newTaskHandlerFixture()
		repo.task = dao.Task{}
		recorder := httptest.NewRecorder()
		request := taskRequest(http.MethodPatch, "/tasks/missing", `{"title":"new"}`, true)
		handler.Update(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("Update status = %d, want 404", recorder.Code)
		}
	})

	t.Run("delete success and missing identity", func(t *testing.T) {
		handler, repo, _ := newTaskHandlerFixture()
		recorder := httptest.NewRecorder()
		request := taskRequest(http.MethodDelete, "/tasks/task-1", "", true)
		handler.Delete(recorder, request)
		if recorder.Code != http.StatusNoContent || repo.deleteCalls != 1 {
			t.Errorf("Delete status/calls = %d/%d, want 204/1", recorder.Code, repo.deleteCalls)
		}

		recorder = httptest.NewRecorder()
		handler.Delete(recorder, taskRequest(http.MethodDelete, "/tasks/task-1", "", false))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated Delete status = %d, want 401", recorder.Code)
		}
	})

	t.Run("internal failure", func(t *testing.T) {
		handler, repo, _ := newTaskHandlerFixture()
		repo.deleteErr = errors.New("private database detail")
		recorder := httptest.NewRecorder()
		request := taskRequest(http.MethodDelete, "/tasks/task-1", "", true)
		handler.Delete(recorder, request)
		if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "private database detail") {
			t.Errorf("Delete error = status %d body %s, want safe 500", recorder.Code, recorder.Body.String())
		}
	})
}

func TestTaskHandlerStatusMethodsDelegateToTheirMatchingUseCase(t *testing.T) {
	tests := []struct {
		name   string
		status string
		invoke func(*TaskHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "start", status: "in_progress", invoke: (*TaskHandler).Start},
		{name: "hold", status: "pending", invoke: (*TaskHandler).Hold},
		{name: "wait", status: "waiting_on_others", invoke: (*TaskHandler).Wait},
		{name: "complete", status: "done", invoke: (*TaskHandler).Complete},
		{name: "reopen", status: "open", invoke: (*TaskHandler).Reopen},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, repo, uow := newTaskHandlerFixture()
			recorder := httptest.NewRecorder()
			request := taskRequest(http.MethodPost, "/tasks/task-1:"+test.name, "", true)
			test.invoke(handler, recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s status = %d, body %s", test.name, recorder.Code, recorder.Body.String())
			}
			if repo.updated.Status.String() != test.status || uow.calls != 1 {
				t.Errorf("%s delegated result = status %q, UOW calls %d; want %q and one UOW", test.name, repo.updated.Status.String(), uow.calls, test.status)
			}
		})
	}
}

func TestTaskHandlerReadsStatusTaskIDFromChiRouter(t *testing.T) {
	handler, repo, _ := newTaskHandlerFixture()
	router := chi.NewRouter()
	router.Post("/tasks/{id}:start", handler.Start)
	request := httptest.NewRequest(http.MethodPost, "/tasks/task-1:start", nil)
	request.Header.Set(ifMatchHeader, `"1"`)
	request = request.WithContext(context.WithValue(request.Context(), UserIDContextKey, "user-1"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("chi routed status = %d, body %s", recorder.Code, recorder.Body.String())
	}
	if repo.updated.ID != "task-1" || repo.updated.Status.String() != "in_progress" {
		t.Errorf("chi route updated task = %q/%q, want task-1/in_progress", repo.updated.ID, repo.updated.Status.String())
	}
}
