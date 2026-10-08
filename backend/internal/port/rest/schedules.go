package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

const scheduleVirtualOccurrenceID = "TASK2TODAYTODO000000000000"

var (
	scheduleListFailure      = NewFailureErrSpec("schedules", ActionList, "Failed to list schedules")
	scheduleCreateFailure    = NewFailureErrSpec("schedules", ActionCreate, "Failed to create schedule")
	scheduleGetFailure       = NewFailureErrSpec("schedules", ActionGet, "Failed to get schedule")
	scheduleUpdateFailure    = NewFailureErrSpec("schedules", ActionUpdate, "Failed to update schedule")
	scheduleDeleteFailure    = NewFailureErrSpec("schedules", ActionDelete, "Failed to delete schedule")
	scheduleFrequencyFailure = NewFailureErrSpec("schedules", "update_frequency", "Failed to update schedule frequency")
	scheduleCompleteFailure  = NewFailureErrSpec("schedules", "complete", "Failed to complete schedule")
	scheduleReopenFailure    = NewFailureErrSpec("schedules", "reopen", "Failed to reopen schedule")
	scheduleSkipFailure      = NewFailureErrSpec("schedules", "skip", "Failed to skip schedule")
	scheduleRestoreFailure   = NewFailureErrSpec("schedules", "restore", "Failed to restore schedule")
)

type ScheduleHandler struct {
	schedules  scheduleusecase.ScheduleUseCases
	ID         shared.ID
	pageTokens *pagination.Codec
}

func NewScheduleHandler(schedules scheduleusecase.ScheduleUseCases, id shared.ID, codecs ...*pagination.Codec) *ScheduleHandler {
	h := &ScheduleHandler{schedules: schedules, ID: id}
	if len(codecs) > 0 {
		h.pageTokens = codecs[0]
	}
	return h
}

type ScheduleFrequencyResponse struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	LabelJp string `json:"label_jp"`
}

type ScheduleResponse struct {
	ID                  string                      `json:"id"`
	UserID              string                      `json:"user_id"`
	ProjectID           string                      `json:"project_id,omitempty"`
	AssigneeID          string                      `json:"assignee_id"`
	Title               string                      `json:"title"`
	Description         string                      `json:"description"`
	Location            string                      `json:"location"`
	StartAt             string                      `json:"start_at"`
	EndAt               string                      `json:"end_at"`
	IntervalWeeks       int                         `json:"interval_weeks"`
	Frequencies         []ScheduleFrequencyResponse `json:"frequencies"`
	RepeatState         string                      `json:"repeat_state"`
	FrequencyAnchorDate *string                     `json:"frequency_anchor_date,omitempty"`
	SeriesID            string                      `json:"series_id"`
	OccurrenceDate      string                      `json:"occurrence_date"`
	Timezone            string                      `json:"timezone"`
	IsException         bool                        `json:"is_exception"`
	Completed           bool                        `json:"completed"`
	CreatedAt           int64                       `json:"created_at"`
	UpdatedAt           int64                       `json:"updated_at"`
	Tags                []ScheduleTagResponse       `json:"tags,omitempty"`
}

type ScheduleTagResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ScheduleListResponse struct {
	Items         []ScheduleResponse `json:"items"`
	NextPageToken string             `json:"next_page_token"`
}

type ScheduleCreateRequest struct {
	ProjectID     string   `json:"project_id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Location      string   `json:"location"`
	StartAt       *string  `json:"start_at"`
	EndAt         *string  `json:"end_at"`
	IntervalWeeks int      `json:"interval_weeks"`
	Frequencies   []string `json:"frequencies"`
}

type ScheduleUpdateRequest struct {
	Scope          *string              `json:"scope"`
	OccurrenceDate string               `json:"occurrence_date"`
	Title          optionalJSON[string] `json:"title" swaggertype:"string"`
	Description    optionalJSON[string] `json:"description" swaggertype:"string"`
	Location       optionalJSON[string] `json:"location" swaggertype:"string"`
}

type ScheduleRescheduleRequest struct {
	Scope          string  `json:"scope"`
	OccurrenceDate string  `json:"occurrence_date"`
	StartAt        *string `json:"start_at"`
	EndAt          *string `json:"end_at"`
}

type ScheduleOccurrenceRequest struct {
	OccurrenceDate string `json:"occurrence_date"`
}
type ScheduleFrequencyRequest struct {
	IntervalWeeks *int     `json:"interval_weeks"`
	Frequencies   []string `json:"frequencies"`
}
type ScheduleProjectRequest struct {
	ProjectID *string `json:"project_id"`
}
type ScheduleAssigneeRequest struct {
	AssigneeID string `json:"assignee_id"`
}
type ScheduleAssigneeResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	IsOwner   bool   `json:"is_owner"`
}
type ScheduleAssigneeListResponse struct {
	Items []ScheduleAssigneeResponse `json:"items"`
}

type ScheduleRevisionResponse struct {
	ID                  string                      `json:"id"`
	Revision            int32                       `json:"revision"`
	UserID              string                      `json:"user_id"`
	ProjectID           *string                     `json:"project_id,omitempty"`
	AssigneeID          string                      `json:"assignee_id"`
	Title               string                      `json:"title"`
	Description         string                      `json:"description"`
	Location            string                      `json:"location"`
	StartAt             string                      `json:"start_at"`
	EndAt               string                      `json:"end_at"`
	SeriesID            string                      `json:"series_id"`
	OccurrenceDate      string                      `json:"occurrence_date"`
	Timezone            string                      `json:"timezone"`
	IsException         bool                        `json:"is_exception"`
	Completed           bool                        `json:"completed"`
	DeletedAt           *int64                      `json:"deleted_at,omitempty"`
	RepeatState         string                      `json:"repeat_state"`
	FrequencyAnchorDate *string                     `json:"frequency_anchor_date,omitempty"`
	IntervalWeeks       int                         `json:"interval_weeks"`
	Frequencies         []ScheduleFrequencyResponse `json:"frequencies"`
	CreatedAt           int64                       `json:"created_at"`
	UpdatedAt           int64                       `json:"updated_at"`
	ChangedBy           string                      `json:"changed_by"`
	ChangedAt           int64                       `json:"changed_at"`
}

type ScheduleRevisionListResponse struct {
	Items         []ScheduleRevisionResponse `json:"items"`
	NextPageToken string                     `json:"next_page_token"`
}

// List returns schedules assigned to the authenticated user.
//
//	@Summary	List schedules
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Param		from_date	query		string	false	"First occurrence date (YYYY-MM-DD)"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	ScheduleListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
//	@Router		/schedules [get]
func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleListFailure, ErrDetailUnauthorized)
		return
	}
	request, err := parseListRequestWithFromDate(r, h.pageTokens, string(actor), "schedules", "", "start_at_asc_series_id_asc_occurrence_date_asc", listEnvelope[ScheduleResponse]{}, true)
	if err != nil {
		writeListError(w, scheduleListFailure, err)
		return
	}
	page, err := h.schedules.List.ExecutePage(r.Context(), actor, scheduleCursorRequest(request))
	if err != nil {
		writeScheduleUseCaseError(w, scheduleListFailure, err)
		return
	}
	h.writePage(w, r.Context(), actor, request, page.Items, page.Next, scheduleListFailure)
}

// ListProject returns schedules visible in a Project.
//
//	@Summary	List Project schedules
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Project ID"
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Param		from_date	query		string	false	"First occurrence date (YYYY-MM-DD)"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	ScheduleListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Failure	500			{object}	ErrResponse
//	@Router		/projects/{id}/schedules [get]
func (h *ScheduleHandler) ListProject(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleListFailure, ErrDetailUnauthorized)
		return
	}
	projectID := domain.ProjectID(chi.URLParam(r, "id"))
	if projectID == "" {
		WriteError(w, http.StatusBadRequest, scheduleListFailure, NewErrDetail("id", "required", "Project ID is required"))
		return
	}
	request, err := parseListRequestWithFromDate(r, h.pageTokens, string(actor), "project_schedules", string(projectID), "start_at_asc_series_id_asc_occurrence_date_asc", listEnvelope[ScheduleResponse]{}, true)
	if err != nil {
		writeListError(w, scheduleListFailure, err)
		return
	}
	page, err := h.schedules.ListProject.ExecutePage(r.Context(), actor, projectID, scheduleCursorRequest(request))
	if err != nil {
		writeScheduleUseCaseError(w, scheduleListFailure, err)
		return
	}
	h.writePage(w, r.Context(), actor, request, page.Items, page.Next, scheduleListFailure)
}

func (h *ScheduleHandler) writePage(w http.ResponseWriter, ctx context.Context, actor domain.UserID, request listRequest, rows []dao.Schedule, next *scheduleusecase.CursorAnchor, spec ErrSpec) {
	scheduleIDs := make([]domain.ScheduleID, 0, len(rows))
	seenScheduleIDs := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		lookupID := scheduleTagLookupID(row)
		if _, exists := seenScheduleIDs[lookupID]; exists {
			continue
		}
		seenScheduleIDs[lookupID] = struct{}{}
		scheduleIDs = append(scheduleIDs, domain.ScheduleID(lookupID))
	}
	tagsBySchedule, err := h.schedules.ListTagsForSchedules.Execute(ctx, actor, scheduleIDs)
	if err != nil {
		writeScheduleUseCaseError(w, spec, err)
		return
	}
	items := make([]ScheduleResponse, 0, len(rows))
	anchors := make([]listAnchor, 0, len(rows))
	for _, row := range rows {
		response := scheduleResponse(row)
		tags := tagsBySchedule[scheduleTagLookupID(row)]
		response.Tags = make([]ScheduleTagResponse, 0, len(tags))
		for _, tag := range tags {
			response.Tags = append(response.Tags, ScheduleTagResponse{ID: tag.ID, Name: tag.Name, CreatedAt: tag.CreatedAt, UpdatedAt: tag.UpdatedAt})
		}
		items = append(items, response)
		asOf := ""
		if next != nil {
			asOf = next.AsOf
		}
		anchors = append(anchors, listAnchor{At: row.CursorStartAt, SeriesID: row.SeriesID, OccurrenceDate: row.OccurrenceDate, AsOf: asOf})
	}
	writeListResponse(w, items, anchors, next != nil, request, h.pageTokens, spec)
}

func scheduleTagLookupID(row dao.Schedule) string {
	if !row.IsException && row.SeriesID != "" && row.ID != row.SeriesID {
		return row.SeriesID
	}
	return row.ID
}

// Create creates a personal Schedule or a Schedule in the named Project.
//
//	@Summary	Create schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		ScheduleCreateRequest	true	"Schedule"
//	@Success	201		{object}	ScheduleResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/schedules [post]
func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, "")
}

// CreateForProject creates a Schedule owned by the Project owner.
//
//	@Summary	Create Project schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string					true	"Project ID"
//	@Param		request	body		ScheduleCreateRequest	true	"Schedule"
//	@Success	201		{object}	ScheduleResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/projects/{id}/schedules [post]
func (h *ScheduleHandler) CreateForProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		WriteError(w, http.StatusBadRequest, scheduleCreateFailure, scheduleField("id", "required", "Project ID is required"))
		return
	}
	h.create(w, r, projectID)
}

func (h *ScheduleHandler) create(w http.ResponseWriter, r *http.Request, pathProjectID string) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleCreateFailure, ErrDetailUnauthorized)
		return
	}
	var request ScheduleCreateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	start, err := parseScheduleTimestamp(request.StartAt)
	if err != nil {
		WriteError(w, http.StatusBadRequest, scheduleCreateFailure, scheduleField("start_at", "invalid_timestamp", "Start time must be an RFC 3339 timestamp"))
		return
	}
	end, err := parseScheduleTimestamp(request.EndAt)
	if err != nil {
		WriteError(w, http.StatusBadRequest, scheduleCreateFailure, scheduleField("end_at", "invalid_timestamp", "End time must be an RFC 3339 timestamp"))
		return
	}
	projectID := request.ProjectID
	if pathProjectID != "" {
		projectID = pathProjectID
	}
	schedule, err := h.schedules.Create.Execute(r.Context(), scheduleusecase.CreateScheduleInput{ID: domain.ScheduleID(h.ID.Generate()), UserID: actor, ProjectID: domain.ProjectID(projectID), Title: request.Title, Description: request.Description, Location: request.Location, StartAt: start, EndAt: end, IntervalWeeks: request.IntervalWeeks, Frequencies: request.Frequencies})
	if err != nil {
		writeScheduleUseCaseError(w, scheduleCreateFailure, err)
		return
	}
	WriteJSON(w, http.StatusCreated, scheduleResponse(schedule))
}

// Get returns a Schedule visible to the authenticated user.
//
//	@Summary	Get schedule
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Schedule ID"
//	@Success	200	{object}	ScheduleResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/schedules/{id} [get]
func (h *ScheduleHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleGetFailure, true)
	if !ok {
		return
	}
	schedule, err := h.schedules.Get.Execute(r.Context(), actor, id)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleGetFailure, err)
		return
	}
	tags, err := h.schedules.ListTags.Execute(r.Context(), actor, id)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleGetFailure, err)
		return
	}
	response := scheduleResponse(schedule)
	response.Tags = make([]ScheduleTagResponse, 0, len(tags))
	for _, tag := range tags {
		response.Tags = append(response.Tags, ScheduleTagResponse{ID: tag.ID, Name: tag.Name, CreatedAt: tag.CreatedAt, UpdatedAt: tag.UpdatedAt})
	}
	WriteJSON(w, http.StatusOK, response)
}

// ListRevisions returns schedule snapshots available to the authenticated user.
//
//	@Summary	List schedule revisions
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Schedule ID"
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque next page token"
//	@Param		fields		query		string	false	"Response field mask"
//	@Success	200			{object}	ScheduleRevisionListResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	404			{object}	ErrResponse
//	@Router		/schedules/{id}/revisions [get]
func (h *ScheduleHandler) ListRevisions(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleGetFailure, true)
	if !ok {
		return
	}
	request, err := parseListRequest(r, h.pageTokens, string(actor), "schedule_revisions", string(id), "revision_desc", listEnvelope[ScheduleRevisionResponse]{})
	if err != nil {
		writeListError(w, scheduleGetFailure, err)
		return
	}
	var anchor *scheduleusecase.CursorAnchor
	if request.Anchor != nil {
		anchor = &scheduleusecase.CursorAnchor{Revision: request.Anchor.Revision}
	}
	page, err := h.schedules.Revisions.Execute(r.Context(), actor, id, scheduleusecase.CursorPageRequest{Size: request.Size, Anchor: anchor})
	if err != nil {
		writeScheduleUseCaseError(w, scheduleGetFailure, err)
		return
	}
	items := make([]ScheduleRevisionResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, scheduleRevisionResponse(row))
		anchors = append(anchors, listAnchor{Revision: row.Revision})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, scheduleGetFailure)
}

func scheduleRevisionResponse(row dao.ScheduleRevision) ScheduleRevisionResponse {
	location, err := time.LoadLocation(row.Timezone)
	if err != nil {
		location = time.UTC
	}
	result := ScheduleRevisionResponse{
		ID: row.ID, Revision: row.Revision, UserID: row.UserID, AssigneeID: row.AssigneeID,
		Title: row.Title, Description: row.Description, Location: row.Location,
		StartAt: time.Unix(row.StartAt, 0).In(location).Format(time.RFC3339), EndAt: time.Unix(row.EndAt, 0).In(location).Format(time.RFC3339),
		SeriesID: row.SeriesID, OccurrenceDate: row.OccurrenceDate, Timezone: row.Timezone, IsException: row.IsException,
		Completed: row.Completed, DeletedAt: row.DeletedAt, RepeatState: row.RepeatState, FrequencyAnchorDate: row.FrequencyAnchorDate,
		IntervalWeeks: row.IntervalWeeks, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ChangedBy: row.ChangedBy, ChangedAt: row.ChangedAt,
		Frequencies: make([]ScheduleFrequencyResponse, 0, len(row.Frequencies)),
	}
	if row.ProjectID != "" {
		value := row.ProjectID
		result.ProjectID = &value
	}
	for _, frequency := range row.Frequencies {
		result.Frequencies = append(result.Frequencies, ScheduleFrequencyResponse{Value: frequency.Value, Label: frequency.Label, LabelJp: frequency.LabelJp})
	}
	return result
}

// Update partially updates Schedule content, with optional current/future occurrence scope.
//
//	@Summary	Update schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string					true	"Schedule ID"
//	@Param		request	body		ScheduleUpdateRequest	true	"Schedule changes"
//	@Success	200		{object}	ScheduleResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/schedules/{id} [patch]
func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleUpdateFailure, true)
	if !ok {
		return
	}
	var request ScheduleUpdateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	scope := "current"
	if request.Scope != nil {
		scope = *request.Scope
	}
	fields := map[string]json.RawMessage{}
	for name, field := range map[string]optionalJSON[string]{"title": request.Title, "description": request.Description, "location": request.Location} {
		if !field.Present {
			continue
		}
		if field.Value == nil {
			fields[name] = json.RawMessage("null")
			continue
		}
		value, _ := json.Marshal(*field.Value)
		fields[name] = value
	}
	title, err := schedulePatchField[string](fields, "title", decodeScheduleString)
	if err != nil {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("title", "invalid_value", "Title must be a string"))
		return
	}
	description, err := schedulePatchField[string](fields, "description", decodeScheduleString)
	if err != nil {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("description", "invalid_value", "Description must be a string"))
		return
	}
	location, err := schedulePatchField[string](fields, "location", decodeScheduleString)
	if err != nil {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("location", "invalid_value", "Location must be a string"))
		return
	}
	var updated dao.Schedule
	if request.OccurrenceDate != "" {
		if _, err := parseDate(request.OccurrenceDate); err != nil {
			WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
		updated, err = h.schedules.Update.ExecuteOccurrence(r.Context(), actor, id, request.OccurrenceDate, scope, title, description, location)
	} else {
		updated, err = h.schedules.Update.Execute(r.Context(), actor, id, scope, title, description, location)
	}
	if err != nil {
		writeScheduleUseCaseError(w, scheduleUpdateFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, scheduleResponse(updated))
}

// Delete removes a Schedule or one occurrence when occurrence_date is provided.
//
//	@Summary	Delete schedule
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id				path		string	true	"Schedule ID"
//	@Param		occurrence_date	query		string	false	"Occurrence date (YYYY-MM-DD)"
//	@Success	200				{object}	MessageResponse
//	@Failure	403				{object}	ErrResponse
//	@Failure	404				{object}	ErrResponse
//	@Failure	500				{object}	ErrResponse
//	@Router		/schedules/{id} [delete]
func (h *ScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleDeleteFailure, true)
	if !ok {
		return
	}
	var err error
	if date := r.URL.Query().Get("occurrence_date"); date != "" {
		err = h.schedules.Delete.ExecuteOccurrence(r.Context(), actor, id, date)
	} else {
		err = h.schedules.Delete.Execute(r.Context(), actor, id)
	}
	if err != nil {
		writeScheduleUseCaseError(w, scheduleDeleteFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Schedule deleted")
}

// Complete marks a Schedule or occurrence complete.
//
//	@Summary	Complete schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Schedule ID"
//	@Param		request	body		ScheduleOccurrenceRequest	false	"Occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/schedules/{id}:complete [post]
func (h *ScheduleHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.changeOccurrence(w, r, scheduleCompleteFailure, "Schedule completed", true, false)
}

// Reopen clears completion for a Schedule or occurrence.
//
//	@Summary	Reopen schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Schedule ID"
//	@Param		request	body		ScheduleOccurrenceRequest	false	"Occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/schedules/{id}:reopen [post]
func (h *ScheduleHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	h.changeOccurrence(w, r, scheduleReopenFailure, "Schedule reopened", false, false)
}

// Skip marks a recurring occurrence skipped.
//
//	@Summary	Skip schedule occurrence
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Schedule ID"
//	@Param		request	body		ScheduleOccurrenceRequest	true	"Occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Router		/schedules/{id}:skip [post]
func (h *ScheduleHandler) Skip(w http.ResponseWriter, r *http.Request) {
	h.changeOccurrence(w, r, scheduleSkipFailure, "Schedule occurrence skipped", false, true)
}

// Restore makes a skipped occurrence active again.
//
//	@Summary	Restore schedule occurrence
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Schedule ID"
//	@Param		request	body		ScheduleOccurrenceRequest	true	"Occurrence date"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Router		/schedules/{id}:restore [post]
func (h *ScheduleHandler) Restore(w http.ResponseWriter, r *http.Request) {
	h.changeOccurrence(w, r, scheduleRestoreFailure, "Schedule occurrence restored", false, true)
}

func (h *ScheduleHandler) changeOccurrence(w http.ResponseWriter, r *http.Request, spec ErrSpec, message string, completion, skip bool) {
	actor, id, ok := scheduleRequestIDs(w, r, spec, true)
	if !ok {
		return
	}
	occurrenceDate, err := decodeScheduleOccurrenceDate(r)
	if err != nil {
		WriteError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
		return
	}
	if occurrenceDate != "" {
		if _, err := parseDate(occurrenceDate); err != nil {
			WriteError(w, http.StatusBadRequest, spec, scheduleField("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	var commandErr error
	switch {
	case skip:
		commandErr = h.schedules.Skip.Execute(r.Context(), actor, id, occurrenceDate)
	case spec == scheduleRestoreFailure:
		commandErr = h.schedules.Restore.Execute(r.Context(), actor, id, occurrenceDate)
	case completion && occurrenceDate != "":
		commandErr = h.schedules.Complete.ExecuteOccurrence(r.Context(), actor, id, occurrenceDate)
	case !completion && occurrenceDate != "":
		commandErr = h.schedules.Reopen.ExecuteOccurrence(r.Context(), actor, id, occurrenceDate)
	case completion:
		commandErr = h.schedules.Complete.Execute(r.Context(), actor, id)
	default:
		commandErr = h.schedules.Reopen.Execute(r.Context(), actor, id)
	}
	if commandErr != nil {
		writeScheduleUseCaseError(w, spec, commandErr)
		return
	}
	WriteMessage(w, http.StatusOK, message)
}

// UpdateFrequency updates the recurrence rule for a Schedule series.
//
//	@Summary	Update schedule frequency
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Schedule ID"
//	@Param		request	body		ScheduleFrequencyRequest	true	"Recurrence rule"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/schedules/{id}/frequency [put]
func (h *ScheduleHandler) UpdateFrequency(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleFrequencyFailure, true)
	if !ok {
		return
	}
	var request ScheduleFrequencyRequest
	if !decodeScheduleJSON(w, r, &request) || request.IntervalWeeks == nil {
		WriteError(w, http.StatusBadRequest, scheduleFrequencyFailure, ErrDetailInvalidRequestBody)
		return
	}
	if err := h.schedules.UpdateFrequency.Execute(r.Context(), scheduleusecase.UpdateScheduleFrequencyInput{UserID: actor, ScheduleID: id, IntervalWeeks: *request.IntervalWeeks, Frequencies: request.Frequencies}); err != nil {
		writeScheduleUseCaseError(w, scheduleFrequencyFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Schedule frequency updated")
}

// Reschedule moves one occurrence or a current/future portion of a Schedule series.
//
//	@Summary	Reschedule occurrence
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Schedule ID"
//	@Param		request	body		ScheduleRescheduleRequest	true	"New occurrence time"
//	@Success	200		{object}	ScheduleResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/schedules/{id}:reschedule [post]
func (h *ScheduleHandler) Reschedule(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleUpdateFailure, true)
	if !ok {
		return
	}
	var request ScheduleRescheduleRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	start, err := parseScheduleTimestamp(request.StartAt)
	if err != nil {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("start_at", "invalid_timestamp", "Start time must be an RFC 3339 timestamp"))
		return
	}
	end, err := parseScheduleTimestamp(request.EndAt)
	if err != nil {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("end_at", "invalid_timestamp", "End time must be an RFC 3339 timestamp"))
		return
	}
	if request.OccurrenceDate != "" {
		if _, err := parseDate(request.OccurrenceDate); err != nil {
			WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD format"))
			return
		}
	}
	result, err := h.schedules.Reschedule.Execute(r.Context(), scheduleusecase.RescheduleScheduleInput{UserID: actor, ScheduleID: id, OccurrenceDate: request.OccurrenceDate, StartAt: start, EndAt: end, Scope: request.Scope})
	if err != nil {
		writeScheduleUseCaseError(w, scheduleUpdateFailure, err)
		return
	}
	WriteJSON(w, http.StatusOK, scheduleResponse(result))
}

type ScheduleProjectAssignmentRequest struct {
	ScheduleID string `json:"schedule_id"`
}

// AddToProject associates an existing Schedule with the Project.
//
//	@Summary	Add schedule to Project
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string								true	"Project ID"
//	@Param		request	body		ScheduleProjectAssignmentRequest	true	"Schedule ID"
//	@Success	200		{object}	MessageResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/projects/{id}/schedules:add [post]
func (h *ScheduleHandler) AddToProject(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleUpdateFailure, ErrDetailUnauthorized)
		return
	}
	projectID := domain.ProjectID(chi.URLParam(r, "id"))
	if projectID == "" {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("id", "required", "Project ID is required"))
		return
	}
	var request ScheduleProjectAssignmentRequest
	if !decodeScheduleJSON(w, r, &request) || request.ScheduleID == "" {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("schedule_id", "required", "Schedule ID is required"))
		return
	}
	if err := h.schedules.SetProject.Execute(r.Context(), actor, domain.ScheduleID(request.ScheduleID), &projectID); err != nil {
		writeScheduleUseCaseError(w, scheduleUpdateFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Schedule added to Project")
}

// RemoveFromProject detaches a Schedule from this Project.
//
//	@Summary	Remove schedule from Project
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string								true	"Project ID"
//	@Param		request	body		ScheduleProjectAssignmentRequest	true	"Schedule ID"
//	@Success	200		{object}	MessageResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/projects/{id}/schedules:remove [post]
func (h *ScheduleHandler) RemoveFromProject(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleUpdateFailure, ErrDetailUnauthorized)
		return
	}
	projectID := domain.ProjectID(chi.URLParam(r, "id"))
	if projectID == "" {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("id", "required", "Project ID is required"))
		return
	}
	var request ScheduleProjectAssignmentRequest
	if !decodeScheduleJSON(w, r, &request) || request.ScheduleID == "" {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("schedule_id", "required", "Schedule ID is required"))
		return
	}
	if err := h.schedules.RemoveFromProject.Execute(r.Context(), actor, domain.ScheduleID(request.ScheduleID), projectID); err != nil {
		writeScheduleUseCaseError(w, scheduleUpdateFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Schedule removed from Project")
}

// ListAssignees returns users eligible to be assigned to the Schedule.
//
//	@Summary	List schedule assignees
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Schedule ID"
//	@Success	200	{object}	ScheduleAssigneeListResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Router		/schedules/{id}/assignees [get]
func (h *ScheduleHandler) ListAssignees(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleListFailure, true)
	if !ok {
		return
	}
	rows, err := h.schedules.ListAssignees.Execute(r.Context(), actor, id)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleListFailure, err)
		return
	}
	items := make([]ScheduleAssigneeResponse, 0, len(rows))
	for _, row := range rows {
		items = append(items, ScheduleAssigneeResponse{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, IsOwner: row.IsOwner})
	}
	WriteJSON(w, http.StatusOK, ScheduleAssigneeListResponse{Items: items})
}

// Assign changes the Schedule's assignee.
//
//	@Summary	Assign schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string					true	"Schedule ID"
//	@Param		request	body		ScheduleAssigneeRequest	true	"Assignee"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/schedules/{id}/assignees [patch]
func (h *ScheduleHandler) Assign(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleUpdateFailure, true)
	if !ok {
		return
	}
	var request ScheduleAssigneeRequest
	if !decodeScheduleJSON(w, r, &request) || request.AssigneeID == "" {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("assignee_id", "required", "Assignee ID is required"))
		return
	}
	if err := h.schedules.Assign.Execute(r.Context(), actor, id, domain.UserID(request.AssigneeID)); err != nil {
		writeScheduleUseCaseError(w, scheduleUpdateFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Schedule assigned")
}

type ScheduleTagAssignmentRequest struct {
	TagID string `json:"tag_id"`
}

// AddTag links a Tag from the Schedule owner's catalog to the Schedule.
//
//	@Summary	Add a tag to a schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"Schedule ID"
//	@Param		request	body		ScheduleTagAssignmentRequest	true	"Tag ID"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/schedules/{id}/tags:add [post]
func (h *ScheduleHandler) AddTag(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleUpdateFailure, true)
	if !ok {
		return
	}
	var request ScheduleTagAssignmentRequest
	if !decodeScheduleJSON(w, r, &request) || request.TagID == "" {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("tag_id", "required", "Tag ID is required"))
		return
	}
	if err := h.schedules.AddTag.Execute(r.Context(), actor, id, request.TagID); err != nil {
		writeScheduleUseCaseError(w, scheduleUpdateFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Tag added to Schedule")
}

// RemoveTag unlinks a Tag from the Schedule.
//
//	@Summary	Remove a tag from a schedule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"Schedule ID"
//	@Param		request	body		ScheduleTagAssignmentRequest	true	"Tag ID"
//	@Success	200		{object}	MessageResponse
//	@Failure	400		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	404		{object}	ErrResponse
//	@Router		/schedules/{id}/tags:remove [post]
func (h *ScheduleHandler) RemoveTag(w http.ResponseWriter, r *http.Request) {
	actor, id, ok := scheduleRequestIDs(w, r, scheduleUpdateFailure, true)
	if !ok {
		return
	}
	var request ScheduleTagAssignmentRequest
	if !decodeScheduleJSON(w, r, &request) || request.TagID == "" {
		WriteError(w, http.StatusBadRequest, scheduleUpdateFailure, scheduleField("tag_id", "required", "Tag ID is required"))
		return
	}
	if err := h.schedules.RemoveTag.Execute(r.Context(), actor, id, request.TagID); err != nil {
		writeScheduleUseCaseError(w, scheduleUpdateFailure, err)
		return
	}
	WriteMessage(w, http.StatusOK, "Tag removed from Schedule")
}

func scheduleActor(ctx context.Context) (domain.UserID, bool) {
	id, ok := ctx.Value(UserIDContextKey).(string)
	return domain.UserID(id), ok && id != ""
}

func scheduleRequestIDs(w http.ResponseWriter, r *http.Request, spec ErrSpec, requireID bool) (domain.UserID, domain.ScheduleID, bool) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", "", false
	}
	var id domain.ScheduleID
	if requireID {
		id = domain.ScheduleID(chi.URLParam(r, "id"))
		if id == "" {
			WriteError(w, http.StatusBadRequest, spec, scheduleField("id", "required", "Schedule ID is required"))
			return "", "", false
		}
	}
	return actor, id, true
}

func scheduleCursorRequest(request listRequest) scheduleusecase.CursorPageRequest {
	cursor := scheduleusecase.CursorPageRequest{Size: request.Size, FromDate: request.FromDate, AsOf: request.AsOf}
	if request.Anchor != nil {
		cursor.Anchor = &scheduleusecase.CursorAnchor{At: request.Anchor.At, ID: request.Anchor.ID, SeriesID: request.Anchor.SeriesID, Date: request.Anchor.OccurrenceDate, OccurrenceDate: request.Anchor.OccurrenceDate, AsOf: request.Anchor.AsOf}
	}
	return cursor
}

func scheduleResponse(row dao.Schedule) ScheduleResponse {
	id := row.ID
	if !row.IsException && row.SeriesID != "" && row.ID != row.SeriesID {
		id = scheduleVirtualOccurrenceID
	}
	frequencies := make([]ScheduleFrequencyResponse, 0, len(row.Frequencies))
	for _, f := range row.Frequencies {
		frequencies = append(frequencies, ScheduleFrequencyResponse{Value: f.Value, Label: f.Label, LabelJp: f.LabelJp})
	}
	location, err := time.LoadLocation(row.Timezone)
	if err != nil {
		location = time.UTC
	}
	result := ScheduleResponse{ID: id, UserID: row.UserID, ProjectID: row.ProjectID, AssigneeID: row.AssigneeID, Title: row.Title, Description: row.Description, Location: row.Location, StartAt: time.Unix(row.StartAt, 0).In(location).Format(time.RFC3339), EndAt: time.Unix(row.EndAt, 0).In(location).Format(time.RFC3339), IntervalWeeks: row.IntervalWeeks, Frequencies: frequencies, RepeatState: row.RepeatState, SeriesID: row.SeriesID, OccurrenceDate: row.OccurrenceDate, Timezone: row.Timezone, IsException: row.IsException, Completed: row.Completed, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.FrequencyAnchorDate != 0 {
		value := time.Unix(row.FrequencyAnchorDate, 0).UTC().Format("2006-01-02")
		result.FrequencyAnchorDate = &value
	}
	return result
}

func parseScheduleTimestamp(value *string) (time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return time.Time{}, errors.New("timestamp required")
	}
	return time.Parse(time.RFC3339, *value)
}
func decodeScheduleJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}

func decodeScheduleOccurrenceDate(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil {
		return "", err
	}
	if len(body) > 1<<20 {
		return "", errors.New("request body too large")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return "", nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	var request ScheduleOccurrenceRequest
	if err := decoder.Decode(&request); err != nil {
		return "", err
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) {
		return "", errors.New("request body must contain one JSON value")
	}
	return request.OccurrenceDate, nil
}
func scheduleField(field, code, message string) ErrDetail { return NewErrDetail(field, code, message) }
func decodeScheduleString(raw json.RawMessage) (string, error) {
	var value string
	err := json.Unmarshal(raw, &value)
	return value, err
}
func schedulePatchField[T any](fields map[string]json.RawMessage, field string, decode func(json.RawMessage) (T, error)) (scheduleusecase.PatchField[T], error) {
	raw, ok := fields[field]
	if !ok {
		return scheduleusecase.PatchField[T]{}, nil
	}
	patch := scheduleusecase.PatchField[T]{Present: true}
	if string(raw) == "null" {
		return patch, nil
	}
	value, err := decode(raw)
	if err != nil {
		return scheduleusecase.PatchField[T]{}, err
	}
	patch.Value = &value
	return patch, nil
}

func writeScheduleUseCaseError(w http.ResponseWriter, spec ErrSpec, err error) {
	status, detail := scheduleUseCaseError(err)
	WriteError(w, status, spec, detail)
}

func scheduleUseCaseError(err error) (int, ErrDetail) {
	switch {
	case errors.Is(err, scheduleusecase.ErrPermissionDenied):
		return http.StatusForbidden, NewErrDetail("", "permission_denied", "The caller lacks permission for this schedule operation")
	case errors.Is(err, domain.ErrScheduleNotFound), errors.Is(err, scheduleusecase.ErrScheduleProjectNotFound):
		return http.StatusNotFound, NewErrDetail("id", "not_found", "Schedule or Project was not found")
	case errors.Is(err, scheduleusecase.ErrScheduleProjectChanged):
		return http.StatusConflict, NewErrDetail("project_id", "schedule_changed", "Schedule changed while its Project association was being updated")
	case errors.Is(err, scheduleusecase.ErrScheduleTagNotFound):
		return http.StatusNotFound, NewErrDetail("tag_id", "not_found", "Tag was not found")
	case errors.Is(err, scheduleusecase.ErrOccurrenceNotFound):
		return http.StatusNotFound, scheduleField("occurrence_date", "occurrence_not_found", "Schedule occurrence was not found")
	case errors.Is(err, scheduleusecase.ErrOccurrenceDateRequired), errors.Is(err, scheduleusecase.ErrOccurrenceRuleMismatch):
		return http.StatusBadRequest, scheduleField("occurrence_date", "invalid_occurrence", "A valid occurrence date is required")
	case errors.Is(err, scheduleusecase.ErrRescheduleDateMismatch):
		return http.StatusBadRequest, scheduleField("start_at", "date_mismatch", "Future scope must keep the occurrence's local calendar date")
	case errors.Is(err, scheduleusecase.ErrOccurrenceInactive):
		return http.StatusConflict, scheduleField("occurrence_date", "occurrence_inactive", "Occurrence is no longer active")
	case errors.Is(err, scheduleusecase.ErrOccurrenceCompleted):
		return http.StatusConflict, scheduleField("occurrence_date", "occurrence_completed", "Completed occurrence cannot be skipped")
	case errors.Is(err, scheduleusecase.ErrScheduleScopeInvalid):
		return http.StatusBadRequest, scheduleField("scope", "invalid_scope", "Scope must be current or future")
	case errors.Is(err, scheduleusecase.ErrScheduleAssigneeNotEligible):
		return http.StatusBadRequest, scheduleField("assignee_id", "assignee_not_eligible", "Assignee must be the owner or an eligible Project member")
	case errors.Is(err, recurrence.ErrFrequencyInvalid):
		return http.StatusBadRequest, scheduleField("frequencies", "invalid_frequency", "Frequency contains an unsupported weekday")
	case errors.Is(err, domain.ErrScheduleIDEmpty):
		return http.StatusBadRequest, scheduleField("id", "required", "Schedule ID is required")
	case errors.Is(err, domain.ErrScheduleTitleEmpty):
		return http.StatusBadRequest, scheduleField("title", "required", "Title is required")
	case errors.Is(err, domain.ErrScheduleStartAtEmpty):
		return http.StatusBadRequest, scheduleField("start_at", "required", "Start time is required")
	case errors.Is(err, domain.ErrScheduleEndAtEmpty):
		return http.StatusBadRequest, scheduleField("end_at", "required", "End time is required")
	case errors.Is(err, domain.ErrScheduleEndAtMustBeAfterStartAt):
		return http.StatusBadRequest, scheduleField("end_at", "invalid_time_range", "End time must be after start time")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}
