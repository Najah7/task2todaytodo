package rest

import (
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/usecase"
)

type ProjectRevisionResponse struct {
	ID          string  `json:"id"`
	Revision    int32   `json:"revision"`
	UserID      string  `json:"user_id"`
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Goal        string  `json:"goal"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
	DeletedAt   *int64  `json:"deleted_at"`
	CreatedAt   int64   `json:"created_at"`
	UpdatedAt   int64   `json:"updated_at"`
	ChangedBy   string  `json:"changed_by"`
	ChangedAt   int64   `json:"changed_at"`
}

type ProjectRevisionListResponse struct {
	Items         []ProjectRevisionResponse `json:"items"`
	NextPageToken string                    `json:"next_page_token"`
}

// ListRevisions returns the entity snapshots visible to the authenticated user.
//
//	@Summary	List project revisions
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Project ID"
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Success	200			{object}	ProjectRevisionListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Router		/projects/{id}/revisions [get]
func (h *ProjectHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	actor, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectListFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectListFailure)
	if !ok {
		return
	}
	request, err := parseListRequest(r, h.pageTokens, string(actor), "project_revisions", string(projectID), "revision_desc", listEnvelope[ProjectRevisionResponse]{})
	if err != nil {
		writeListError(w, projectListFailure, err)
		return
	}
	var anchor *usecase.CursorAnchor
	if request.Anchor != nil {
		anchor = &usecase.CursorAnchor{Revision: request.Anchor.Revision}
	}
	page, err := h.projects.Revisions.Execute(r.Context(), actor, projectID, usecase.CursorPageRequest{Size: request.Size, Anchor: anchor})
	if err != nil {
		writeProjectUseCaseError(w, projectListFailure, err)
		return
	}
	items := make([]ProjectRevisionResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, projectRevisionResponse(row))
		anchors = append(anchors, listAnchor{Revision: row.Revision})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, projectListFailure)
}

func projectRevisionResponse(row dao.ProjectRevision) ProjectRevisionResponse {
	return ProjectRevisionResponse{
		ID: row.ID, Revision: row.Revision, UserID: row.UserID, Type: row.Type, Title: row.Title,
		Goal: row.Goal, Description: row.Description, Priority: row.Priority,
		StartDate: cloneProjectString(row.StartDate), EndDate: cloneProjectString(row.EndDate),
		DeletedAt: cloneInt64(row.DeletedAt), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ChangedBy: row.ChangedBy, ChangedAt: row.ChangedAt,
	}
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
