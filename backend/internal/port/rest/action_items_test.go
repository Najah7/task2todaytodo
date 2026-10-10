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

type actionItemsHandlerID struct{ id string }

func (generator actionItemsHandlerID) Generate() string { return generator.id }

type actionItemsHandlerTimezoneReader struct {
	timezone string
	err      error
}

func (reader actionItemsHandlerTimezoneReader) GetTimezone(context.Context, string) (string, error) {
	return reader.timezone, reader.err
}

type actionItemsHandlerTaskRepository struct {
	taskusecase.TaskRepository
	task dao.Task
	err  error
}

func (repository actionItemsHandlerTaskRepository) LockByUserID(ctx context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	return repository.GetByUserID(ctx, userID, taskID)
}

func (repository actionItemsHandlerTaskRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repository.GetByUserID(ctx, userID, taskID)
}

func (repository *actionItemsHandlerTaskRepository) SetStatusByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus) error {
	if repository.task.ID != string(taskID) || repository.task.UserID != string(userID) {
		return taskusecase.ErrTaskNotFound
	}
	if repository.err != nil {
		return repository.err
	}
	repository.task.Status = dao.TaskStatus{Value: status.Value}
	return nil
}

func (repository *actionItemsHandlerTaskRepository) BumpRevisionByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32, _ shared.Capability) error {
	if repository.task.ID != string(taskID) || repository.task.UserID != string(userID) {
		return taskusecase.ErrTaskNotFound
	}
	if repository.err != nil {
		return repository.err
	}
	if repository.task.Revision != expectedRevision {
		return taskusecase.ErrRevisionConflict
	}
	repository.task.Revision++
	return nil
}

func (repository *actionItemsHandlerTaskRepository) SetStatusByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, _ int32, _ shared.Capability) error {
	return repository.SetStatusByUserID(ctx, userID, taskID, status)
}

func (repository actionItemsHandlerTaskRepository) ReadTaskProgressSources(_ context.Context, taskIDs []string, _ time.Time) (dao.TaskProgressSources, error) {
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

func (repository actionItemsHandlerTaskRepository) GetByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID) (dao.Task, error) {
	if repository.err != nil {
		return dao.Task{}, repository.err
	}
	if repository.task.ID != string(taskID) || repository.task.UserID != string(userID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	return repository.task, nil
}

func (repository actionItemsHandlerTaskRepository) GetByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	return repository.GetByUserID(ctx, userID, taskID)
}

type actionItemsHandlerActionItemRepository struct {
	taskusecase.ActionItemRepository
	rows              []dao.ActionItem
	overrides         []dao.ActionItem
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
	lastItemID        domain.ActionItemID
	lastPosition      int
	lastScope         string
	created           domain.ActionItem
	updated           domain.ActionItem
	createdAppendTail bool
	currentUpdates    int
	completeCalls     int
	reopenCalls       int
	deleteCalls       int
	seriesUpdateCalls int
	frequencyCalls    int
}

func (repository *actionItemsHandlerActionItemRepository) ListByTask(_ context.Context, userID domain.UserID, taskID domain.TaskID) ([]dao.ActionItem, error) {
	repository.lastUserID, repository.lastTaskID = userID, taskID
	if repository.listErr != nil {
		return nil, repository.listErr
	}
	items := make([]dao.ActionItem, 0, len(repository.rows)+len(repository.overrides))
	for _, item := range append(append([]dao.ActionItem(nil), repository.rows...), repository.overrides...) {
		if item.TaskID == string(taskID) && !item.Deleted {
			items = append(items, item)
		}
	}
	return items, nil
}

func (repository *actionItemsHandlerActionItemRepository) UpsertActionItemOverride(_ context.Context, _ domain.UserID, item domain.ActionItem) (string, error) {
	repository.currentUpdates++
	repository.lastTaskID, repository.lastItemID, repository.lastPosition = item.TaskID, item.SeriesID, item.Position
	repository.updated = item
	if repository.updateErr != nil {
		return "", repository.updateErr
	}
	if repository.reorderErr != nil {
		return "", repository.reorderErr
	}
	row := actionItemsHandlerDAO(item)
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

func (repository *actionItemsHandlerActionItemRepository) ListActionItemSkippedOccurrences(_ context.Context, _ domain.UserID, _ domain.TaskID, seriesID domain.ActionItemID) ([]int64, error) {
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

func (repository *actionItemsHandlerActionItemRepository) SetActionItemSkippedOccurrence(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, date time.Time, skipped bool) error {
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
		repository.overrides = append(repository.overrides, dao.ActionItem{
			ID: "skip-" + string(seriesID) + "-" + dateString, TaskID: string(taskID), SeriesID: string(seriesID),
			OccurrenceDate: dateString, Deleted: true,
		})
	}
	return nil
}

func (repository *actionItemsHandlerActionItemRepository) ListByTaskCursor(_ context.Context, userID domain.UserID, taskID domain.TaskID, limit int, _ *taskusecase.CursorAnchor) ([]dao.ActionItem, error) {
	repository.lastUserID, repository.lastTaskID = userID, taskID
	if repository.listErr != nil {
		return nil, repository.listErr
	}
	rows := append(append([]dao.ActionItem(nil), repository.rows...), repository.overrides...)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (repository *actionItemsHandlerActionItemRepository) CreateForOwnedTask(_ context.Context, _ domain.UserID, item domain.ActionItem, appendToTail bool) (dao.ActionItem, error) {
	repository.created = item
	repository.createdAppendTail = appendToTail
	if repository.createErr != nil {
		return dao.ActionItem{}, repository.createErr
	}
	created := actionItemsHandlerDAO(item)
	created.Position = 3
	repository.rows = append(repository.rows, created)
	return created, nil
}

func (repository *actionItemsHandlerActionItemRepository) GetForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) (dao.ActionItem, error) {
	repository.lastUserID, repository.lastTaskID, repository.lastItemID = userID, taskID, itemID
	if repository.getErr != nil {
		return dao.ActionItem{}, repository.getErr
	}
	for _, item := range repository.rows {
		if item.ID == string(itemID) && item.TaskID == string(taskID) && !item.Deleted {
			return item, nil
		}
	}
	return dao.ActionItem{}, taskusecase.ErrActionItemNotFound
}

func (repository *actionItemsHandlerActionItemRepository) UpdateForOwnedTask(_ context.Context, _ domain.UserID, item domain.ActionItem) (dao.ActionItem, error) {
	repository.currentUpdates++
	repository.updated = item
	if repository.updateErr != nil {
		return dao.ActionItem{}, repository.updateErr
	}
	updated := actionItemsHandlerDAO(item)
	updated.IsException = true
	for index := range repository.rows {
		if repository.rows[index].ID == string(item.ID) && repository.rows[index].TaskID == string(item.TaskID) {
			repository.rows[index] = updated
		}
	}
	return updated, nil
}

func (repository *actionItemsHandlerActionItemRepository) UpdateSeriesByTaskAndUserID(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID domain.ActionItemID, _ time.Time, item domain.ActionItem) error {
	repository.seriesUpdateCalls++
	repository.lastTaskID, repository.lastItemID, repository.updated = taskID, seriesID, item
	for index := range repository.rows {
		if repository.rows[index].ID == string(seriesID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index] = actionItemsHandlerDAO(item)
		}
	}
	return repository.seriesErr
}

func (repository *actionItemsHandlerActionItemRepository) UpdateActionItemSeriesTemplate(_ context.Context, _ domain.UserID, taskID domain.TaskID, seriesID, _ domain.ActionItemID, item domain.ActionItem) (dao.ActionItem, error) {
	repository.seriesUpdateCalls++
	repository.lastTaskID, repository.lastItemID, repository.updated = taskID, seriesID, item
	for index := range repository.rows {
		if repository.rows[index].ID == string(seriesID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index] = actionItemsHandlerDAO(item)
			return repository.rows[index], nil
		}
	}
	return dao.ActionItem{}, taskusecase.ErrActionItemNotFound
}

func (repository *actionItemsHandlerActionItemRepository) SetActionItemRecurrence(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, anchor time.Time, interval int, frequencies []dao.TaskFrequency) error {
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
	return taskusecase.ErrActionItemNotFound
}

func (repository *actionItemsHandlerActionItemRepository) StopActionItemRecurrence(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repository.frequencyCalls++
	for index := range repository.rows {
		if repository.rows[index].ID == string(itemID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index].IntervalWeeks = 0
			repository.rows[index].Frequencies = nil
			repository.rows[index].RepeatState = "stopped"
			return nil
		}
	}
	return taskusecase.ErrActionItemNotFound
}

func (repository *actionItemsHandlerActionItemRepository) TombstoneForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
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

func (repository *actionItemsHandlerActionItemRepository) DeleteUneditedFutureBySeries(context.Context, domain.UserID, domain.TaskID, domain.ActionItemID, time.Time) (int64, error) {
	return 0, nil
}

func (repository *actionItemsHandlerActionItemRepository) CheckForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
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
	return taskusecase.ErrActionItemNotFound
}

func (repository *actionItemsHandlerActionItemRepository) UncheckForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
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
	return taskusecase.ErrActionItemNotFound
}

func (repository *actionItemsHandlerActionItemRepository) ReorderForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, position int) (dao.ActionItem, error) {
	repository.lastTaskID, repository.lastItemID, repository.lastPosition = taskID, itemID, position
	if repository.reorderErr != nil {
		return dao.ActionItem{}, repository.reorderErr
	}
	for index := range repository.rows {
		if repository.rows[index].ID == string(itemID) && repository.rows[index].TaskID == string(taskID) {
			repository.rows[index].Position = position
			return repository.rows[index], nil
		}
	}
	return dao.ActionItem{}, taskusecase.ErrActionItemNotFound
}

func (repository *actionItemsHandlerActionItemRepository) ReplaceFrequenciesForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, intervalWeeks int, frequencies domain.TaskFrequencies) error {
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

func actionItemsHandlerDAO(item domain.ActionItem) dao.ActionItem {
	var dueDate int64
	if !item.DueDate.IsZero() {
		dueDate = item.DueDate.Unix()
	}
	frequencies := make([]dao.TaskFrequency, 0, len(item.Frequencies))
	for _, frequency := range item.Frequencies {
		frequencies = append(frequencies, dao.TaskFrequency{Value: frequency.Value})
	}
	return dao.ActionItem{
		ID: string(item.ID), TaskID: string(item.TaskID), Title: item.Title, Description: item.Description,
		DueDate: dueDate, Completed: item.Completed, Position: item.Position,
		IntervalWeeks: item.IntervalWeeks, Frequencies: frequencies, SeriesID: string(item.SeriesID),
		OccurrenceDate: item.OccurrenceDate.Format("2006-01-02"), Timezone: item.Timezone,
		IsException: item.IsException, CreatedAt: item.CreatedAt.Unix(), UpdatedAt: item.UpdatedAt.Unix(),
	}
}

type actionItemsHandlerRepositories struct {
	taskusecase.Repositories
	tasks taskusecase.TaskRepository
	items taskusecase.ActionItemRepository
}

func (repositories actionItemsHandlerRepositories) Tasks() taskusecase.TaskRepository {
	return repositories.tasks
}
func (repositories actionItemsHandlerRepositories) ActionItems() taskusecase.ActionItemRepository {
	return repositories.items
}

func (repository *actionItemsHandlerActionItemRepository) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	skipped := make(map[string]map[string]map[string]bool, len(taskIDs))
	for _, id := range taskIDs {
		items[id] = nil
		skipped[id] = nil
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items, SkippedByTask: skipped}, nil
}

type actionItemsHandlerUOW struct {
	repositories taskusecase.Repositories
	calls        int
}

func (uow *actionItemsHandlerUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	uow.calls++
	return fn(ctx, uow.repositories)
}

type actionItemsHandlerFixture struct {
	handler  *ActionItemHandler
	taskRepo *actionItemsHandlerTaskRepository
	itemRepo *actionItemsHandlerActionItemRepository
	uow      *actionItemsHandlerUOW
}

func newActionItemsHandlerFixture() actionItemsHandlerFixture {
	itemRepo := &actionItemsHandlerActionItemRepository{rows: []dao.ActionItem{
		{ID: "item-1", TaskID: "task-1", Title: "Write tests", Description: "Cover API", DueDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).Unix(), Position: 0, IntervalWeeks: 0, SeriesID: "item-1", OccurrenceDate: "2026-10-05", Timezone: "Asia/Tokyo", CreatedAt: 100, UpdatedAt: 200},
		{ID: "item-2", TaskID: "task-1", Title: "Review tests", Position: 1, SeriesID: "item-2", OccurrenceDate: "2026-10-05", Timezone: "Asia/Tokyo"},
	}}
	taskRepo := &actionItemsHandlerTaskRepository{task: dao.Task{ID: "task-1", UserID: "user-1"}}
	repositories := actionItemsHandlerRepositories{tasks: taskRepo, items: itemRepo}
	uow := &actionItemsHandlerUOW{repositories: repositories}
	ID := actionItemsHandlerID{id: "item-new"}
	usecases := taskusecase.ActionItemUseCases{
		Create:          taskusecase.NewCreateActionItemUseCase(uow, actionItemsHandlerTimezoneReader{timezone: "Asia/Tokyo"}, nil),
		List:            taskusecase.NewListActionItemsUseCase(uow, nil),
		Update:          taskusecase.NewUpdateActionItemUseCase(uow, nil),
		Delete:          taskusecase.NewDeleteActionItemUseCase(uow, nil),
		Complete:        taskusecase.NewCompleteActionItemUseCase(uow, nil, ID),
		Reopen:          taskusecase.NewReopenActionItemUseCase(uow, nil),
		Skip:            taskusecase.NewSkipActionItemUseCase(uow, nil),
		Restore:         taskusecase.NewRestoreActionItemUseCase(uow, nil),
		Reorder:         taskusecase.NewReorderActionItemUseCase(uow, nil, ID),
		UpdateFrequency: taskusecase.NewUpdateActionItemFrequencyUseCase(uow, nil),
	}
	return actionItemsHandlerFixture{
		handler: NewActionItemHandler(usecases, ID, listTestCodec()), taskRepo: taskRepo,
		itemRepo: itemRepo, uow: uow,
	}
}

func actionItemRequest(method, path, body string, authenticated bool) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	routeContext := chi.NewRouteContext()
	taskID, itemID := "", ""
	requestPath := strings.SplitN(path, "?", 2)[0]
	segments := strings.Split(strings.Trim(requestPath, "/"), "/")
	for index, segment := range segments {
		if segment == "tasks" && index+1 < len(segments) {
			taskID = strings.SplitN(segments[index+1], ":", 2)[0]
		}
		if segment == "action-items" && index+1 < len(segments) {
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

func TestActionItemHandlerListReturnsParentScopedOrderedItems(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	recorder := httptest.NewRecorder()
	fixture.handler.List(recorder, actionItemRequest(http.MethodGet, "/tasks/task-1/action-items", "", true))
	if recorder.Code != http.StatusOK {
		t.Fatalf("List status = %d, body %s", recorder.Code, recorder.Body.String())
	}
	var response listEnvelope[ActionItemResponse]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 2 || response.Items[0].Position != 0 || response.Items[1].ID != "item-2" || fixture.itemRepo.lastUserID != "user-1" || fixture.itemRepo.lastTaskID != "task-1" {
		t.Errorf("List = %#v; parent scope = %q/%q, want two ordered items for authenticated task", response, fixture.itemRepo.lastUserID, fixture.itemRepo.lastTaskID)
	}
}

func TestActionItemHandlerCreateReturnsCreatedItemAndMapsErrors(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		request := `{"title":"Prepare release","description":"Check notes","due_date":"2026-10-12","interval_weeks":0,"frequencies":[]}`
		recorder := httptest.NewRecorder()
		fixture.handler.Create(recorder, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items", request, true))
		if recorder.Code != http.StatusCreated {
			t.Fatalf("Create status = %d, body %s", recorder.Code, recorder.Body.String())
		}
		if fixture.itemRepo.created.ID != "item-new" || fixture.itemRepo.created.TaskID != "task-1" || fixture.itemRepo.created.Title != "Prepare release" || !fixture.itemRepo.createdAppendTail {
			t.Errorf("created item = %#v, append-to-tail %t", fixture.itemRepo.created, fixture.itemRepo.createdAppendTail)
		}
		var response ActionItemResponse
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
		{name: "position conflict", body: `{"title":"Task"}`, createErr: taskusecase.ErrActionItemPositionConflict, wantStatus: http.StatusConflict},
		{name: "unknown failure", body: `{"title":"Task"}`, createErr: errors.New("private sql detail"), wantStatus: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			fixture.itemRepo.createErr = test.createErr
			recorder := httptest.NewRecorder()
			fixture.handler.Create(recorder, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items", test.body, true))
			if recorder.Code != test.wantStatus {
				t.Fatalf("Create status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if test.wantStatus == http.StatusInternalServerError && strings.Contains(recorder.Body.String(), "private sql detail") {
				t.Error("response exposes internal error")
			}
		})
	}
}

func TestActionItemHandlerRequiresAuthenticationAndHidesWrongParent(t *testing.T) {
	t.Run("missing auth", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.List(recorder, actionItemRequest(http.MethodGet, "/tasks/task-1/action-items", "", false))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", recorder.Code)
		}
	})

	tests := []struct {
		name  string
		path  string
		setup func(*actionItemsHandlerFixture)
	}{
		{name: "item does not belong to path task", path: "/tasks/task-1/action-items/item-foreign", setup: func(fixture *actionItemsHandlerFixture) {
			fixture.itemRepo.rows = append(fixture.itemRepo.rows, dao.ActionItem{ID: "item-foreign", TaskID: "task-other", SeriesID: "item-foreign", OccurrenceDate: "2026-10-05", Timezone: "UTC"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			if test.setup != nil {
				test.setup(&fixture)
			}
			recorder := httptest.NewRecorder()
			fixture.handler.Complete(recorder, actionItemRequest(http.MethodPost, test.path, "", true))
			if recorder.Code != http.StatusNotFound {
				t.Errorf("status = %d, want 404: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
	t.Run("missing parent task", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		fixture.taskRepo.task = dao.Task{}
		recorder := httptest.NewRecorder()
		fixture.handler.List(recorder, actionItemRequest(http.MethodGet, "/tasks/missing/action-items", "", true))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestActionItemHandlerUpdateRequiresScopeAndDelegatesCurrentOrFuture(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Update(recorder, actionItemRequest(http.MethodPatch, "/tasks/task-1/action-items/item-1", `{"scope":"current","occurrence_date":"2026-10-05","title":"Changed once"}`, true))
		if recorder.Code != http.StatusOK || fixture.itemRepo.currentUpdates != 1 || fixture.itemRepo.seriesUpdateCalls != 0 || fixture.itemRepo.updated.Title != "Changed once" {
			t.Errorf("current update = status %d calls current/future %d/%d title %q", recorder.Code, fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls, fixture.itemRepo.updated.Title)
		}
	})

	t.Run("future", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Update(recorder, actionItemRequest(http.MethodPatch, "/tasks/task-1/action-items/item-1", `{"scope":"future","description":"Series update"}`, true))
		if recorder.Code != http.StatusOK || fixture.itemRepo.currentUpdates != 1 || fixture.itemRepo.seriesUpdateCalls != 0 || fixture.itemRepo.updated.Description != "Series update" {
			t.Errorf("future update = status %d writes current/template %d/%d description %q", recorder.Code, fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls, fixture.itemRepo.updated.Description)
		}
	})

	for _, test := range []struct {
		name string
		path string
		body string
	}{
		{name: "missing scope", path: "/tasks/task-1/action-items/item-1", body: `{"occurrence_date":"2026-10-05","title":"Changed"}`},
		{name: "query only", path: "/tasks/task-1/action-items/item-1?scope=current", body: `{"occurrence_date":"2026-10-05","title":"Changed"}`},
		{name: "invalid body scope", path: "/tasks/task-1/action-items/item-1", body: `{"scope":"all","occurrence_date":"2026-10-05","title":"Changed"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			fixture.handler.Update(recorder, actionItemRequest(http.MethodPatch, test.path, test.body, true))
			if recorder.Code != http.StatusBadRequest || fixture.itemRepo.currentUpdates != 0 || fixture.itemRepo.seriesUpdateCalls != 0 {
				t.Errorf("invalid scope status/writes = %d/%d/%d, want 400 and no writes", recorder.Code, fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls)
			}
		})
	}
}

func TestActionItemHandlerCompleteAndReopenUsePathIDs(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(*ActionItemHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "complete", invoke: (*ActionItemHandler).Complete},
		{name: "reopen", invoke: (*ActionItemHandler).Reopen},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			test.invoke(fixture.handler, recorder, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:"+test.name, `{"occurrence_date":"2026-10-05"}`, true))
			if recorder.Code != http.StatusOK || fixture.itemRepo.lastTaskID != "task-1" || (test.name == "complete" && fixture.itemRepo.lastItemID != "item-1") {
				t.Errorf("%s response = %d IDs=%q/%q, want 200 and both route IDs", test.name, recorder.Code, fixture.itemRepo.lastTaskID, fixture.itemRepo.lastItemID)
			}
		})
	}
}

func TestActionItemHandlerSkipAndRestoreRecurringOccurrence(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	root := fixture.itemRepo.rows[0]
	root.IntervalWeeks = 1
	root.Frequencies = []dao.TaskFrequency{{Value: "mon"}}
	fixture.itemRepo.rows[0] = root
	for _, operation := range []struct {
		name   string
		invoke func(*ActionItemHandler, http.ResponseWriter, *http.Request)
	}{
		{name: "skip", invoke: (*ActionItemHandler).Skip},
		{name: "restore", invoke: (*ActionItemHandler).Restore},
	} {
		t.Run(operation.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			operation.invoke(fixture.handler, response, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:"+operation.name, `{"occurrence_date":"2026-10-05"}`, true))
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

func TestActionItemHandlerAllowsMissingOccurrenceDateForOneOff(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	response := httptest.NewRecorder()
	fixture.handler.Complete(response, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:complete", "", true))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s; want one-off completion without occurrence_date", response.Code, response.Body.String())
	}

	recurring := newActionItemsHandlerFixture()
	root := recurring.itemRepo.rows[0]
	root.IntervalWeeks = 1
	root.Frequencies = []dao.TaskFrequency{{Value: "mon"}}
	recurring.itemRepo.rows[0] = root
	response = httptest.NewRecorder()
	recurring.handler.Complete(response, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:complete", "", true))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("recurring status = %d body=%s; want missing occurrence_date rejected", response.Code, response.Body.String())
	}
}

func TestActionItemHandlerDeleteSoftDeletesOwnedItem(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/item-1", "", true))
		if recorder.Code != http.StatusNoContent || fixture.itemRepo.deleteCalls != 1 || fixture.itemRepo.lastTaskID != "task-1" || fixture.itemRepo.lastItemID != "item-1" {
			t.Errorf("Delete status/calls/IDs = %d/%d/%q/%q, want 204 and matching parent/item IDs", recorder.Code, fixture.itemRepo.deleteCalls, fixture.itemRepo.lastTaskID, fixture.itemRepo.lastItemID)
		}
	})

	t.Run("missing item", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/missing", "", true))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("Delete status = %d, want 404: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		fixture.itemRepo.deleteErr = errors.New("private delete failure")
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/item-1", "", true))
		if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "private delete failure") {
			t.Errorf("Delete error response = %d %s, want safe 500", recorder.Code, recorder.Body.String())
		}
	})
}

func TestActionItemHandlerReorderValidatesPositionAndMapsConflict(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Reorder(recorder, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:reorder", `{"occurrence_date":"2026-10-05","position":1}`, true))
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
		{name: "concurrent conflict", position: 1, reorderErr: taskusecase.ErrActionItemPositionConflict, wantStatus: http.StatusConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			fixture.itemRepo.reorderErr = test.reorderErr
			recorder := httptest.NewRecorder()
			fixture.handler.Reorder(recorder, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:reorder", fmt.Sprintf(`{"occurrence_date":"2026-10-05","position":%d}`, test.position), true))
			if recorder.Code != test.wantStatus {
				t.Errorf("status = %d, want %d: %s", recorder.Code, test.wantStatus, recorder.Body.String())
			}
		})
	}
}

func TestActionItemHandlerFrequencyStopDisablesRecurrence(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	root := fixture.itemRepo.rows[0]
	root.IntervalWeeks = 1
	root.Frequencies = []dao.TaskFrequency{{Value: "mon"}}
	fixture.itemRepo.rows[0] = root
	recorder := httptest.NewRecorder()
	fixture.handler.UpdateFrequency(recorder, actionItemRequest(http.MethodPut, "/tasks/task-1/action-items/item-1/frequency", `{"interval_weeks":0,"frequencies":[]}`, true))
	if recorder.Code != http.StatusOK || fixture.itemRepo.frequencyCalls != 1 || fixture.itemRepo.rows[0].IntervalWeeks != 0 || len(fixture.itemRepo.rows[0].Frequencies) != 0 || fixture.itemRepo.rows[0].RepeatState != "stopped" {
		t.Errorf("frequency stop = status %d writes %d interval %d days %v state %q", recorder.Code, fixture.itemRepo.frequencyCalls, fixture.itemRepo.rows[0].IntervalWeeks, fixture.itemRepo.rows[0].Frequencies, fixture.itemRepo.rows[0].RepeatState)
	}
}

func TestActionItemHandlerFrequencyRequiresIntervalAndValidSettings(t *testing.T) {
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
			fixture := newActionItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			fixture.handler.UpdateFrequency(recorder, actionItemRequest(http.MethodPut, "/tasks/task-1/action-items/item-1/frequency", test.body, true))
			if recorder.Code != http.StatusBadRequest || fixture.itemRepo.frequencyCalls != 0 {
				t.Errorf("frequency input status/writes = %d/%d, want 400 and no writes", recorder.Code, fixture.itemRepo.frequencyCalls)
			}
		})
	}
}

func TestActionItemHandlerFutureScopeRejectsDueDate(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "date value", body: `{"scope":"future","due_date":"2026-10-12"}`},
		{name: "explicit null", body: `{"scope":"future","due_date":null}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			recorder := httptest.NewRecorder()
			fixture.handler.Update(recorder, actionItemRequest(http.MethodPatch, "/tasks/task-1/action-items/item-1", test.body, true))
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"field":"due_date"`) || !strings.Contains(recorder.Body.String(), "change recurrence weekdays") {
				t.Errorf("future due_date status/body = %d %s; want 400 with due_date guidance", recorder.Code, recorder.Body.String())
			}
			if fixture.itemRepo.currentUpdates != 0 || fixture.itemRepo.seriesUpdateCalls != 0 {
				t.Errorf("future due_date caused repository writes: current/template %d/%d", fixture.itemRepo.currentUpdates, fixture.itemRepo.seriesUpdateCalls)
			}
		})
	}
}
