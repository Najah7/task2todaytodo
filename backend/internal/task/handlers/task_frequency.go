package handlers

import (
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	"net/http"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
)

type TaskFrequencyHandler struct {
	service *taskusecase.TaskFrequencyService
}

func NewTaskFrequencyHandler(service *taskusecase.TaskFrequencyService) *TaskFrequencyHandler {
	return &TaskFrequencyHandler{
		service: service,
	}
}

type TaskFrequencyResponse struct {
	Frequency string `json:"frequency"`
	Label     string `json:"label"`
	LabelJp   string `json:"label_jp"`
}

type TaskFrequencyListResponse struct {
	Data []TaskFrequencyResponse `json:"data"`
}

// List godoc
//
//	@Summary		List task frequencies
//	@Description	Returns all task frequency master data.
//	@Tags			Tasks
//	@Produce		json
//	@Success		200	{object}	TaskFrequencyListResponse
//	@Failure		500	{object}	sharedhandlers.ErrResponse	"Failed to list task frequencies"
//	@Router			/tasks/frequencies [get]
func (h *TaskFrequencyHandler) List(w http.ResponseWriter, r *http.Request) {
	frequencies, err := h.service.List(r.Context())
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, sharedhandlers.ErrSpecTaskFrequenciesListFailed)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, TaskFrequencyListResponse{Data: newTaskFrequencyResponses(frequencies)})
}

func newTaskFrequencyResponses(frequencies []domain.TaskFrequency) []TaskFrequencyResponse {
	responses := make([]TaskFrequencyResponse, 0, len(frequencies))
	for _, frequency := range frequencies {
		responses = append(responses, TaskFrequencyResponse{
			Frequency: frequency.String(),
			Label:     frequency.Label,
			LabelJp:   frequency.LabelJp,
		})
	}
	return responses
}
