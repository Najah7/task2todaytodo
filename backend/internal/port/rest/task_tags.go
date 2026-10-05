package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

const taskTagResource = "task_tags"

var (
	taskTagCreateErrorSpec         = NewFailureErrSpec(taskTagResource, "create", "Failed to create task tag")
	taskTagListErrorSpec           = NewFailureErrSpec(taskTagResource, "list", "Failed to list task tags")
	taskTagRenameErrorSpec         = NewFailureErrSpec(taskTagResource, "update", "Failed to update task tag")
	taskTagDeleteErrorSpec         = NewFailureErrSpec(taskTagResource, "delete", "Failed to delete task tag")
	taskTagAddToTaskErrorSpec      = NewFailureErrSpec(taskTagResource, "add_to_task", "Failed to add tag to task")
	taskTagRemoveFromTaskErrorSpec = NewFailureErrSpec(taskTagResource, "remove_from_task", "Failed to remove tag from task")
)

type TaskTagHandler struct {
	taskTag    taskusecase.TaskTagUseCases
	ID         shared.ID
	pageTokens *pagination.Codec
}

func NewTaskTagHandler(taskTag taskusecase.TaskTagUseCases, ID shared.ID, codecs ...*pagination.Codec) *TaskTagHandler {
	h := &TaskTagHandler{taskTag: taskTag, ID: ID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type TaskTagResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type TaskTagListResponse struct {
	Items         []TaskTagResponse `json:"items"`
	NextPageToken string            `json:"next_page_token"`
}

type TaskTagCreateRequest struct {
	Name string `json:"name"`
}

type TaskTagRenameRequest struct {
	Name string `json:"name"`
}

type TaskTagAssignmentRequest struct {
	TagID string `json:"tag_id"`
}

func taskTagResponse(tag taskdao.TaskTag) TaskTagResponse {
	return TaskTagResponse{
		ID:        tag.ID,
		Name:      tag.Name,
		CreatedAt: time.Unix(tag.CreatedAt, 0).UTC().Format(time.RFC3339),
		UpdatedAt: time.Unix(tag.UpdatedAt, 0).UTC().Format(time.RFC3339),
	}
}

func taskTagUserID(r *http.Request) (taskdomain.UserID, bool) {
	id, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || id == "" {
		return "", false
	}
	return taskdomain.UserID(id), true
}

func decodeTaskTagRequest(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func taskTagIDFromPath(r *http.Request) (taskdomain.TaskTagID, bool) {
	id := chi.URLParam(r, "id")
	if id == "" {
		return "", false
	}
	return taskdomain.TaskTagID(id), true
}

func taskTagTaskIDFromPath(r *http.Request) (taskdomain.TaskID, bool) {
	id := chi.URLParam(r, "id")
	if id == "" {
		return "", false
	}
	return taskdomain.TaskID(id), true
}

func writeTaskTagError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		WriteError(w, http.StatusForbidden, spec, NewErrDetail("", "permission_denied", "The caller lacks permission to change task tags"))
	case errors.Is(err, taskdomain.ErrTaskTagNameEmpty):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("name", "name_required", "Name is required"))
	case errors.Is(err, taskusecase.ErrTaskTagNameConflict), IsUniqueConstraint(err, "task_tags_user_id_name_key"):
		WriteError(w, http.StatusConflict, spec, NewErrDetail("name", "name_already_exists", "Tag name already exists"))
	case errors.Is(err, taskusecase.ErrTaskTagNotFound), errors.Is(err, taskusecase.ErrTaskNotFound), errors.Is(err, taskusecase.ErrTaskTagAssignmentNotOwned):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("", "not_found", "Resource not found"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}

// List returns TaskTags owned by the authenticated user.
//
//	@Summary		List task tags
//	@Description	Returns task tags owned by the authenticated user.
//	@Tags			Task Tags
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param			page_token	query		string	false	"Opaque next page token"
//	@Param			fields		query		string	false	"Response field mask"
//	@Success		200			{object}	TaskTagListResponse
//	@Failure		400			{object}	ErrResponse	"Invalid pagination or fields"
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		500			{object}	ErrResponse	"Failed to list task tags"
//	@Router			/task-tags [get]
func (h *TaskTagHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskTagUserID(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, taskTagListErrorSpec, ErrDetailUnauthorized)
		return
	}

	request, err := parseListRequest(r, h.pageTokens, string(userID), "task_tags", "", "name_asc_id_asc", listEnvelope[TaskTagResponse]{})
	if err != nil {
		writeListError(w, taskTagListErrorSpec, err)
		return
	}
	var cursor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		cursor = &taskusecase.CursorAnchor{Name: request.Anchor.Name, ID: request.Anchor.ID}
	}
	page, err := h.taskTag.List.ExecutePage(r.Context(), userID, taskusecase.CursorPageRequest{Size: request.Size, Anchor: cursor})
	if err != nil {
		writeTaskTagError(w, taskTagListErrorSpec, err)
		return
	}
	items := make([]TaskTagResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, tag := range page.Items {
		items = append(items, taskTagResponse(tag))
		anchors = append(anchors, listAnchor{Name: tag.Name, ID: tag.ID})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, taskTagListErrorSpec)
}

// Create creates a TaskTag for the authenticated user.
//
//	@Summary		Create task tag
//	@Description	Creates a task tag for the authenticated user.
//	@Tags			Task Tags
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		TaskTagCreateRequest	true	"Task tag create request"
//	@Success		201		{object}	TaskTagResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body or name"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		409		{object}	ErrResponse	"Tag name already exists"
//	@Failure		500		{object}	ErrResponse	"Failed to create task tag"
//	@Router			/task-tags [post]
func (h *TaskTagHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskTagUserID(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, taskTagCreateErrorSpec, ErrDetailUnauthorized)
		return
	}
	var request TaskTagCreateRequest
	if err := decodeTaskTagRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, taskTagCreateErrorSpec, ErrDetailInvalidRequestBody)
		return
	}
	if h.ID == nil {
		WriteError(w, http.StatusInternalServerError, taskTagCreateErrorSpec, ErrDetailInternalServerError)
		return
	}

	tag, err := h.taskTag.Create.Execute(r.Context(), taskusecase.CreateTaskTagInput{
		ID:     taskdomain.TaskTagID(h.ID.Generate()),
		UserID: userID,
		Name:   request.Name,
	})
	if err != nil {
		writeTaskTagError(w, taskTagCreateErrorSpec, err)
		return
	}
	WriteJSON(w, http.StatusCreated, taskTagResponse(tag))
}

// Rename renames a TaskTag owned by the authenticated user.
//
//	@Summary		Rename task tag
//	@Description	Renames a task tag owned by the authenticated user.
//	@Tags			Task Tags
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string					true	"Task tag ID"
//	@Param			request	body		TaskTagRenameRequest	true	"Task tag rename request"
//	@Success		200		{object}	TaskTagResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body or name"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task tag not found"
//	@Failure		409		{object}	ErrResponse	"Tag name already exists"
//	@Failure		500		{object}	ErrResponse	"Failed to update task tag"
//	@Router			/task-tags/{id} [patch]
func (h *TaskTagHandler) Rename(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskTagUserID(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, taskTagRenameErrorSpec, ErrDetailUnauthorized)
		return
	}
	tagID, ok := taskTagIDFromPath(r)
	if !ok {
		WriteError(w, http.StatusBadRequest, taskTagRenameErrorSpec, NewErrDetail("id", "invalid_id", "Task tag ID is required"))
		return
	}
	var request TaskTagRenameRequest
	if err := decodeTaskTagRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, taskTagRenameErrorSpec, ErrDetailInvalidRequestBody)
		return
	}

	tag, err := h.taskTag.Rename.Execute(r.Context(), userID, tagID, request.Name)
	if err != nil {
		writeTaskTagError(w, taskTagRenameErrorSpec, err)
		return
	}
	WriteJSON(w, http.StatusOK, taskTagResponse(tag))
}

// Delete removes a TaskTag owned by the authenticated user.
//
//	@Summary		Delete task tag
//	@Description	Deletes a task tag owned by the authenticated user.
//	@Tags			Task Tags
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Task tag ID"
//	@Success		204	"Task tag deleted"
//	@Failure		400	{object}	ErrResponse	"Invalid task tag ID"
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		404	{object}	ErrResponse	"Task tag not found"
//	@Failure		500	{object}	ErrResponse	"Failed to delete task tag"
//	@Router			/task-tags/{id} [delete]
func (h *TaskTagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := taskTagUserID(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, taskTagDeleteErrorSpec, ErrDetailUnauthorized)
		return
	}
	tagID, ok := taskTagIDFromPath(r)
	if !ok {
		WriteError(w, http.StatusBadRequest, taskTagDeleteErrorSpec, NewErrDetail("id", "invalid_id", "Task tag ID is required"))
		return
	}
	if err := h.taskTag.Delete.Execute(r.Context(), userID, tagID); err != nil {
		writeTaskTagError(w, taskTagDeleteErrorSpec, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddToTask assigns one TaskTag to an owned Task.
//
//	@Summary		Add tag to task
//	@Description	Assigns one user-owned tag to a task owned by the authenticated user.
//	@Tags			Task Tags
//	@Accept			json
//	@Security		BearerAuth
//	@Param			id		path	string						true	"Task ID"
//	@Param			request	body	TaskTagAssignmentRequest	true	"Tag assignment request"
//	@Success		204		"Tag assigned"
//	@Failure		400		{object}	ErrResponse	"Invalid request body or task ID"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or tag not found"
//	@Failure		500		{object}	ErrResponse	"Failed to add tag to task"
//	@Router			/tasks/{id}/tags:add [post]
func (h *TaskTagHandler) AddToTask(w http.ResponseWriter, r *http.Request) {
	h.assignTag(w, r, taskTagAddToTaskErrorSpec, h.taskTag.AddToTask.Execute)
}

// RemoveFromTask removes one TaskTag assignment from an owned Task.
//
//	@Summary		Remove tag from task
//	@Description	Removes one user-owned tag from a task owned by the authenticated user.
//	@Tags			Task Tags
//	@Accept			json
//	@Security		BearerAuth
//	@Param			id		path	string						true	"Task ID"
//	@Param			request	body	TaskTagAssignmentRequest	true	"Tag assignment request"
//	@Success		204		"Tag removed"
//	@Failure		400		{object}	ErrResponse	"Invalid request body or task ID"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		404		{object}	ErrResponse	"Task or tag not found"
//	@Failure		500		{object}	ErrResponse	"Failed to remove tag from task"
//	@Router			/tasks/{id}/tags:remove [post]
func (h *TaskTagHandler) RemoveFromTask(w http.ResponseWriter, r *http.Request) {
	h.assignTag(w, r, taskTagRemoveFromTaskErrorSpec, h.taskTag.RemoveFromTask.Execute)
}

func (h *TaskTagHandler) assignTag(
	w http.ResponseWriter,
	r *http.Request,
	spec ErrSpec,
	execute func(context.Context, taskdomain.UserID, taskdomain.TaskID, taskdomain.TaskTagID) error,
) {
	userID, taskID, tagID, ok := taskTagAssignmentInput(w, r, spec)
	if !ok {
		return
	}
	taskTagAssignmentError(w, spec, execute(r.Context(), userID, taskID, tagID))
}

func taskTagAssignmentError(w http.ResponseWriter, spec ErrSpec, err error) {
	if err != nil {
		writeTaskTagError(w, spec, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func taskTagAssignmentInput(w http.ResponseWriter, r *http.Request, spec ErrSpec) (taskdomain.UserID, taskdomain.TaskID, taskdomain.TaskTagID, bool) {
	userID, ok := taskTagUserID(r)
	if !ok {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", "", "", false
	}
	taskID, ok := taskTagTaskIDFromPath(r)
	if !ok {
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("id", "invalid_id", "Task ID is required"))
		return "", "", "", false
	}
	var request TaskTagAssignmentRequest
	if err := decodeTaskTagRequest(r, &request); err != nil {
		WriteError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
		return "", "", "", false
	}
	tagID := request.TagID
	return userID, taskID, taskdomain.TaskTagID(tagID), true
}
