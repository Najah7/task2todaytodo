package rest

import (
	"net/http"

	jobusecase "github.com/Najah7/task2todaytodo/internal/application/admin/job/usecase"
	operationusecase "github.com/Najah7/task2todaytodo/internal/application/admin/operation/usecase"
	restpagination "github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type AdminHandler struct {
	jobs       jobusecase.UseCases
	operations operationusecase.UseCases
	pageTokens *restpagination.Codec
}

func NewAdminHandler(jobs jobusecase.UseCases, operations operationusecase.UseCases, pageTokens *restpagination.Codec) *AdminHandler {
	return &AdminHandler{jobs: jobs, operations: operations, pageTokens: pageTokens}
}

func adminActor(w http.ResponseWriter, r *http.Request, spec ErrSpec) (string, bool) {
	actor, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || actor == "" {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", false
	}
	return actor, true
}
