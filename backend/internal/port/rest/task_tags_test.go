package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/go-chi/chi/v5"
)

type taskTagHandlerID struct{ id string }

func (g taskTagHandlerID) Generate() string { return g.id }

type taskTagHandlerRepository struct {
	taskusecase.TaskTagRepository
	createResult taskdao.TaskTag
	createErr    error
	listResult   []taskdao.TaskTag
	listErr      error
	getResult    taskdao.TaskTag
	getErr       error
	renameResult taskdao.TaskTag
	renameErr    error
	deleteErr    error
	addErr       error
	removeErr    error
	createInput  taskdomain.TaskTag
	createCalls  int
	listUserID   taskdomain.UserID
	listCalls    int
	getUserID    taskdomain.UserID
	getTagID     taskdomain.TaskTagID
	getCalls     int
	renameUserID taskdomain.UserID
	renameInput  taskdomain.TaskTag
	renameCalls  int
	deleteUserID taskdomain.UserID
	deleteTagID  taskdomain.TaskTagID
	deleteCalls  int
	addUserID    taskdomain.UserID
	addTaskID    taskdomain.TaskID
	addTagID     taskdomain.TaskTagID
	addCalls     int
	removeUserID taskdomain.UserID
	removeTaskID taskdomain.TaskID
	removeTagID  taskdomain.TaskTagID
	removeCalls  int
}

func (repo *taskTagHandlerRepository) Create(_ context.Context, tag taskdomain.TaskTag) (taskdao.TaskTag, error) {
	repo.createCalls++
	repo.createInput = tag
	if repo.createErr != nil {
		return taskdao.TaskTag{}, repo.createErr
	}
	if repo.createResult.ID == "" {
		repo.createResult = taskdao.TaskTag{ID: string(tag.ID), UserID: string(tag.UserID), Name: tag.Name}
	}
	return repo.createResult, nil
}

func (repo *taskTagHandlerRepository) ListByUserID(_ context.Context, userID taskdomain.UserID) ([]taskdao.TaskTag, error) {
	repo.listCalls++
	repo.listUserID = userID
	return repo.listResult, repo.listErr
}

func (repo *taskTagHandlerRepository) ListByUserIDCursor(_ context.Context, userID taskdomain.UserID, limit int, _ *taskusecase.CursorAnchor) ([]taskdao.TaskTag, error) {
	repo.listCalls++
	repo.listUserID = userID
	return repo.listResult, repo.listErr
}

func (repo *taskTagHandlerRepository) GetByUserID(_ context.Context, userID taskdomain.UserID, tagID taskdomain.TaskTagID) (taskdao.TaskTag, error) {
	repo.getCalls++
	repo.getUserID, repo.getTagID = userID, tagID
	if repo.getErr != nil {
		return taskdao.TaskTag{}, repo.getErr
	}
	if repo.getResult.ID == "" {
		repo.getResult = taskdao.TaskTag{ID: string(tagID), UserID: string(userID), Name: "Existing"}
	}
	return repo.getResult, nil
}

func (repo *taskTagHandlerRepository) RenameByUserID(_ context.Context, userID taskdomain.UserID, tag taskdomain.TaskTag) (taskdao.TaskTag, error) {
	repo.renameCalls++
	repo.renameUserID, repo.renameInput = userID, tag
	if repo.renameErr != nil {
		return taskdao.TaskTag{}, repo.renameErr
	}
	if repo.renameResult.ID == "" {
		repo.renameResult = taskdao.TaskTag{ID: string(tag.ID), UserID: string(userID), Name: tag.Name}
	}
	return repo.renameResult, nil
}

func (repo *taskTagHandlerRepository) DeleteByUserID(_ context.Context, userID taskdomain.UserID, tagID taskdomain.TaskTagID) error {
	repo.deleteCalls++
	repo.deleteUserID, repo.deleteTagID = userID, tagID
	return repo.deleteErr
}

func (repo *taskTagHandlerRepository) AddToTask(_ context.Context, userID taskdomain.UserID, taskID taskdomain.TaskID, tagID taskdomain.TaskTagID) error {
	repo.addCalls++
	repo.addUserID, repo.addTaskID, repo.addTagID = userID, taskID, tagID
	return repo.addErr
}

func (repo *taskTagHandlerRepository) RemoveFromTask(_ context.Context, userID taskdomain.UserID, taskID taskdomain.TaskID, tagID taskdomain.TaskTagID) error {
	repo.removeCalls++
	repo.removeUserID, repo.removeTaskID, repo.removeTagID = userID, taskID, tagID
	return repo.removeErr
}

type taskTagHandlerTaskRepository struct {
	taskusecase.TaskRepository
	task taskdao.Task
	err  error
}

func (repo taskTagHandlerTaskRepository) GetByUserID(_ context.Context, _ taskdomain.UserID, _ taskdomain.TaskID) (taskdao.Task, error) {
	return repo.task, repo.err
}

func (repo taskTagHandlerTaskRepository) GetByUserIDWithPermission(ctx context.Context, userID taskdomain.UserID, taskID taskdomain.TaskID, _ shared.Capability) (taskdao.Task, error) {
	return repo.GetByUserID(ctx, userID, taskID)
}

type taskTagHandlerRepositories struct {
	taskusecase.Repositories
	tasks taskusecase.TaskRepository
	tags  taskusecase.TaskTagRepository
}

func (repos taskTagHandlerRepositories) Tasks() taskusecase.TaskRepository       { return repos.tasks }
func (repos taskTagHandlerRepositories) TaskTags() taskusecase.TaskTagRepository { return repos.tags }

type taskTagHandlerUOW struct {
	repositories taskusecase.Repositories
	err          error
}

func (uow taskTagHandlerUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	if uow.err != nil {
		return uow.err
	}
	return fn(ctx, uow.repositories)
}

func newTaskTagHandlerFixture() (*TaskTagHandler, *taskTagHandlerRepository) {
	repository := &taskTagHandlerRepository{}
	taskRepository := taskTagHandlerTaskRepository{task: taskdao.Task{ID: "task-1", UserID: "user-1"}}
	repositories := taskTagHandlerRepositories{tasks: taskRepository, tags: repository}
	uow := taskTagHandlerUOW{repositories: repositories}
	return NewTaskTagHandler(taskusecase.TaskTagUseCases{
		Create:         taskusecase.NewCreateTaskTagUseCase(repository, nil),
		List:           taskusecase.NewListTaskTagsUseCase(repository, nil),
		Rename:         taskusecase.NewRenameTaskTagUseCase(repository, nil),
		Delete:         taskusecase.NewDeleteTaskTagUseCase(repository, nil),
		AddToTask:      taskusecase.NewAddTagToTaskUseCase(uow, nil),
		RemoveFromTask: taskusecase.NewRemoveTagFromTaskUseCase(uow, nil),
	}, taskTagHandlerID{id: "generated-tag"}, listTestCodec()), repository
}

func taskTagHandlerRequest(method, target, body string, userID string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if userID != "" {
		request = request.WithContext(context.WithValue(request.Context(), UserIDContextKey, userID))
	}
	return request
}

func taskTagHandlerRequestWithPathParam(request *http.Request, key, value string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(key, value)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func TestTaskTagHandlerCreateAndListUseAuthenticatedOwner(t *testing.T) {
	handler, repo := newTaskTagHandlerFixture()
	createResponse := httptest.NewRecorder()
	handler.Create(createResponse, taskTagHandlerRequest(http.MethodPost, "/task-tags", `{"name":" Planning "}`, "user-1"))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("Create() status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	if repo.createInput.ID != "generated-tag" || repo.createInput.UserID != "user-1" || repo.createInput.Name != "Planning" {
		t.Errorf("Create() input = %#v, want generated ID and authenticated owner with trimmed name", repo.createInput)
	}
	var created TaskTagResponse
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("Create() response JSON: %v", err)
	}
	if created.ID != "generated-tag" || created.Name != "Planning" {
		t.Errorf("Create() response = %#v, want generated tag", created)
	}

	repo.listResult = []taskdao.TaskTag{{ID: "tag-1", UserID: "user-1", Name: "Planning", CreatedAt: 100, UpdatedAt: 200}}
	listResponse := httptest.NewRecorder()
	handler.List(listResponse, taskTagHandlerRequest(http.MethodGet, "/task-tags", "", "user-1"))
	if listResponse.Code != http.StatusOK || repo.listUserID != "user-1" {
		t.Fatalf("List() status/owner = %d/%q, want 200/user-1", listResponse.Code, repo.listUserID)
	}
	var listed TaskTagListResponse
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil {
		t.Fatalf("List() response JSON: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != "tag-1" || listed.Items[0].CreatedAt == "" {
		t.Errorf("List() response = %#v, want one public tag with timestamps", listed)
	}
}

func TestTaskTagHandlerRenameDeleteAndAssignmentUseOneOwnedID(t *testing.T) {
	handler, repo := newTaskTagHandlerFixture()
	renameRequest := taskTagHandlerRequestWithPathParam(taskTagHandlerRequest(http.MethodPatch, "/task-tags/tag-1", `{"name":" Focus "}`, "user-1"), "id", "tag-1")
	renameResponse := httptest.NewRecorder()
	handler.Rename(renameResponse, renameRequest)
	if renameResponse.Code != http.StatusOK || repo.renameUserID != "user-1" || repo.renameInput.Name != "Focus" {
		t.Fatalf("Rename() status/owner/name = %d/%q/%q", renameResponse.Code, repo.renameUserID, repo.renameInput.Name)
	}

	deleteRequest := taskTagHandlerRequestWithPathParam(taskTagHandlerRequest(http.MethodDelete, "/task-tags/tag-2", "", "user-1"), "id", "tag-2")
	deleteResponse := httptest.NewRecorder()
	handler.Delete(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || repo.deleteUserID != "user-1" || repo.deleteTagID != "tag-2" {
		t.Fatalf("Delete() status/owner/tag = %d/%q/%q", deleteResponse.Code, repo.deleteUserID, repo.deleteTagID)
	}

	addRequest := taskTagHandlerRequestWithPathParam(taskTagHandlerRequest(http.MethodPost, "/tasks/task-7/tags:add", `{"tag_id":"tag-3"}`, "user-1"), "id", "task-7")
	addResponse := httptest.NewRecorder()
	handler.AddToTask(addResponse, addRequest)
	if addResponse.Code != http.StatusNoContent || repo.addCalls != 1 || repo.addUserID != "user-1" || repo.addTaskID != "task-7" || repo.addTagID != "tag-3" {
		t.Fatalf("AddToTask() status/call/IDs = %d/%d/%q/%q/%q", addResponse.Code, repo.addCalls, repo.addUserID, repo.addTaskID, repo.addTagID)
	}

	removeRequest := taskTagHandlerRequestWithPathParam(taskTagHandlerRequest(http.MethodPost, "/tasks/task-7/tags:remove", `{"tag_id":"tag-4"}`, "user-1"), "id", "task-7")
	removeResponse := httptest.NewRecorder()
	handler.RemoveFromTask(removeResponse, removeRequest)
	if removeResponse.Code != http.StatusNoContent || repo.removeCalls != 1 || repo.removeUserID != "user-1" || repo.removeTaskID != "task-7" || repo.removeTagID != "tag-4" {
		t.Fatalf("RemoveFromTask() status/call/IDs = %d/%d/%q/%q/%q", removeResponse.Code, repo.removeCalls, repo.removeUserID, repo.removeTaskID, repo.removeTagID)
	}
}

func TestTaskTagHandlerReadsPathIDsThroughChiRouter(t *testing.T) {
	handler, repo := newTaskTagHandlerFixture()
	router := chi.NewRouter()
	router.Get("/task-tags", handler.List)
	router.Patch("/task-tags/{id}", handler.Rename)
	router.Delete("/task-tags/{id}", handler.Delete)
	router.Post("/tasks/{id}/tags:add", handler.AddToTask)
	router.Post("/tasks/{id}/tags:remove", handler.RemoveFromTask)

	request := taskTagHandlerRequest(http.MethodGet, "/task-tags", "", "user-1")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || repo.listUserID != "user-1" {
		t.Fatalf("chi GET /task-tags status/owner = %d/%q, want 200/user-1", response.Code, repo.listUserID)
	}

	request = taskTagHandlerRequest(http.MethodPatch, "/task-tags/tag-from-chi", `{"name":"Renamed"}`, "user-1")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || repo.getTagID != "tag-from-chi" || repo.renameInput.ID != "tag-from-chi" {
		t.Fatalf("chi PATCH tag path ID = status %d, get %q, rename %q", response.Code, repo.getTagID, repo.renameInput.ID)
	}

	request = taskTagHandlerRequest(http.MethodDelete, "/task-tags/delete-from-chi", "", "user-1")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || repo.deleteTagID != "delete-from-chi" {
		t.Fatalf("chi DELETE tag path ID = status %d, delete %q", response.Code, repo.deleteTagID)
	}

	request = taskTagHandlerRequest(http.MethodPost, "/tasks/add-from-chi/tags:add", `{"tag_id":"tag-1"}`, "user-1")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || repo.addTaskID != "add-from-chi" {
		t.Fatalf("chi tags:add path ID = status %d, task %q", response.Code, repo.addTaskID)
	}

	request = taskTagHandlerRequest(http.MethodPost, "/tasks/remove-from-chi/tags:remove", `{"tag_id":"tag-2"}`, "user-1")
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || repo.removeTaskID != "remove-from-chi" {
		t.Fatalf("chi tags:remove path ID = status %d, task %q", response.Code, repo.removeTaskID)
	}
}

func TestTaskTagHandlerUnauthorizedForAllOperations(t *testing.T) {
	handler, repo := newTaskTagHandlerFixture()
	tests := []struct {
		name string
		run  func(http.ResponseWriter, *http.Request)
		path string
		body string
	}{
		{name: "list", run: handler.List, path: "/task-tags"},
		{name: "create", run: handler.Create, path: "/task-tags", body: `{"name":"x"}`},
		{name: "rename", run: handler.Rename, path: "/task-tags/tag-1", body: `{"name":"x"}`},
		{name: "delete", run: handler.Delete, path: "/task-tags/tag-1"},
		{name: "add", run: handler.AddToTask, path: "/tasks/task-1/tags:add", body: `{"tag_id":"tag-1"}`},
		{name: "remove", run: handler.RemoveFromTask, path: "/tasks/task-1/tags:remove", body: `{"tag_id":"tag-1"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()
			test.run(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
		})
	}
	if repo.createCalls+repo.listCalls+repo.getCalls+repo.renameCalls+repo.deleteCalls+repo.addCalls+repo.removeCalls != 0 {
		t.Errorf("repository calls without authentication = create:%d list:%d get:%d rename:%d delete:%d add:%d remove:%d", repo.createCalls, repo.listCalls, repo.getCalls, repo.renameCalls, repo.deleteCalls, repo.addCalls, repo.removeCalls)
	}
}

func TestTaskTagHandlerMapsBadRequestNotFoundConflictAndInternalErrors(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*TaskTagHandler, *taskTagHandlerRepository)
		run        func(*TaskTagHandler, http.ResponseWriter)
		wantStatus int
	}{
		{
			name: "blank create name",
			run: func(handler *TaskTagHandler, response http.ResponseWriter) {
				handler.Create(response, taskTagHandlerRequest(http.MethodPost, "/task-tags", `{"name":"  "}`, "user-1"))
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "multi tag IDs rejected",
			run: func(handler *TaskTagHandler, response http.ResponseWriter) {
				request := taskTagHandlerRequest(http.MethodPost, "/tasks/task-1/tags:add", `{"tag_ids":["one","two"]}`, "user-1")
				handler.AddToTask(response, request)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unowned tag hidden",
			configure: func(_ *TaskTagHandler, repo *taskTagHandlerRepository) {
				repo.getErr = taskusecase.ErrTaskTagNotFound
			},
			run: func(handler *TaskTagHandler, response http.ResponseWriter) {
				request := taskTagHandlerRequestWithPathParam(taskTagHandlerRequest(http.MethodPatch, "/task-tags/tag-1", `{"name":"x"}`, "user-1"), "id", "tag-1")
				handler.Rename(response, request)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "task or tag owner mismatch hidden",
			configure: func(_ *TaskTagHandler, repo *taskTagHandlerRepository) {
				repo.addErr = taskusecase.ErrTaskTagAssignmentNotOwned
			},
			run: func(handler *TaskTagHandler, response http.ResponseWriter) {
				request := taskTagHandlerRequestWithPathParam(taskTagHandlerRequest(http.MethodPost, "/tasks/task-1/tags:add", `{"tag_id":"tag-1"}`, "user-1"), "id", "task-1")
				handler.AddToTask(response, request)
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "case insensitive name conflict",
			configure: func(_ *TaskTagHandler, repo *taskTagHandlerRepository) {
				repo.createErr = taskusecase.ErrTaskTagNameConflict
			},
			run: func(handler *TaskTagHandler, response http.ResponseWriter) {
				handler.Create(response, taskTagHandlerRequest(http.MethodPost, "/task-tags", `{"name":"work"}`, "user-1"))
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "database failure is safe",
			configure: func(_ *TaskTagHandler, repo *taskTagHandlerRepository) {
				repo.listErr = errors.New("secret database failure")
			},
			run: func(handler *TaskTagHandler, response http.ResponseWriter) {
				handler.List(response, taskTagHandlerRequest(http.MethodGet, "/task-tags", "", "user-1"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, repo := newTaskTagHandlerFixture()
			if test.configure != nil {
				test.configure(handler, repo)
			}
			response := httptest.NewRecorder()
			test.run(handler, response)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus == http.StatusInternalServerError && strings.Contains(response.Body.String(), "secret database failure") {
				t.Errorf("response leaked internal error: %s", response.Body.String())
			}
		})
	}
}
