package handlers

import (
	"net/http"
)

type HealthResponse struct {
	Status string `json:"status"`
}

// HealthCheckHandler godoc
//
//	@Summary		Check API health
//	@Description	Returns the API health status.
//	@Tags			Monitor
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Failure		500	{object}	ErrResponse	"Failed to marshal response"
//	@Router			/monitor/health [get]
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}
