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
)

type todoItemsHandlerID struct{ id string }

func (generator todoItemsHandlerID) Generate() string { return generator.id }

type todoItemsHandlerTimezoneReader struct {
	timezone string
	err      error
}

func (reader todoItemsHandlerTimezoneReader) GetTimezone(context.Context, domain.UserID) (string, error) {
	return reader.timezone, reader.err
}

type todoItemsHandlerTaskRepository struct {
	taskusecase.TaskRepository
	task dao.Task
	err  error
}

func (repository todoItemsHandlerTaskRepository) LockByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return repository.GetByUserID(ctx, userID, taskID)
}

func (repository todoItemsHandlerTaskRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repository.GetByUserID(ctx, userID, taskID)
}

func (repository *todoItemsHandlerTaskRepository) SetStatusByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus) error {
	if repository.task.ID != string(taskID) || repository.task.UserID != string(userID) {
		return taskusecase.ErrTaskNotFound
	}
	if repository.err != nil {
		return repository.err
	}
	repository.task.Status = dao.TaskStatus{Value: status.Value}
	return nil
}

func (repository *todoItemsHandlerTaskRepository) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, _ int32, _ shared.Capability) error {
	return repository.SetStatusByUserID(ctx, userID, taskID, status)
}

func (repository todoItemsHandlerTaskRepository) ReadTaskProgressSources(_ context.Context, taskIDs, _ []string, _ time.Time) (dao.TaskProgressSources, error) {
	sources := dao.TaskProgressSources{
		Counts:   make(map[string]dao.TaskProgressCounts),
		Statuses: make(map[string]dao.TaskStatus),
	}
	for _, taskID := range taskIDs {
		if repository.task.ID == taskID {
			sources.Statuses[taskID] = repository.task.Status
		}
	}
	return sources, nil
}

func (repository todoItemsHandlerTaskRepository) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	if repository.err != nil {
		return dao.Task{}, repository.err
	}
	if repository.task.ID != string(taskID) || repository.task.UserID != string(userID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	return repository.task, nil
}

func (repository todoItemsHandlerTaskRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repository.GetByUserID(ctx, userID, taskID)
}

type todoItemsHandlerTodoRepository struct {
	taskusecase.TodoItemRepository
	rows              []dao.TodoItem
	overrides         []dao.TodoItem
	getErr            error
	listErr           error
	createErr         error
	updateErr         error
	deleteErr         error
	completeErr       error
	reopenErr         error
	reorderErr        error
	frequencyErr      error
	seriesErr         error
	lastUserID        domain.UserID
	lastTaskID        domain.TaskID
	lastItemID        domain.TodoItemID
	lastPosition      int
	lastScope         string
	created           domain.TodoItem
	updated           domain.TodoItem
	createdAppendTail bool
	currentUpdates    int
	completeCalls     int
	reopenCalls       int
	deleteCalls       int
	seriesUpdateCalls int
	frequencyCalls    int
}

func (repository *todoItemsHandlerTodoRepository) ListByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.TodoItem, error) {
	repository.lastUserID, repository.lastTaskID = userID, taskID
	if repository.listErr != nil {
		return nil, repository.listErr
	}
	items := make([]dao.TodoItem, 0, len(repository.rows)+len(repository.overrides))
	for _, item := range append(append([]dao.TodoItem(nil), repository.rows...), repository.overrides...) {
		if item.TaskID == string(taskID) && !item.Deleted {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repository *todoItemsHandlerTodoRepository) UpsertTodoItemOverride(_ context.Context, _ domain.UserID, item domain.TodoItem) (string, error) {
	repository.currentUpdates++
	repository.lastTaskID, repository.lastItemID, repository.lastPosition = item.TaskID, item.SeriesID, item.Position
	repository.updated = item
	if repository.updateErr != nil {
		return "", repository.updateErr
	}
	if repository.reorderErr != nil {
		return "", repository.reorderErr
	}
	row := todoItemsHandlerDAO(item)
	row.ID = "override-" + row.SeriesID + "-" + row.OccurrenceDate
	row.IsException = true
	for index := range repository.overrides {
		if repository.overrides[index].SeriesID == row.SeriesID && repository.overrides[index].OccurrenceDate == row.OccurrenceDate {
			repository.overrides[index] = row
			return row.ID, nil
		}
	}
	repository.overrides = append(repository.overrides, row)
	return row.ID, nil
}

func (repository *todoItemsHandlerTodoRepository) ListTodoItemSkippedOccurrences(_ context.Context, _ domain.UserID, _ domain.TaskID, seriesID domain.TodoItemID) ([]int64, error) {
	var dates []int64
	for _, row := range repository.overrides {
		if row.SeriesID == string(seriesID) && row.Deleted {
			if date, err := time.Parse("2006-01-02", row.OccurrenceDate); err == nil {
				dates = append(dates, date.Unix())
			}
		}
	}
	return dates, nil
}

func (repository *todoItemsHandlerTodoRepository) SetTodoItemSkippedOccurrence(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, date time.Time, skipped bool) error {
	dateString := date.Format("2006-01-02")
	for index := range repository.overrides {
		row := &repository.overrides[index]
		if row.SeriesID != string(seriesID) || row.OccurrenceDate != dateString {
			continue
		}
		if skipped {
			row.Deleted = true
		} else if row.IsException {
			row.Deleted = false
		} else {
			repository.overrides = append(repository.overrides[:index], repository.overrides[index+1:]...)
		}
		return nil
	}
	if skipped {
		repository.overrides = append(repository.overrides, dao.TodoItem{
			ID: "skip-" + string(seriesID) + "-" + dateString, TaskID: string(taskID), SeriesID: string(seriesID),
			OccurrenceDate: dateString, Deleted: true,
		})
	}
	return nil
}

func (repository *todoItemsHandlerTodoRepository) ListByTaskCursor(_ context.Context, userID domain.UserID, taskID domain.TaskID, limit int, _ *taskusecase.CursorAnchor) ([]dao.TodoItem, error) {
	repository.lastUserID, repository.lastTaskID = userID, taskID
	if repository.listErr != nil {
		return nil, repository.listErr
	}
	rows := append(append([]dao.TodoItem(nil), repository.rows...), repository.overrides...)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (repository *todoItemsHandlerTodoRepository) CreateForOwnedTask(_ context.Context, _ domain.UserID, item domain.TodoItem, appendToTail bool) (dao.TodoItem, error) {
	repository.created = item
	repository.createdAppendTail = appendToTail
	if repository.createErr != nil {
		return dao.TodoItem{}, repository.createErr
	}
	created := todoItemsHandlerDAO(item)
	created.Position = 3
	repository.rows = append(repository.rows, created)
	return created, nil
}

func (repository *todoItemsHandlerTodoRepository) GetForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) (dao.TodoItem, error) {
	repository.lastUserID, repository.lastTaskID, repository.lastItemID = userID, taskID, itemID
	if repository.getErr != nil {
		return dao.TodoItem{}, repository.getErr
	}
	for _, item := range repository.rows {
		if item.ID == string(itemID) && item.TaskID == string(taskID) && !item.Deleted {
			return item, nil
		}
	}
	return dao.TodoItem{}, taskusecase.ErrTodoItemNotFound
}

func (repository *todoItemsHandlerTodoRepository) UpdateForOwnedTask(_ context.Context, _ domain.UserID, item domain.TodoItem) (dao.TodoItem, error) {
	repository.currentUpdates++
	repository.updated = item
	if repository.updateErr != nil {
		return dao.TodoItem{}, repository.updateErr
	}
	updated := todoItemsHandlerDAO(item)
	updated.IsException = true
	for index := range repository.rows {
		if repository.rows[index].ID == string(item.ID) && repository.rows[index].TaskID == string(item.TaskID) {
			repository.rows[index] = updated
		}
	}
	return updated, nil
}

func (repository *todoItemsHandlerTodoRepository) UpdateSeriesByTaskAndUserID(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID domain.TodoItemID, _ time.Time, item domain.TodoItem) error {
	repository.seriesUpdateCalls++
	repository.lastTaskID, repository.lastItemID, repository.updated = taskID, seriesID, item
	for index := range repository.rows {
		if repository.rows[index].ID == string(seriesID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index] = todoItemsHandlerDAO(item)
		}
	}
	return repository.seriesErr
}

func (repository *todoItemsHandlerTodoRepository) UpdateTodoItemSeriesTemplate(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID, _ domain.TodoItemID, item domain.TodoItem) (dao.TodoItem, error) {
	repository.seriesUpdateCalls++
	repository.lastTaskID, repository.lastItemID, repository.updated = taskID, seriesID, item
	for index := range repository.rows {
		if repository.rows[index].ID == string(seriesID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index] = todoItemsHandlerDAO(item)
			return repository.rows[index], nil
		}
	}
	return dao.TodoItem{}, taskusecase.ErrTodoItemNotFound
}

func (repository *todoItemsHandlerTodoRepository) SetTodoItemRecurrence(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID, anchor time.Time, interval int, frequencies []dao.TaskFrequency) error {
	repository.frequencyCalls++
	for index := range repository.rows {
		if repository.rows[index].ID == string(itemID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index].FrequencyAnchorDate = anchor.Unix()
			repository.rows[index].IntervalWeeks = interval
			repository.rows[index].Frequencies = frequencies
			repository.rows[index].RepeatState = "active"
			return nil
		}
	}
	return taskusecase.ErrTodoItemNotFound
}

func (repository *todoItemsHandlerTodoRepository) StopTodoItemRecurrence(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) error {
	repository.frequencyCalls++
	for index := range repository.rows {
		if repository.rows[index].ID == string(itemID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index].IntervalWeeks = 0
			repository.rows[index].Frequencies = nil
			repository.rows[index].RepeatState = "stopped"
			return nil
		}
	}
	return taskusecase.ErrTodoItemNotFound
}

func (repository *todoItemsHandlerTodoRepository) TombstoneForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) error {
	repository.deleteCalls++
	repository.lastTaskID, repository.lastItemID = taskID, itemID
	if repository.deleteErr != nil {
		return repository.deleteErr
	}
	for index := range repository.rows {
		if repository.rows[index].ID == string(itemID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index].Deleted = true
		}
	}
	return nil
}

func (repository *todoItemsHandlerTodoRepository) DeleteUneditedFutureBySeries(context.Context, domain.UserID, domain.TaskID, domain.TodoItemID, time.Time) (int64, error) {
	return 0, nil
}

func (repository *todoItemsHandlerTodoRepository) CheckForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) error {
	repository.completeCalls++
	repository.lastTaskID, repository.lastItemID = taskID, itemID
	if repository.completeErr != nil {
		return repository.completeErr
	}
	for _, item := range repository.rows {
		if item.ID == string(itemID) && item.TaskID == string(taskID) && !item.Deleted {
			return nil
		}
	}
	return taskusecase.ErrTodoItemNotFound
}

func (repository *todoItemsHandlerTodoRepository) UncheckForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID) error {
	repository.reopenCalls++
	repository.lastTaskID, repository.lastItemID = taskID, itemID
	if repository.reopenErr != nil {
		return repository.reopenErr
	}
	for _, item := range repository.rows {
		if item.ID == string(itemID) && item.TaskID == string(taskID) && !item.Deleted {
			return nil
		}
	}
	return taskusecase.ErrTodoItemNotFound
}

func (repository *todoItemsHandlerTodoRepository) ReorderForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID, position int) (dao.TodoItem, error) {
	repository.lastTaskID, repository.lastItemID, repository.lastPosition = taskID, itemID, position
	if repository.reorderErr != nil {
		return dao.TodoItem{}, repository.reorderErr
	}
	for index := range repository.rows {
		if repository.rows[index].ID == string(itemID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index].Position = position
			return repository.rows[index], nil
		}
	}
	return dao.TodoItem{}, taskusecase.ErrTodoItemNotFound
}

func (repository *todoItemsHandlerTodoRepository) ReplaceFrequenciesForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.TodoItemID, intervalWeeks int, frequencies domain.TaskFrequencies) error {
	repository.frequencyCalls++
	repository.lastUserID, repository.lastTaskID, repository.lastItemID = userID, taskID, itemID
	repository.updated.IntervalWeeks = intervalWeeks
	repository.updated.Frequencies = frequencies
	if repository.frequencyErr != nil {
		return repository.frequencyErr
	}
	for index := range repository.rows {
		if repository.rows[index].ID == string(itemID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index].IntervalWeeks = intervalWeeks
			repository.rows[index].Frequencies = nil
			for _, frequency := range frequencies {
				repository.rows[index].Frequencies = append(repository.rows[index].Frequencies, dao.TaskFrequency{Value: frequency.Value})
			}
		}
	}
	return nil
}

func todoItemsHandlerDAO(item domain.TodoItem) dao.TodoItem {
	var dueDate int64
	if !item.DueDate.IsZero() {
		dueDate = item.DueDate.Unix()
	}
	frequencies := make([]dao.TaskFrequency, 0, len(item.Frequencies))
	for _, frequency := range item.Frequencies {
		frequencies = append(frequencies, dao.TaskFrequency{Value: frequency.Value})
	}
	return dao.TodoItem{
		ID: string(item.ID), TaskID: string(item.TaskID), Title: item.Title, Description: item.Description,
		DueDate: dueDate, Completed: item.Completed, Position: item.Position,
		IntervalWeeks: item.IntervalWeeks, Frequencies: frequencies, SeriesID: string(item.SeriesID),
		OccurrenceDate: item.OccurrenceDate.Format("2006-01-02"), Timezone: item.Timezone,
		IsException: item.IsException, CreatedAt: item.CreatedAt.Unix(), UpdatedAt: item.UpdatedAt.Unix(),
	}
}

type todoItemsHandlerRepositories struct {
	taskusecase.Repositories
	tasks taskusecase.TaskRepository
	items taskusecase.TodoItemRepository
}

func (repositories todoItemsHandlerRepositories) Tasks() taskusecase.TaskRepository {
	return repositories.tasks
}
func (repositories todoItemsHandlerRepositories) TodoItems() taskusecase.TodoItemRepository {
	return repositories.items
}

type todoItemsHandlerUOW struct {
	repositories taskusecase.Repositories
	calls        int
}

func (uow *todoItemsHandlerUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	uow.calls++
	return fn(ctx, uow.repositories)
}

type todoItemsHandlerFixture struct {
	handler  *TodoItemHandler
	taskRepo *todoItemsHandlerTaskRepository
	itemRepo *todoItemsHandlerTodoRepository
	uow      *todoItemsHandlerUOW
}

func newTodoItemsHandlerFixture() todoItemsHandlerFixture {
	itemRepo := &todoItemsHandlerTodoRepository{rows: []dao.TodoItem{
		{ID: "item-1", TaskID: "task-1", Title: "Write tests", Description: "Cover API", DueDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).Unix(), Position: 0, IntervalWeeks: 0, SeriesID: "item-1", OccurrenceDate: "2026-10-05", Timezone: "Asia/Tokyo", CreatedAt: 100, UpdatedAt: 200},
		{ID: "item-2", TaskID: "task-1", Title: "Review tests", Position: 1, SeriesID: "item-2", OccurrenceDate: "2026-10-05", Timezone: "Asia/Tokyo"},
	}}
	taskRepo := &todoItemsHandlerTaskRepository{task: dao.Task{ID: "task-1", UserID: "user-1"}}
	repositories := todoItemsHandlerRepositories{tasks: taskRepo, items: itemRepo}
	uow := &todoItemsHandlerUOW{repositories: repositories}
	ID := todoItemsHandlerID{id: "item-new"}
	usecases := taskusecase.TodoItemUseCases{
		Create:          taskusecase.NewCreateTodoItemUseCase(uow, todoItemsHandlerTimezoneReader{timezone: "Asia/Tokyo"}, nil),
		List:            taskusecase.NewListTodoItemsUseCase(uow, nil),
		Update:          taskusecase.NewUpdateTodoItemUseCase(uow, nil),
		Delete:          taskusecase.NewDeleteTodoItemUseCase(uow, nil),
		Complete:        taskusecase.NewCompleteTodoItemUseCase(uow, nil, ID),
		Reopen:          taskusecase.NewReopenTodoItemUseCase(uow, nil),
		Skip:            taskusecase.NewSkipTodoItemUseCase(uow, nil),
		Restore:         taskusecase.NewRestoreTodoItemUseCase(uow, nil),
		Reorder:         taskusecase.NewReorderTodoItemUseCase(uow, nil, ID),
		UpdateFrequency: taskusecase.NewUpdateTodoItemFrequencyUseCase(uow, nil),
	}
	return todoItemsHandlerFixture{
		handler: NewTodoItemHandler(usecases, ID, listTestCodec()), taskRepo: taskRepo,
		itemRepo: itemRepo, uow: uow,
	}
}

func todoItemRequest(method, path, body string, authenticated bool) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	routeContext := chi.NewRouteContext()
	taskID, itemID := "", ""
	requestPath := strings.SplitN(path, "?", 2)[0]
	segments := strings.Split(strings.Trim(requestPath, "/"), "/")
	for index, segment := range segments {
		if segment == "tasks" && index+1 < len(segments) {
			taskID = strings.SplitN(segments[index+1], ":", 2)[0]
		}
		if segment == "todo-items" && index+1 < len(segments) {
			itemID = strings.SplitN(segments[index+1], ":", 2)[0]
		}
	}
	routeContext.URLParams.Add("taskId", taskID)
	routeContext.URLParams.Add("id", itemID)
	ctx := context.WithValue(request.Context(), chi.RouteCtxKey, routeContext)
	if authenticated {
		ctx = context.WithValue(ctx, UserIDContextKey, "user-1")
	}
	return request.WithContext(ctx)
}

func TestTodoItemHandlerListReturnsParentScopedOrderedItems(t *testing.T) {
	fixture := newTodoItemsHandlerFixture()
	recorder := httptest.NewRecorder()
	fixture.handler.List(recorder, todoItemRequest(http.MethodGet, "/tasks/task-1/todo-items", "", true))
	if recorder.Code != http.StatusOK {
		t.Fatalf("List status = %d, body %s", recorder.Code, recorder.Body.String())
	}
	var response listEnvelope[TodoItemResponse]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 2 || response.Items[0].Position != 0 || response.Items[1].ID != "item-2" || fixture.itemRepo.lastUserID != "user-1" || fixture.itemRepo.lastTaskID != "task-1" {
		t.Errorf("List = %#v; parent scope = %q/%q, want two ordered items for authenticated task", response, fixture.itemRepo.lastUserID, fixture.itemRepo.lastTaskID)
	}
}

func TestTodoItemHandlerCreateReturnsCreatedItemAndMapsErrors(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		request := `{"title":"Prepare release","description":"Check notes","due_date":"2026-10-12","interval_weeks":0,"frequencies":[]}`
		recorder := httptest.NewRecorder()
		fixture.handler.Create(recorder, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items", request, true))
		if recorder.Code != http.StatusCreated {
			t.Fatalf("Create status = %d, body %s", recorder.Code, recorder.Body.String())
		}
		if fixture.itemRepo.created.ID != "item-new" || fixture.itemRepo.created.TaskID != "task-1" || fixture.itemRepo.created.Title != "Prepare release" || !fixture.itemRepo.createdAppendTail {
			t.Errorf("created item = %#v, append-to-tail %t", fixture.itemRepo.created, fixture.itemRepo.createdAppendTail)
		}
		var response TodoItemResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.ID != "item-new" || response.IntervalWeeks != 0 || response.DueDate == nil || *response.DueDate != "2026-10-12" {
			t.Errorf("Create response = %#v, want created one-off item and date", response)
		}
	})

	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
	}{
		{name: "invalid JSON", body: "{", wantStatus: http.StatusBadRequest},
		{name: "invalid date", body: `{"title":"Task","due_date":"10/12/2026"}`, wantStatus: http.StatusBadRequest},
		{name: "invalid frequency", body: `{"title":"Task","interval_weeks":1,"frequencies":["funday"]}`, wantStatus: http.StatusBadRequest},
		{name: "position conflict", body: `{"title":"Task"}`, createErr: taskusecase.ErrTodoItemPositionConflict, wantStatus: http.StatusConflict},
		{name: "unknown failure", body: `{"title":"Task"}`, createErr: errors.New("private sql detail"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTodoItemsHandlerFixture()
			fixture.itemRepo.createErr = test.createErr
			recorder := httptest.NewRecorder()
			fixture.handler.Create(recorder, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items", test.body, true))
			if recorder.Code != test.wantStatus {
				t.Fatalf("Create status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if test.wantStatus == http.StatusInternalServerError && strings.Contains(recorder.Body.String(), "private sql detail") {
				t.Error("response exposes internal error")
			}
		})
	}
}

func TestTodoItemHandlerRequiresAuthenticationAndHidesWrongParent(t *testing.T) {
	t.Run("missing auth", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.List(recorder, todoItemRequest(http.MethodGet, "/tasks/task-1/todo-items", "", false))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", recorder.Code)
		}
	})

	tests := []struct {
		name  string
		path  string
		setup func(*todoItemsHandlerFixture)
	}{
		{name: "item does not belong to path task", path: "/tasks/task-1/todo-items/item-foreign", setup: func(fixture *todoItemsHandlerFixture) {
			fixture.itemRepo.rows = append(fixture.itemRepo.rows, dao.TodoItem{ID: "item-foreign", TaskID: "task-other", SeriesID: "item-foreign", OccurrenceDate: "2026-10-05", Timezone: "UTC"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTodoItemsHandlerFixture()
			if test.setup != nil {
				test.setup(&fixture)
			}
			recorder := httptest.NewRecorder()
			fixture.handler.Complete(recorder, todoItemRequest(http.MethodPost, test.path, "", true))
			if recorder.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
	t.Run("missing parent task", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		fixture.taskRepo.task = dao.Task{}
		recorder := httptest.NewRecorder()
		fixture.handler.List(recorder, todoItemRequest(http.MethodGet, "/tasks/missing/todo-items", "", true))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestTodoItemHandlerUpdateRequiresScopeAndDelegatesCurrentOrFuture(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Update(recorder, todoItemRequest(http.MethodPatch, "/tasks/task-1/todo-items/item-1", `{"scope":"current","occurrence_date":"2026-10-05","title":"Changed once"}`, true))
		if recorder.Code != http.StatusOK || fixture.itemRepo.currentUpdates != 1 || fixture.itemRepo.seriesUpdateCalls != 0 || fixture.itemRepo.updated.Title != "Changed once" {
			t.Errorf("current update = status %d calls current/future %d/%d title %q", recorder.Code, fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls, fixture.itemRepo.updated.Title)
		}
	})

	t.Run("future", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Update(recorder, todoItemRequest(http.MethodPatch, "/tasks/task-1/todo-items/item-1", `{"scope":"future","description":"Series update"}`, true))
		if recorder.Code != http.StatusOK || fixture.itemRepo.currentUpdates != 1 || fixture.itemRepo.seriesUpdateCalls != 0 || fixture.itemRepo.updated.Description != "Series update" {
			t.Errorf("future update = status %d writes current/template %d/%d description %q", recorder.Code, fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls, fixture.itemRepo.updated.Description)
		}
	})

	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{name: "missing scope", path: "/tasks/task-1/todo-items/item-1", body: `{"occurrence_date":"2026-10-05","title":"Changed"}`},
		{name: "query only", path: "/tasks/task-1/todo-items/item-1?scope=current", body: `{"occurrence_date":"2026-10-05","title":"Changed"}`},
		{name: "invalid body scope", path: "/tasks/task-1/todo-items/item-1", body: `{"scope":"all","occurrence_date":"2026-10-05","title":"Changed"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTodoItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			fixture.handler.Update(recorder, todoItemRequest(http.MethodPatch, test.path, test.body, true))
			if recorder.Code != http.StatusBadRequest || fixture.itemRepo.currentUpdates != 0 || fixture.itemRepo.seriesUpdateCalls != 0 {
				t.Errorf("invalid scope status/writes = %d/%d/%d, want 400 and no writes", recorder.Code, fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls)
			}
		})
	}
}

func TestTodoItemHandlerCompleteAndReopenUsePathIDs(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(*TodoItemHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "complete", invoke: (*TodoItemHandler).Complete},
		{name: "reopen", invoke: (*TodoItemHandler).Reopen},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTodoItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			test.invoke(fixture.handler, recorder, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items/item-1:"+test.name, `{"occurrence_date":"2026-10-05"}`, true))
			if recorder.Code != http.StatusOK || fixture.itemRepo.lastTaskID != "task-1" || (test.name == "complete" && fixture.itemRepo.lastItemID != "item-1") {
				t.Errorf("%s response = %d IDs=%q/%q, want 200 and both route IDs", test.name, recorder.Code, fixture.itemRepo.lastTaskID, fixture.itemRepo.lastItemID)
			}
		})
	}
}

func TestTodoItemHandlerSkipAndRestoreRecurringOccurrence(t *testing.T) {
	fixture := newTodoItemsHandlerFixture()
	root := fixture.itemRepo.rows[0]
	root.IntervalWeeks = 1
	root.Frequencies = []dao.TaskFrequency{{Value: "mon"}}
	fixture.itemRepo.rows[0] = root
	for _, operation := range []struct {
		name   string
		invoke func(*TodoItemHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "skip", invoke: (*TodoItemHandler).Skip},
		{name: "restore", invoke: (*TodoItemHandler).Restore},
	} {
		t.Run(operation.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			operation.invoke(fixture.handler, response, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items/item-1:"+operation.name, `{"occurrence_date":"2026-10-05"}`, true))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s; want 200", response.Code, response.Body.String())
			}
			if operation.name == "skip" && (len(fixture.itemRepo.overrides) != 1 || !fixture.itemRepo.overrides[0].Deleted || fixture.itemRepo.overrides[0].IsException) {
				t.Fatalf("skip child row = %+v; want one deleted non-exception row", fixture.itemRepo.overrides)
			}
			if operation.name == "restore" && len(fixture.itemRepo.overrides) != 0 {
				t.Fatalf("restore child rows = %+v; want skip-only child removed", fixture.itemRepo.overrides)
			}
		})
	}
}

func TestTodoItemHandlerAllowsMissingOccurrenceDateForOneOff(t *testing.T) {
	fixture := newTodoItemsHandlerFixture()
	response := httptest.NewRecorder()
	fixture.handler.Complete(response, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items/item-1:complete", "", true))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s; want one-off completion without occurrence_date", response.Code, response.Body.String())
	}

	recurring := newTodoItemsHandlerFixture()
	root := recurring.itemRepo.rows[0]
	root.IntervalWeeks = 1
	root.Frequencies = []dao.TaskFrequency{{Value: "mon"}}
	recurring.itemRepo.rows[0] = root
	response = httptest.NewRecorder()
	recurring.handler.Complete(response, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items/item-1:complete", "", true))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("recurring status = %d body=%s; want missing occurrence_date rejected", response.Code, response.Body.String())
	}
}

func TestTodoItemHandlerDeleteSoftDeletesOwnedItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, todoItemRequest(http.MethodDelete, "/tasks/task-1/todo-items/item-1", "", true))
		if recorder.Code != http.StatusNoContent || fixture.itemRepo.deleteCalls != 1 || fixture.itemRepo.lastTaskID != "task-1" || fixture.itemRepo.lastItemID != "item-1" {
			t.Errorf("Delete status/calls/IDs = %d/%d/%q/%q, want 204 and matching parent/item IDs", recorder.Code, fixture.itemRepo.deleteCalls, fixture.itemRepo.lastTaskID, fixture.itemRepo.lastItemID)
		}
	})

	t.Run("missing item", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, todoItemRequest(http.MethodDelete, "/tasks/task-1/todo-items/missing", "", true))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("Delete status = %d, want 404: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		fixture.itemRepo.deleteErr = errors.New("private delete failure")
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, todoItemRequest(http.MethodDelete, "/tasks/task-1/todo-items/item-1", "", true))
		if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "private delete failure") {
			t.Errorf("Delete error response = %d %s, want safe 500", recorder.Code, recorder.Body.String())
		}
	})
}

func TestTodoItemHandlerReorderValidatesPositionAndMapsConflict(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fixture := newTodoItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Reorder(recorder, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items/item-1:reorder", `{"occurrence_date":"2026-10-05","position":1}`, true))
		if recorder.Code != http.StatusOK || fixture.itemRepo.lastPosition != 1 || fixture.itemRepo.lastTaskID != "task-1" || fixture.itemRepo.lastItemID != "item-1" {
			t.Errorf("Reorder status/args = %d/%d/%q/%q", recorder.Code, fixture.itemRepo.lastPosition, fixture.itemRepo.lastTaskID, fixture.itemRepo.lastItemID)
		}
	})

	tests := []struct {
		name       string
		position   int
		reorderErr error
		wantStatus int
	}{
		{name: "out of range", position: -1, wantStatus: http.StatusBadRequest},
		{name: "concurrent conflict", position: 1, reorderErr: taskusecase.ErrTodoItemPositionConflict, wantStatus: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTodoItemsHandlerFixture()
			fixture.itemRepo.reorderErr = test.reorderErr
			recorder := httptest.NewRecorder()
			fixture.handler.Reorder(recorder, todoItemRequest(http.MethodPost, "/tasks/task-1/todo-items/item-1:reorder", fmt.Sprintf(`{"occurrence_date":"2026-10-05","position":%d}`, test.position), true))
			if recorder.Code != test.wantStatus {
				t.Errorf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestTodoItemHandlerFrequencyStopDisablesRecurrence(t *testing.T) {
	fixture := newTodoItemsHandlerFixture()
	root := fixture.itemRepo.rows[0]
	root.IntervalWeeks = 1
	root.Frequencies = []dao.TaskFrequency{{Value: "mon"}}
	fixture.itemRepo.rows[0] = root
	recorder := httptest.NewRecorder()
	fixture.handler.UpdateFrequency(recorder, todoItemRequest(http.MethodPut, "/tasks/task-1/todo-items/item-1/frequency", `{"interval_weeks":0,"frequencies":[]}`, true))
	if recorder.Code != http.StatusOK || fixture.itemRepo.frequencyCalls != 1 || fixture.itemRepo.rows[0].IntervalWeeks != 0 || len(fixture.itemRepo.rows[0].Frequencies) != 0 || fixture.itemRepo.rows[0].RepeatState != "stopped" {
		t.Errorf("frequency stop = status %d writes %d interval %d days %v state %q", recorder.Code, fixture.itemRepo.frequencyCalls, fixture.itemRepo.rows[0].IntervalWeeks, fixture.itemRepo.rows[0].Frequencies, fixture.itemRepo.rows[0].RepeatState)
	}
}

func TestTodoItemHandlerFrequencyRequiresIntervalAndValidSettings(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing interval", body: `{"frequencies":["mon"]}`},
		{name: "invalid interval", body: `{"interval_weeks":-1,"frequencies":["mon"]}`},
		{name: "invalid weekday", body: `{"interval_weeks":1,"frequencies":["funday"]}`},
		{name: "frequency on stopped series", body: `{"interval_weeks":0,"frequencies":["mon"]}`},
		{name: "occurrence date is not a frequency setting", body: `{"occurrence_date":"2026-10-05","interval_weeks":1,"frequencies":["mon"]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTodoItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			fixture.handler.UpdateFrequency(recorder, todoItemRequest(http.MethodPut, "/tasks/task-1/todo-items/item-1/frequency", test.body, true))
			if recorder.Code != http.StatusBadRequest || fixture.itemRepo.frequencyCalls != 0 {
				t.Errorf("frequency input status/writes = %d/%d, want 400 and no writes", recorder.Code, fixture.itemRepo.frequencyCalls)
			}
		})
	}
}

func TestTodoItemHandlerFutureScopeRejectsDueDate(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "date value", body: `{"scope":"future","due_date":"2026-10-12"}`},
		{name: "explicit null", body: `{"scope":"future","due_date":null}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newTodoItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			fixture.handler.Update(recorder, todoItemRequest(http.MethodPatch, "/tasks/task-1/todo-items/item-1", test.body, true))
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"field":"due_date"`) || !strings.Contains(recorder.Body.String(), "change recurrence weekdays") {
				t.Errorf("future due_date status/body = %d %s; want 400 with due_date guidance", recorder.Code, recorder.Body.String())
			}
			if fixture.itemRepo.currentUpdates != 0 || fixture.itemRepo.seriesUpdateCalls != 0 {
				t.Errorf("future due_date caused repository writes: current/template %d/%d", fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls)
			}
		})
	}
}
