package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	projectdao "github.com/Najah7/task2todaytodo/internal/application/project/dao"
	projectdomain "github.com/Najah7/task2todaytodo/internal/application/project/domain"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	sharedstatus "github.com/Najah7/task2todaytodo/internal/application/shared/status"
	taskdao "github.com/Najah7/task2todaytodo/internal/application/task/dao"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type ProjectHandler struct {
	projects   *projectusecase.UseCases
	tasks      taskusecase.TaskUseCases
	ID         shared.ID
	pageTokens *pagination.Codec
}

func NewProjectHandler(projects *projectusecase.UseCases, tasks taskusecase.TaskUseCases, ID shared.ID, codecs ...*pagination.Codec) *ProjectHandler {
	h := &ProjectHandler{projects: projects, tasks: tasks, ID: ID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type ProjectTypeResponse struct {
	Value   string `json:"value" validate:"required"`
	Label   string `json:"label" validate:"required"`
	LabelJp string `json:"label_jp" validate:"required"`
}

type ProjectPriorityResponse struct {
	Value   string `json:"value" validate:"required"`
	Label   string `json:"label" validate:"required"`
	LabelJp string `json:"label_jp" validate:"required"`
	Weight  int    `json:"weight" validate:"required"`
}

type ProjectResponse struct {
	ID            string                  `json:"id" validate:"required"`
	UserID        string                  `json:"user_id" validate:"required"`
	Type          ProjectTypeResponse     `json:"type" validate:"required"`
	Title         string                  `json:"title" validate:"required"`
	Goal          string                  `json:"goal" validate:"required"`
	Description   string                  `json:"description" validate:"required"`
	Progress      int                     `json:"progress" validate:"required"`
	Status        string                  `json:"status" validate:"required" enums:"open,pending,waiting_on_others,in_progress,done"`
	Priority      ProjectPriorityResponse `json:"priority" validate:"required"`
	StartDate     *string                 `json:"start_date" validate:"required" extensions:"x-nullable"`
	EndDate       *string                 `json:"end_date" validate:"required" extensions:"x-nullable"`
	RemainingDays *int                    `json:"remaining_days" validate:"required" extensions:"x-nullable"`
	DeletedAt     *int64                  `json:"deleted_at" validate:"required" extensions:"x-nullable"`
	CanUpdate     bool                    `json:"can_update" validate:"required"`
	CanDelete     bool                    `json:"can_delete" validate:"required"`
	CreatedAt     int64                   `json:"created_at" validate:"required"`
	UpdatedAt     int64                   `json:"updated_at" validate:"required"`
	Revision      int32                   `json:"revision" validate:"required"`
}

type ProjectListResponse struct {
	Items             []ProjectResponse          `json:"items" validate:"required"`
	Summary           ProjectListSummaryResponse `json:"summary" validate:"required"`
	NextPageToken     string                     `json:"next_page_token" validate:"required"`
	PreviousPageToken string                     `json:"previous_page_token" validate:"required"`
}

type ProjectListSummaryResponse struct {
	TotalCount   int64               `json:"total_count" validate:"required"`
	DueSoonCount int64               `json:"due_soon_count" validate:"required"`
	OverdueCount int64               `json:"overdue_count" validate:"required"`
	StatusCounts ProjectStatusCounts `json:"status_counts" validate:"required"`
	TrashCount   int64               `json:"trash_count" validate:"required"`
	Today        string              `json:"today" validate:"required"`
	Timezone     string              `json:"timezone" validate:"required"`
}

type ProjectStatusCounts struct {
	InProgress      int64 `json:"in_progress" validate:"required"`
	Pending         int64 `json:"pending" validate:"required"`
	Done            int64 `json:"done" validate:"required"`
	Open            int64 `json:"open" validate:"required"`
	WaitingOnOthers int64 `json:"waiting_on_others" validate:"required"`
}

type ProjectOptionsResponse struct {
	Types      []ProjectTypeResponse       `json:"types" validate:"required"`
	Priorities []ProjectPriorityResponse   `json:"priorities" validate:"required"`
	Statuses   []ProjectTaskStatusResponse `json:"statuses" validate:"required"`
}

type ProjectStatusRequest struct {
	Status string `json:"status" validate:"required" enums:"open,pending,waiting_on_others,in_progress,done"`
}

type ProjectCreateRequest struct {
	Type        string  `json:"type"`
	Title       string  `json:"title" validate:"required"`
	Goal        string  `json:"goal"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	StartDate   *string `json:"start_date" extensions:"x-nullable"`
	EndDate     *string `json:"end_date" extensions:"x-nullable"`
}

type ProjectTaskCreateRequest struct {
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	DueDate          *string `json:"due_date"`
	EstimatedMinutes *int    `json:"estimated_minutes"`
	Priority         string  `json:"priority"`
}

type ProjectTaskAssignmentRequest struct {
	TaskID string `json:"task_id"`
}

type ProjectTaskResponse struct {
	ID               string                    `json:"id"`
	UserID           string                    `json:"user_id"`
	AssigneeID       string                    `json:"assignee_id"`
	Revision         int32                     `json:"revision"`
	ProjectID        string                    `json:"project_id,omitempty"`
	Title            string                    `json:"title"`
	Description      string                    `json:"description"`
	DueDate          *string                   `json:"due_date"`
	EstimatedMinutes *int                      `json:"estimated_minutes"`
	ActualMinutes    *int                      `json:"actual_minutes"`
	Progress         int                       `json:"progress"`
	Priority         ProjectPriorityResponse   `json:"priority"`
	Status           ProjectTaskStatusResponse `json:"status"`
	CreatedAt        int64                     `json:"created_at"`
	UpdatedAt        int64                     `json:"updated_at"`
}

type ProjectTaskStatusResponse struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	LabelJp string `json:"label_jp"`
}

type ProjectTaskPageResponse struct {
	Items         []ProjectTaskResponse `json:"items"`
	NextPageToken string                `json:"next_page_token"`
}

type ProjectUpdateRequest map[string]json.RawMessage

// ProjectUpdateRequestSchema documents the nullable fields accepted by the PATCH endpoint.
// The handler keeps a raw object so it can distinguish omitted fields from null.
type ProjectUpdateRequestSchema struct {
	Title       *string `json:"title"`
	Goal        *string `json:"goal" extensions:"x-nullable"`
	Description *string `json:"description" extensions:"x-nullable"`
	Type        *string `json:"type"`
	Priority    *string `json:"priority"`
	StartDate   *string `json:"start_date" extensions:"x-nullable"`
	EndDate     *string `json:"end_date" extensions:"x-nullable"`
}

var (
	projectListFailure       = NewFailureErrSpec(ResourceProjects, ActionList, "Failed to list projects")
	projectCreateTaskFailure = NewFailureErrSpec(ResourceTasks, ActionCreate, "Failed to create task")
	projectListTasksFailure  = NewFailureErrSpec(ResourceTasks, ActionList, "Failed to list project tasks")
	projectAddTaskFailure    = NewFailureErrSpec(ResourceTasks, "add_to_project", "Failed to add task to project")
	projectRemoveTaskFailure = NewFailureErrSpec(ResourceTasks, "remove_from_project", "Failed to remove task from project")
	projectStatusFailure     = NewFailureErrSpec(ResourceProjects, ActionUpdate, "Failed to update project status")
	projectRestoreFailure    = NewFailureErrSpec(ResourceProjects, "restore", "Failed to restore project")
	projectOptionsFailure    = NewFailureErrSpec(ResourceProjects, "options", "Failed to list project form options")
)

// List returns projects owned by or shared with the authenticated user.
//
//	@Summary	List projects
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		status		query		string	false	"Filter by Project status"																													Enums(in_progress,pending,done,open,waiting_on_others)
//	@Param		view		query		string	false	"active or trash"																															Enums(active,trash)
//	@Param		sort_by		query		string	false	"Server-side sort column; end_date sorts by remaining days. Null dates sort last; equal dates sort by priority high-first, then stable ID."	Enums(created_at,title,progress,end_date)
//	@Param		sort_order	query		string	false	"Sort direction"																															Enums(asc,desc)
//	@Param		page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param		page_token	query		string	false	"Opaque cursor for previous or next page"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	ProjectListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
//	@Router		/projects [get]
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectListFailure, ErrDetailUnauthorized)
		return
	}
	listReq, filter, err := parseProjectListRequest(r, h.pageTokens, string(userID))
	if err != nil {
		writeListError(w, projectListFailure, err)
		return
	}
	page, err := h.projects.List.ExecuteFilteredPage(r.Context(), userID, filter)
	if err != nil {
		writeProjectUseCaseError(w, projectListFailure, err)
		return
	}
	items := make([]ProjectResponse, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, projectResponse(row))
	}
	summary := ProjectListSummaryResponse{
		TotalCount: page.Summary.TotalCount, DueSoonCount: page.Summary.DueSoonCount,
		OverdueCount: page.Summary.OverdueCount, TrashCount: page.Summary.TrashCount,
		Today: page.Summary.Today, Timezone: page.Summary.Timezone,
		StatusCounts: ProjectStatusCounts{InProgress: page.Summary.InProgressCount, Pending: page.Summary.PendingCount, Done: page.Summary.DoneCount, Open: page.Summary.OpenCount, WaitingOnOthers: page.Summary.WaitingOnOthersCount},
	}
	response := ProjectListResponse{Items: items, Summary: summary}
	if page.Next != nil {
		response.NextPageToken, err = encodeProjectCursor(h.pageTokens, listReq.Scope, *page.Next)
	}
	if err == nil && page.Previous != nil {
		response.PreviousPageToken, err = encodeProjectCursor(h.pageTokens, listReq.Scope, *page.Previous)
	}
	if err != nil {
		writeProjectError(w, http.StatusInternalServerError, projectListFailure, ErrDetailInternalServerError)
		return
	}
	encoded, err := listReq.Mask.Project(response)
	if err != nil {
		writeProjectError(w, http.StatusInternalServerError, projectListFailure, ErrDetailInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

// Create creates a project for the authenticated user.
//
//	@Summary	Create project
//	@Tags		Projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		ProjectCreateRequest	true	"Project"
//	@Success	201		{object}	ProjectResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/projects [post]
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, ErrSpecProjectsCreateFailed, ErrDetailUnauthorized)
		return
	}
	var request ProjectCreateRequest
	if !decodeProjectJSON(w, r, &request) {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	startDate, err := parseProjectDate(request.StartDate)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsCreateFailed, projectInvalidDate("start_date"))
		return
	}
	endDate, err := parseProjectDate(request.EndDate)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsCreateFailed, projectInvalidDate("end_date"))
		return
	}
	project, err := h.projects.Create.Execute(r.Context(), projectusecase.CreateProjectInput{
		ID: projectdomain.ProjectID(h.ID.Generate()), UserID: userID, Type: request.Type,
		Title: request.Title, Goal: request.Goal, Description: request.Description, Priority: request.Priority,
		StartDate: startDate, EndDate: endDate,
	})
	if err != nil {
		writeProjectUseCaseError(w, ErrSpecProjectsCreateFailed, err)
		return
	}
	setResourceETag(w, project.Revision)
	WriteJSON(w, http.StatusCreated, projectResponse(project))
}

// Options returns backend-owned Project form catalogs.
//
//	@Summary	List Project form options
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	ProjectOptionsResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/options [get]
func (h *ProjectHandler) Options(w http.ResponseWriter, r *http.Request) {
	if _, ok := projectUserID(r.Context()); !ok {
		writeProjectError(w, http.StatusUnauthorized, projectOptionsFailure, ErrDetailUnauthorized)
		return
	}
	options, err := h.projects.Options.Execute(r.Context())
	if err != nil {
		writeProjectUseCaseError(w, projectOptionsFailure, err)
		return
	}
	response := ProjectOptionsResponse{}
	for _, item := range options.Types {
		response.Types = append(response.Types, ProjectTypeResponse{Value: item.Value, Label: item.Label, LabelJp: item.LabelJp})
	}
	for _, item := range options.Priorities {
		response.Priorities = append(response.Priorities, ProjectPriorityResponse{Value: item.Value, Label: item.Label, LabelJp: item.LabelJp, Weight: item.Weight})
	}
	for _, item := range options.Statuses {
		response.Statuses = append(response.Statuses, ProjectTaskStatusResponse{Value: item.Value, Label: item.Label, LabelJp: item.LabelJp})
	}
	if response.Types == nil {
		response.Types = []ProjectTypeResponse{}
	}
	if response.Priorities == nil {
		response.Priorities = []ProjectPriorityResponse{}
	}
	if response.Statuses == nil {
		response.Statuses = []ProjectTaskStatusResponse{}
	}
	WriteJSON(w, http.StatusOK, response)
}

// Get returns a project the authenticated user may read.
//
//	@Summary	Get project
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Project ID"
//	@Success	200	{object}	ProjectResponse
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/{id} [get]
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, ErrSpecProjectsGetFailed, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, ErrSpecProjectsGetFailed)
	if !ok {
		return
	}
	project, err := h.projects.Get.Execute(r.Context(), userID, projectID)
	if err != nil {
		writeProjectUseCaseError(w, ErrSpecProjectsGetFailed, err)
		return
	}
	setResourceETag(w, project.Revision)
	WriteJSON(w, http.StatusOK, projectResponse(project))
}

// Update partially updates a project. Null clears nullable goal, description,
// start_date, and end_date; omitted fields keep their stored values.
//
//	@Param		If-Match	header		string		true	"Send the current ETag header from the response; it is a quoted revision number."
//	@Header		200			{string}	ETag		"Current project revision"
//	@Failure	409			{object}	ErrResponse	"Revision conflict"
//	@Failure	428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary	Update project
//	@Tags		Projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Project ID"
//	@Param		request	body		ProjectUpdateRequestSchema	true	"Project fields to update"
//	@Success	200		{object}	ProjectResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/projects/{id} [patch]
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, ErrSpecProjectsUpdateFailed, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, ErrSpecProjectsUpdateFailed)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, ErrSpecProjectsUpdateFailed)
	if !ok {
		return
	}
	var request ProjectUpdateRequest
	if !decodeProjectJSON(w, r, &request) {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if request == nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if !onlyProjectPatchFields(request) {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidBody("", "unknown_field", "Request contains an unsupported field"))
		return
	}
	title, err := patchProjectField(request, "title", decodeJSONString)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidBody("title", "invalid_title", "Title must be a string or null"))
		return
	}
	goal, err := patchProjectField(request, "goal", decodeJSONString)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidBody("goal", "invalid_goal", "Goal must be a string or null"))
		return
	}
	description, err := patchProjectField(request, "description", decodeJSONString)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidBody("description", "invalid_description", "Description must be a string or null"))
		return
	}
	projectType, err := patchProjectField(request, "type", decodeJSONString)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidBody("type", "invalid_project_type", "Type must be a string or null"))
		return
	}
	priority, err := patchProjectField(request, "priority", decodeJSONString)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidBody("priority", "invalid_priority", "Priority must be a string or null"))
		return
	}
	startDate, err := patchProjectField(request, "start_date", decodeProjectDateValue)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidDate("start_date"))
		return
	}
	endDate, err := patchProjectField(request, "end_date", decodeProjectDateValue)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, projectInvalidDate("end_date"))
		return
	}
	project, err := h.projects.Update.Execute(r.Context(), userID, projectID, expected, title, goal, description, projectType, priority, startDate, endDate)
	if err != nil {
		writeProjectUseCaseError(w, ErrSpecProjectsUpdateFailed, err)
		return
	}
	setResourceETag(w, project.Revision)
	WriteJSON(w, http.StatusOK, projectResponse(project))
}

// ChangeStatus changes a Project status immediately.
//
//	@Summary	Change project status
//	@Tags		Projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string					true	"Project ID"
//	@Param		If-Match	header		string					true	"Send the current ETag header from the response; it is a quoted revision number."
//	@Param		request		body		ProjectStatusRequest	true	"New Project status"
//	@Success	200			{object}	ProjectResponse
//	@Header		200			{string}	ETag	"Current project revision"
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Failure	409			{object}	ErrResponse
//	@Failure	428			{object}	ErrResponse
//	@Router		/projects/{id}/status [patch]
func (h *ProjectHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectStatusFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectStatusFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, projectStatusFailure)
	if !ok {
		return
	}
	var request ProjectStatusRequest
	if !decodeProjectJSON(w, r, &request) || request.Status == "" {
		writeProjectError(w, http.StatusBadRequest, projectStatusFailure, projectInvalidBody("status", "invalid_status", "Status must be one of the supported values"))
		return
	}
	project, err := h.projects.ChangeStatus.Execute(r.Context(), userID, projectID, expected, request.Status)
	if err != nil {
		writeProjectUseCaseError(w, projectStatusFailure, err)
		return
	}
	setResourceETag(w, project.Revision)
	WriteJSON(w, http.StatusOK, projectResponse(project))
}

// Restore clears a Project's tombstone while preserving the Project and descendants.
//
//	@Summary	Restore project
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Project ID"
//	@Param		If-Match	header		string	true	"Send the current ETag header from the response; it is a quoted revision number."
//	@Success	200			{object}	ProjectResponse
//	@Header		200			{string}	ETag	"Current project revision"
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Failure	409			{object}	ErrResponse
//	@Failure	428			{object}	ErrResponse
//	@Router		/projects/{id}/restore [post]
func (h *ProjectHandler) Restore(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectRestoreFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectRestoreFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, projectRestoreFailure)
	if !ok {
		return
	}
	project, err := h.projects.Restore.Execute(r.Context(), userID, projectID, expected)
	if err != nil {
		writeProjectUseCaseError(w, projectRestoreFailure, err)
		return
	}
	setResourceETag(w, project.Revision)
	WriteJSON(w, http.StatusOK, projectResponse(project))
}

// Delete hides a project and its descendants from ordinary views until restored.
//
//	@Param		If-Match	header		string		true	"Send the current ETag header from the response; it is a quoted revision number."
//	@Failure	409			{object}	ErrResponse	"Revision conflict"
//	@Failure	428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary	Delete project
//	@Tags		Projects
//	@Security	BearerAuth
//	@Param		id	path	string	true	"Project ID"
//	@Success	204
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/projects/{id} [delete]
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, ErrSpecProjectsDeleteFailed, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, ErrSpecProjectsDeleteFailed)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, ErrSpecProjectsDeleteFailed)
	if !ok {
		return
	}
	if err := h.projects.Delete.Execute(r.Context(), userID, projectID, expected); err != nil {
		writeProjectUseCaseError(w, ErrSpecProjectsDeleteFailed, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTasks returns a page of tasks for a project the authenticated user may read.
//
//	@Summary	List project tasks
//	@Tags		Tasks
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Project ID"
//	@Param		page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	ProjectTaskPageResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
//	@Router		/projects/{id}/tasks [get]
func (h *ProjectHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectListTasksFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectListTasksFailure)
	if !ok {
		return
	}
	request, err := parseListRequest(r, h.pageTokens, string(userID), "project_tasks", string(projectID), "created_at_desc_id_desc", listEnvelope[ProjectTaskResponse]{})
	if err != nil {
		writeListError(w, projectListTasksFailure, err)
		return
	}
	var cursor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		cursor = &taskusecase.CursorAnchor{At: request.Anchor.At, ID: request.Anchor.ID}
	}
	page, err := h.tasks.ListByProject.Execute(r.Context(), taskdomain.UserID(userID), taskdomain.ProjectID(projectID), taskusecase.CursorPageRequest{Size: request.Size, Anchor: cursor})
	if err != nil {
		writeProjectUseCaseError(w, projectListTasksFailure, err)
		return
	}
	items := make([]ProjectTaskResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, task := range page.Items {
		items = append(items, projectTaskResponse(task))
		anchors = append(anchors, listAnchor{At: task.CursorCreatedAt, ID: task.ID})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, projectListTasksFailure)
}

// CreateTask creates a task in a project the authenticated user may edit.
//
//	@Summary	Create task in project
//	@Tags		Tasks
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Project ID"
//	@Param		request	body		ProjectTaskCreateRequest	true	"Task"
//	@Success	201		{object}	ProjectTaskResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/projects/{id}/tasks [post]
func (h *ProjectHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectCreateTaskFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectCreateTaskFailure)
	if !ok {
		return
	}
	var request ProjectTaskCreateRequest
	if !decodeProjectJSON(w, r, &request) {
		writeProjectError(w, http.StatusBadRequest, projectCreateTaskFailure, ErrDetailInvalidRequestBody)
		return
	}
	dueDate, err := parseProjectTaskDueDate(request.DueDate)
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, projectCreateTaskFailure, projectInvalidDate("due_date"))
		return
	}
	task, err := h.tasks.CreateInProject.Execute(r.Context(), taskusecase.CreateTaskInProjectInput{
		ID: taskdomain.TaskID(h.ID.Generate()), UserID: taskdomain.UserID(userID), ProjectID: taskdomain.ProjectID(projectID),
		Title: request.Title, Description: request.Description, DueDate: dueDate,
		EstimatedMinutes: request.EstimatedMinutes, Priority: request.Priority,
	})
	if err != nil {
		writeProjectUseCaseError(w, projectCreateTaskFailure, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusCreated, projectTaskResponse(task))
}

// AddTask moves one task into a project the authenticated user may edit.
//
//	@Param		If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header		200			{string}	ETag		"Current task revision"
//	@Failure	409			{object}	ErrResponse	"Revision conflict"
//	@Failure	428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary	Add task to project
//	@Tags		Tasks
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"Project ID"
//	@Param		request	body		ProjectTaskAssignmentRequest	true	"Single task ID"
//	@Success	200		{object}	ProjectTaskResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/projects/{id}/tasks:add [post]
func (h *ProjectHandler) AddTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectAddTaskFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectAddTaskFailure)
	if !ok {
		return
	}
	request, ok := projectTaskAssignment(w, r, projectAddTaskFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, projectAddTaskFailure)
	if !ok {
		return
	}
	task, err := h.tasks.AddToProject.Execute(r.Context(), taskdomain.UserID(userID), taskdomain.ProjectID(projectID), taskdomain.TaskID(request.TaskID), expected)
	if err != nil {
		writeProjectUseCaseError(w, projectAddTaskFailure, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusOK, projectTaskResponse(task))
}

// RemoveTask detaches a task from a project the authenticated user may edit.
//
//	@Param		If-Match	header		string		true	"Current task ETag, for example \"3\""
//	@Header		200			{string}	ETag		"Current task revision"
//	@Failure	409			{object}	ErrResponse	"Revision conflict"
//	@Failure	428			{object}	ErrResponse	"If-Match is required"
//
//	@Summary	Remove task from project
//	@Tags		Tasks
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"Project ID"
//	@Param		request	body		ProjectTaskAssignmentRequest	true	"Single task ID"
//	@Success	200		{object}	ProjectTaskResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/projects/{id}/tasks:remove [post]
func (h *ProjectHandler) RemoveTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := projectUserID(r.Context())
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, projectRemoveTaskFailure, ErrDetailUnauthorized)
		return
	}
	projectID, ok := projectPathID(w, r, projectRemoveTaskFailure)
	if !ok {
		return
	}
	request, ok := projectTaskAssignment(w, r, projectRemoveTaskFailure)
	if !ok {
		return
	}
	expected, ok := expectedRevision(w, r, projectRemoveTaskFailure)
	if !ok {
		return
	}
	task, err := h.tasks.RemoveFromProject.Execute(r.Context(), taskdomain.UserID(userID), taskdomain.ProjectID(projectID), taskdomain.TaskID(request.TaskID), expected)
	if err != nil {
		writeProjectUseCaseError(w, projectRemoveTaskFailure, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusOK, projectTaskResponse(task))
}

func projectUserID(ctx context.Context) (projectdomain.UserID, bool) {
	id, ok := ctx.Value(UserIDContextKey).(string)
	if !ok || id == "" {
		return "", false
	}
	return projectdomain.UserID(id), true
}

func projectPathID(w http.ResponseWriter, r *http.Request, spec ErrSpec) (projectdomain.ProjectID, bool) {
	value := r.PathValue("id")
	if value == "" {
		writeProjectError(w, http.StatusBadRequest, spec, projectInvalidBody("id", "required", "Project ID is required"))
		return "", false
	}
	return projectdomain.ProjectID(value), true
}

func projectTaskAssignment(w http.ResponseWriter, r *http.Request, spec ErrSpec) (ProjectTaskAssignmentRequest, bool) {
	var request ProjectTaskAssignmentRequest
	if !decodeProjectJSON(w, r, &request) {
		writeProjectError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
		return request, false
	}
	if request.TaskID == "" {
		writeProjectError(w, http.StatusBadRequest, spec, projectInvalidBody("task_id", "required", "One task_id is required"))
		return request, false
	}
	return request, true
}

func decodeProjectJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}

func onlyProjectPatchFields(fields ProjectUpdateRequest) bool {
	allowed := map[string]struct{}{
		"title": {}, "goal": {}, "description": {}, "type": {}, "priority": {}, "start_date": {}, "end_date": {},
	}
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			return false
		}
	}
	return true
}

func patchProjectField[T any](fields ProjectUpdateRequest, field string, decode func(json.RawMessage) (T, error)) (projectusecase.PatchField[T], error) {
	raw, present := fields[field]
	if !present {
		return projectusecase.PatchField[T]{}, nil
	}
	patch := projectusecase.PatchField[T]{Present: true}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return patch, nil
	}
	value, err := decode(raw)
	if err != nil {
		return projectusecase.PatchField[T]{}, err
	}
	patch.Value = &value
	return patch, nil
}

func decodeJSONString(raw json.RawMessage) (string, error) {
	var value string
	err := json.Unmarshal(raw, &value)
	return value, err
}

func decodeProjectDateValue(raw json.RawMessage) (time.Time, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return time.Time{}, err
	}
	return time.Parse("2006-01-02", value)
}

func parseProjectDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	date, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func parseProjectTaskDueDate(value *string) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", *value)
	return parsed, err
}

func projectResponse(project projectdao.Project) ProjectResponse {
	return ProjectResponse{
		ID:     project.ID,
		UserID: project.UserID,
		Type:   ProjectTypeResponse{Value: project.Type.Value, Label: project.Type.Label, LabelJp: project.Type.LabelJp},
		Title:  project.Title, Goal: project.Goal, Description: project.Description, Progress: project.Progress, Status: project.Status,
		Priority:  ProjectPriorityResponse{Value: project.Priority.Value, Label: project.Priority.Label, LabelJp: project.Priority.LabelJp, Weight: project.Priority.Weight},
		StartDate: cloneProjectString(project.StartDate), EndDate: cloneProjectString(project.EndDate),
		RemainingDays: cloneProjectInt(project.RemainingDays), DeletedAt: cloneInt64(project.DeletedAt), CanUpdate: project.CanUpdate, CanDelete: project.CanDelete,
		CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt, Revision: project.Revision,
	}
}

func cloneProjectInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func projectTaskResponse(task taskdao.Task) ProjectTaskResponse {
	return ProjectTaskResponse{
		ID: task.ID, UserID: task.UserID, AssigneeID: task.AssigneeID, Revision: task.Revision,
		ProjectID: task.ProjectID, Title: task.Title, Description: task.Description,
		DueDate: projectUnixDate(task.DueDate), EstimatedMinutes: task.EstimatedMinutes, ActualMinutes: task.ActualMinutes,
		Progress:  task.Progress,
		Priority:  ProjectPriorityResponse{Value: task.Priority.Value, Label: task.Priority.Label, LabelJp: task.Priority.LabelJp, Weight: task.Priority.Weight},
		Status:    ProjectTaskStatusResponse{Value: task.Status.Value, Label: task.Status.Label, LabelJp: task.Status.LabelJp},
		CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
	}
}

func cloneProjectString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func projectUnixDate(value int64) *string {
	if value == 0 {
		return nil
	}
	formatted := time.Unix(value, 0).UTC().Format("2006-01-02")
	return &formatted
}

func writeProjectUseCaseError(w http.ResponseWriter, spec ErrSpec, err error) {
	status, detail := projectUseCaseError(err)
	writeProjectError(w, status, spec, detail)
}

func projectUseCaseError(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, projectusecase.ErrRevisionConflict), errors.Is(err, taskusecase.ErrRevisionConflict):
		return http.StatusConflict, NewErrDetail("If-Match", "revision_conflict", "The resource changed since the supplied revision")
	case errors.Is(err, projectusecase.ErrPermissionDenied), errors.Is(err, taskusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The required project permission is not granted")
	case errors.Is(err, projectdomain.ErrProjectNotFound), errors.Is(err, taskusecase.ErrTaskNotFound), errors.Is(err, taskusecase.ErrTaskProjectNotFound):
		return http.StatusNotFound, NewErrDetail("id", "not_found", "Project or task was not found")
	case IsUniqueConstraint(err, "projects_pkey"):
		return http.StatusConflict, NewErrDetail("id", "project_id_conflict", "Project ID already exists")
	case IsUniqueConstraint(err, "tasks_pkey"):
		return http.StatusConflict, NewErrDetail("id", "task_id_conflict", "Task ID already exists")
	case errors.Is(err, projectdomain.ErrProjectIDEmpty):
		return http.StatusBadRequest, projectInvalidBody("id", "required", "Project ID is required")
	case errors.Is(err, taskdomain.ErrTaskEstimatedMinutesInvalid):
		return http.StatusBadRequest, projectInvalidBody("estimated_minutes", "invalid_estimated_minutes", "Estimated minutes must be non-negative")
	case errors.Is(err, projectdomain.ErrProjectTitleEmpty), errors.Is(err, taskdomain.ErrTaskTitleEmpty):
		return http.StatusBadRequest, projectInvalidBody("title", "required", "Title is required")
	case errors.Is(err, projectdomain.ErrProjectTypeEmpty), errors.Is(err, projectdomain.ErrProjectTypeInvalid):
		return http.StatusBadRequest, projectInvalidBody("type", "invalid_project_type", "Type is not supported")
	case errors.Is(err, sharedstatus.ErrEmpty), errors.Is(err, sharedstatus.ErrInvalid):
		return http.StatusBadRequest, projectInvalidBody("status", "invalid_status", "Status is not supported")
	case errors.Is(err, taskdomain.ErrTaskPriorityEmpty), errors.Is(err, taskdomain.ErrTaskPriorityInvalid), errors.Is(err, projectdomain.ErrProjectPriorityEmpty), errors.Is(err, projectdomain.ErrProjectPriorityInvalid):
		return http.StatusBadRequest, projectInvalidBody("priority", "invalid_priority", "Priority is not supported")
	case errors.Is(err, projectdomain.ErrProjectEndDateBeforeStartDate):
		return http.StatusBadRequest, projectInvalidBody("end_date", "invalid_date_range", "End date must be on or after start date")
	case errors.Is(err, projectusecase.ErrProjectPatchRequiredFieldNull):
		return http.StatusBadRequest, projectInvalidBody("", "null_not_allowed", "Required fields cannot be null")
	case errors.Is(err, taskusecase.ErrInvalidTaskPage), errors.Is(err, projectusecase.ErrInvalidProjectPage):
		return http.StatusBadRequest, projectInvalidBody("page_size", "invalid_pagination", "Page size or cursor is invalid")
	case errors.Is(err, projectusecase.ErrInvalidProjectListRequest):
		return http.StatusBadRequest, NewErrDetail("query", "invalid_project_list_request", "Project filters, sort order, or cursor are invalid")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}

func projectInvalidDate(field string) ErrDetail {
	return projectInvalidBody(field, "invalid_date", "Date must use YYYY-MM-DD format")
}

func projectInvalidBody(field, code, message string) ErrDetail {
	return NewErrDetail(field, code, message)
}

func writeProjectError(w http.ResponseWriter, status int, spec ErrSpec, detail ErrDetail) {
	WriteError(w, status, spec, detail)
}
