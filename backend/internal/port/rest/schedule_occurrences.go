package rest

import (
	"net/http"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	scheduleusecase "github.com/Najah7/task2todaytodo/internal/application/schedule/usecase"
	"github.com/go-chi/chi/v5"
)

var scheduleOccurrenceListFailure = NewFailureErrSpec("schedule_occurrences", ActionList, "Failed to list schedule occurrences")

type ScheduleOccurrenceListResponse struct {
	Items         []ScheduleOccurrenceResponse `json:"items"`
	NextPageToken string                       `json:"next_page_token"`
}

// ListOccurrences returns saved and virtual schedules overlapping a UTC interval.
//
//	@Summary	List schedule occurrences
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		from			query		string	true	"Inclusive UTC range start"
//	@Param		to				query		string	true	"Exclusive UTC range end"
//	@Param		include_skipped	query		bool	false	"Include skipped occurrences"
//	@Param		page_size		query		int		false	"Items per page"
//	@Param		page_token		query		string	false	"Opaque page token"
//	@Success	200				{object}	ScheduleOccurrenceListResponse
//	@Router		/schedules [get]
func (h *ScheduleHandler) ListOccurrences(w http.ResponseWriter, r *http.Request) {
	h.listOccurrences(w, r, "")
}

// ListProjectOccurrences lists overlapping schedules in a Project.
//
//	@Summary	List Project schedule occurrences
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id				path		string	true	"Project ID"
//	@Param		from			query		string	true	"Inclusive UTC range start"
//	@Param		to				query		string	true	"Exclusive UTC range end"
//	@Param		include_skipped	query		bool	false	"Include skipped occurrences"
//	@Success	200				{object}	ScheduleOccurrenceListResponse
//	@Param		page_size		query		int		false	"Items per page"
//	@Param		page_token		query		string	false	"Opaque page token"
//	@Router		/projects/{id}/schedules [get]
func (h *ScheduleHandler) ListProjectOccurrences(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		WriteError(w, http.StatusBadRequest, scheduleOccurrenceListFailure, scheduleField("id", "required", "Project ID is required"))
		return
	}
	h.listOccurrences(w, r, projectID)
}

func (h *ScheduleHandler) listOccurrences(w http.ResponseWriter, r *http.Request, projectID string) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleOccurrenceListFailure, ErrDetailUnauthorized)
		return
	}
	request, err := parseListRequestWithRange(r, h.pageTokens, string(actor), "schedule_occurrences", projectID, "start_at_asc_rule_id_asc_occurrence_date_asc", listEnvelope[ScheduleOccurrenceResponse]{})
	if err != nil {
		writeListError(w, scheduleOccurrenceListFailure, err)
		return
	}
	page, err := h.schedules.ListOccurrences.ExecutePage(r.Context(), string(actor), scheduleusecase.CursorPageRequest{
		Size: request.Size, From: request.From, To: request.To, ProjectID: projectID,
		IncludeSkipped: request.IncludeSkipped,
		Anchor:         occurrenceCursorAnchor(request.Anchor),
	})
	if err != nil {
		writeScheduleUseCaseError(w, scheduleOccurrenceListFailure, err)
		return
	}
	items := make([]ScheduleOccurrenceResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, scheduleOccurrenceResponse(row))
		anchors = append(anchors, listAnchor{
			At:     time.Unix(row.StartAt, 0).UTC().Format(time.RFC3339Nano),
			RuleID: row.RuleID, OccurrenceDate: row.OccurrenceDate,
		})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, scheduleOccurrenceListFailure)
}

func occurrenceCursorAnchor(anchor *listAnchor) *scheduleusecase.CursorAnchor {
	if anchor == nil {
		return nil
	}
	return &scheduleusecase.CursorAnchor{At: anchor.At, RuleID: anchor.RuleID, OccurrenceDate: anchor.OccurrenceDate}
}

// GetOccurrence returns a saved or virtual schedule.
//
//	@Summary	Get schedule occurrence
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		rule_id			path		string	true	"Rule ID"
//	@Param		occurrence_date	path		string	true	"Original local occurrence date"
//	@Success	200				{object}	ScheduleOccurrenceResponse
//	@Router		/schedules/{rule_id}/{occurrence_date} [get]
func (h *ScheduleHandler) GetOccurrence(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleOccurrenceGetFailure, ErrDetailUnauthorized)
		return
	}
	ruleID, occurrenceDate, ok := scheduleOccurrencePath(w, r, scheduleOccurrenceGetFailure)
	if !ok {
		return
	}
	row, err := h.schedules.GetOccurrence.Execute(r.Context(), string(actor), ruleID, occurrenceDate)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleOccurrenceGetFailure, err)
		return
	}
	setResourceETag(w, row.Revision)
	WriteJSON(w, http.StatusOK, scheduleOccurrenceResponse(row))
}

type ScheduleOccurrenceRevisionResponse struct {
	Revision         int32  `json:"revision"`
	SourceRevisionID string `json:"source_revision_id"`
	UserID           string `json:"user_id"`
	OccurrenceDate   string `json:"occurrence_date"`
	StartAt          string `json:"start_at"`
	EndAt            string `json:"end_at"`
	AllDay           bool   `json:"all_day"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Location         string `json:"location"`
	ProjectID        string `json:"project_id,omitempty"`
	AssigneeID       string `json:"assignee_id"`
	Timezone         string `json:"timezone"`
	Completed        bool   `json:"completed"`
	SkippedAt        *int64 `json:"skipped_at,omitempty"`
	DeletedAt        *int64 `json:"deleted_at,omitempty"`
	ChangedBy        string `json:"changed_by"`
	ChangedAt        int64  `json:"changed_at"`
}

// ListOccurrenceRevisions returns audit history for one occurrence origin.
//
//	@Summary	List schedule occurrence revisions
//	@Tags		Schedules
//	@Produce	json
//	@Security	BearerAuth
//	@Param		rule_id			path		string	true	"Rule ID"
//	@Param		occurrence_date	path		string	true	"Original local occurrence date"
//	@Param		page_size		query		int		false	"Items per page"
//	@Param		page_token		query		string	false	"Opaque page token"
//	@Success	200				{object}	listEnvelope[ScheduleOccurrenceRevisionResponse]
//	@Router		/schedules/{rule_id}/{occurrence_date}/revisions [get]
func (h *ScheduleHandler) ListOccurrenceRevisions(w http.ResponseWriter, r *http.Request) {
	spec := scheduleOccurrenceGetFailure
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return
	}
	ruleID, occurrenceDate, ok := scheduleOccurrencePath(w, r, spec)
	if !ok {
		return
	}
	parent := ruleID + "/" + occurrenceDate
	request, err := parseListRequest(r, h.pageTokens, actor, "schedule_revisions", parent, "revision_desc", listEnvelope[ScheduleOccurrenceRevisionResponse]{})
	if err != nil {
		writeListError(w, spec, err)
		return
	}
	page, err := h.schedules.OccurrenceRevisions.ExecutePage(r.Context(), actor, ruleID, occurrenceDate, request.Size, anchorRevision(request.Anchor))
	if err != nil {
		writeScheduleUseCaseError(w, spec, err)
		return
	}
	items := make([]ScheduleOccurrenceRevisionResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, row := range page.Items {
		items = append(items, ScheduleOccurrenceRevisionResponse{
			Revision: row.Revision, SourceRevisionID: row.SourceRevisionID, UserID: row.UserID,
			OccurrenceDate: row.OccurrenceDate, StartAt: time.Unix(row.StartAt, 0).UTC().Format(time.RFC3339),
			EndAt: time.Unix(row.EndAt, 0).UTC().Format(time.RFC3339), AllDay: row.AllDay, Title: row.Title, Description: row.Description,
			Location: row.Location, ProjectID: row.ProjectID, AssigneeID: row.AssigneeID, Timezone: row.Timezone,
			Completed: row.Completed, SkippedAt: row.SkippedAt, DeletedAt: row.DeletedAt,
			ChangedBy: row.ChangedBy, ChangedAt: row.ChangedAt,
		})
		anchors = append(anchors, listAnchor{Revision: row.Revision})
	}
	writeListResponse(w, items, anchors, page.Next != nil, request, h.pageTokens, spec)
}

// UpdateOccurrence edits one concrete occurrence.
//
//	@Summary	Update schedule occurrence
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		rule_id			path		string							true	"Rule ID"
//	@Param		occurrence_date	path		string							true	"Original local occurrence date"
//	@Param		If-Match		header		string							true	"Current occurrence revision, or 0 for a virtual occurrence"
//	@Param		request			body		ScheduleOccurrenceUpdateRequest	true	"Occurrence fields"
//	@Success	200				{object}	ScheduleOccurrenceResponse
//	@Router		/schedules/{rule_id}/{occurrence_date} [patch]
func (h *ScheduleHandler) UpdateOccurrence(w http.ResponseWriter, r *http.Request) {
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, scheduleOccurrenceUpdateFailure, ErrDetailUnauthorized)
		return
	}
	expected, ok := expectedOccurrenceRevision(w, r, scheduleOccurrenceUpdateFailure)
	if !ok {
		return
	}
	ruleID, occurrenceDate, ok := scheduleOccurrencePath(w, r, scheduleOccurrenceUpdateFailure)
	if !ok {
		return
	}
	var request ScheduleOccurrenceUpdateRequest
	if !decodeScheduleJSON(w, r, &request) {
		WriteError(w, http.StatusBadRequest, scheduleOccurrenceUpdateFailure, ErrDetailInvalidRequestBody)
		return
	}
	input, err := request.input()
	if err != nil {
		writeScheduleRequestError(w, scheduleOccurrenceUpdateFailure, err)
		return
	}
	row, err := h.schedules.UpdateOccurrence.Execute(r.Context(), string(actor), ruleID, occurrenceDate, input, expected)
	if err != nil {
		writeScheduleUseCaseError(w, scheduleOccurrenceUpdateFailure, err)
		return
	}
	setResourceETag(w, row.Revision)
	WriteJSON(w, http.StatusOK, scheduleOccurrenceResponse(row))
}

// CompleteOccurrence marks one occurrence complete.
//
//	@Summary	Complete schedule occurrence
//	@Tags		Schedules
//	@Security	BearerAuth
//	@Param		rule_id			path	string	true	"Rule ID"
//	@Param		occurrence_date	path	string	true	"Original local occurrence date"
//	@Param		If-Match		header	string	true	"Current occurrence revision, or 0 for a virtual occurrence"
//	@Success	204
//	@Router		/schedules/{rule_id}/{occurrence_date}:complete [post]
func (h *ScheduleHandler) CompleteOccurrence(w http.ResponseWriter, r *http.Request) {
	h.setOccurrenceState(w, r, h.schedules.CompleteOccurrence, "Completed")
}

// ReopenOccurrence reopens one completed occurrence.
//
//	@Summary	Reopen schedule occurrence
//	@Tags		Schedules
//	@Security	BearerAuth
//	@Param		rule_id			path	string	true	"Rule ID"
//	@Param		occurrence_date	path	string	true	"Original local occurrence date"
//	@Param		If-Match		header	string	true	"Current occurrence revision, or 0 for a virtual occurrence"
//	@Success	204
//	@Router		/schedules/{rule_id}/{occurrence_date}:reopen [post]
func (h *ScheduleHandler) ReopenOccurrence(w http.ResponseWriter, r *http.Request) {
	h.setOccurrenceState(w, r, h.schedules.ReopenOccurrence, "Reopened")
}

// SkipOccurrence skips one occurrence.
//
//	@Summary	Skip schedule occurrence
//	@Tags		Schedules
//	@Security	BearerAuth
//	@Param		rule_id						path	string	true	"Rule ID"
//	@Param		occurrence_date				path	string	true	"Original local occurrence date"
//	@Param		If-Match					header	string	true	"Current occurrence revision, or 0 for a virtual occurrence"
//	@Param		confirm_todo_regeneration	query	bool	false	"Confirm regeneration of affected Todos"
//	@Success	204
//	@Router		/schedules/{rule_id}/{occurrence_date}:skip [post]
func (h *ScheduleHandler) SkipOccurrence(w http.ResponseWriter, r *http.Request) {
	h.setOccurrenceState(w, r, h.schedules.SkipOccurrence, "Skipped")
}

// RestoreOccurrence restores a skipped or deleted occurrence.
//
//	@Summary	Restore schedule occurrence
//	@Tags		Schedules
//	@Security	BearerAuth
//	@Param		rule_id						path	string	true	"Rule ID"
//	@Param		occurrence_date				path	string	true	"Original local occurrence date"
//	@Param		If-Match					header	string	true	"Current occurrence revision, or 0 for a virtual occurrence"
//	@Param		confirm_todo_regeneration	query	bool	false	"Confirm regeneration of affected Todos"
//	@Success	204
//	@Router		/schedules/{rule_id}/{occurrence_date}:restore [post]
func (h *ScheduleHandler) RestoreOccurrence(w http.ResponseWriter, r *http.Request) {
	h.setOccurrenceState(w, r, h.schedules.RestoreOccurrence, "Restored")
}

// DeleteOccurrence deletes one occurrence while retaining its stable origin key.
//
//	@Summary	Delete schedule occurrence
//	@Tags		Schedules
//	@Security	BearerAuth
//	@Param		rule_id						path	string	true	"Rule ID"
//	@Param		occurrence_date				path	string	true	"Original local occurrence date"
//	@Param		If-Match					header	string	true	"Current occurrence revision, or 0 for a virtual occurrence"
//	@Param		confirm_todo_regeneration	query	bool	false	"Confirm regeneration of affected Todos"
//	@Success	204
//	@Router		/schedules/{rule_id}/{occurrence_date} [delete]
func (h *ScheduleHandler) DeleteOccurrence(w http.ResponseWriter, r *http.Request) {
	h.setOccurrenceState(w, r, h.schedules.DeleteOccurrence, "Deleted")
}

func (h *ScheduleHandler) setOccurrenceState(w http.ResponseWriter, r *http.Request, uc *scheduleusecase.SetOccurrenceStateUseCase, verb string) {
	spec := NewFailureErrSpec("schedule_occurrences", strings.ToLower(verb), "Failed to "+strings.ToLower(verb)+" schedule occurrence")
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return
	}
	expected, ok := expectedOccurrenceRevision(w, r, spec)
	if !ok {
		return
	}
	ruleID, occurrenceDate, ok := scheduleOccurrencePath(w, r, spec)
	if !ok {
		return
	}
	if uc == nil {
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
		return
	}
	if err := uc.Execute(r.Context(), string(actor), ruleID, occurrenceDate, expected, r.URL.Query().Get("confirm_todo_regeneration") == "true"); err != nil {
		writeScheduleUseCaseError(w, spec, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ScheduleOccurrenceRescheduleRequest struct {
	StartAt                 *string `json:"start_at"`
	EndAt                   *string `json:"end_at"`
	ConfirmTodoRegeneration bool    `json:"confirm_todo_regeneration"`
}

// RescheduleOccurrence moves one concrete occurrence.
//
//	@Summary	Reschedule occurrence
//	@Tags		Schedules
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		rule_id			path		string								true	"Rule ID"
//	@Param		occurrence_date	path		string								true	"Original local occurrence date"
//	@Param		If-Match		header		string								true	"Current occurrence revision, or 0 for a virtual occurrence"
//	@Param		request			body		ScheduleOccurrenceRescheduleRequest	true	"New interval"
//	@Success	200				{object}	ScheduleOccurrenceResponse
//	@Router		/schedules/{rule_id}/{occurrence_date}:reschedule [post]
func (h *ScheduleHandler) RescheduleOccurrence(w http.ResponseWriter, r *http.Request) {
	spec := NewFailureErrSpec("schedule_occurrences", "reschedule", "Failed to reschedule schedule occurrence")
	actor, ok := scheduleActor(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return
	}
	expected, ok := expectedOccurrenceRevision(w, r, spec)
	if !ok {
		return
	}
	ruleID, occurrenceDate, ok := scheduleOccurrencePath(w, r, spec)
	if !ok {
		return
	}
	var request ScheduleOccurrenceRescheduleRequest
	if !decodeScheduleJSON(w, r, &request) || request.StartAt == nil || request.EndAt == nil {
		WriteError(w, http.StatusBadRequest, spec, ErrDetailInvalidRequestBody)
		return
	}
	startAt, err := time.Parse(time.RFC3339Nano, *request.StartAt)
	if err != nil {
		WriteError(w, http.StatusBadRequest, spec, scheduleField("start_at", "invalid_timestamp", "Timestamp must be a valid RFC3339 instant"))
		return
	}
	endAt, err := time.Parse(time.RFC3339Nano, *request.EndAt)
	if err != nil {
		WriteError(w, http.StatusBadRequest, spec, scheduleField("end_at", "invalid_timestamp", "Timestamp must be a valid RFC3339 instant"))
		return
	}
	row, err := h.schedules.RescheduleOccurrence.Execute(r.Context(), string(actor), ruleID, occurrenceDate, startAt, endAt, expected, request.ConfirmTodoRegeneration)
	if err != nil {
		writeScheduleUseCaseError(w, spec, err)
		return
	}
	setResourceETag(w, row.Revision)
	WriteJSON(w, http.StatusOK, scheduleOccurrenceResponse(row))
}

func scheduleOccurrencePath(w http.ResponseWriter, r *http.Request, spec ErrSpec) (string, string, bool) {
	ruleID := chi.URLParam(r, "rule_id")
	date := chi.URLParam(r, "occurrence_date")
	if ruleID == "" {
		WriteError(w, http.StatusBadRequest, spec, scheduleField("rule_id", "required", "Rule ID is required"))
		return "", "", false
	}
	if date == "" {
		WriteError(w, http.StatusBadRequest, spec, scheduleField("occurrence_date", "required", "Occurrence date is required"))
		return "", "", false
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		WriteError(w, http.StatusBadRequest, spec, scheduleField("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD"))
		return "", "", false
	}
	return ruleID, date, true
}

func (request ScheduleOccurrenceUpdateRequest) input() (scheduleusecase.UpdateScheduleInput, error) {
	input := scheduleusecase.UpdateScheduleInput{ConfirmTodoRegeneration: request.ConfirmTodoRegeneration}
	for _, field := range []struct {
		name    string
		present bool
		isNull  bool
	}{
		{"title", request.Title.Present, request.Title.Value == nil},
		{"description", request.Description.Present, request.Description.Value == nil},
		{"location", request.Location.Present, request.Location.Value == nil},
		{"assignee_id", request.AssigneeID.Present, request.AssigneeID.Value == nil},
		{"start_at", request.StartAt.Present, request.StartAt.Value == nil},
		{"end_at", request.EndAt.Present, request.EndAt.Value == nil},
		{"all_day", request.AllDay.Present, request.AllDay.Value == nil},
	} {
		if field.present && field.isNull {
			return input, scheduleRequestFieldError{field: field.name, code: "null_not_allowed", message: "Field cannot be null"}
		}
	}
	input.Title, input.Description, input.Location = request.Title.Value, request.Description.Value, request.Location.Value
	if request.ProjectID.Present {
		value := ""
		if request.ProjectID.Value != nil {
			value = *request.ProjectID.Value
		}
		input.ProjectID = &value
	}
	if request.AssigneeID.Present {
		input.AssigneeID = request.AssigneeID.Value
	}
	if request.StartAt.Present {
		if request.StartAt.Value == nil {
			return input, scheduleRequestFieldError{field: "start_at", code: "null_not_allowed", message: "Field cannot be null"}
		}
		value, err := time.Parse(time.RFC3339Nano, *request.StartAt.Value)
		if err != nil {
			return input, scheduleRequestFieldError{field: "start_at", code: "invalid_timestamp", message: "Timestamp must be a valid RFC3339 instant"}
		}
		input.StartAt = &value
	}
	if request.EndAt.Present {
		if request.EndAt.Value == nil {
			return input, scheduleRequestFieldError{field: "end_at", code: "null_not_allowed", message: "Field cannot be null"}
		}
		value, err := time.Parse(time.RFC3339Nano, *request.EndAt.Value)
		if err != nil {
			return input, scheduleRequestFieldError{field: "end_at", code: "invalid_timestamp", message: "Timestamp must be a valid RFC3339 instant"}
		}
		input.EndAt = &value
	}
	if request.AllDay.Present {
		input.AllDay = request.AllDay.Value
	}
	return input, nil
}

func scheduleOccurrenceResponse(row dao.ScheduleOccurrence) ScheduleOccurrenceResponse {
	return ScheduleOccurrenceResponse{
		ID: row.ID, Persisted: row.ID != nil, RuleID: row.RuleID,
		OccurrenceDate: row.OccurrenceDate, SourceRevisionID: row.SourceRevisionID,
		UserID: row.UserID, ProjectID: row.ProjectID, AssigneeID: row.AssigneeID,
		Title: row.Title, Description: row.Description, Location: row.Location, Timezone: row.Timezone,
		StartAt: time.Unix(row.StartAt, 0).UTC().Format(time.RFC3339), EndAt: time.Unix(row.EndAt, 0).UTC().Format(time.RFC3339), AllDay: row.AllDay,
		Completed: row.Completed, Skipped: row.Skipped, Deleted: row.Deleted,
		CanUpdate: row.CanUpdate, CanDelete: row.CanDelete, CanSkip: row.CanSkip, CanAssign: row.CanAssign,
		CanUpdateFollowing: row.CanUpdateFollowing, CanDeleteFollowing: row.CanDeleteFollowing,
		Revision: row.Revision, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
