package rest

import (
	"net/http"
	"strconv"

	metricdao "github.com/Najah7/task2todaytodo/internal/application/metric/dao"
	metricusecase "github.com/Najah7/task2todaytodo/internal/application/metric/usecase"
)

var (
	metricsTodoCompletionFailure = NewFailureErrSpec("metrics", "get_todo_completion", "Failed to get Todo completion metrics")
	metricsOverdueWorkFailure    = NewFailureErrSpec("metrics", "get_overdue_work", "Failed to get overdue work metrics")
)

type MetricsHandler struct {
	todoCompletion *metricusecase.TodoCompletionUseCase
	overdueWork    *metricusecase.OverdueWorkUseCase
}

func NewMetricsHandler(todoCompletion *metricusecase.TodoCompletionUseCase, overdueWork *metricusecase.OverdueWorkUseCase) *MetricsHandler {
	return &MetricsHandler{todoCompletion: todoCompletion, overdueWork: overdueWork}
}

type TodoCompletionDayResponse struct {
	Date            string   `json:"date"`
	PlannedItems    int32    `json:"planned_items"`
	CompletedItems  int32    `json:"completed_items"`
	CompletionRatio *float64 `json:"completion_ratio" extensions:"x-nullable"`
}

type TodoCompletionMetricsResponse struct {
	Timezone          string                      `json:"timezone"`
	StartDate         string                      `json:"start_date"`
	EndDate           string                      `json:"end_date"`
	TotalPlannedItems int32                       `json:"total_planned_items"`
	CompletedItems    int32                       `json:"completed_items"`
	CompletionRatio   *float64                    `json:"completion_ratio" extensions:"x-nullable"`
	Days              []TodoCompletionDayResponse `json:"days"`
}

type OverdueProjectResponse struct {
	ProjectID    *string `json:"project_id" extensions:"x-nullable"`
	ProjectTitle *string `json:"project_title" extensions:"x-nullable"`
	Count        int32   `json:"count"`
}

type OverdueWorkMetricsResponse struct {
	Timezone          string                   `json:"timezone"`
	AsOfDate          string                   `json:"as_of_date"`
	TotalOverdueCount int32                    `json:"total_overdue_count"`
	Projects          []OverdueProjectResponse `json:"projects"`
	OtherCount        int32                    `json:"other_count"`
}

// TodoCompletion returns completion counts for each local calendar day in selected period.
//
//	@Summary	Get Todo completion metrics
//	@ID		GetTodoCompletionMetrics
//	@Tags		Metrics
//	@Produce	json
//	@Security	BearerAuth
//	@Param		days	query	int	false	"Calendar days ending today (7 or 30; default 7)" Enums(7, 30) default(7)
//	@Success	200	{object}	TodoCompletionMetricsResponse
//	@Failure	400	{object}	ErrResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/metrics/todo-completion [get]
func (h *MetricsHandler) TodoCompletion(w http.ResponseWriter, r *http.Request) {
	actorID, ok := metricsActorID(w, r, metricsTodoCompletionFailure)
	if !ok {
		return
	}
	days := 7
	if values, exists := r.URL.Query()["days"]; exists {
		if len(values) != 1 || (values[0] != "7" && values[0] != "30") {
			writeInvalidMetricsDays(w, metricsTodoCompletionFailure)
			return
		}
		parsed, err := strconv.Atoi(values[0])
		if err != nil {
			writeInvalidMetricsDays(w, metricsTodoCompletionFailure)
			return
		}
		days = parsed
	}
	metrics, err := h.todoCompletion.Execute(r.Context(), actorID, days)
	if err != nil {
		if metricusecase.ErrInvalidDays == err {
			writeInvalidMetricsDays(w, metricsTodoCompletionFailure)
			return
		}
		WriteError(w, http.StatusInternalServerError, metricsTodoCompletionFailure, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, todoCompletionMetricsResponse(metrics))
}

// OverdueWork returns current overdue, non-done work grouped by project.
//
//	@Summary	Get overdue work metrics
//	@ID		GetOverdueWorkMetrics
//	@Tags		Metrics
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	OverdueWorkMetricsResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/metrics/overdue-work [get]
func (h *MetricsHandler) OverdueWork(w http.ResponseWriter, r *http.Request) {
	actorID, ok := metricsActorID(w, r, metricsOverdueWorkFailure)
	if !ok {
		return
	}
	metrics, err := h.overdueWork.Execute(r.Context(), actorID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, metricsOverdueWorkFailure, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, overdueWorkMetricsResponse(metrics))
}

func metricsActorID(w http.ResponseWriter, r *http.Request, spec ErrSpec) (string, bool) {
	actorID, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || actorID == "" {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", false
	}
	return actorID, true
}

func writeInvalidMetricsDays(w http.ResponseWriter, spec ErrSpec) {
	WriteError(w, http.StatusBadRequest, spec, NewErrDetail("days", "invalid_value", "Days must be 7 or 30"))
}

func todoCompletionMetricsResponse(metrics metricdao.TodoCompletion) TodoCompletionMetricsResponse {
	response := TodoCompletionMetricsResponse{
		Timezone: metrics.Timezone, StartDate: metrics.StartDate, EndDate: metrics.EndDate,
		TotalPlannedItems: metrics.TotalPlannedItems, CompletedItems: metrics.CompletedItems,
		CompletionRatio: metrics.CompletionRatio,
		Days:            make([]TodoCompletionDayResponse, 0, len(metrics.Days)),
	}
	for _, day := range metrics.Days {
		response.Days = append(response.Days, TodoCompletionDayResponse{
			Date: day.Date, PlannedItems: day.PlannedItems, CompletedItems: day.CompletedItems,
			CompletionRatio: day.CompletionRatio,
		})
	}
	return response
}

func overdueWorkMetricsResponse(metrics metricdao.OverdueWork) OverdueWorkMetricsResponse {
	response := OverdueWorkMetricsResponse{
		Timezone: metrics.Timezone, AsOfDate: metrics.AsOfDate, TotalOverdueCount: metrics.TotalCount,
		Projects: make([]OverdueProjectResponse, 0, len(metrics.Projects)), OtherCount: metrics.OtherCount,
	}
	for _, project := range metrics.Projects {
		response.Projects = append(response.Projects, OverdueProjectResponse{
			ProjectID: project.ProjectID, ProjectTitle: project.ProjectTitle, Count: project.Count,
		})
	}
	return response
}
