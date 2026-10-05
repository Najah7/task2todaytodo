package rest

import (
	"errors"
	"net/http"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

var (
	ErrUnauthorizedError = errors.New("unauthorized access")
)

type AccessTokenHandler struct {
	accessToken authusecase.AccessTokenUseCases
}

func NewAccessTokenHandler(accessToken authusecase.AccessTokenUseCases) *AccessTokenHandler {
	return &AccessTokenHandler{
		accessToken: accessToken,
	}
}

type AccessTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// Revoke godoc
//
//	@Summary		Revoke access token
//	@Description	Revokes the current access token from the Authorization header.
//	@Tags			Access Tokens
//	@Security		BearerAuth
//	@Success		200	{object}	MessageResponse	"OK"
//	@Failure		401	{object}	ErrResponse		"Missing or invalid access token"
//	@Failure		500	{object}	ErrResponse		"Failed to revoke access token"
//	@Router			/access-token:revoke [delete]
func (h *AccessTokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token, ok := ctx.Value(AccessTokenContextKey).(string)
	if !ok || token == "" {
		WriteError(w, http.StatusUnauthorized, ErrSpecAccessTokensRevokeFailed, ErrDetailMissingOrInvalidAccessToken)
		return
	}

	err := h.accessToken.Revoke.Execute(ctx, token)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecAccessTokensRevokeFailed)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}
