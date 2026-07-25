package handlers

import (
	"net/http"

	domain "github.com/Najah7/task2schedule/internal/domain/task"
)

type TaskStatusHandler struct {
	service *domain.TaskStatusService
}

func NewTaskStatusHandler(service *domain.TaskStatusService) *TaskStatusHandler {
	return &TaskStatusHandler{
		service: service,
	}
}

type TaskStatusResponse struct {
	Status  string `json:"status"`
	Label   string `json:"label"`
	LabelJp string `json:"label_jp"`
}

type TaskStatusListResponse struct {
	Data []TaskStatusResponse `json:"data"`
}

// List godoc
//
//	@Summary		List task statuses
//	@Description	Returns all task status master data.
//	@Tags			Task Master Data
//	@Produce		json
//	@Success		200	{object}	TaskStatusListResponse
//	@Failure		500	{object}	ErrResponse	"Failed to list task statuses"
//	@Router			/tasks/statuses [get]
func (h *TaskStatusHandler) List(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.service.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecTaskStatusesListFailed)
		return
	}

	WriteJSON(w, http.StatusOK, TaskStatusListResponse{Data: newTaskStatusResponses(statuses)})
}

func newTaskStatusResponses(statuses []domain.TaskStatus) []TaskStatusResponse {
	responses := make([]TaskStatusResponse, 0, len(statuses))
	for _, status := range statuses {
		responses = append(responses, TaskStatusResponse{
			Status:  status.String(),
			Label:   status.Label,
			LabelJp: status.LabelJp,
		})
	}
	return responses
}
