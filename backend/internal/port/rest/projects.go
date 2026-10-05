package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type ProjectHandler struct {
	projects   taskusecase.ProjectUseCases
	ID         shared.ID
	pageTokens *pagination.Codec
}

func NewProjectHandler(projects taskusecase.ProjectUseCases, ID shared.ID, codecs ...*pagination.Codec) *ProjectHandler {
	h := &ProjectHandler{projects: projects, ID: ID}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type ProjectTypeResponse struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	LabelJp string `json:"label_jp"`
}

type ProjectPriorityResponse struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	LabelJp string `json:"label_jp"`
	Weight  int    `json:"weight"`
}

type ProjectResponse struct {
	ID          string                  `json:"id"`
	UserID      string                  `json:"user_id"`
	Type        ProjectTypeResponse     `json:"type"`
	Title       string                  `json:"title"`
	Goal        string                  `json:"goal"`
	Description string                  `json:"description"`
	Progress    int                     `json:"progress"`
	Priority    ProjectPriorityResponse `json:"priority"`
	StartDate   *string                 `json:"start_date"`
	EndDate     *string                 `json:"end_date"`
	CreatedAt   int64                   `json:"created_at"`
	UpdatedAt   int64                   `json:"updated_at"`
	Revision    int32                   `json:"revision"`
}

type ProjectListResponse struct {
	Items         []ProjectResponse `json:"items"`
	NextPageToken string            `json:"next_page_token"`
}

type ProjectCreateRequest struct {
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Goal        string  `json:"goal"`
	Description string  `json:"description"`
	Priority    string  `json:"priority"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
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

var (
	projectListFailure       = NewFailureErrSpec(ResourceProjects, ActionList, "Failed to list projects")
	projectCreateTaskFailure = NewFailureErrSpec(ResourceTasks, ActionCreate, "Failed to create task")
	projectListTasksFailure  = NewFailureErrSpec(ResourceTasks, ActionList, "Failed to list project tasks")
	projectAddTaskFailure    = NewFailureErrSpec(ResourceTasks, "add_to_project", "Failed to add task to project")
	projectRemoveTaskFailure = NewFailureErrSpec(ResourceTasks, "remove_from_project", "Failed to remove task from project")
)

// List returns projects owned by or shared with the authenticated user.
//
//	@Summary	List projects
//	@Tags		Projects
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param		page_token	query		string	false	"Opaque next page token"
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
	request, err := parseListRequest(r, h.pageTokens, string(userID), "projects", "", "created_at_desc_id_desc", listEnvelope[ProjectResponse]{})
	if err != nil {
		writeListError(w, projectListFailure, err)
		return
	}
	var cursor *taskusecase.CursorAnchor
	if request.Anchor != nil {
		cursor = &taskusecase.CursorAnchor{At: request.Anchor.At, ID: request.Anchor.ID}
	}
	page, err := h.projects.List.ExecutePage(r.Context(), userID, taskusecase.CursorPageRequest{Size: request.Size, Anchor: cursor})
	if err != nil {
		writeProjectUseCaseError(w, projectListFailure, err)
		return
	}
	items := make([]ProjectResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, projectResponse(row))
		anchors = append(anchors, listAnchor{At: row.CursorCreatedAt, ID: row.ID})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, projectListFailure)
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
	project, err := h.projects.Create.Execute(r.Context(), taskusecase.CreateProjectInput{
		ID: domain.ProjectID(h.ID.Generate()), UserID: userID, Type: request.Type,
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
// @Param If-Match header string true "Current project ETag, for example \"3\""
// @Header 200 {string} ETag "Current project revision"
// @Failure 409 {object} ErrResponse "Revision conflict"
// @Failure 428 {object} ErrResponse "If-Match is required"
//
//	@Summary	Update project
//	@Tags		Projects
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string					true	"Project ID"
//	@Param		request	body		ProjectUpdateRequest	true	"Project fields to update"
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

// Delete logically deletes a project and all of its tasks and child records.
// @Param If-Match header string true "Current project ETag, for example \"3\""
// @Failure 409 {object} ErrResponse "Revision conflict"
// @Failure 428 {object} ErrResponse "If-Match is required"
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
//	@Tags		Projects
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
	page, err := h.projects.ListTasks.Execute(r.Context(), userID, projectID, taskusecase.CursorPageRequest{Size: request.Size, Anchor: cursor})
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
//	@Tags		Projects
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
	task, err := h.projects.CreateTask.Execute(r.Context(), taskusecase.CreateTaskInProjectInput{
		ID: domain.TaskID(h.ID.Generate()), UserID: userID, ProjectID: projectID,
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
// @Param If-Match header string true "Current task ETag, for example \"3\""
// @Header 200 {string} ETag "Current task revision"
// @Failure 409 {object} ErrResponse "Revision conflict"
// @Failure 428 {object} ErrResponse "If-Match is required"
//
//	@Summary	Add task to project
//	@Tags		Projects
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
	task, err := h.projects.AddTask.Execute(r.Context(), userID, projectID, domain.TaskID(request.TaskID), expected)
	if err != nil {
		writeProjectUseCaseError(w, projectAddTaskFailure, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusOK, projectTaskResponse(task))
}

// RemoveTask detaches a task from a project the authenticated user may edit.
// @Param If-Match header string true "Current task ETag, for example \"3\""
// @Header 200 {string} ETag "Current task revision"
// @Failure 409 {object} ErrResponse "Revision conflict"
// @Failure 428 {object} ErrResponse "If-Match is required"
//
//	@Summary	Remove task from project
//	@Tags		Projects
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
	task, err := h.projects.RemoveTask.Execute(r.Context(), userID, projectID, domain.TaskID(request.TaskID), expected)
	if err != nil {
		writeProjectUseCaseError(w, projectRemoveTaskFailure, err)
		return
	}
	setResourceETag(w, task.Revision)
	WriteJSON(w, http.StatusOK, projectTaskResponse(task))
}

func projectUserID(ctx context.Context) (domain.UserID, bool) {
	id, ok := ctx.Value(UserIDContextKey).(string)
	if !ok || id == "" {
		return "", false
	}
	return domain.UserID(id), true
}

func projectPathID(w http.ResponseWriter, r *http.Request, spec ErrSpec) (domain.ProjectID, bool) {
	value := r.PathValue("id")
	if value == "" {
		writeProjectError(w, http.StatusBadRequest, spec, projectInvalidBody("id", "required", "Project ID is required"))
		return "", false
	}
	return domain.ProjectID(value), true
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

func patchProjectField[T any](fields ProjectUpdateRequest, field string, decode func(json.RawMessage) (T, error)) (taskusecase.PatchField[T], error) {
	raw, present := fields[field]
	if !present {
		return taskusecase.PatchField[T]{}, nil
	}
	patch := taskusecase.PatchField[T]{Present: true}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return patch, nil
	}
	value, err := decode(raw)
	if err != nil {
		return taskusecase.PatchField[T]{}, err
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

func projectResponse(project dao.Project) ProjectResponse {
	return ProjectResponse{
		ID:     project.ID,
		UserID: project.UserID,
		Type:   ProjectTypeResponse{Value: project.Type.Value, Label: project.Type.Label, LabelJp: project.Type.LabelJp},
		Title:  project.Title, Goal: project.Goal, Description: project.Description, Progress: project.Progress,
		Priority:  ProjectPriorityResponse{Value: project.Priority.Value, Label: project.Priority.Label, LabelJp: project.Priority.LabelJp, Weight: project.Priority.Weight},
		StartDate: cloneProjectString(project.StartDate), EndDate: cloneProjectString(project.EndDate),
		CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt, Revision: project.Revision,
	}
}

func projectTaskResponse(task dao.Task) ProjectTaskResponse {
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
	case errors.Is(err, taskusecase.ErrRevisionConflict):
		return http.StatusConflict, NewErrDetail("If-Match", "revision_conflict", "The resource changed since the supplied revision")
	case errors.Is(err, taskusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The required project permission is not granted")
	case errors.Is(err, domain.ErrProjectNotFound), errors.Is(err, taskusecase.ErrTaskNotFound), errors.Is(err, taskusecase.ErrTaskProjectNotFound):
		return http.StatusNotFound, NewErrDetail("id", "not_found", "Project or task was not found")
	case IsUniqueConstraint(err, "projects_pkey"):
		return http.StatusConflict, NewErrDetail("id", "project_id_conflict", "Project ID already exists")
	case IsUniqueConstraint(err, "tasks_pkey"):
		return http.StatusConflict, NewErrDetail("id", "task_id_conflict", "Task ID already exists")
	case errors.Is(err, domain.ErrProjectIDEmpty):
		return http.StatusBadRequest, projectInvalidBody("id", "required", "Project ID is required")
	case errors.Is(err, domain.ErrTaskEstimatedMinutesInvalid):
		return http.StatusBadRequest, projectInvalidBody("estimated_minutes", "invalid_estimated_minutes", "Estimated minutes must be non-negative")
	case errors.Is(err, domain.ErrProjectTitleEmpty), errors.Is(err, domain.ErrTaskTitleEmpty):
		return http.StatusBadRequest, projectInvalidBody("title", "required", "Title is required")
	case errors.Is(err, domain.ErrProjectTypeEmpty), errors.Is(err, domain.ErrProjectTypeInvalid):
		return http.StatusBadRequest, projectInvalidBody("type", "invalid_project_type", "Type is not supported")
	case errors.Is(err, domain.ErrTaskPriorityEmpty), errors.Is(err, domain.ErrTaskPriorityInvalid):
		return http.StatusBadRequest, projectInvalidBody("priority", "invalid_priority", "Priority is not supported")
	case errors.Is(err, domain.ErrProjectEndDateBeforeStartDate):
		return http.StatusBadRequest, projectInvalidBody("end_date", "invalid_date_range", "End date must be on or after start date")
	case errors.Is(err, taskusecase.ErrProjectPatchRequiredFieldNull):
		return http.StatusBadRequest, projectInvalidBody("", "null_not_allowed", "Required fields cannot be null")
	case errors.Is(err, taskusecase.ErrInvalidTaskPage):
		return http.StatusBadRequest, projectInvalidBody("page_size", "invalid_pagination", "Page size or cursor is invalid")
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
