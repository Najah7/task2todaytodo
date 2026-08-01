package handlers

import (
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	"net/http"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
)

type TaskPriorityHandler struct {
	service *taskusecase.TaskPriorityService
}

func NewTaskPriorityHandler(service *taskusecase.TaskPriorityService) *TaskPriorityHandler {
	return &TaskPriorityHandler{
		service: service,
	}
}

type TaskPriorityResponse struct {
	Priority string `json:"priority"`
	Label    string `json:"label"`
	LabelJp  string `json:"label_jp"`
	Weight   int    `json:"weight"`
}

type TaskPriorityListResponse struct {
	Data []TaskPriorityResponse `json:"data"`
}

// List godoc
//
//	@Summary		List task priorities
//	@Description	Returns all task priority master data.
//	@Tags			Tasks
//	@Produce		json
//	@Success		200	{object}	TaskPriorityListResponse
//	@Failure		500	{object}	sharedhandlers.ErrResponse	"Failed to list task priorities"
//	@Router			/tasks/priorities [get]
func (h *TaskPriorityHandler) List(w http.ResponseWriter, r *http.Request) {
	priorities, err := h.service.List(r.Context())
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, sharedhandlers.ErrSpecTaskPrioritiesListFailed)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, TaskPriorityListResponse{Data: newTaskPriorityResponses(priorities)})
}

func newTaskPriorityResponses(priorities []domain.TaskPriority) []TaskPriorityResponse {
	responses := make([]TaskPriorityResponse, 0, len(priorities))
	for _, priority := range priorities {
		responses = append(responses, TaskPriorityResponse{
			Priority: priority.String(),
			Label:    priority.Label,
			LabelJp:  priority.LabelJp,
			Weight:   priority.Weight,
		})
	}
	return responses
}
