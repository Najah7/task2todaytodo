package rest

import (
	"context"
	"encoding/json"
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

type actionItemsHandlerTaskRepository struct {
	taskusecase.TaskRepository
	task dao.Task
}

func (repo *actionItemsHandlerTaskRepository) GetByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, _ shared.Capability) (dao.Task, error) {
	if repo.task.ID != string(taskID) || repo.task.UserID != string(userID) {
		return dao.Task{}, taskusecase.ErrTaskNotFound
	}
	return repo.task, nil
}

func (repo *actionItemsHandlerTaskRepository) LockByUserIDWithPermission(ctx context.Context, userID domain.UserID, taskID domain.TaskID, capability shared.Capability) (dao.Task, error) {
	return repo.GetByUserIDWithPermission(ctx, userID, taskID, capability)
}

func (repo *actionItemsHandlerTaskRepository) BumpRevisionByUserID(_ context.Context, userID domain.UserID, taskID domain.TaskID, expectedRevision int32, _ shared.Capability) error {
	if repo.task.ID != string(taskID) || repo.task.UserID != string(userID) {
		return taskusecase.ErrTaskNotFound
	}
	if repo.task.Revision != expectedRevision {
		return taskusecase.ErrRevisionConflict
	}
	repo.task.Revision++
	return nil
}

func (repo *actionItemsHandlerTaskRepository) SetStatusByUserIDWithPermission(_ context.Context, userID domain.UserID, taskID domain.TaskID, status domain.TaskStatus, expectedRevision int32, _ shared.Capability) error {
	if repo.task.ID != string(taskID) || repo.task.UserID != string(userID) {
		return taskusecase.ErrTaskNotFound
	}
	if repo.task.Revision != expectedRevision {
		return taskusecase.ErrRevisionConflict
	}
	repo.task.Status = dao.TaskStatus{Value: status.Value}
	repo.task.Revision++
	return nil
}

type actionItemsHandlerActionItemRepository struct {
	taskusecase.ActionItemRepository
	rows              []dao.ActionItem
	createErr         error
	updateErr         error
	deleteErr         error
	completeErr       error
	reopenErr         error
	actualMinutes     *int
	reorderErr        error
	lastUserID        domain.UserID
	lastTaskID        domain.TaskID
	lastItemID        domain.ActionItemID
	lastPosition      int
	created           domain.ActionItem
	updated           domain.ActionItem
	createdAppendTail bool
	createCalls       int
	updateCalls       int
	deleteCalls       int
	completeCalls     int
	reopenCalls       int
	reorderCalls      int
}

func (repo *actionItemsHandlerActionItemRepository) ReadTaskListProjection(_ context.Context, _ domain.UserID, taskIDs []string) (dao.TaskListProjectionSources, error) {
	items := make(map[string][]dao.ActionItem, len(taskIDs))
	for _, taskID := range taskIDs {
		for _, item := range repo.rows {
			if item.TaskID == taskID && !item.Deleted {
				items[taskID] = append(items[taskID], item)
			}
		}
	}
	return dao.TaskListProjectionSources{ActionItemsByTask: items}, nil
}

func (repo *actionItemsHandlerActionItemRepository) CountForOwnedTask(_ context.Context, _ domain.UserID, taskID domain.TaskID, _ shared.Capability) (int, error) {
	count := 0
	for _, item := range repo.rows {
		if item.TaskID == string(taskID) && !item.Deleted {
			count++
		}
	}
	return count, nil
}

func (repo *actionItemsHandlerActionItemRepository) ListByTaskCursor(_ context.Context, userID domain.UserID, taskID domain.TaskID, limit int, anchor *taskusecase.CursorAnchor) ([]dao.ActionItem, error) {
	repo.lastUserID, repo.lastTaskID = userID, taskID
	items := make([]dao.ActionItem, 0)
	for _, item := range repo.rows {
		if item.TaskID != string(taskID) || item.Deleted {
			continue
		}
		if anchor != nil && (item.Position < anchor.Position || item.Position == anchor.Position && item.ID <= anchor.ID) {
			continue
		}
		items = append(items, item)
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (repo *actionItemsHandlerActionItemRepository) CreateForOwnedTask(_ context.Context, userID domain.UserID, item domain.ActionItem, appendTail bool) (dao.ActionItem, error) {
	repo.createCalls++
	repo.lastUserID, repo.lastTaskID, repo.created, repo.createdAppendTail = userID, item.TaskID, item, appendTail
	if repo.createErr != nil {
		return dao.ActionItem{}, repo.createErr
	}
	item.Position = len(repo.rows)
	created := actionItemDAO(item)
	repo.rows = append(repo.rows, created)
	return created, nil
}

func (repo *actionItemsHandlerActionItemRepository) GetForCommand(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, _ shared.Capability) (dao.ActionItem, error) {
	repo.lastUserID, repo.lastTaskID, repo.lastItemID = userID, taskID, itemID
	for _, item := range repo.rows {
		if item.ID == string(itemID) && item.TaskID == string(taskID) && !item.Deleted {
			return item, nil
		}
	}
	return dao.ActionItem{}, taskusecase.ErrActionItemNotFound
}

func (repo *actionItemsHandlerActionItemRepository) UpdateForOwnedTask(_ context.Context, userID domain.UserID, item domain.ActionItem) (dao.ActionItem, error) {
	repo.updateCalls++
	repo.lastUserID, repo.lastTaskID, repo.lastItemID, repo.updated = userID, item.TaskID, item.ID, item
	if repo.updateErr != nil {
		return dao.ActionItem{}, repo.updateErr
	}
	updated := actionItemDAO(item)
	for index := range repo.rows {
		if repo.rows[index].ID == updated.ID && repo.rows[index].TaskID == updated.TaskID {
			repo.rows[index] = updated
			return updated, nil
		}
	}
	return dao.ActionItem{}, taskusecase.ErrActionItemNotFound
}

func (repo *actionItemsHandlerActionItemRepository) CheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repo.completeCalls++
	repo.lastUserID, repo.lastTaskID, repo.lastItemID = userID, taskID, itemID
	if repo.completeErr != nil {
		return repo.completeErr
	}
	return repo.setCompleted(taskID, itemID, true)
}

func (repo *actionItemsHandlerActionItemRepository) CompleteWithActualMinutesForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, actualMinutes *int) error {
	repo.completeCalls++
	repo.lastUserID, repo.lastTaskID, repo.lastItemID = userID, taskID, itemID
	repo.actualMinutes = cloneInt(actualMinutes)
	if repo.completeErr != nil {
		return repo.completeErr
	}
	return repo.setCompleted(taskID, itemID, true)
}

func (repo *actionItemsHandlerActionItemRepository) UncheckForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repo.reopenCalls++
	repo.lastUserID, repo.lastTaskID, repo.lastItemID = userID, taskID, itemID
	if repo.reopenErr != nil {
		return repo.reopenErr
	}
	return repo.setCompleted(taskID, itemID, false)
}

func (repo *actionItemsHandlerActionItemRepository) setCompleted(taskID domain.TaskID, itemID domain.ActionItemID, completed bool) error {
	for index := range repo.rows {
		if repo.rows[index].ID == string(itemID) && repo.rows[index].TaskID == string(taskID) && !repo.rows[index].Deleted {
			repo.rows[index].Completed = completed
			return nil
		}
	}
	return taskusecase.ErrActionItemNotFound
}

func (repo *actionItemsHandlerActionItemRepository) DeleteForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID) error {
	repo.deleteCalls++
	repo.lastUserID, repo.lastTaskID, repo.lastItemID = userID, taskID, itemID
	if repo.deleteErr != nil {
		return repo.deleteErr
	}
	for index := range repo.rows {
		if repo.rows[index].ID == string(itemID) && repo.rows[index].TaskID == string(taskID) && !repo.rows[index].Deleted {
			repo.rows[index].Deleted = true
			return nil
		}
	}
	return taskusecase.ErrActionItemNotFound
}

func (repo *actionItemsHandlerActionItemRepository) ReorderForOwnedTask(_ context.Context, userID domain.UserID, taskID domain.TaskID, itemID domain.ActionItemID, position int) (dao.ActionItem, error) {
	repo.reorderCalls++
	repo.lastUserID, repo.lastTaskID, repo.lastItemID, repo.lastPosition = userID, taskID, itemID, position
	if repo.reorderErr != nil {
		return dao.ActionItem{}, repo.reorderErr
	}
	indices := make([]int, 0)
	target := -1
	for index, item := range repo.rows {
		if item.TaskID == string(taskID) && !item.Deleted {
			indices = append(indices, index)
			if item.ID == string(itemID) {
				target = len(indices) - 1
			}
		}
	}
	if target < 0 {
		return dao.ActionItem{}, taskusecase.ErrActionItemNotFound
	}
	if position < 0 || position >= len(indices) {
		return dao.ActionItem{}, taskusecase.ErrActionItemPositionOutOfRange
	}
	ordered := make([]dao.ActionItem, len(indices))
	for index, rowIndex := range indices {
		ordered[index] = repo.rows[rowIndex]
	}
	targetItem := ordered[target]
	ordered = append(ordered[:target], ordered[target+1:]...)
	ordered = append(ordered, dao.ActionItem{})
	copy(ordered[position+1:], ordered[position:])
	ordered[position] = targetItem
	for index, rowIndex := range indices {
		ordered[index].Position = index
		repo.rows[rowIndex] = ordered[index]
	}
	return repo.rows[indices[position]], nil
}

func actionItemDAO(item domain.ActionItem) dao.ActionItem {
	var dueDate int64
	if !item.DueDate.IsZero() {
		dueDate = item.DueDate.Unix()
	}
	return dao.ActionItem{
		ID: string(item.ID), TaskID: string(item.TaskID), Title: item.Title, Description: item.Description,
		DueDate: dueDate, EstimatedMinutes: item.EstimatedMinutes, Priority: dao.Priority{Value: item.Priority.Value},
		Completed: item.Completed, Position: item.Position,
	}
}

type actionItemsHandlerRepositories struct {
	taskusecase.Repositories
	tasks taskusecase.TaskRepository
	items taskusecase.ActionItemRepository
}

func (repos actionItemsHandlerRepositories) Tasks() taskusecase.TaskRepository { return repos.tasks }
func (repos actionItemsHandlerRepositories) ActionItems() taskusecase.ActionItemRepository {
	return repos.items
}
func (repos actionItemsHandlerRepositories) ActionItemTodos() taskusecase.ActionItemTodoLifecycle {
	return actionItemsHandlerTodoLifecycle{}
}

type actionItemsHandlerTodoLifecycle struct{}

func (actionItemsHandlerTodoLifecycle) HasCompletedFrame(context.Context, domain.ActionItemID) (bool, error) {
	return false, nil
}
func (actionItemsHandlerTodoLifecycle) HasTodayActionItem(context.Context, domain.UserID, domain.ActionItemID) (bool, error) {
	return false, nil
}
func (actionItemsHandlerTodoLifecycle) CompleteTodayFrames(context.Context, domain.UserID, domain.ActionItemID) error {
	return nil
}
func (actionItemsHandlerTodoLifecycle) ReopenFrames(context.Context, domain.ActionItemID) error {
	return nil
}
func (actionItemsHandlerTodoLifecycle) RegenerateToday(context.Context, domain.UserID) error {
	return nil
}

type actionItemsHandlerUOW struct{ repositories taskusecase.Repositories }

func (uow actionItemsHandlerUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	return fn(ctx, uow.repositories)
}

type actionItemsHandlerFixture struct {
	handler  *ActionItemHandler
	taskRepo *actionItemsHandlerTaskRepository
	itemRepo *actionItemsHandlerActionItemRepository
}

func newActionItemsHandlerFixture() actionItemsHandlerFixture {
	estimate := 30
	itemRepo := &actionItemsHandlerActionItemRepository{rows: []dao.ActionItem{
		{ID: "item-1", TaskID: "task-1", Title: "Write tests", Description: "Cover API", DueDate: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).Unix(), EstimatedMinutes: &estimate, EstimateEditable: true, Position: 0, CreatedAt: 100, UpdatedAt: 200},
		{ID: "item-2", TaskID: "task-1", Title: "Review tests", EstimatedMinutes: &estimate, EstimateEditable: true, Position: 1},
	}}
	taskRepo := &actionItemsHandlerTaskRepository{task: dao.Task{ID: "task-1", UserID: "user-1", Revision: 1, Status: dao.TaskStatus{Value: "open"}}}
	uow := actionItemsHandlerUOW{repositories: actionItemsHandlerRepositories{tasks: taskRepo, items: itemRepo}}
	ids := actionItemsHandlerID{id: "item-new"}
	usecases := taskusecase.ActionItemUseCases{
		Create: taskusecase.NewCreateActionItemUseCase(uow, nil), List: taskusecase.NewListActionItemsUseCase(uow, nil),
		Update: taskusecase.NewUpdateActionItemUseCase(uow, nil), Delete: taskusecase.NewDeleteActionItemUseCase(uow, nil),
		Complete: taskusecase.NewCompleteActionItemUseCase(uow, nil), Reopen: taskusecase.NewReopenActionItemUseCase(uow, nil),
		Reorder: taskusecase.NewReorderActionItemUseCase(uow, nil),
	}
	return actionItemsHandlerFixture{handler: NewActionItemHandler(usecases, ids.Generate, listTestCodec()), taskRepo: taskRepo, itemRepo: itemRepo}
}

func actionItemRequest(method, path, body string, authenticated bool) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := chi.NewRouteContext()
	segments := strings.Split(strings.Trim(strings.SplitN(path, "?", 2)[0], "/"), "/")
	for index, segment := range segments {
		if segment == "tasks" && index+1 < len(segments) {
			ctx.URLParams.Add("taskId", segments[index+1])
		}
		if segment == "action-items" && index+1 < len(segments) {
			ctx.URLParams.Add("id", strings.SplitN(segments[index+1], ":", 2)[0])
		}
	}
	requestContext := context.WithValue(request.Context(), chi.RouteCtxKey, ctx)
	if authenticated {
		requestContext = context.WithValue(requestContext, UserIDContextKey, "user-1")
	}
	return request.WithContext(requestContext)
}

func TestActionItemHandlerListReturnsSingleItemsWithoutRecurrenceMetadata(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	recorder := httptest.NewRecorder()
	fixture.handler.List(recorder, actionItemRequest(http.MethodGet, "/tasks/task-1/action-items", "", true))
	if recorder.Code != http.StatusOK {
		t.Fatalf("List status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response listEnvelope[ActionItemResponse]
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 2 || response.Items[0].ID != "item-1" || response.Items[1].ID != "item-2" || fixture.itemRepo.lastUserID != "user-1" || fixture.itemRepo.lastTaskID != "task-1" {
		t.Fatalf("List response=%+v scope=%q/%q", response, fixture.itemRepo.lastUserID, fixture.itemRepo.lastTaskID)
	}
	assertNoActionItemRecurrenceFields(t, recorder.Body.Bytes())
}

func TestActionItemHandlerCreateRejectsRecurrenceFieldsAndReturnsConcreteID(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	body := `{"title":"Prepare release","description":"Check notes","due_date":"2026-10-12","estimated_minutes":30,"priority":"high"}`
	recorder := httptest.NewRecorder()
	fixture.handler.Create(recorder, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items", body, true))
	if recorder.Code != http.StatusCreated || fixture.itemRepo.created.ID != "item-new" || !fixture.itemRepo.createdAppendTail {
		t.Fatalf("Create status=%d created=%+v append=%t body=%s", recorder.Code, fixture.itemRepo.created, fixture.itemRepo.createdAppendTail, recorder.Body.String())
	}
	var response ActionItemResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != "item-new" || response.Title != "Prepare release" || response.DueDate == nil || *response.DueDate != "2026-10-12" || response.EstimatedMinutes != 30 {
		t.Errorf("Create response=%+v", response)
	}
	assertNoActionItemRecurrenceFields(t, recorder.Body.Bytes())

	for _, field := range []string{"interval_weeks", "frequencies", "series_id", "occurrence_date", "repeat_state", "timezone"} {
		t.Run("rejects "+field, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			request := `{"title":"Single item","` + field + `":1}`
			response := httptest.NewRecorder()
			fixture.handler.Create(response, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items", request, true))
			if response.Code != http.StatusBadRequest || fixture.itemRepo.createCalls != 0 {
				t.Errorf("status=%d creates=%d; want strict decode rejection", response.Code, fixture.itemRepo.createCalls)
			}
		})
	}
}

func TestActionItemHandlerUpdateUsesConcreteIDAndRejectsOccurrenceScope(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	recorder := httptest.NewRecorder()
	fixture.handler.Update(recorder, actionItemRequest(http.MethodPatch, "/tasks/task-1/action-items/item-1", `{"title":"Changed","estimated_minutes":45,"priority":"urgent"}`, true))
	if recorder.Code != http.StatusOK || fixture.itemRepo.updateCalls != 1 || fixture.itemRepo.lastItemID != "item-1" || fixture.itemRepo.updated.Title != "Changed" {
		t.Fatalf("Update status=%d calls=%d id=%q item=%+v body=%s", recorder.Code, fixture.itemRepo.updateCalls, fixture.itemRepo.lastItemID, fixture.itemRepo.updated, recorder.Body.String())
	}
	assertNoActionItemRecurrenceFields(t, recorder.Body.Bytes())

	for _, field := range []string{"scope", "occurrence_date", "interval_weeks", "frequencies"} {
		t.Run("rejects "+field, func(t *testing.T) {
			fixture := newActionItemsHandlerFixture()
			response := httptest.NewRecorder()
			fixture.handler.Update(response, actionItemRequest(http.MethodPatch, "/tasks/task-1/action-items/item-1", `{"`+field+`":"current"}`, true))
			if response.Code != http.StatusBadRequest || fixture.itemRepo.updateCalls != 0 {
				t.Errorf("status=%d updates=%d; want strict decode rejection", response.Code, fixture.itemRepo.updateCalls)
			}
		})
	}
}

func TestActionItemHandlerCompleteAndReopenUseConcretePathIDWithoutBody(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	complete := httptest.NewRecorder()
	fixture.handler.Complete(complete, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:complete", "", true))
	if complete.Code != http.StatusOK || fixture.itemRepo.completeCalls != 1 || fixture.itemRepo.lastTaskID != "task-1" || fixture.itemRepo.lastItemID != "item-1" {
		t.Fatalf("complete status=%d calls=%d IDs=%q/%q body=%s", complete.Code, fixture.itemRepo.completeCalls, fixture.itemRepo.lastTaskID, fixture.itemRepo.lastItemID, complete.Body.String())
	}
	reopen := httptest.NewRecorder()
	fixture.handler.Reopen(reopen, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:reopen", "", true))
	if reopen.Code != http.StatusOK || fixture.itemRepo.reopenCalls != 1 || fixture.itemRepo.lastTaskID != "task-1" || fixture.itemRepo.lastItemID != "item-1" {
		t.Fatalf("reopen status=%d calls=%d IDs=%q/%q body=%s", reopen.Code, fixture.itemRepo.reopenCalls, fixture.itemRepo.lastTaskID, fixture.itemRepo.lastItemID, reopen.Body.String())
	}
	withObsoleteBody := newActionItemsHandlerFixture()
	badBody := httptest.NewRecorder()
	withObsoleteBody.handler.Complete(badBody, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:complete", `{"occurrence_date":"2026-10-05"}`, true))
	if badBody.Code != http.StatusBadRequest || withObsoleteBody.itemRepo.completeCalls != 0 {
		t.Errorf("obsolete body status=%d calls=%d; want 400 and no mutation", badBody.Code, withObsoleteBody.itemRepo.completeCalls)
	}
}

func TestActionItemCompleteAndReopenHandleChunkedBodies(t *testing.T) {
	completeFixture := newActionItemsHandlerFixture()
	request := actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:complete", `{"actual_minutes":0}`, true)
	request.ContentLength = -1
	complete := httptest.NewRecorder()
	completeFixture.handler.Complete(complete, request)
	if complete.Code != http.StatusOK || completeFixture.itemRepo.actualMinutes == nil || *completeFixture.itemRepo.actualMinutes != 0 {
		t.Fatalf("chunked complete status=%d actual=%v body=%s; want actual zero preserved", complete.Code, completeFixture.itemRepo.actualMinutes, complete.Body.String())
	}

	reopenFixture := newActionItemsHandlerFixture()
	reopenRequest := actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:reopen", `{"unexpected":true}`, true)
	reopenRequest.ContentLength = -1
	reopen := httptest.NewRecorder()
	reopenFixture.handler.Reopen(reopen, reopenRequest)
	if reopen.Code != http.StatusBadRequest || reopenFixture.itemRepo.reopenCalls != 0 {
		t.Fatalf("chunked reopen status=%d calls=%d body=%s; want 400 and no mutation", reopen.Code, reopenFixture.itemRepo.reopenCalls, reopen.Body.String())
	}
}

func TestActionItemHandlerDeletePreservesCompletedRestrictionAndSoftDeletes(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	if err := fixture.itemRepo.CheckForOwnedTask(context.Background(), "user-1", "task-1", "item-1"); err != nil {
		t.Fatal(err)
	}
	blocked := httptest.NewRecorder()
	fixture.handler.Delete(blocked, actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/item-1", "", true))
	if blocked.Code != http.StatusConflict || fixture.itemRepo.deleteCalls != 0 {
		t.Fatalf("completed delete status=%d deletes=%d; want 409 and no delete", blocked.Code, fixture.itemRepo.deleteCalls)
	}
	if err := fixture.itemRepo.UncheckForOwnedTask(context.Background(), "user-1", "task-1", "item-1"); err != nil {
		t.Fatal(err)
	}
	deleted := httptest.NewRecorder()
	fixture.handler.Delete(deleted, actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/item-1", "", true))
	if deleted.Code != http.StatusNoContent || fixture.itemRepo.deleteCalls != 1 || fixture.itemRepo.lastItemID != "item-1" {
		t.Fatalf("delete status=%d calls=%d id=%q body=%s", deleted.Code, fixture.itemRepo.deleteCalls, fixture.itemRepo.lastItemID, deleted.Body.String())
	}
}

func TestActionItemDeleteManualEstimateRequiresCurrentTaskRevision(t *testing.T) {
	t.Run("missing revision", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/item-1", `{"manual_estimated_minutes":60}`, true))
		if recorder.Code != http.StatusPreconditionRequired || fixture.itemRepo.deleteCalls != 0 {
			t.Fatalf("status=%d delete calls=%d body=%s; want 428 without mutation", recorder.Code, fixture.itemRepo.deleteCalls, recorder.Body.String())
		}
	})
	t.Run("stale revision", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		request := actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/item-1", `{"manual_estimated_minutes":60,"confirm_todo_regeneration":true}`, true)
		request.Header.Set(ifMatchHeader, `"2"`)
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, request)
		if recorder.Code != http.StatusPreconditionFailed || fixture.itemRepo.deleteCalls != 0 {
			t.Fatalf("status=%d delete calls=%d body=%s; want 412 without mutation", recorder.Code, fixture.itemRepo.deleteCalls, recorder.Body.String())
		}
	})
	t.Run("current revision", func(t *testing.T) {
		fixture := newActionItemsHandlerFixture()
		request := actionItemRequest(http.MethodDelete, "/tasks/task-1/action-items/item-1", `{"manual_estimated_minutes":60,"confirm_todo_regeneration":true}`, true)
		request.Header.Set(ifMatchHeader, `"1"`)
		recorder := httptest.NewRecorder()
		fixture.handler.Delete(recorder, request)
		if recorder.Code != http.StatusNoContent || fixture.itemRepo.deleteCalls != 1 {
			t.Fatalf("status=%d delete calls=%d body=%s; want 204 and one mutation", recorder.Code, fixture.itemRepo.deleteCalls, recorder.Body.String())
		}
	})
}

func TestActionItemHandlerReorderUsesOnlyConcreteIDAndPosition(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	response := httptest.NewRecorder()
	fixture.handler.Reorder(response, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-2:reorder", `{"position":0}`, true))
	if response.Code != http.StatusOK || fixture.itemRepo.reorderCalls != 1 || fixture.itemRepo.lastItemID != "item-2" || fixture.itemRepo.lastPosition != 0 {
		t.Fatalf("reorder status=%d calls=%d id=%q position=%d body=%s", response.Code, fixture.itemRepo.reorderCalls, fixture.itemRepo.lastItemID, fixture.itemRepo.lastPosition, response.Body.String())
	}
	if got := fixture.itemRepo.rows[0].ID; got != "item-2" {
		t.Errorf("first item ID after reorder = %q; want item-2", got)
	}
	invalid := newActionItemsHandlerFixture()
	bad := httptest.NewRecorder()
	invalid.handler.Reorder(bad, actionItemRequest(http.MethodPost, "/tasks/task-1/action-items/item-1:reorder", `{"position":0,"occurrence_date":"2026-10-05"}`, true))
	if bad.Code != http.StatusBadRequest || invalid.itemRepo.reorderCalls != 0 {
		t.Errorf("occurrence reorder status=%d calls=%d; want 400 and no call", bad.Code, invalid.itemRepo.reorderCalls)
	}
}

func TestActionItemHandlerRequiresAuthenticationAndHidesWrongParent(t *testing.T) {
	fixture := newActionItemsHandlerFixture()
	unauthenticated := httptest.NewRecorder()
	fixture.handler.List(unauthenticated, actionItemRequest(http.MethodGet, "/tasks/task-1/action-items", "", false))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated list status=%d; want 401", unauthenticated.Code)
	}
	wrongParent := httptest.NewRecorder()
	fixture.handler.Complete(wrongParent, actionItemRequest(http.MethodPost, "/tasks/task-2/action-items/item-1:complete", "", true))
	if wrongParent.Code != http.StatusNotFound {
		t.Errorf("wrong parent status=%d body=%s; want 404", wrongParent.Code, wrongParent.Body.String())
	}
}

func assertNoActionItemRecurrenceFields(t *testing.T, body []byte) {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"series_id", "occurrence_date", "repeat_state", "is_exception", "interval_weeks", "frequencies", "timezone", "frequency_anchor_date"} {
		if _, exists := fields[field]; exists {
			t.Errorf("response contains obsolete ActionItem field %q: %s", field, body)
		}
	}
}
