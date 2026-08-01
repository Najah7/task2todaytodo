package handlers

import (
	"encoding/json"
	"errors"
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
	"net/http"

	authusecase "github.com/Najah7/task2todaytodo/internal/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/utils"
)

var (
	ErrUnauthorizedError = errors.New("unauthorized access")
)

type AccessTokenHandler struct {
	accessTokenService *authusecase.AccessTokenService
	userService        *authusecase.UserService
}

func NewAccessTokenHandler(accessTokenService *authusecase.AccessTokenService, userService *authusecase.UserService) *AccessTokenHandler {
	return &AccessTokenHandler{
		accessTokenService: accessTokenService,
		userService:        userService,
	}
}

type AccessTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type AccessTokenGenerateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Generate godoc
//
//	@Summary		Generate access token
//	@Description	Generates a new access token for a user.
//	@Tags			Access Tokens
//	@Accept			json
//	@Produce		json
//	@Param			request	body		AccessTokenGenerateRequest	true	"Access token generate request"
//	@Success		200		{object}	AccessTokenResponse
//	@Failure		400		{object}	sharedhandlers.ErrResponse	"Invalid request body"
//	@Failure		401		{object}	sharedhandlers.ErrResponse	"Invalid email or password"
//	@Failure		500		{object}	sharedhandlers.ErrResponse	"Failed to generate access token"
//	@Router			/access-tokens [post]
func (h *AccessTokenHandler) Generate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req AccessTokenGenerateRequest
	requestBody := json.NewDecoder(r.Body)
	if err := requestBody.Decode(&req); err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, sharedhandlers.ErrSpecAccessTokensGenerateFailed, sharedhandlers.ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, authusecase.ErrInvalidCredentials) {
			sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecAccessTokensGenerateFailed, sharedhandlers.ErrDetailInvalidCredentials)
			return
		}

		sharedhandlers.WriteError(w, http.StatusInternalServerError, sharedhandlers.ErrSpecAccessTokensGenerateFailed, sharedhandlers.ErrDetailFailedUserLookup)
		return
	}

	t, err := h.accessTokenService.Generate(ctx, u.ID)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, sharedhandlers.ErrSpecAccessTokensGenerateFailed)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, AccessTokenResponse{
		Token:     t.Token,
		ExpiresAt: utils.UnixToJST(t.ExpiresAt),
	})
}

// Revoke godoc
//
//	@Summary		Revoke access token
//	@Description	Revokes the current access token from the Authorization header.
//	@Tags			Access Tokens
//	@Security		BearerAuth
//	@Success		200	{object}	MessageResponse	"OK"
//	@Failure		401	{object}	sharedhandlers.ErrResponse		"Missing or invalid access token"
//	@Failure		500	{object}	sharedhandlers.ErrResponse		"Failed to revoke access token"
//	@Router			/access-token/current [delete]
func (h *AccessTokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token, ok := ctx.Value(sharedhandlers.AccessTokenContextKey).(string)
	if !ok || token == "" {
		sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecAccessTokensRevokeFailed, sharedhandlers.ErrDetailMissingOrInvalidAccessToken)
		return
	}

	err := h.accessTokenService.Revoke(ctx, token)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, sharedhandlers.ErrSpecAccessTokensRevokeFailed)
		return
	}

	sharedhandlers.WriteMessage(w, http.StatusOK, "OK")
}
