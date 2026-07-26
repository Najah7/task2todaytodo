package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/domain/auth"
	"github.com/Najah7/task2todaytodo/internal/domain/shared"
	domain "github.com/Najah7/task2todaytodo/internal/domain/task"
	"github.com/go-chi/chi/v5"
)

type ProjectHandler struct {
	svc   *domain.ProjectService
	idGen shared.IDGenerator
}

func NewProjectHandler(svc *domain.ProjectService, idGen shared.IDGenerator) *ProjectHandler {
	return &ProjectHandler{
		svc:   svc,
		idGen: idGen,
	}
}

type ProjectResponse struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Goal        string    `json:"goal"`
	Description string    `json:"description"`
	Progress    int       `json:"progress"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectAggregateResponse struct {
	ProjectResponse
	Tasks []ProjectTaskResponse `json:"tasks"`
}

type ProjectTaskResponse struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	EstimatedMinutes *int      `json:"estimated_minutes"`
	ActualMinutes    *int      `json:"actual_minutes"`
	Progress         int       `json:"progress"`
	Priority         string    `json:"priority"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func newProjectResponse(project domain.Project) ProjectResponse {
	return ProjectResponse{
		ID:          string(project.ID),
		Type:        project.Type.String(),
		Title:       project.Title,
		Goal:        project.Goal,
		Description: project.Description,
		Progress:    project.Progress,
		StartAt:     project.StartAt,
		EndAt:       project.EndAt,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}
}

func newProjectAggregateResponse(aggregate domain.ProjectAggregate) ProjectAggregateResponse {
	tasks := make([]ProjectTaskResponse, 0, len(aggregate.Tasks))
	for _, task := range aggregate.Tasks {
		tasks = append(tasks, ProjectTaskResponse{
			ID:               string(task.ID),
			ProjectID:        string(task.ProjectID),
			Title:            task.Title,
			Description:      task.Description,
			EstimatedMinutes: task.EstimatedMinutes,
			ActualMinutes:    task.ActualMinutes,
			Progress:         task.Progress,
			Priority:         task.Priority.String(),
			Status:           task.Status.String(),
			CreatedAt:        task.CreatedAt,
			UpdatedAt:        task.UpdatedAt,
		})
	}

	return ProjectAggregateResponse{
		ProjectResponse: newProjectResponse(aggregate.Project),
		Tasks:           tasks,
	}
}

type ProjectCreateRequest struct {
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Goal        string    `json:"goal"`
	Description string    `json:"description"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
}

// Create godoc
//
//	@Summary		Create project
//	@Description	Creates a Project for the authenticated user.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		ProjectCreateRequest	true	"Project create request"
//	@Success		201		{object}	ProjectResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		409		{object}	ErrResponse	"Project ID already exists"
//	@Failure		500		{object}	ErrResponse	"Failed to create project"
//	@Router			/projects [post]
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := projectUserIDFromContext(ctx)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecProjectsCreateFailed, ErrDetailUnauthorized)
		return
	}

	var req ProjectCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecProjectsCreateFailed, ErrDetailInvalidRequestBody)
		return
	}

	project, err := h.svc.Create(ctx, h.idGen.Generate, userID, req.Type, req.Title, req.Goal, req.Description, req.StartAt, req.EndAt)
	if err != nil {
		status, detail := projectErrToErrResponse(err)
		WriteError(w, status, ErrSpecProjectsCreateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusCreated, newProjectResponse(project))
}

// Get godoc
//
//	@Summary		Get project
//	@Description	Returns an owned Project with its Tasks ordered by creation time.
//	@Tags			Projects
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string	true	"Project ID"
//	@Success		200			{object}	ProjectAggregateResponse
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		404			{object}	ErrResponse	"Project not found"
//	@Failure		500			{object}	ErrResponse	"Failed to get project"
//	@Router			/projects/{project_id} [get]
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := projectUserIDFromContext(ctx)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecProjectsGetFailed, ErrDetailUnauthorized)
		return
	}

	aggregate, err := h.svc.GetAggregate(ctx, userID, projectIDFromRequest(r))
	if err != nil {
		status, detail := projectErrToErrResponse(err)
		WriteError(w, status, ErrSpecProjectsGetFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newProjectAggregateResponse(aggregate))
}

type ProjectUpdateRequest struct {
	Type        *string    `json:"type"`
	Title       *string    `json:"title"`
	Goal        *string    `json:"goal"`
	Description *string    `json:"description"`
	StartAt     *time.Time `json:"start_at"`
	EndAt       *time.Time `json:"end_at"`
}

func (req ProjectUpdateRequest) toDomainUpdate() domain.ProjectUpdate {
	return domain.ProjectUpdate{
		Type:        req.Type,
		Title:       req.Title,
		Goal:        req.Goal,
		Description: req.Description,
		StartAt:     req.StartAt,
		EndAt:       req.EndAt,
	}
}

// Update godoc
//
//	@Summary		Update project
//	@Description	Partially updates owned Project basic information.
//	@Tags			Projects
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string					true	"Project ID"
//	@Param			request		body		ProjectUpdateRequest	true	"Project update request"
//	@Success		200			{object}	ProjectResponse
//	@Failure		400			{object}	ErrResponse	"Invalid request body"
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		404			{object}	ErrResponse	"Project not found"
//	@Failure		500			{object}	ErrResponse	"Failed to update project"
//	@Router			/projects/{project_id} [patch]
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := projectUserIDFromContext(ctx)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecProjectsUpdateFailed, ErrDetailUnauthorized)
		return
	}

	var req ProjectUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}

	project, err := h.svc.Update(ctx, userID, projectIDFromRequest(r), req.toDomainUpdate())
	if err != nil {
		status, detail := projectErrToErrResponse(err)
		WriteError(w, status, ErrSpecProjectsUpdateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newProjectResponse(project))
}

// Delete godoc
//
//	@Summary		Delete project
//	@Description	Deletes an owned Project. Existing Tasks are kept and detached by database constraints.
//	@Tags			Projects
//	@Produce		json
//	@Security		BearerAuth
//	@Param			project_id	path		string			true	"Project ID"
//	@Success		200			{object}	MessageResponse	"OK"
//	@Failure		401			{object}	ErrResponse		"Unauthorized"
//	@Failure		404			{object}	ErrResponse		"Project not found"
//	@Failure		500			{object}	ErrResponse		"Failed to delete project"
//	@Router			/projects/{project_id} [delete]
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := projectUserIDFromContext(ctx)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecProjectsDeleteFailed, ErrDetailUnauthorized)
		return
	}

	if err := h.svc.Delete(ctx, userID, projectIDFromRequest(r)); err != nil {
		status, detail := projectErrToErrResponse(err)
		WriteError(w, status, ErrSpecProjectsDeleteFailed, detail)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}

func projectUserIDFromContext(ctx context.Context) (domain.UserID, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(auth.UserID)
	if !ok || userID == "" {
		return "", false
	}
	return domain.UserID(userID), true
}

func projectIDFromRequest(r *http.Request) domain.ProjectID {
	return domain.ProjectID(chi.URLParam(r, "project_id"))
}

func projectErrToErrResponse(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, domain.ErrProjectNotFound):
		return http.StatusNotFound, NewErrDetail("project_id", "project_not_found", "Project not found")
	case isUniqueConstraint(err, "projects_pkey"):
		return http.StatusConflict, NewErrDetail("project_id", "project_id_already_exists", "Project ID already exists")
	case errors.Is(err, domain.ErrProjectIDEmpty):
		return http.StatusBadRequest, NewErrDetail("project_id", "invalid_project_id", "Project ID is required")
	case errors.Is(err, domain.ErrProjectUserIDEmpty):
		return http.StatusUnauthorized, ErrDetailUnauthorized
	case errors.Is(err, domain.ErrProjectTypeEmpty), errors.Is(err, domain.ErrProjectTypeInvalid):
		return http.StatusBadRequest, NewErrDetail("type", "invalid_project_type", "Project type must be one of the supported values")
	case errors.Is(err, domain.ErrProjectTitleEmpty):
		return http.StatusBadRequest, NewErrDetail("title", "project_title_required", "Project title is required")
	case errors.Is(err, domain.ErrProjectStartAtEmpty):
		return http.StatusBadRequest, NewErrDetail("start_at", "project_start_at_required", "Project start time is required")
	case errors.Is(err, domain.ErrProjectEndAtEmpty):
		return http.StatusBadRequest, NewErrDetail("end_at", "project_end_at_required", "Project end time is required")
	case errors.Is(err, domain.ErrProjectEndAtMustBeAfterStartAt):
		return http.StatusBadRequest, NewErrDetail("end_at", "invalid_project_date_range", "Project end time must be after start time")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}
