package rest

import (
	"context"
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

type taskTagRESTRepository struct {
	taskusecase.TaskTagRepository
	addCalls, removeCalls int
	addErr, removeErr     error
}

func (repo *taskTagRESTRepository) ListByTaskAndUserID(context.Context, taskdomain.UserID, taskdomain.TaskID) ([]taskdao.TaskTag, error) {
	return nil, nil
}
func (repo *taskTagRESTRepository) AddToTask(context.Context, taskdomain.UserID, taskdomain.TaskID, string) error {
	repo.addCalls++
	return repo.addErr
}
func (repo *taskTagRESTRepository) RemoveFromTask(context.Context, taskdomain.UserID, taskdomain.TaskID, string) error {
	repo.removeCalls++
	return repo.removeErr
}

type taskTagRESTTaskRepository struct {
	taskusecase.TaskRepository
	task taskdao.Task
	err  error
}

func (repo taskTagRESTTaskRepository) GetByUserIDWithPermission(context.Context, taskdomain.UserID, taskdomain.TaskID, shared.Capability) (taskdao.Task, error) {
	return repo.task, repo.err
}

type taskTagRESTRepositories struct {
	taskusecase.Repositories
	tasks taskusecase.TaskRepository
	tags  taskusecase.TaskTagRepository
}

func (repos taskTagRESTRepositories) Tasks() taskusecase.TaskRepository       { return repos.tasks }
func (repos taskTagRESTRepositories) TaskTags() taskusecase.TaskTagRepository { return repos.tags }

type taskTagRESTUOW struct{ repos taskusecase.Repositories }

func (uow taskTagRESTUOW) Do(ctx context.Context, fn func(context.Context, taskusecase.Repositories) error) error {
	return fn(ctx, uow.repos)
}

func taskTagRESTRequest(method, path, body, actor string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if actor != "" {
		request = request.WithContext(context.WithValue(request.Context(), UserIDContextKey, actor))
	}
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "task-1")
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeCtx))
}

func newTaskTagRESTHandler() (*TaskTagHandler, *taskTagRESTRepository) {
	tags := &taskTagRESTRepository{}
	repos := taskTagRESTRepositories{tasks: taskTagRESTTaskRepository{task: taskdao.Task{ID: "task-1", UserID: "owner-1"}}, tags: tags}
	uow := taskTagRESTUOW{repos: repos}
	return NewTaskTagHandler(taskusecase.TaskTagUseCases{
		AddToTask:      taskusecase.NewAddTagToTaskUseCase(uow, nil),
		RemoveFromTask: taskusecase.NewRemoveTagFromTaskUseCase(uow, nil),
	}), tags
}

func TestTaskTagHandlerAddAndRemoveUseSharedTagID(t *testing.T) {
	handler, repo := newTaskTagRESTHandler()
	add := httptest.NewRecorder()
	handler.AddToTask(add, taskTagRESTRequest(http.MethodPost, "/tasks/task-1/tags:add", `{"tag_id":"tag-1"}`, "member-1"))
	if add.Code != http.StatusOK || repo.addCalls != 1 {
		t.Fatalf("add status/calls = %d/%d, body=%s", add.Code, repo.addCalls, add.Body.String())
	}
	remove := httptest.NewRecorder()
	handler.RemoveFromTask(remove, taskTagRESTRequest(http.MethodPost, "/tasks/task-1/tags:remove", `{"tag_id":"tag-1"}`, "member-1"))
	if remove.Code != http.StatusOK || repo.removeCalls != 1 {
		t.Fatalf("remove status/calls = %d/%d, body=%s", remove.Code, repo.removeCalls, remove.Body.String())
	}
}

func TestTaskTagHandlerRejectsUnauthenticatedAndUnownedTag(t *testing.T) {
	handler, tags := newTaskTagRESTHandler()
	unauthorized := httptest.NewRecorder()
	handler.AddToTask(unauthorized, taskTagRESTRequest(http.MethodPost, "/tasks/task-1/tags:add", `{"tag_id":"tag-1"}`, ""))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}
	tags.addErr = taskusecase.ErrTaskTagAssignmentNotOwned
	unowned := httptest.NewRecorder()
	handler.AddToTask(unowned, taskTagRESTRequest(http.MethodPost, "/tasks/task-1/tags:add", `{"tag_id":"foreign-tag"}`, "member-1"))
	if unowned.Code != http.StatusNotFound {
		t.Fatalf("unowned tag status = %d, body=%s", unowned.Code, unowned.Body.String())
	}
}
