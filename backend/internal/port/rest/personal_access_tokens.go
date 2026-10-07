package rest

import (
	"errors"
	"net/http"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

var (
	ErrUnauthorizedError = errors.New("unauthorized access")
)

type PersonalAccessTokenHandler struct {
	personalAccessToken authusecase.PersonalAccessTokenUseCases
}

func NewPersonalAccessTokenHandler(personalAccessToken authusecase.PersonalAccessTokenUseCases) *PersonalAccessTokenHandler {
	return &PersonalAccessTokenHandler{
		personalAccessToken: personalAccessToken,
	}
}

type PersonalAccessTokenResponse struct {
	PersonalAccessToken string `json:"personal_access_token"`
	ExpiresAt           string `json:"expires_at"`
}

// Revoke godoc
//
//	@Summary		Revoke personal access token
//	@Description	Revokes the current access token from the Authorization header.
//	@Tags			Personal Access Tokens
//	@Security		BearerAuth
//	@Success		200	{object}	MessageResponse	"OK"
//	@Failure		401	{object}	ErrResponse		"Missing or invalid access token"
//	@Failure		500	{object}	ErrResponse		"Failed to revoke access token"
//	@Router			/personal-access-token:revoke [delete]
func (h *PersonalAccessTokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token, ok := ctx.Value(PersonalAccessTokenContextKey).(string)
	if !ok || token == "" {
		WriteError(w, http.StatusUnauthorized, ErrSpecPersonalAccessTokensRevokeFailed, ErrDetailMissingOrInvalidPersonalAccessToken)
		return
	}

	err := h.personalAccessToken.Revoke.Execute(ctx, token)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecPersonalAccessTokensRevokeFailed)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}
