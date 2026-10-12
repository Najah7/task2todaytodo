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
	scheduleRuleCreateFailure       = NewFailureErrSpec("schedule_rules", ActionCreate, "Failed to create schedule rule")
	scheduleRuleGetFailure          = NewFailureErrSpec("schedule_rules", ActionGet, "Failed to get schedule rule")
	scheduleRuleUpdateFailure       = NewFailureErrSpec("schedule_rules", ActionUpdate, "Failed to update schedule rule")
	scheduleRuleDeleteFailure       = NewFailureErrSpec("schedule_rules", ActionDelete, "Failed to delete following schedule occurrences")
	scheduleOccurrenceGetFailure    = NewFailureErrSpec("schedule_occurrences", ActionGet, "Failed to get schedule occurrence")
	scheduleOccurrenceUpdateFailure = NewFailureErrSpec("schedule_occurrences", ActionUpdate, "Failed to update schedule occurrence")
)

type ScheduleRuleResponse struct {
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
	RuleID             string  `json:"rule_id"`
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

type ScheduleRuleCreateRequest struct {
	Title                   string   `json:"title"`
	Description             string   `json:"description"`
	Location                string   `json:"location"`
	ProjectID               string   `json:"project_id"`
	AssigneeID              string   `json:"assignee_id"`
	Timezone                string   `json:"timezone"`
	LocalStart              string   `json:"local_start"`
	LocalEnd                string   `json:"local_end"`
	AllDay                  bool     `json:"all_day"`
	Frequency               string   `json:"frequency"`
	Interval                int      `json:"interval"`
	Weekdays                []string `json:"weekdays"`
	EndDate                 *string  `json:"end_date"`
	ConfirmTodoRegeneration bool     `json:"confirm_todo_regeneration"`
}

type ScheduleRuleUpdateRequest struct {
	OccurrenceDate          string                 `json:"occurrence_date"`
	Scope                   string                 `json:"scope"`
	Title                   optionalJSON[string]   `json:"title" swaggertype:"string"`
	Description             optionalJSON[string]   `json:"description" swaggertype:"string"`
	Location                optionalJSON[string]   `json:"location" swaggertype:"string"`
	ProjectID               optionalJSON[string]   `json:"project_id" swaggertype:"string" extensions:"x-nullable"`
	AssigneeID              optionalJSON[string]   `json:"assignee_id" swaggertype:"string"`
	Timezone                optionalJSON[string]   `json:"timezone" swaggertype:"string"`
	LocalStart              optionalJSON[string]   `json:"local_start" swaggertype:"string"`
	LocalEnd                optionalJSON[string]   `json:"local_end" swaggertype:"string"`
	AllDay                  optionalJSON[bool]     `json:"all_day" swaggertype:"boolean"`
	Frequency               optionalJSON[string]   `json:"frequency" swaggertype:"string"`
	Interval                optionalJSON[int]      `json:"interval" swaggertype:"integer"`
	Weekdays                optionalJSON[[]string] `json:"weekdays" swaggertype:"array,string"`
	EndDate                 optionalJSON[string]   `json:"end_date" swaggertype:"string" extensions:"x-nullable"`
	ConfirmTodoRegeneration bool                   `json:"confirm_todo_regeneration"`
}

type ScheduleOccurrenceUpdateRequest struct {
	Title                   optionalJSON[string] `json:"title" swaggertype:"string"`
	Description             optionalJSON[string] `json:"description" swaggertype:"string"`
	Location                optionalJSON[string] `json:"location" swaggertype:"string"`
	ProjectID               optionalJSON[string] `json:"project_id" swaggertype:"string" extensions:"x-nullable"`
	AssigneeID              optionalJSON[string] `json:"assignee_id" swaggertype:"string"`
	StartAt                 optionalJSON[string] `json:"start_at" swaggertype:"string"`
	EndAt                   optionalJSON[string] `json:"end_at" swaggertype:"string"`
	AllDay                  optionalJSON[bool]   `json:"all_day" swaggertype:"boolean"`
	ConfirmTodoRegeneration bool                 `json:"confirm_todo_regeneration"`
}

// CreateRule creates a one-off or recurring definition.
//
//	@Summary	Create schedule rule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		ScheduleRuleCreateRequest	true	"Schedule definition"
//	@Success	201		{object}	ScheduleRuleResponse
//	@Router		/schedule-rules [post]
func (h *ScheduleHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleRuleCreateFailure, ErrDetailUnauthorized)
		return
	}
	var request ScheduleRuleCreateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleRuleCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	input, err := request.input()
	if err != nil {
		writeScheduleRequestError(w, scheduleRuleCreateFailure, err)
		return
	}
	created, err := h.schedules.CreateRule.Execute(r.Context(), string(actor), input)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleRuleCreateFailure, err)
		return
	}
	setResourceETag(w, created.Revision)
	WriteJSON(w, http.StatusCreated, scheduleRuleResponse(created))
}

// CreateProjectRule creates a definition assigned to the Project.
//
//	@Summary	Create Project schedule rule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Project ID"
//	@Param		request	body		ScheduleRuleCreateRequest	true	"Schedule definition"
//	@Success	201		{object}	ScheduleRuleResponse
//	@Router		/projects/{id}/schedules [post]
func (h *ScheduleHandler) CreateProjectRule(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		WriteError(w, http.StatusBadRequest, scheduleRuleCreateFailure, scheduleField("id", "required", "Project ID is required"))
		return
	}
	var request ScheduleRuleCreateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleRuleCreateFailure, ErrDetailInvalidRequestBody)
		return
	}
	if request.ProjectID != "" && request.ProjectID != projectID {
		WriteError(w, http.StatusBadRequest, scheduleRuleCreateFailure, scheduleField("project_id", "path_mismatch", "Project ID must match the path"))
		return
	}
	request.ProjectID = projectID
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleRuleCreateFailure, ErrDetailUnauthorized)
		return
	}
	input, err := request.input()
	if err != nil {
		writeScheduleRequestError(w, scheduleRuleCreateFailure, err)
		return
	}
	created, err := h.schedules.CreateRule.Execute(r.Context(), string(actor), input)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleRuleCreateFailure, err)
		return
	}
	setResourceETag(w, created.Revision)
	WriteJSON(w, http.StatusCreated, scheduleRuleResponse(created))
}

// GetRule returns the current effective definition.
//
//	@Summary	Get schedule rule
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Rule ID"
//	@Success	200	{object}	ScheduleRuleResponse
//	@Router		/schedule-rules/{id} [get]
func (h *ScheduleHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleRuleGetFailure, ErrDetailUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, scheduleRuleGetFailure, scheduleField("id", "required", "Rule ID is required"))
		return
	}
	rule, err := h.schedules.GetRule.Execute(r.Context(), string(actor), id)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleRuleGetFailure, err)
		return
	}
	setResourceETag(w, rule.Revision)
	WriteJSON(w, http.StatusOK, scheduleRuleResponse(rule))
}

type ScheduleRuleRevisionResponse struct {
	ScheduleRuleResponse
	RevisionID     string  `json:"revision_id"`
	Superseded     bool    `json:"superseded"`
	EffectiveUntil *string `json:"effective_until,omitempty"`
	ChangedBy      string  `json:"changed_by"`
	ChangedAt      int64   `json:"changed_at"`
}

// ListRuleRevisions returns effective and superseded definition history.
//
//	@Summary	List schedule rule revisions
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Rule ID"
//	@Param		page_size	query		int		false	"Items per page"
//	@Param		page_token	query		string	false	"Opaque page token"
//	@Success	200			{object}	listEnvelope[ScheduleRuleRevisionResponse]
//	@Router		/schedule-rules/{id}/revisions [get]
func (h *ScheduleHandler) ListRuleRevisions(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleRuleGetFailure, ErrDetailUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		WriteError(w, http.StatusBadRequest, scheduleRuleGetFailure, scheduleField("id", "required", "Rule ID is required"))
		return
	}
	request, err := parseListRequest(r, h.pageTokens, string(actor), "schedule_rule_revisions", id, "revision_desc", listEnvelope[ScheduleRuleRevisionResponse]{})
	if err != nil {
		writeListError(w, scheduleRuleGetFailure, err)
		return
	}
	page, err := h.schedules.ListRuleRevisions.ExecutePage(r.Context(), string(actor), id, request.Size, anchorRevision(request.Anchor))
	if err != nil {
		writeScheduleUseCaseError(w, scheduleRuleGetFailure, err)
		return
	}
	items := make([]ScheduleRuleRevisionResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		rule := scheduleRuleResponse(row.Rule)
		rule.CutoffAfter = row.CutoffAfter
		items = append(items, ScheduleRuleRevisionResponse{
			ScheduleRuleResponse: rule, RevisionID: row.RevisionID,
			Superseded: row.Superseded, EffectiveUntil: row.EffectiveUntil, ChangedBy: row.ChangedBy, ChangedAt: row.ChangedAt,
		})
		anchors = append(anchors, listAnchor{Revision: row.Revision})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, scheduleRuleGetFailure)
}

func anchorRevision(anchor *listAnchor) int32 {
	if anchor == nil {
		return 0
	}
	return anchor.Revision
}

// UpdateRule changes a definition for the selected occurrence and following dates.
//
//	@Summary	Update schedule rule
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string						true	"Rule ID"
//	@Param		If-Match	header		string						true	"Current rule revision"
//	@Param		request		body		ScheduleRuleUpdateRequest	true	"Following definition changes"
//	@Success	200			{object}	ScheduleRuleResponse
//	@Router		/schedule-rules/{id} [patch]
func (h *ScheduleHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleRuleUpdateFailure, ErrDetailUnauthorized)
		return
	}
	expected, ok := expectedRevision(w, r, scheduleRuleUpdateFailure)
	if !ok {
		return
	}
	var request ScheduleRuleUpdateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleRuleUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	input, err := request.input()
	if err != nil {
		writeScheduleRequestError(w, scheduleRuleUpdateFailure, err)
		return
	}
	rule, err := h.schedules.UpdateRule.Execute(r.Context(), string(actor), chi.URLParam(r, "id"), request.OccurrenceDate, request.Scope, input, expected)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleRuleUpdateFailure, err)
		return
	}
	setResourceETag(w, rule.Revision)
	WriteJSON(w, http.StatusOK, scheduleRuleResponse(rule))
}

// DeleteRuleFollowing ends the definition at the selected occurrence.
//
//	@Summary	Delete following schedule occurrences
//	@Tags		Schedules
//	@Security	BearerAuth
//	@Param		id							path	string	true	"Rule ID"
//	@Param		occurrence_date				path	string	true	"Original local occurrence date"
//	@Param		If-Match					header	string	true	"Current rule revision"
//	@Param		confirm_todo_regeneration	query	bool	false	"Confirm regeneration of affected Todos"
//	@Success	204
//	@Router		/schedule-rules/{id}/following/{occurrence_date} [delete]
func (h *ScheduleHandler) DeleteRuleFollowing(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleRuleDeleteFailure, ErrDetailUnauthorized)
		return
	}
	expected, ok := expectedRevision(w, r, scheduleRuleDeleteFailure)
	if !ok {
		return
	}
	if err := h.schedules.DeleteRuleFollowing.Execute(r.Context(), string(actor), chi.URLParam(r, "id"), chi.URLParam(r, "occurrence_date"), expected, r.URL.Query().Get("confirm_todo_regeneration") == "true"); err != nil {
		writeScheduleUseCaseError(w, scheduleRuleDeleteFailure, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (request ScheduleRuleCreateRequest) input() (scheduleusecase.CreateRuleInput, error) {
	if err := validateRuleWeekdays(request.Weekdays); err != nil {
		return scheduleusecase.CreateRuleInput{}, err
	}
	start, err := parseRuleLocal(request.LocalStart)
	if err != nil {
		return scheduleusecase.CreateRuleInput{}, scheduleRequestFieldError{field: "local_start", code: "invalid_local_time", message: "Local datetime must use YYYY-MM-DDTHH:MM:SS without an offset"}
	}
	end, err := parseRuleLocal(request.LocalEnd)
	if err != nil {
		return scheduleusecase.CreateRuleInput{}, scheduleRequestFieldError{field: "local_end", code: "invalid_local_time", message: "Local datetime must use YYYY-MM-DDTHH:MM:SS without an offset"}
	}
	var endDate *time.Time
	if request.EndDate != nil {
		parsed, err := time.Parse("2006-01-02", *request.EndDate)
		if err != nil {
			return scheduleusecase.CreateRuleInput{}, scheduleRequestFieldError{field: "end_date", code: "invalid_date", message: "End date must use YYYY-MM-DD"}
		}
		endDate = &parsed
	}
	return scheduleusecase.CreateRuleInput{
		Title: request.Title, Description: request.Description, Location: request.Location,
		ProjectID: request.ProjectID, AssigneeID: request.AssigneeID, Timezone: request.Timezone,
		LocalStart: start, LocalEnd: end, Frequency: request.Frequency, Interval: request.Interval,
		AllDay: request.AllDay, Weekdays: request.Weekdays, EndDate: endDate,
		ConfirmTodoRegeneration: request.ConfirmTodoRegeneration,
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

func parseRuleLocal(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("invalid local datetime")
}

func validateRuleWeekdays(values []string) error {
	for _, value := range values {
		switch value {
		case "mon", "tue", "wed", "thu", "fri", "sat", "sun":
		default:
			return scheduleRequestFieldError{field: "weekdays", code: "invalid_weekday", message: "Weekday must be one of mon, tue, wed, thu, fri, sat, or sun"}
		}
	}
	return nil
}

func (request ScheduleRuleUpdateRequest) input() (scheduleusecase.UpdateRuleInput, error) {
	input := scheduleusecase.UpdateRuleInput{ConfirmTodoRegeneration: request.ConfirmTodoRegeneration}
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
		if err := validateRuleWeekdays(*request.Weekdays.Value); err != nil {
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
		parsed, err := parseRuleLocal(*request.LocalStart.Value)
		if err != nil {
			return input, scheduleRequestFieldError{field: "local_start", code: "invalid_local_time", message: "Local datetime must use YYYY-MM-DDTHH:MM:SS without an offset"}
		}
		input.LocalStart = &parsed
	}
	if request.LocalEnd.Present && request.LocalEnd.Value != nil {
		parsed, err := parseRuleLocal(*request.LocalEnd.Value)
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

func scheduleRuleResponse(row dao.Rule) ScheduleRuleResponse {
	return ScheduleRuleResponse{
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
