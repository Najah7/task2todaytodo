package handlers

import (
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	"net/http"

	domain "github.com/Najah7/task2todaytodo/internal/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/task/usecase"
)

type ProjectTypeHandler struct {
	service *taskusecase.ProjectTypeService
}

func NewProjectTypeHandler(service *taskusecase.ProjectTypeService) *ProjectTypeHandler {
	return &ProjectTypeHandler{
		service: service,
	}
}

type ProjectTypeResponse struct {
	Type    string `json:"type"`
	Label   string `json:"label"`
	LabelJp string `json:"label_jp"`
}

type ProjectTypeListResponse struct {
	Data []ProjectTypeResponse `json:"data"`
}

// List godoc
//
//	@Summary		List project types
//	@Description	Returns all project type master data.
//	@Tags			Projects
//	@Produce		json
//	@Success		200	{object}	ProjectTypeListResponse
//	@Failure		500	{object}	sharedhandlers.ErrResponse	"Failed to list project types"
//	@Router			/projects/types [get]
func (h *ProjectTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	projectTypes, err := h.service.List(r.Context())
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, sharedhandlers.ErrSpecProjectTypesListFailed)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, ProjectTypeListResponse{Data: newProjectTypeResponses(projectTypes)})
}

func newProjectTypeResponses(projectTypes []domain.ProjectType) []ProjectTypeResponse {
	responses := make([]ProjectTypeResponse, 0, len(projectTypes))
	for _, projectType := range projectTypes {
		responses = append(responses, ProjectTypeResponse{
			Type:    projectType.String(),
			Label:   projectType.Label,
			LabelJp: projectType.LabelJp,
		})
	}
	return responses
}
