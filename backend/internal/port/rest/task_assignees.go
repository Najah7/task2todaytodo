package rest

import (
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	"github.com/go-chi/chi/v5"
)

type TaskAssigneeResponse struct {
	ID           string `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`
	ProjectOwner bool   `json:"project_owner"`
}

type TaskAssigneeListResponse struct {
	Items []TaskAssigneeResponse `json:"items"`
}

type TaskAssignmentRequest struct {
	AssigneeID string `json:"assignee_id"`
}

// ListAssignees lists the project owner and current project members eligible for assignment.
//
//	@Summary	List task assignees
//	@Tags		Tasks
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Task ID"
//	@Success	200	{object}	TaskAssigneeListResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Router		/tasks/{id}/assignees [get]
func (h *TaskHandler) ListAssignees(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksGetFailed)
		return
	}
	taskID := chi.URLParam(r, "id")
	assignees, err := h.tasks.ListAssignees.Execute(r.Context(), actor, domain.TaskID(taskID))
	if err != nil {
		writeTaskError(w, errSpecTasksGetFailed, err)
		return
	}
	items := make([]TaskAssigneeResponse, 0, len(assignees))
	for _, assignee := range assignees {
		items = append(items, taskAssigneeResponse(assignee))
	}
	WriteJSON(w, http.StatusOK, TaskAssigneeListResponse{Items: items})
}

// Assign changes a task's single assignee using the task revision in If-Match.
//
//	@Summary	Assign task
//	@Tags		Tasks
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path	string	true	"Task ID"
//	@Param		If-Match	header	string	true	"Current task ETag, for example \"3\""
//	@Param		request	body	TaskAssignmentRequest	true	"Assignee"
//	@Success	200	{object}	TaskResponse
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	409	{object}	ErrResponse
//	@Failure	428	{object}	ErrResponse
//	@Router		/tasks/{id}/assignees [patch]
func (h *TaskHandler) Assign(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksUpdateFailed)
		return
	}
	expected, ok := expectedRevision(w, r, errSpecTasksUpdateFailed)
	if !ok {
		return
	}
	var request TaskAssignmentRequest
	if err := decodeRequest(r, &request); err != nil || request.AssigneeID == "" {
		WriteError(w, http.StatusBadRequest, errSpecTasksUpdateFailed, NewErrDetail("assignee_id", "required", "Assignee ID is required"))
		return
	}
	updated, err := h.tasks.Assign.Execute(r.Context(), actor, domain.TaskID(chi.URLParam(r, "id")), domain.UserID(request.AssigneeID), expected)
	if err != nil {
		writeTaskError(w, errSpecTasksUpdateFailed, err)
		return
	}
	setResourceETag(w, updated.Revision)
	WriteJSON(w, http.StatusOK, taskResponse(updated))
}

func taskAssigneeResponse(row dao.TaskAssignee) TaskAssigneeResponse {
	return TaskAssigneeResponse{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, ProjectOwner: row.ProjectOwner}
}
