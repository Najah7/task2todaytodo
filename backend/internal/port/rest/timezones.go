package rest

import (
	"net/http"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

type TimezoneHandler struct{ timezones authusecase.TimezoneUseCases }

func NewTimezoneHandler(timezones authusecase.TimezoneUseCases) *TimezoneHandler {
	return &TimezoneHandler{timezones: timezones}
}

type TimezoneListResponse struct {
	Items []string `json:"items"`
}

// List returns supported timezone identifiers.
//
//	@Summary	List timezones
//	@Tags		Authorization
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	TimezoneListResponse
//	@Failure	401	{object}	ErrResponse
//	@Failure	500	{object}	ErrResponse
//	@Router		/timezones [get]
func (h *TimezoneHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := r.Context().Value(UserIDContextKey).(string); !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecTimezonesListFailed, ErrDetailUnauthorized)
		return
	}
	items, err := h.timezones.List.Execute(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecTimezonesListFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, TimezoneListResponse{Items: items})
}
