package rest

import (
	"errors"
	"net/http"
	"time"

	admindao "github.com/Najah7/task2todaytodo/internal/application/admin/dao"
	adminusecase "github.com/Najah7/task2todaytodo/internal/application/admin/usecase"
	restpagination "github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/go-chi/chi/v5"
)

type AdminHandler struct {
	usecases   adminusecase.UseCases
	pageTokens *restpagination.Codec
}

func NewAdminHandler(usecases adminusecase.UseCases, pageTokens *restpagination.Codec) *AdminHandler {
	return &AdminHandler{usecases: usecases, pageTokens: pageTokens}
}

type OperationMetadata struct {
	State               string     `json:"state"`
	CreatedAt           time.Time  `json:"created_at"`
	StartedAt           time.Time  `json:"started_at"`
	FinishedAt          *time.Time `json:"finished_at,omitempty"`
	Cutoff              time.Time  `json:"cutoff"`
	ProcessedBlueprints int64      `json:"processed_blueprints"`
	GeneratedSchedules  int64      `json:"generated_schedules"`
}

type OperationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type OperationResponse struct {
	Name     string            `json:"name"`
	Done     bool              `json:"done"`
	Metadata OperationMetadata `json:"metadata"`
	Response map[string]int64  `json:"response,omitempty"`
	Error    *OperationError   `json:"error,omitempty"`
}
type AdminOperationListResponse struct {
	Items         []OperationResponse `json:"items"`
	NextPageToken string              `json:"next_page_token"`
}

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
	op, err := h.usecases.RunBackfill.Execute(r.Context(), actor)
	if err != nil {
		if errors.Is(err, adminusecase.ErrPermissionDenied) {
			WriteError(w, http.StatusForbidden, ErrSpecAdminRunFailed, ErrDetailForbidden)
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecAdminRunFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusAccepted, operationResponse(op))
}

// ListOperations lists recent administrative jobs.
//
//	@Summary	List admin operations
//	@Tags		Admin
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page_size	query		int		false	"Page size"
//	@Param		page_token	query		string	false	"Next page token"
//	@Success	200			{object}	AdminOperationListResponse
//	@Failure	401			{object}	ErrResponse
//	@Failure	403			{object}	ErrResponse
//	@Router		/admin/operations [get]
func (h *AdminHandler) ListOperations(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActor(w, r, ErrSpecAdminListFailed)
	if !ok {
		return
	}
	parsed, err := parseListRequest(r, h.pageTokens, actor, "admin_operations", "", "created_at_desc_id_desc", AdminOperationListResponse{})
	if err != nil {
		writeListError(w, ErrSpecAdminListFailed, err)
		return
	}
	cursor := ""
	if parsed.Anchor != nil {
		cursor = parsed.Anchor.ID
	}
	page, err := h.usecases.ListOperations.Execute(r.Context(), actor, cursor, parsed.Size)
	if err != nil {
		if errors.Is(err, adminusecase.ErrPermissionDenied) {
			WriteError(w, http.StatusForbidden, ErrSpecAdminListFailed, ErrDetailForbidden)
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecAdminListFailed, ErrDetailInternalServerError)
		return
	}
	items := make([]OperationResponse, 0, len(page.Items))
	anchors := make([]listAnchor, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, operationResponse(item))
		anchors = append(anchors, listAnchor{ID: item.ID})
	}
	writeListResponse(w, items, anchors, page.HasMore, parsed, h.pageTokens, ErrSpecAdminListFailed)
}

// GetOperation returns the current state of one administrative operation.
//
//	@Summary	Get admin operation
//	@Tags		Admin
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Operation ID"
//	@Success	200	{object}	OperationResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	403	{object}	ErrResponse
//	@Failure	404	{object}	ErrResponse
//	@Router		/admin/operations/{id} [get]
func (h *AdminHandler) GetOperation(w http.ResponseWriter, r *http.Request) {
	actor, ok := adminActor(w, r, ErrSpecAdminGetFailed)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if len(id) != 26 {
		WriteError(w, http.StatusBadRequest, ErrSpecAdminGetFailed, ErrDetailInvalidRequestBody)
		return
	}
	op, err := h.usecases.GetOperation.Execute(r.Context(), actor, id)
	if err != nil {
		if errors.Is(err, adminusecase.ErrPermissionDenied) {
			WriteError(w, http.StatusForbidden, ErrSpecAdminGetFailed, ErrDetailForbidden)
			return
		}
		if errors.Is(err, adminusecase.ErrOperationNotFound) {
			WriteError(w, http.StatusNotFound, ErrSpecAdminGetFailed, ErrDetailNotFound)
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecAdminGetFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, operationResponse(op))
}

func adminActor(w http.ResponseWriter, r *http.Request, spec ErrSpec) (string, bool) {
	actor, ok := r.Context().Value(UserIDContextKey).(string)
	if !ok || actor == "" {
		WriteError(w, http.StatusUnauthorized, spec, ErrDetailUnauthorized)
		return "", false
	}
	return actor, true
}

func operationResponse(op admindao.Operation) OperationResponse {
	out := OperationResponse{Name: op.Name, Done: op.Done(), Metadata: OperationMetadata{
		State: op.State, CreatedAt: op.CreatedAt, StartedAt: op.StartedAt, FinishedAt: op.FinishedAt,
		Cutoff: op.Cutoff, ProcessedBlueprints: op.ProcessedBlueprints, GeneratedSchedules: op.GeneratedSchedules,
	}}
	if op.Done() && op.State == "succeeded" {
		out.Response = op.Response
	}
	if op.Done() && op.State == "failed" {
		out.Error = &OperationError{Code: op.ErrorCode, Message: op.ErrorMessage}
	}
	return out
}
