package rest

import (
	"errors"
	"net/http"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/go-chi/chi/v5"
)

var (
	scheduleBlueprintCreateFailure  = NewFailureErrSpec("schedule_blueprints", ActionCreate, "Failed to create schedule blueprint")
	scheduleBlueprintGetFailure     = NewFailureErrSpec("schedule_blueprints", ActionGet, "Failed to get schedule blueprint")
	scheduleBlueprintUpdateFailure  = NewFailureErrSpec("schedule_blueprints", ActionUpdate, "Failed to update schedule blueprint")
	scheduleBlueprintDeleteFailure  = NewFailureErrSpec("schedule_blueprints", ActionDelete, "Failed to delete following schedule occurrences")
	scheduleOccurrenceGetFailure    = NewFailureErrSpec("schedule_occurrences", ActionGet, "Failed to get schedule occurrence")
	scheduleOccurrenceUpdateFailure = NewFailureErrSpec("schedule_occurrences", ActionUpdate, "Failed to update schedule occurrence")
)

type ScheduleBlueprintResponse struct {
	ID                   string   `json:"id"`
	UserID               string   `json:"user_id"`
	Revision             int32    `json:"revision"`
	EffectiveFrom        string   `json:"effective_from"`
	RecurrenceAnchorDate string   `json:"recurrence_anchor_date"`
	LocalStart           string   `json:"local_start"`
	LocalEnd             string   `json:"local_end"`
	AllDay               bool     `json:"all_day"`
	Timezone             string   `json:"timezone"`
	Frequency            string   `json:"frequency"`
	Interval             int      `json:"interval"`
	Weekdays             []string `json:"weekdays,omitempty"`
	EndDate              *string  `json:"end_date,omitempty"`
	CutoffAfter          *string  `json:"cutoff_after,omitempty"`
	Title                string   `json:"title"`
	Description          string   `json:"description"`
	Location             string   `json:"location"`
	ProjectID            string   `json:"project_id,omitempty"`
	AssigneeID           string   `json:"assignee_id"`
	Deleted              bool     `json:"deleted"`
	CreatedAt            int64    `json:"created_at"`
}

type ScheduleOccurrenceResponse struct {
	ID                 *string `json:"id,omitempty"`
	Persisted          bool    `json:"persisted"`
	BlueprintID        string  `json:"blueprint_id"`
	OccurrenceDate     string  `json:"occurrence_date"`
	SourceRevisionID   string  `json:"source_revision_id,omitempty"`
	UserID             string  `json:"user_id"`
	ProjectID          string  `json:"project_id,omitempty"`
	AssigneeID         string  `json:"assignee_id"`
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	Location           string  `json:"location"`
	Timezone           string  `json:"timezone"`
	StartAt            string  `json:"start_at"`
	EndAt              string  `json:"end_at"`
	AllDay             bool    `json:"all_day"`
	Completed          bool    `json:"completed"`
	Skipped            bool    `json:"skipped"`
	Deleted            bool    `json:"deleted"`
	IsOverride         bool    `json:"is_override"`
	Revision           int32   `json:"revision"`
	CreatedAt          int64   `json:"created_at"`
	UpdatedAt          int64   `json:"updated_at"`
	CanUpdate          bool    `json:"can_update"`
	CanDelete          bool    `json:"can_delete"`
	CanSkip            bool    `json:"can_skip"`
	CanAssign          bool    `json:"can_assign"`
	CanUpdateFollowing bool    `json:"can_update_following"`
	CanDeleteFollowing bool    `json:"can_delete_following"`
}

type ScheduleBlueprintCreateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	ProjectID   string   `json:"project_id"`
	AssigneeID  string   `json:"assignee_id"`
	Timezone    string   `json:"timezone"`
	LocalStart  string   `json:"local_start"`
	LocalEnd    string   `json:"local_end"`
	AllDay      bool     `json:"all_day"`
	Frequency   string   `json:"frequency"`
	Interval    int      `json:"interval"`
	Weekdays    []string `json:"weekdays"`
	EndDate     *string  `json:"end_date"`
}

type ScheduleBlueprintUpdateRequest struct {
	OccurrenceDate string                 `json:"occurrence_date"`
	Scope          string                 `json:"scope"`
	Title          optionalJSON[string]   `json:"title" swaggertype:"string"`
	Description    optionalJSON[string]   `json:"description" swaggertype:"string"`
	Location       optionalJSON[string]   `json:"location" swaggertype:"string"`
	ProjectID      optionalJSON[string]   `json:"project_id" swaggertype:"string" extensions:"x-nullable"`
	AssigneeID     optionalJSON[string]   `json:"assignee_id" swaggertype:"string"`
	Timezone       optionalJSON[string]   `json:"timezone" swaggertype:"string"`
	LocalStart     optionalJSON[string]   `json:"local_start" swaggertype:"string"`
	LocalEnd       optionalJSON[string]   `json:"local_end" swaggertype:"string"`
	AllDay         optionalJSON[bool]     `json:"all_day" swaggertype:"boolean"`
	Frequency      optionalJSON[string]   `json:"frequency" swaggertype:"string"`
	Interval       optionalJSON[int]      `json:"interval" swaggertype:"integer"`
	Weekdays       optionalJSON[[]string] `json:"weekdays" swaggertype:"array,string"`
	EndDate        optionalJSON[string]   `json:"end_date" swaggertype:"string" extensions:"x-nullable"`
}

type ScheduleOccurrenceUpdateRequest struct {
	Title       optionalJSON[string] `json:"title" swaggertype:"string"`
	Description optionalJSON[string] `json:"description" swaggertype:"string"`
	Location    optionalJSON[string] `json:"location" swaggertype:"string"`
	ProjectID   optionalJSON[string] `json:"project_id" swaggertype:"string" extensions:"x-nullable"`
	AssigneeID  optionalJSON[string] `json:"assignee_id" swaggertype:"string"`
	StartAt     optionalJSON[string] `json:"start_at" swaggertype:"string"`
	EndAt       optionalJSON[string] `json:"end_at" swaggertype:"string"`
	AllDay      optionalJSON[bool]   `json:"all_day" swaggertype:"boolean"`
}

// CreateBlueprint creates a one-off or recurring definition.
//
//	@Summary	Create schedule blueprint
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		ScheduleBlueprintCreateRequest	true	"Schedule definition"
//	@Success	201		{object}	ScheduleBlueprintResponse
//	@Router		/schedule-blueprints [post]
func (h *ScheduleHandler) CreateBlueprint(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleBlueprintCreateFailure, ErrDetailUnauthorized)
		return
	}
	var request ScheduleBlueprintCreateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleBlueprintCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	input, err := request.input()
	if err != nil {
		writeScheduleRequestError(w, scheduleBlueprintCreateFailure, err)
		return
	}
	created, err := h.schedules.CreateBlueprint.Execute(r.Context(), string(actor), input)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleBlueprintCreateFailure, err)
		return
	}
	setResourceETag(w, created.Revision)
	WriteJSON(w, http.StatusCreated, scheduleBlueprintResponse(created))
}

// CreateProjectBlueprint creates a definition assigned to the Project.
//
//	@Summary	Create Project schedule blueprint
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"Project ID"
//	@Param		request	body		ScheduleBlueprintCreateRequest	true	"Schedule definition"
//	@Success	201		{object}	ScheduleBlueprintResponse
//	@Router		/projects/{id}/schedules [post]
func (h *ScheduleHandler) CreateProjectBlueprint(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		WriteError(w, http.StatusBadRequest, scheduleBlueprintCreateFailure, scheduleField("id", "required", "Project ID is required"))
		return
	}
	var request ScheduleBlueprintCreateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleBlueprintCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	if request.ProjectID != "" && request.ProjectID != projectID {
		WriteError(w, http.StatusBadRequest, scheduleBlueprintCreateFailure, scheduleField("project_id", "path_mismatch", "Project ID must match the path"))
		return
	}
	request.ProjectID = projectID
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleBlueprintCreateFailure, ErrDetailUnauthorized)
		return
	}
	input, err := request.input()
	if err != nil {
		writeScheduleRequestError(w, scheduleBlueprintCreateFailure, err)
		return
	}
	created, err := h.schedules.CreateBlueprint.Execute(r.Context(), string(actor), input)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleBlueprintCreateFailure, err)
		return
	}
	setResourceETag(w, created.Revision)
	WriteJSON(w, http.StatusCreated, scheduleBlueprintResponse(created))
}

// GetBlueprint returns the current effective definition.
//
//	@Summary	Get schedule blueprint
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Blueprint ID"
//	@Success	200	{object}	ScheduleBlueprintResponse
//	@Router		/schedule-blueprints/{id} [get]
func (h *ScheduleHandler) GetBlueprint(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleBlueprintGetFailure, ErrDetailUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, scheduleBlueprintGetFailure, scheduleField("id", "required", "Blueprint ID is required"))
		return
	}
	blueprint, err := h.schedules.GetBlueprint.Execute(r.Context(), string(actor), id)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleBlueprintGetFailure, err)
		return
	}
	setResourceETag(w, blueprint.Revision)
	WriteJSON(w, http.StatusOK, scheduleBlueprintResponse(blueprint))
}

type ScheduleBlueprintRevisionResponse struct {
	ScheduleBlueprintResponse
	RevisionID     string  `json:"revision_id"`
	Superseded     bool    `json:"superseded"`
	EffectiveUntil *string `json:"effective_until,omitempty"`
	ChangedBy      string  `json:"changed_by"`
	ChangedAt      int64   `json:"changed_at"`
}

// ListBlueprintRevisions returns effective and superseded definition history.
//
//	@Summary	List schedule blueprint revisions
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Blueprint ID"
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque page token"
//	@Success	200			{object}	listEnvelope[ScheduleBlueprintRevisionResponse]
//	@Router		/schedule-blueprints/{id}/revisions [get]
func (h *ScheduleHandler) ListBlueprintRevisions(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleBlueprintGetFailure, ErrDetailUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, scheduleBlueprintGetFailure, scheduleField("id", "required", "Blueprint ID is required"))
		return
	}
	request, err := parseListRequest(r, h.pageTokens, string(actor), "schedule_blueprint_revisions", id, "revision_desc", listEnvelope[ScheduleBlueprintRevisionResponse]{})
	if err != nil {
		writeListError(w, scheduleBlueprintGetFailure, err)
		return
	}
	page, err := h.schedules.ListBlueprintRevisions.ExecutePage(r.Context(), string(actor), id, request.Size, anchorRevision(request.Anchor))
	if err != nil {
		writeScheduleUseCaseError(w, scheduleBlueprintGetFailure, err)
		return
	}
	items := make([]ScheduleBlueprintRevisionResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		blueprint := scheduleBlueprintResponse(row.Blueprint)
		blueprint.CutoffAfter = row.CutoffAfter
		items = append(items, ScheduleBlueprintRevisionResponse{
			ScheduleBlueprintResponse: blueprint, RevisionID: row.RevisionID,
			Superseded: row.Superseded, EffectiveUntil: row.EffectiveUntil, ChangedBy: row.ChangedBy, ChangedAt: row.ChangedAt,
		})
		anchors = append(anchors, listAnchor{Revision: row.Revision})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, scheduleBlueprintGetFailure)
}

func anchorRevision(anchor *listAnchor) int32 {
	if anchor == nil {
		return 0
	}
	return anchor.Revision
}

// UpdateBlueprint changes a definition for the selected occurrence and following dates.
//
//	@Summary	Update schedule blueprint
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string							true	"Blueprint ID"
//	@Param		If-Match	header		string							true	"Current blueprint revision"
//	@Param		request		body		ScheduleBlueprintUpdateRequest	true	"Following definition changes"
//	@Success	200			{object}	ScheduleBlueprintResponse
//	@Router		/schedule-blueprints/{id} [patch]
func (h *ScheduleHandler) UpdateBlueprint(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleBlueprintUpdateFailure, ErrDetailUnauthorized)
		return
	}
	expected, ok := expectedRevision(w, r, scheduleBlueprintUpdateFailure)
	if !ok {
		return
	}
	var request ScheduleBlueprintUpdateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleBlueprintUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	input, err := request.input()
	if err != nil {
		writeScheduleRequestError(w, scheduleBlueprintUpdateFailure, err)
		return
	}
	blueprint, err := h.schedules.UpdateBlueprint.Execute(r.Context(), string(actor), chi.URLParam(r, "id"), request.OccurrenceDate, request.Scope, input, expected)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleBlueprintUpdateFailure, err)
		return
	}
	setResourceETag(w, blueprint.Revision)
	WriteJSON(w, http.StatusOK, scheduleBlueprintResponse(blueprint))
}

// DeleteBlueprintFollowing ends the definition at the selected occurrence.
//
//	@Summary	Delete following schedule occurrences
//	@Tags		Schedules
//	@Security	BearerAuth
//	@Param		id				path	string	true	"Blueprint ID"
//	@Param		occurrence_date	path	string	true	"Original local occurrence date"
//	@Param		If-Match		header	string	true	"Current blueprint revision"
//	@Success	204
//	@Router		/schedule-blueprints/{id}/following/{occurrence_date} [delete]
func (h *ScheduleHandler) DeleteBlueprintFollowing(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleBlueprintDeleteFailure, ErrDetailUnauthorized)
		return
	}
	expected, ok := expectedRevision(w, r, scheduleBlueprintDeleteFailure)
	if !ok {
		return
	}
	if err := h.schedules.DeleteBlueprintFollowing.Execute(r.Context(), string(actor), chi.URLParam(r, "id"), chi.URLParam(r, "occurrence_date"), expected); err != nil {
		writeScheduleUseCaseError(w, scheduleBlueprintDeleteFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (request ScheduleBlueprintCreateRequest) input() (scheduleusecase.CreateBlueprintInput, error) {
	if err := validateBlueprintWeekdays(request.Weekdays); err != nil {
		return scheduleusecase.CreateBlueprintInput{}, err
	}
	start, err := parseBlueprintLocal(request.LocalStart)
	if err != nil {
		return scheduleusecase.CreateBlueprintInput{}, scheduleRequestFieldError{field: "local_start", code: "invalid_local_time", message: "Local datetime must use YYYY-MM-DDTHH:MM:SS without an offset"}
	}
	end, err := parseBlueprintLocal(request.LocalEnd)
	if err != nil {
		return scheduleusecase.CreateBlueprintInput{}, scheduleRequestFieldError{field: "local_end", code: "invalid_local_time", message: "Local datetime must use YYYY-MM-DDTHH:MM:SS without an offset"}
	}
	var endDate *time.Time
	if request.EndDate != nil {
		parsed, err := time.Parse("2006-01-02", *request.EndDate)
		if err != nil {
			return scheduleusecase.CreateBlueprintInput{}, scheduleRequestFieldError{field: "end_date", code: "invalid_date", message: "End date must use YYYY-MM-DD"}
		}
		endDate = &parsed
	}
	return scheduleusecase.CreateBlueprintInput{
		Title: request.Title, Description: request.Description, Location: request.Location,
		ProjectID: request.ProjectID, AssigneeID: request.AssigneeID, Timezone: request.Timezone,
		LocalStart: start, LocalEnd: end, Frequency: request.Frequency, Interval: request.Interval,
		AllDay: request.AllDay, Weekdays: request.Weekdays, EndDate: endDate,
	}, nil
}

type scheduleRequestFieldError struct{ field, code, message string }

func (e scheduleRequestFieldError) Error() string { return e.message }

func writeScheduleRequestError(w http.ResponseWriter, spec ErrSpec, err error) {
	var fieldError scheduleRequestFieldError
	if errors.As(err, &fieldError) {
		WriteError(w, http.StatusBadRequest, spec, scheduleField(fieldError.field, fieldError.code, fieldError.message))
		return
	}
	WriteError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
}

func parseBlueprintLocal(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("invalid local datetime")
}

func validateBlueprintWeekdays(values []string) error {
	for _, value := range values {
		switch value {
		case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		default:
			return scheduleRequestFieldError{field: "weekdays", code: "invalid_weekday", message: "Weekday must be one of mon, tue, wed, thu, fri, sat, or sun"}
		}
	}
	return nil
}

func (request ScheduleBlueprintUpdateRequest) input() (scheduleusecase.UpdateBlueprintInput, error) {
	var input scheduleusecase.UpdateBlueprintInput
	for _, field := range []struct {
		name    string
		present bool
		isNull  bool
	}{
		{"title", request.Title.Present, request.Title.Value == nil},
		{"description", request.Description.Present, request.Description.Value == nil},
		{"location", request.Location.Present, request.Location.Value == nil},
		{"assignee_id", request.AssigneeID.Present, request.AssigneeID.Value == nil},
		{"timezone", request.Timezone.Present, request.Timezone.Value == nil},
		{"local_start", request.LocalStart.Present, request.LocalStart.Value == nil},
		{"local_end", request.LocalEnd.Present, request.LocalEnd.Value == nil},
		{"all_day", request.AllDay.Present, request.AllDay.Value == nil},
		{"frequency", request.Frequency.Present, request.Frequency.Value == nil},
		{"interval", request.Interval.Present, request.Interval.Value == nil},
		{"weekdays", request.Weekdays.Present, request.Weekdays.Value == nil},
	} {
		if field.present && field.isNull {
			return input, scheduleRequestFieldError{field: field.name, code: "null_not_allowed", message: "Field cannot be null"}
		}
	}
	if request.Title.Present {
		input.Title = request.Title.Value
	}
	if request.Weekdays.Present {
		if err := validateBlueprintWeekdays(*request.Weekdays.Value); err != nil {
			return input, err
		}
	}
	if request.Description.Present {
		input.Description = request.Description.Value
	}
	if request.Location.Present {
		input.Location = request.Location.Value
	}
	if request.ProjectID.Present {
		if request.ProjectID.Value == nil {
			value := ""
			input.ProjectID = &value
		} else {
			input.ProjectID = request.ProjectID.Value
		}
	}
	if request.AssigneeID.Present {
		input.AssigneeID = request.AssigneeID.Value
	}
	if request.Timezone.Present {
		input.Timezone = request.Timezone.Value
	}
	if request.AllDay.Present {
		input.AllDay = request.AllDay.Value
	}
	if request.Frequency.Present {
		input.Frequency = request.Frequency.Value
	}
	if request.Interval.Present {
		input.Interval = request.Interval.Value
	}
	if request.Weekdays.Present {
		input.Weekdays = request.Weekdays.Value
	}
	if request.LocalStart.Present && request.LocalStart.Value != nil {
		parsed, err := parseBlueprintLocal(*request.LocalStart.Value)
		if err != nil {
			return input, scheduleRequestFieldError{field: "local_start", code: "invalid_local_time", message: "Local datetime must use YYYY-MM-DDTHH:MM:SS without an offset"}
		}
		input.LocalStart = &parsed
	}
	if request.LocalEnd.Present && request.LocalEnd.Value != nil {
		parsed, err := parseBlueprintLocal(*request.LocalEnd.Value)
		if err != nil {
			return input, scheduleRequestFieldError{field: "local_end", code: "invalid_local_time", message: "Local datetime must use YYYY-MM-DDTHH:MM:SS without an offset"}
		}
		input.LocalEnd = &parsed
	}
	if request.EndDate.Present {
		var value *time.Time
		if request.EndDate.Value != nil {
			parsed, err := time.Parse("2006-01-02", *request.EndDate.Value)
			if err != nil {
				return input, scheduleRequestFieldError{field: "end_date", code: "invalid_date", message: "End date must use YYYY-MM-DD"}
			}
			value = &parsed
		}
		input.EndDate = &value
	}
	return input, nil
}

func scheduleBlueprintResponse(row dao.Blueprint) ScheduleBlueprintResponse {
	return ScheduleBlueprintResponse{
		ID: row.ID, UserID: row.UserID, Revision: row.Revision, EffectiveFrom: row.EffectiveFrom,
		RecurrenceAnchorDate: row.RecurrenceAnchorDate, LocalStart: restLocalTimestamp(row.LocalStart), LocalEnd: restLocalTimestamp(row.LocalEnd),
		Timezone: row.Timezone, AllDay: row.AllDay, Frequency: row.Frequency, Interval: row.Interval, Weekdays: row.Weekdays,
		EndDate: row.EndDate, CutoffAfter: row.CutoffAfter, Title: row.Title, Description: row.Description, Location: row.Location,
		ProjectID: row.ProjectID, AssigneeID: row.AssigneeID, Deleted: row.Deleted, CreatedAt: row.CreatedAt,
	}
}

func restLocalTimestamp(value string) string {
	if len(value) > 10 && value[10] == ' ' {
		return value[:10] + "T" + value[11:]
	}
	return value
}
