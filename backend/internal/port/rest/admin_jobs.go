package rest

import (
	"errors"
	"net/http"

	jobusecase "github.com/Najah7/task2todaytodo/internal/application/admin/job/usecase"
)

// RunBackfill starts or returns the active schedule backfill operation.
//
//	@Summary	Run schedule backfill
//	@Tags		Admin
//	@Produce	json
//	@Security	BearerAuth
//	@Success	202	{object}	OperationResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/admin/jobs/backfill-schedules:run [post]
func (h *AdminHandler) RunBackfill(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActor(w, r, ErrSpecAdminRunFailed)
	if !ok {
		return
	}
	op, err := h.jobs.RunBackfill.Execute(r.Context(), actor)
	if err != nil {
		if errors.Is(err, jobusecase.ErrPermissionDenied) {
			WriteError(w, http.StatusForbidden, ErrSpecAdminRunFailed, ErrDetailForbidden)
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecAdminRunFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusAccepted, operationResponse(op))
}

// RunAllocationPrefill starts or returns the active allocation prefill operation.
//
//	@Summary	Materialize allocation occurrences
//	@Tags		Admin
//	@Produce	json
//	@Security	BearerAuth
//	@Success	202	{object}	OperationResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/admin/jobs/prefill-allocations:run [post]
func (h *AdminHandler) RunAllocationPrefill(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActor(w, r, ErrSpecAdminRunFailed)
	if !ok {
		return
	}
	op, err := h.jobs.RunPrefillAllocations.Execute(r.Context(), actor)
	if err != nil {
		if errors.Is(err, jobusecase.ErrPermissionDenied) {
			WriteError(w, http.StatusForbidden, ErrSpecAdminRunFailed, ErrDetailForbidden)
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecAdminRunFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusAccepted, operationResponse(op))
}
