package rest

import (
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
)

type TaskRevisionResponse struct {
	ID               string  `json:"id"`
	Revision         int32   `json:"revision"`
	UserID           string  `json:"user_id"`
	ProjectID        *string `json:"project_id"`
	AssigneeID       string  `json:"assignee_id"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	DueDate          *string `json:"due_date"`
	EstimatedMinutes *int    `json:"estimated_minutes"`
	ActualMinutes    *int    `json:"actual_minutes"`
	Priority         string  `json:"priority"`
	Status           string  `json:"status"`
	DeletedAt        *int64  `json:"deleted_at"`
	CreatedAt        int64   `json:"created_at"`
	UpdatedAt        int64   `json:"updated_at"`
	ChangedBy        string  `json:"changed_by"`
	ChangedAt        int64   `json:"changed_at"`
}

type TaskRevisionListResponse struct {
	Items         []TaskRevisionResponse `json:"items"`
	NextPageToken string                 `json:"next_page_token"`
}

// ListRevisions returns task entity snapshots available to the actor.
//
//	@Summary	List task revisions
//	@Tags		Tasks
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Task ID"
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Success	200			{object}	TaskRevisionListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Router		/tasks/{id}/revisions [get]
func (h *TaskHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.userID(r)
	if !ok {
		writeUnauthorized(w, errSpecTasksGetFailed)
		return
	}
	taskID := taskIDFromRequest(r)
	if taskID == "" {
		writeTaskError(w, errSpecTasksGetFailed, taskusecase.ErrTaskNotFound)
		return
	}
	request, err := parseListRequest(r, h.pageTokens, string(actor), "task_revisions", taskID, "revision_desc", listEnvelope[TaskRevisionResponse]{})
	if err != nil {
		writeListError(w, errSpecTasksListFailed, err)
		return
	}
	var anchor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		anchor = &taskusecase.CursorAnchor{Revision: request.Anchor.Revision}
	}
	page, err := h.tasks.Revisions.Execute(r.Context(), actor, domain.TaskID(taskID), taskusecase.CursorPageRequest{Size: request.Size, Anchor: anchor})
	if err != nil {
		writeTaskError(w, errSpecTasksGetFailed, err)
		return
	}
	items := make([]TaskRevisionResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, taskRevisionResponse(row))
		anchors = append(anchors, listAnchor{Revision: row.Revision})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, errSpecTasksGetFailed)
}

func taskRevisionResponse(row dao.TaskRevision) TaskRevisionResponse {
	response := TaskRevisionResponse{
		ID: row.ID, Revision: row.Revision, UserID: row.UserID, AssigneeID: row.AssigneeID,
		Title: row.Title, Description: row.Description, EstimatedMinutes: cloneInt(row.EstimatedMinutes),
		ActualMinutes: cloneInt(row.ActualMinutes), Priority: row.Priority, Status: row.Status,
		DeletedAt: cloneInt64(row.DeletedAt), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ChangedBy: row.ChangedBy, ChangedAt: row.ChangedAt,
	}
	if row.ProjectID != "" {
		projectID := row.ProjectID
		response.ProjectID = &projectID
	}
	if row.DueDate != 0 {
		dueDate := time.Unix(row.DueDate, 0).UTC().Format("2006-01-02")
		response.DueDate = &dueDate
	}
	return response
}
