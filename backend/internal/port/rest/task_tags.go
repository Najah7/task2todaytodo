package rest

import (
	"context"
	"errors"
	"net/http"

	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/go-chi/chi/v5"
)

const taskTagResource = "task_tags"

var (
	taskTagAddFailure    = NewFailureErrSpec(taskTagResource, "add_to_task", "Failed to add tag to task")
	taskTagRemoveFailure = NewFailureErrSpec(taskTagResource, "remove_from_task", "Failed to remove tag from task")
)

type TaskTagHandler struct{ taskTags taskusecase.TaskTagUseCases }

func NewTaskTagHandler(taskTags taskusecase.TaskTagUseCases) *TaskTagHandler {
	return &TaskTagHandler{taskTags: taskTags}
}

type TaskTagAssignmentRequest struct {
	TagID string `json:"tag_id"`
}

// AddToTask links a shared Tag to a Task the caller may update.
//
//	@Summary	Add tag to task
//	@Tags		Tasks
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Task ID"
//	@Param		request	body		TaskTagAssignmentRequest	true	"Tag ID"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/tasks/{id}/tags:add [post]
func (handler *TaskTagHandler) AddToTask(w http.ResponseWriter, r *http.Request) {
	handler.assign(w, r, false)
}

// RemoveFromTask unlinks a shared Tag from a Task the caller may update.
//
//	@Summary	Remove tag from task
//	@Tags		Tasks
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Task ID"
//	@Param		request	body		TaskTagAssignmentRequest	true	"Tag ID"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/tasks/{id}/tags:remove [post]
func (handler *TaskTagHandler) RemoveFromTask(w http.ResponseWriter, r *http.Request) {
	handler.assign(w, r, true)
}

func (handler *TaskTagHandler) assign(w http.ResponseWriter, r *http.Request, remove bool) {
	spec := taskTagAddFailure
	if remove {
		spec = taskTagRemoveFailure
	}
	actor, ok := taskTagActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return
	}
	taskID := taskdomain.TaskID(chi.URLParam(r, "id"))
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("id", "required", "Task ID is required"))
		return
	}
	var request TaskTagAssignmentRequest
	if !decodeTagJSON(w, r, &request) || request.TagID == "" {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("tag_id", "required", "Tag ID is required"))
		return
	}
	var err error
	if remove {
		err = handler.taskTags.RemoveFromTask.Execute(r.Context(), actor, taskID, request.TagID)
	} else {
		err = handler.taskTags.AddToTask.Execute(r.Context(), actor, taskID, request.TagID)
	}
	if err != nil {
		writeTaskTagAssociationError(w, spec, err)
		return
	}
	message := "Tag added to task"
	if remove {
		message = "Tag removed from task"
	}
	WriteMessage(w, http.StatusOK, message)
}

func taskTagActor(ctx context.Context) (taskdomain.UserID, bool) {
	value, ok := ctx.Value(UserIDContextKey).(string)
	return taskdomain.UserID(value), ok && value != ""
}

func writeTaskTagAssociationError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("", "permission_denied", "The caller lacks permission to change Task tags"))
	case errors.Is(err, taskusecase.ErrTaskNotFound), errors.Is(err, taskusecase.ErrTaskTagNotFound), errors.Is(err, taskusecase.ErrTaskTagAssignmentNotOwned):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("tag_id", "not_found", "Task or Tag was not found"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}
