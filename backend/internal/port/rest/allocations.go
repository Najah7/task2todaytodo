package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	allocationdao "github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	allocationusecase "github.com/Najah7/task2todaytodo/internal/application/allocation/usecase"
	"github.com/go-chi/chi/v5"
)

type AllocationHandler struct{ usecases allocationusecase.UseCases }

func NewAllocationHandler(usecases allocationusecase.UseCases) *AllocationHandler {
	return &AllocationHandler{usecases: usecases}
}

type AllocationWeekResponse struct {
	WeekStart         string                          `json:"week_start"`
	Timezone          string                          `json:"timezone"`
	Projects          []AllocationWeekProjectResponse `json:"projects"`
	Allocations       []AllocationOccurrenceResponse  `json:"allocations"`
	Days              []AllocationDayResponse         `json:"days"`
	SleepIntervals    []SleepIntervalResponse         `json:"sleep_intervals"`
	AllocatedMinutes  int                             `json:"allocated_minutes"`
	TargetMinutes     int                             `json:"target_minutes"`
	DifferenceMinutes int                             `json:"difference_minutes"`
	FreeMinutes       int                             `json:"free_minutes"`
	PausedRules       []PausedAllocationRuleResponse  `json:"paused_rules"`
}
type AllocationWeekProjectResponse struct {
	ProjectID         string `json:"project_id"`
	Title             string `json:"title"`
	TargetMinutes     int    `json:"target_minutes"`
	AllocatedMinutes  int    `json:"allocated_minutes"`
	DifferenceMinutes int    `json:"difference_minutes"`
	ColorIndex        *int   `json:"color_index,omitempty"`
}
type AllocationOccurrenceResponse struct {
	ID                 string    `json:"id,omitempty"`
	RuleID             string    `json:"rule_id"`
	OccurrenceDate     string    `json:"occurrence_date"`
	SourceRevisionID   string    `json:"source_revision_id"`
	ProjectID          string    `json:"project_id"`
	ProjectTitle       string    `json:"project_title"`
	Timezone           string    `json:"timezone"`
	StartAt            time.Time `json:"start_at"`
	EndAt              time.Time `json:"end_at"`
	Revision           int32     `json:"revision"`
	RuleRevision       int32     `json:"rule_revision"`
	IsException        bool      `json:"is_exception"`
	CanUpdate          bool      `json:"can_update"`
	CanDelete          bool      `json:"can_delete"`
	CanUpdateFollowing bool      `json:"can_update_following"`
	CanDeleteFollowing bool      `json:"can_delete_following"`
}
type AllocationDayResponse struct {
	Date              string    `json:"date"`
	WakeAt            time.Time `json:"wake_at"`
	BedtimeAt         time.Time `json:"bedtime_at"`
	SleepRevision     int32     `json:"sleep_revision"`
	HasSleepException bool      `json:"has_sleep_exception"`
	AllocatedMinutes  int       `json:"allocated_minutes"`
	FreeMinutes       int       `json:"free_minutes"`
}
type SleepIntervalResponse struct {
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
	LocalDate string    `json:"local_date"`
}
type PausedAllocationRuleResponse struct {
	RuleID    string `json:"rule_id"`
	ProjectID string `json:"project_id"`
	Reason    string `json:"reason"`
}
type AllocationRulesResponse struct {
	Revision string                   `json:"revision"`
	Timezone string                   `json:"timezone"`
	Items    []AllocationRuleResponse `json:"items"`
}
type AllocationPlansResponse struct {
	Revision string                       `json:"revision"`
	Items    []AllocationPlanItemResponse `json:"items"`
}
type AllocationRuleResponse struct {
	ID           string `json:"id"`
	Revision     int32  `json:"revision"`
	ProjectID    string `json:"project_id"`
	ProjectTitle string `json:"project_title"`
	Weekday      int    `json:"weekday"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	EndDayOffset int    `json:"end_day_offset"`
	Timezone     string `json:"timezone"`
	PausedReason string `json:"paused_reason,omitempty"`
}
type AllocationPlanItemResponse struct {
	ProjectID     string `json:"project_id"`
	Title         string `json:"title"`
	TargetMinutes int    `json:"target_minutes"`
}
type SleepExceptionResponse struct {
	LocalDate    string `json:"local_date"`
	WakeLocal    string `json:"wake_local"`
	BedtimeLocal string `json:"bedtime_local"`
	Timezone     string `json:"timezone"`
	Revision     int32  `json:"revision"`
	HasException bool   `json:"has_exception"`
}

type AllocationRuleRequest struct {
	ID           string `json:"id,omitempty"`
	Revision     int32  `json:"revision,omitempty"`
	ProjectID    string `json:"project_id"`
	Weekday      int    `json:"weekday" example:"0"`
	StartTime    string `json:"start_time" example:"09:00"`
	EndTime      string `json:"end_time" example:"10:30"`
	EndDayOffset int    `json:"end_day_offset"`
	Timezone     string `json:"timezone" example:"Asia/Tokyo"`
}
type AllocationRulesRequest struct {
	Items []AllocationRuleRequest `json:"items"`
}
type AllocationPlanItemRequest struct {
	ProjectID     string `json:"project_id"`
	TargetMinutes int    `json:"target_minutes"`
}
type AllocationPlansRequest struct {
	Items []AllocationPlanItemRequest `json:"items"`
}
type CreateAllocationRequest struct {
	ProjectID    string    `json:"project_id"`
	Timezone     string    `json:"timezone"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	RepeatWeekly bool      `json:"repeat_weekly"`
}
type UpdateAllocationRequest struct {
	ProjectID    string    `json:"project_id"`
	StartAt      time.Time `json:"start_at"`
	EndAt        time.Time `json:"end_at"`
	Scope        string    `json:"scope" example:"current"`
	RuleRevision int32     `json:"rule_revision,omitempty"`
}
type DailySleepExceptionRequest struct {
	WakeLocal    string `json:"wake_local" example:"2026-10-12T07:00"`
	BedtimeLocal string `json:"bedtime_local" example:"2026-10-13T01:00"`
	Timezone     string `json:"timezone" example:"Asia/Tokyo"`
}

// GetWeek returns bounded, server-projected allocation and sleep data.
//
//	@Summary	Get project allocation week
//	@Tags		Allocations
//	@Produce	json
//	@Security	BearerAuth
//	@Param		week_start	path		string	true	"Monday date (YYYY-MM-DD)"
//	@Success	200			{object}	AllocationWeekResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	401			{object}	ErrResponse
//	@Router		/allocation-weeks/{week_start} [get]
func (h *AllocationHandler) GetWeek(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	weekStart, err := time.Parse("2006-01-02", chi.URLParam(r, "week_start"))
	if err != nil || weekStart.Weekday() != time.Monday {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsGetFailed, NewErrDetail("week_start", "invalid_week_start", "Week start must be a Monday date"))
		return
	}
	result, err := h.usecases.GetWeek.Execute(r.Context(), actor, weekStart)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsGetFailed, err)
		return
	}
	WriteJSON(w, http.StatusOK, allocationWeekResponse(result))
}

// GetRules returns the current user's weekly allocation Rules.
//
//	@Summary	Get allocation Rules
//	@Tags		Allocations
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	AllocationRulesResponse
//	@Failure	401	{object}	ErrResponse
//	@Router		/allocation-rules [get]
func (h *AllocationHandler) GetRules(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	result, err := h.usecases.GetRules.Execute(r.Context(), actor)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsGetFailed, err)
		return
	}
	w.Header().Set("ETag", `"`+result.Revision+`"`)
	WriteJSON(w, http.StatusOK, allocationRulesResponse(result))
}

// PutRules atomically replaces the current user's visible weekly Rule collection.
//
//	@Summary	Replace weekly allocation Rules
//	@Tags		Allocations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		If-Match	header		string					true	"Current allocation rules revision"
//	@Param		request		body		AllocationRulesRequest	true	"Weekly Rules"
//	@Success	200			{object}	AllocationRulesResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	409			{object}	ErrResponse
//	@Failure	412			{object}	ErrResponse
//	@Failure	428			{object}	ErrResponse
//	@Router		/allocation-rules [put]
func (h *AllocationHandler) PutRules(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	revision, ok := expectedCollectionRevision(w, r, ErrSpecAllocationsUpdateFailed)
	if !ok {
		return
	}
	var req AllocationRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	items := make([]allocationdao.Rule, 0, len(req.Items))
	for _, rule := range req.Items {
		items = append(items, allocationdao.Rule{ID: rule.ID, Revision: rule.Revision, ProjectID: rule.ProjectID, Weekday: rule.Weekday, StartTime: rule.StartTime, EndTime: rule.EndTime, EndDayOffset: rule.EndDayOffset, Timezone: rule.Timezone})
	}
	result, err := h.usecases.PutRules.Execute(r.Context(), actor, revision, items)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsUpdateFailed, err)
		return
	}
	w.Header().Set("ETag", `"`+result.Revision+`"`)
	WriteJSON(w, http.StatusOK, allocationRulesResponse(result))
}

// GetPlans returns the current user's explicit weekly plan rows.
//
//	@Summary	Get weekly allocation plans
//	@Tags		Allocations
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	AllocationPlansResponse
//	@Failure	401	{object}	ErrResponse
//	@Router		/allocation-plans [get]
func (h *AllocationHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	result, err := h.usecases.GetPlans.Execute(r.Context(), actor)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsGetFailed, err)
		return
	}
	w.Header().Set("ETag", `"`+result.Revision+`"`)
	WriteJSON(w, http.StatusOK, allocationPlansResponse(result))
}

// PutPlans atomically replaces only the current user's explicit weekly plans.
//
//	@Summary	Replace weekly allocation plans
//	@Tags		Allocations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		If-Match	header		string					true	"Current allocation plans revision"
//	@Param		request		body		AllocationPlansRequest	true	"Weekly plans"
//	@Success	200			{object}	AllocationPlansResponse
//	@Failure	400			{object}	ErrResponse
//	@Failure	409			{object}	ErrResponse
//	@Failure	412			{object}	ErrResponse
//	@Failure	428			{object}	ErrResponse
//	@Router		/allocation-plans [put]
func (h *AllocationHandler) PutPlans(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	revision, ok := expectedCollectionRevision(w, r, ErrSpecAllocationsUpdateFailed)
	if !ok {
		return
	}
	var req AllocationPlansRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	items := make([]allocationdao.PlanItem, 0, len(req.Items))
	for _, plan := range req.Items {
		items = append(items, allocationdao.PlanItem{ProjectID: plan.ProjectID, TargetMinutes: plan.TargetMinutes})
	}
	result, err := h.usecases.PutPlans.Execute(r.Context(), actor, revision, items)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsUpdateFailed, err)
		return
	}
	w.Header().Set("ETag", `"`+result.Revision+`"`)
	WriteJSON(w, http.StatusOK, allocationPlansResponse(result))
}

// Create creates a one-off or weekly Project allocation.
//
//	@Summary	Create Project allocation
//	@Tags		Allocations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		request	body		CreateAllocationRequest	true	"Allocation"
//	@Success	201		{object}	AllocationOccurrenceResponse
//	@Failure	409		{object}	ErrResponse
//	@Router		/allocations [post]
func (h *AllocationHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	var req CreateAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	result, err := h.usecases.Create.Execute(r.Context(), actor, req.ProjectID, req.Timezone, req.StartAt, req.EndAt, req.RepeatWeekly)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsCreateFailed, err)
		return
	}
	setResourceETag(w, result.Revision)
	WriteJSON(w, http.StatusCreated, allocationOccurrenceResponse(result))
}

// Update edits an allocation occurrence or its remaining recurrence.
//
//	@Summary	Update Project allocation occurrence
//	@Tags		Allocations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		rule_id			path		string					true	"Rule ID"
//	@Param		occurrence_date	path		string					true	"Original local occurrence date"
//	@Param		If-Match		header		string					true	"Occurrence revision"
//	@Param		request			body		UpdateAllocationRequest	true	"Occurrence update"
//	@Success	200				{object}	AllocationOccurrenceResponse
//	@Failure	409				{object}	ErrResponse
//	@Router		/allocations/{rule_id}/{occurrence_date} [patch]
func (h *AllocationHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	revision, ok := expectedOccurrenceRevision(w, r, ErrSpecAllocationsUpdateFailed)
	if !ok {
		return
	}
	date, err := time.Parse("2006-01-02", chi.URLParam(r, "occurrence_date"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsUpdateFailed, NewErrDetail("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD"))
		return
	}
	var req UpdateAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	result, err := h.usecases.Update.Execute(r.Context(), actor, chi.URLParam(r, "rule_id"), date, revision, req.ProjectID, req.StartAt, req.EndAt, req.Scope, req.RuleRevision)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsUpdateFailed, err)
		return
	}
	setResourceETag(w, result.Revision)
	WriteJSON(w, http.StatusOK, allocationOccurrenceResponse(result))
}

// Delete removes the selected occurrence or stops its remaining recurrence.
//
//	@Summary	Delete Project allocation occurrence
//	@Tags		Allocations
//	@Security	BearerAuth
//	@Param		rule_id			path	string	true	"Rule ID"
//	@Param		occurrence_date	path	string	true	"Original local occurrence date"
//	@Param		If-Match		header	string	true	"Occurrence revision"
//	@Param		scope			query	string	true	"current or following"
//	@Param		rule_revision	query	int		true	"Rule revision when scope is following"
//	@Success	204
//	@Failure	409	{object}	ErrResponse
//	@Router		/allocations/{rule_id}/{occurrence_date} [delete]
func (h *AllocationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	revision, ok := expectedOccurrenceRevision(w, r, ErrSpecAllocationsDeleteFailed)
	if !ok {
		return
	}
	date, err := time.Parse("2006-01-02", chi.URLParam(r, "occurrence_date"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsDeleteFailed, NewErrDetail("occurrence_date", "invalid_date", "Occurrence date must use YYYY-MM-DD"))
		return
	}
	ruleRevision, _ := strconv.ParseInt(r.URL.Query().Get("rule_revision"), 10, 32)
	if err := h.usecases.Delete.Execute(r.Context(), actor, chi.URLParam(r, "rule_id"), date, revision, r.URL.Query().Get("scope"), int32(ruleRevision)); err != nil {
		writeAllocationError(w, ErrSpecAllocationsDeleteFailed, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PutSleepException saves a full per-date sleep pair.
//
//	@Summary	Set daily sleep exception
//	@Tags		Allocations
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		local_date	path		string						true	"Waking date"
//	@Param		If-Match	header		string						true	"Exception revision, or 0 when absent"
//	@Param		request		body		DailySleepExceptionRequest	true	"Sleep exception"
//	@Success	200			{object}	SleepExceptionResponse
//	@Failure	409			{object}	ErrResponse
//	@Router		/daily-sleep-exceptions/{local_date} [put]
func (h *AllocationHandler) PutSleepException(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	revision, ok := expectedOccurrenceRevision(w, r, ErrSpecAllocationsUpdateFailed)
	if !ok {
		return
	}
	date, err := time.Parse("2006-01-02", chi.URLParam(r, "local_date"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsUpdateFailed, NewErrDetail("local_date", "invalid_date", "Date must use YYYY-MM-DD"))
		return
	}
	var req DailySleepExceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	wake, err1 := time.Parse("2006-01-02T15:04", req.WakeLocal)
	bed, err2 := time.Parse("2006-01-02T15:04", req.BedtimeLocal)
	if err1 != nil || err2 != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsUpdateFailed, NewErrDetail("wake_local", "invalid_local_time", "Sleep window must use local YYYY-MM-DDTHH:mm values"))
		return
	}
	result, err := h.usecases.PutSleepException.Execute(r.Context(), actor, date, revision, wake, bed, req.Timezone)
	if err != nil {
		writeAllocationError(w, ErrSpecAllocationsUpdateFailed, err)
		return
	}
	setResourceETag(w, result.Revision)
	WriteJSON(w, http.StatusOK, sleepExceptionResponse(result))
}

// DeleteSleepException resets a date to the current user sleep profile.
//
//	@Summary	Reset daily sleep exception
//	@Tags		Allocations
//	@Security	BearerAuth
//	@Param		local_date	path	string	true	"Waking date"
//	@Param		If-Match	header	string	true	"Exception revision"
//	@Success	204
//	@Router		/daily-sleep-exceptions/{local_date} [delete]
func (h *AllocationHandler) DeleteSleepException(w http.ResponseWriter, r *http.Request) {
	actor, ok := allocationActor(w, r)
	if !ok {
		return
	}
	revision, ok := expectedOccurrenceRevision(w, r, ErrSpecAllocationsDeleteFailed)
	if !ok {
		return
	}
	date, err := time.Parse("2006-01-02", chi.URLParam(r, "local_date"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAllocationsDeleteFailed, NewErrDetail("local_date", "invalid_date", "Date must use YYYY-MM-DD"))
		return
	}
	if err := h.usecases.DeleteSleepException.Execute(r.Context(), actor, date, revision); err != nil {
		writeAllocationError(w, ErrSpecAllocationsDeleteFailed, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func allocationActor(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || actor == "" {
		WriteError(w, http.StatusUnauthorized, ErrSpecAllocationsGetFailed, ErrDetailUnauthorized)
		return "", false
	}
	return actor, true
}

func writeAllocationError(w http.ResponseWriter, spec ErrSpec, err error) {
	switch {
	case errors.Is(err, allocationusecase.ErrConflict):
		var conflict allocationusecase.ConflictError
		detail := NewErrDetail("allocation", "allocation_conflict", "Allocation conflicts with an existing allocation or sleep interval")
		if errors.As(err, &conflict) {
			detail.Metadata = &AllocationConflictMetadata{Kind: conflict.Kind, RuleID: conflict.RuleID, OccurrenceDate: conflict.OccurrenceDate.Format("2006-01-02"), ProjectID: conflict.ProjectID, StartAt: conflict.StartAt.UTC().Format(time.RFC3339), EndAt: conflict.EndAt.UTC().Format(time.RFC3339)}
		}
		WriteError(w, http.StatusConflict, spec, detail)
	case errors.Is(err, allocationusecase.ErrPreconditionFailed):
		WriteError(w, http.StatusPreconditionFailed, spec, NewErrDetail("If-Match", "revision_mismatch", "Resource revision no longer matches"))
	case errors.Is(err, allocationusecase.ErrNotFound):
		WriteError(w, http.StatusNotFound, spec, ErrDetailNotFound)
	case errors.Is(err, allocationusecase.ErrProjectUnavailable):
		WriteError(w, http.StatusNotFound, spec, NewErrDetail("project_id", "project_unavailable", "Project is unavailable"))
	case errors.Is(err, allocationusecase.ErrInvalidRequest):
		WriteError(w, http.StatusBadRequest, spec, NewErrDetail("allocation", "invalid_request", "Allocation request is invalid"))
	default:
		WriteError(w, http.StatusInternalServerError, spec, ErrDetailInternalServerError)
	}
}

func allocationWeekResponse(value allocationdao.Week) AllocationWeekResponse {
	out := AllocationWeekResponse{
		WeekStart: value.WeekStart.Format("2006-01-02"), Timezone: value.Timezone,
		AllocatedMinutes: value.AllocatedMinutes, TargetMinutes: value.TargetMinutes,
		DifferenceMinutes: value.DifferenceMinutes, FreeMinutes: value.FreeMinutes,
		Projects:       make([]AllocationWeekProjectResponse, 0, len(value.Projects)),
		Allocations:    make([]AllocationOccurrenceResponse, 0, len(value.Allocations)),
		Days:           make([]AllocationDayResponse, 0, len(value.Days)),
		SleepIntervals: make([]SleepIntervalResponse, 0, len(value.SleepIntervals)),
		PausedRules:    make([]PausedAllocationRuleResponse, 0, len(value.PausedRules)),
	}
	for _, p := range value.Projects {
		out.Projects = append(out.Projects, AllocationWeekProjectResponse{ProjectID: p.ID, Title: p.Title, TargetMinutes: p.TargetMinutes, AllocatedMinutes: p.AllocatedMinutes, DifferenceMinutes: p.DifferenceMinutes})
	}
	for _, a := range value.Allocations {
		out.Allocations = append(out.Allocations, allocationRowResponse(a))
	}
	for _, d := range value.Days {
		out.Days = append(out.Days, AllocationDayResponse{Date: d.Date.Format("2006-01-02"), WakeAt: d.WakeAt, BedtimeAt: d.BedtimeAt, SleepRevision: d.SleepRevision, HasSleepException: d.HasSleepException, AllocatedMinutes: d.AllocatedMinutes, FreeMinutes: d.FreeMinutes})
	}
	for _, i := range value.SleepIntervals {
		out.SleepIntervals = append(out.SleepIntervals, SleepIntervalResponse{StartAt: i.StartAt, EndAt: i.EndAt, LocalDate: i.LocalDate.Format("2006-01-02")})
	}
	for _, rule := range value.PausedRules {
		out.PausedRules = append(out.PausedRules, PausedAllocationRuleResponse{RuleID: rule.RuleID, ProjectID: rule.ProjectID, Reason: rule.Reason})
	}
	return out
}

func allocationRulesResponse(value allocationdao.RuleCollection) AllocationRulesResponse {
	out := AllocationRulesResponse{Revision: value.Revision, Timezone: value.Timezone, Items: make([]AllocationRuleResponse, 0, len(value.Items))}
	for _, rule := range value.Items {
		out.Items = append(out.Items, AllocationRuleResponse{ID: rule.ID, Revision: rule.Revision, ProjectID: rule.ProjectID, ProjectTitle: rule.ProjectTitle, Weekday: rule.Weekday, StartTime: rule.StartTime, EndTime: rule.EndTime, EndDayOffset: rule.EndDayOffset, Timezone: rule.Timezone, PausedReason: rule.PausedReason})
	}
	return out
}

func allocationPlansResponse(value allocationdao.PlanCollection) AllocationPlansResponse {
	out := AllocationPlansResponse{Revision: value.Revision, Items: make([]AllocationPlanItemResponse, 0, len(value.Items))}
	for _, item := range value.Items {
		out.Items = append(out.Items, AllocationPlanItemResponse{ProjectID: item.ProjectID, Title: item.Title, TargetMinutes: item.TargetMinutes})
	}
	return out
}

func allocationOccurrenceResponse(value allocationdao.Occurrence) AllocationOccurrenceResponse {
	return AllocationOccurrenceResponse{ID: value.ID, RuleID: value.RuleID, OccurrenceDate: value.OccurrenceDate.Format("2006-01-02"), SourceRevisionID: value.SourceRevisionID, ProjectID: value.ProjectID, ProjectTitle: value.ProjectTitle, Timezone: value.Timezone, StartAt: value.StartAt, EndAt: value.EndAt, Revision: value.Revision, RuleRevision: value.RuleRevision, IsException: value.IsException, CanUpdate: value.CanUpdate, CanDelete: value.CanDelete, CanUpdateFollowing: value.CanUpdateFollowing, CanDeleteFollowing: value.CanDeleteFollowing}
}

func allocationRowResponse(value allocationdao.Allocation) AllocationOccurrenceResponse {
	return AllocationOccurrenceResponse{ID: value.ID, RuleID: value.RuleID, OccurrenceDate: value.OccurrenceDate.Format("2006-01-02"), SourceRevisionID: value.SourceRevisionID, ProjectID: value.ProjectID, ProjectTitle: value.ProjectTitle, Timezone: value.Timezone, StartAt: value.StartAt, EndAt: value.EndAt, Revision: value.Revision, RuleRevision: value.RuleRevision, IsException: value.IsException, CanUpdate: value.CanUpdate, CanDelete: value.CanDelete, CanUpdateFollowing: value.CanUpdateFollowing, CanDeleteFollowing: value.CanDeleteFollowing}
}

func sleepExceptionResponse(value allocationdao.SleepException) SleepExceptionResponse {
	return SleepExceptionResponse{LocalDate: value.LocalDate.Format("2006-01-02"), WakeLocal: value.WakeLocal.Format("2006-01-02T15:04"), BedtimeLocal: value.BedtimeLocal.Format("2006-01-02T15:04"), Timezone: value.Timezone, Revision: value.Revision, HasException: value.HasException}
}
