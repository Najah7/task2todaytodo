package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

type LoginHandler struct {
	accessToken authusecase.AccessTokenUseCases
}

func NewLoginHandler(accessToken authusecase.AccessTokenUseCases) *LoginHandler {
	return &LoginHandler{accessToken: accessToken}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login authenticates a user and generates an access token.
//
//	@Summary		Log in
//	@Description	Authenticates a user and generates an access token.
//	@Tags			Access Tokens
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Login request"
//	@Success		200		{object}	AccessTokenResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Invalid email or password"
//	@Failure		500		{object}	ErrResponse	"Failed to generate access token"
//	@Router			/login [post]
func (h *LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAccessTokensGenerateFailed, ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.accessToken.Login.Execute(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, authusecase.ErrInvalidCredentials) {
			WriteError(w, http.StatusUnauthorized, ErrSpecAccessTokensGenerateFailed, ErrDetailInvalidCredentials)
			return
		}

		WriteError(w, http.StatusInternalServerError, ErrSpecAccessTokensGenerateFailed, ErrDetailFailedUserLookup)
		return
	}

	t, err := h.accessToken.Generate.Execute(ctx, u)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecAccessTokensGenerateFailed)
		return
	}

	WriteJSON(w, http.StatusOK, AccessTokenResponse{
		Token:     t.Token,
		ExpiresAt: time.Unix(t.ExpiresAt, 0).In(time.FixedZone("JST", 9*60*60)).Format("2006-01-02 15:04:05"),
	})
}
