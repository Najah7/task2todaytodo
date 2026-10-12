package rest

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	inboxdao "github.com/Najah7/task2todaytodo/internal/application/inbox/dao"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

var errSpecInboxListFailed = NewFailureErrSpec("inbox", ActionList, "Failed to list inbox")

type InboxHandler struct {
	usecases   inboxusecase.InboxUseCases
	pageTokens *pagination.Codec
}

func NewInboxHandler(usecases inboxusecase.InboxUseCases, pageTokens *pagination.Codec) *InboxHandler {
	return &InboxHandler{usecases: usecases, pageTokens: pageTokens}
}

type InboxTaskPayloadResponse struct {
	ID                       string  `json:"id" validate:"required"`
	Revision                 int32   `json:"revision" validate:"required"`
	Description              string  `json:"description" validate:"required"`
	AssigneeID               string  `json:"assignee_id" validate:"required"`
	DueDate                  *string `json:"due_date" validate:"required" extensions:"x-nullable"`
	RemainingDays            *int    `json:"remaining_days" validate:"required" extensions:"x-nullable"`
	EstimatedMinutes         *int    `json:"estimated_minutes" validate:"required" extensions:"x-nullable"`
	EstimateSource           string  `json:"estimate_source" validate:"required"`
	ActualMinutes            *int    `json:"actual_minutes" validate:"required" extensions:"x-nullable"`
	Progress                 int     `json:"progress" validate:"required"`
	Priority                 string  `json:"priority" validate:"required"`
	Status                   string  `json:"status" validate:"required"`
	ActionItemCount          int     `json:"action_item_count" validate:"required"`
	ActionItemCompletedCount int     `json:"action_item_completed_count" validate:"required"`
	CanUpdate                bool    `json:"can_update" validate:"required"`
	CanAssign                bool    `json:"can_assign" validate:"required"`
}

type InboxSchedulePayloadResponse struct {
	OccurrenceID       *string `json:"occurrence_id" validate:"required" extensions:"x-nullable"`
	RuleID             string  `json:"rule_id" validate:"required"`
	OccurrenceDate     string  `json:"occurrence_date" validate:"required"`
	SourceRevisionID   string  `json:"source_revision_id" validate:"required"`
	AssigneeID         string  `json:"assignee_id" validate:"required"`
	Description        string  `json:"description" validate:"required"`
	Location           string  `json:"location" validate:"required"`
	Timezone           string  `json:"timezone" validate:"required"`
	AllDay             bool    `json:"all_day" validate:"required"`
	Completed          bool    `json:"completed" validate:"required"`
	Skipped            bool    `json:"skipped" validate:"required"`
	StartAt            string  `json:"start_at" validate:"required"`
	EndAt              string  `json:"end_at" validate:"required"`
	Revision           int32   `json:"revision" validate:"required"`
	CanUpdate          bool    `json:"can_update" validate:"required"`
	CanAssign          bool    `json:"can_assign" validate:"required"`
	CanUpdateFollowing bool    `json:"can_update_following" validate:"required"`
}

type InboxItemResponse struct {
	Type        string                        `json:"type" validate:"required" enums:"task,schedule"`
	ID          string                        `json:"id" validate:"required"`
	Title       string                        `json:"title" validate:"required"`
	ProjectID   *string                       `json:"project_id" validate:"required" extensions:"x-nullable"`
	ProjectName *string                       `json:"project_name" validate:"required" extensions:"x-nullable"`
	CreatedAt   string                        `json:"created_at" validate:"required"`
	Task        *InboxTaskPayloadResponse     `json:"task" validate:"required" extensions:"x-nullable"`
	Schedule    *InboxSchedulePayloadResponse `json:"schedule" validate:"required" extensions:"x-nullable"`
}

type InboxCountsResponse struct {
	All      int `json:"all" validate:"required"`
	Task     int `json:"task" validate:"required"`
	Schedule int `json:"schedule" validate:"required"`
}

type InboxListResponse struct {
	Items             []InboxItemResponse `json:"items" validate:"required"`
	NextPageToken     string              `json:"next_page_token" validate:"required"`
	PreviousPageToken string              `json:"previous_page_token" validate:"required"`
	Counts            InboxCountsResponse `json:"counts" validate:"required"`
}

type inboxPageAnchor struct {
	At              string `json:"at,omitempty"`
	ID              string `json:"id,omitempty"`
	Type            string `json:"type,omitempty"`
	Direction       string `json:"direction,omitempty"`
	RuleID          string `json:"rule_id,omitempty"`
	OccurrenceDate  string `json:"occurrence_date,omitempty"`
	ScheduleStartAt int64  `json:"schedule_start_at,omitempty"`
	AsOf            string `json:"as_of,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	Today           string `json:"today,omitempty"`
}

// List returns a page of the caller's execution inbox.
//
//	@ID				listInbox
//	@Summary		List inbox
//	@Description	Returns a paginated, read-only projection of all incomplete Tasks assigned to the caller and uncompleted, unskipped Schedule occurrences from the caller's local today through six days later, including virtual recurring occurrences. Results are ordered by original Task or Schedule rule creation time descending; occurrences from one rule are ordered by scheduled start time ascending.
//	@Tags			Inbox
//	@Produce		json
//	@Security		BearerAuth
//	@Param			type		query		string	false	"Item type filter"	Enums(all,task,schedule)
//	@Param			project_id	query		string	false	"Project filter: all, no-project, or a Project ID"
//	@Param			title		query		string	false	"Literal case-insensitive title substring"
//	@Param			page_size	query		int		false	"Items per page (default 50, maximum 100)"
//	@Param			page_token	query		string	false	"Opaque cursor for the next or previous page"
//	@Param			fields		query		string	false	"Response field mask"
//	@Success		200			{object}	InboxListResponse
//	@Failure		400			{object}	ErrResponse	"Invalid filter or pagination"
//	@Failure		401			{object}	ErrResponse	"Unauthorized"
//	@Failure		500			{object}	ErrResponse	"Failed to list inbox"
//	@Router			/inbox [get]
func (h *InboxHandler) List(w http.ResponseWriter, r *http.Request) {
	actorID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || actorID == "" {
		writeUnauthorized(w, errSpecInboxListFailed)
		return
	}
	query := r.URL.Query()
	for _, key := range []string{"type", "project_id", "title"} {
		if len(query[key]) > 1 {
			writeListError(w, errSpecInboxListFailed, invalidListRequestError{field: key})
			return
		}
	}
	typeFilter := query.Get("type")
	if typeFilter == "" {
		typeFilter = "all"
	}
	if typeFilter != "all" && typeFilter != "task" && typeFilter != "schedule" {
		writeListError(w, errSpecInboxListFailed, invalidListRequestError{field: "type"})
		return
	}
	projectValue := query.Get("project_id")
	projectMode, projectID := inboxusecase.ProjectFilterAll, ""
	switch projectValue {
	case "", "all":
	case "no-project":
		projectMode = inboxusecase.ProjectFilterNoProject
	default:
		projectMode, projectID = inboxusecase.ProjectFilterProject, projectValue
	}
	title := query.Get("title")
	order := "type=" + typeFilter + "|project_mode=" + string(projectMode) + "|project_id=" + url.QueryEscape(projectID) + "|title=" + url.QueryEscape(title)
	parsed, err := parseListRequest(r, h.pageTokens, actorID, "inbox", "", order, InboxListResponse{})
	if err != nil {
		writeListError(w, errSpecInboxListFailed, err)
		return
	}
	request := inboxusecase.ListRequest{
		Size:          parsed.Size,
		TypeFilter:    typeFilter,
		ProjectFilter: inboxusecase.ProjectFilter{Mode: projectMode, ProjectID: projectID},
		Title:         title,
	}
	if parsed.Anchor != nil {
		anchor, decodeErr := pagination.Decode[inboxPageAnchor](h.pageTokens, query.Get("page_token"), parsed.Scope)
		if decodeErr != nil || anchor.Type == "" || anchor.ID == "" || anchor.At == "" || anchor.Direction == "" || anchor.AsOf == "" || anchor.Timezone == "" || anchor.Today == "" {
			writeListError(w, errSpecInboxListFailed, invalidListRequestError{field: "page_token"})
			return
		}
		asOf, parseErr := time.Parse(time.RFC3339Nano, anchor.AsOf)
		if parseErr != nil {
			writeListError(w, errSpecInboxListFailed, invalidListRequestError{field: "page_token"})
			return
		}
		request.Anchor = &inboxusecase.CursorAnchor{
			CreatedAt: anchor.At, Type: inboxdao.ItemType(anchor.Type), ID: anchor.ID, Direction: anchor.Direction,
			RuleID: anchor.RuleID, OccurrenceDate: anchor.OccurrenceDate, ScheduleStartAt: anchor.ScheduleStartAt,
		}
		request.Snapshot = inboxusecase.Snapshot{AsOf: asOf, Timezone: anchor.Timezone, Today: anchor.Today}
	}
	page, err := h.usecases.List.Execute(r.Context(), actorID, request)
	if err != nil {
		if errors.Is(err, inboxusecase.ErrInvalidFilter) || errors.Is(err, inboxusecase.ErrInvalidPageRequest) {
			writeListError(w, errSpecInboxListFailed, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, errSpecInboxListFailed, ErrDetailInternalServerError)
		return
	}
	response := InboxListResponse{
		Items:         make([]InboxItemResponse, 0, len(page.Items)),
		NextPageToken: "", PreviousPageToken: "",
		Counts: InboxCountsResponse{All: page.Counts.All, Task: page.Counts.Task, Schedule: page.Counts.Schedule},
	}
	for _, item := range page.Items {
		response.Items = append(response.Items, inboxItemResponse(item))
	}
	if page.Next != nil {
		token, encodeErr := encodeInboxPageToken(h.pageTokens, parsed.Scope, *page.Next, page.Snapshot)
		if encodeErr != nil {
			WriteError(w, http.StatusInternalServerError, errSpecInboxListFailed, ErrDetailInternalServerError)
			return
		}
		response.NextPageToken = token
	}
	if page.Previous != nil {
		token, encodeErr := encodeInboxPageToken(h.pageTokens, parsed.Scope, *page.Previous, page.Snapshot)
		if encodeErr != nil {
			WriteError(w, http.StatusInternalServerError, errSpecInboxListFailed, ErrDetailInternalServerError)
			return
		}
		response.PreviousPageToken = token
	}
	encoded, err := parsed.Mask.Project(response)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, errSpecInboxListFailed, ErrDetailInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

func encodeInboxPageToken(codec *pagination.Codec, scope pagination.Scope, anchor inboxusecase.CursorAnchor, snapshot inboxusecase.Snapshot) (string, error) {
	return pagination.Encode(codec, scope, inboxPageAnchor{
		At: anchor.CreatedAt, ID: anchor.ID, Type: string(anchor.Type), Direction: anchor.Direction,
		RuleID: anchor.RuleID, OccurrenceDate: anchor.OccurrenceDate, ScheduleStartAt: anchor.ScheduleStartAt,
		AsOf: snapshot.AsOf.UTC().Format(time.RFC3339Nano), Timezone: snapshot.Timezone, Today: snapshot.Today,
	})
}

func inboxItemResponse(item inboxdao.Item) InboxItemResponse {
	response := InboxItemResponse{Type: string(item.Type), ID: item.ID, Title: item.Title, CreatedAt: item.CreatedAt}
	if item.ProjectID != "" {
		projectID, projectName := item.ProjectID, item.ProjectName
		response.ProjectID, response.ProjectName = &projectID, &projectName
	}
	if item.Task != nil {
		response.Task = &InboxTaskPayloadResponse{
			ID: item.Task.TaskID, Revision: item.Task.Revision, Description: item.Task.Description, AssigneeID: item.Task.AssigneeID,
			RemainingDays: cloneInt(item.Task.RemainingDays), EstimatedMinutes: cloneInt(item.Task.EstimatedMinutes),
			EstimateSource: item.Task.EstimateSource, ActualMinutes: cloneInt(item.Task.ActualMinutes),
			Progress: item.Task.Progress, Priority: item.Task.Priority, Status: item.Task.Status,
			ActionItemCount: item.Task.ActionItemCount, ActionItemCompletedCount: item.Task.ActionItemCompletedCount,
			CanUpdate: item.Task.CanUpdate, CanAssign: item.Task.CanAssign,
		}
		if item.Task.DueDate != 0 {
			dueDate := time.Unix(item.Task.DueDate, 0).UTC().Format("2006-01-02")
			response.Task.DueDate = &dueDate
		}
	}
	if item.Schedule != nil {
		response.Schedule = &InboxSchedulePayloadResponse{
			OccurrenceID: cloneString(item.Schedule.OccurrenceID), RuleID: item.Schedule.RuleID,
			OccurrenceDate: item.Schedule.OccurrenceDate, SourceRevisionID: item.Schedule.SourceRevisionID,
			AssigneeID: item.Schedule.AssigneeID, Description: item.Schedule.Description,
			Location: item.Schedule.Location, Timezone: item.Schedule.Timezone,
			AllDay: item.Schedule.AllDay, Completed: item.Schedule.Completed, Skipped: item.Schedule.Skipped,
			StartAt: time.Unix(item.Schedule.StartAt, 0).UTC().Format(time.RFC3339Nano),
			EndAt:   time.Unix(item.Schedule.EndAt, 0).UTC().Format(time.RFC3339Nano), Revision: item.Schedule.Revision,
			CanUpdate: item.Schedule.CanUpdate, CanAssign: item.Schedule.CanAssign,
			CanUpdateFollowing: item.Schedule.CanUpdateFollowing,
		}
	}
	return response
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
