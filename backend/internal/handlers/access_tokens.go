package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/domain/auth"
	"github.com/Najah7/task2todaytodo/internal/utils"
)

const AccessTokenContextKey = "accessToken"

var (
	ErrUnauthorizedError = errors.New("unauthorized access")
)

type AccessTokenHandler struct {
	accessTokenService *auth.AccessTokenService
	userService        *auth.UserService
}

func NewAccessTokenHandler(accessTokenService *auth.AccessTokenService, userService *auth.UserService) *AccessTokenHandler {
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
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Invalid email or password"
//	@Failure		500		{object}	ErrResponse	"Failed to generate access token"
//	@Router			/access-tokens [post]
func (h *AccessTokenHandler) Generate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req AccessTokenGenerateRequest
	requestBody := json.NewDecoder(r.Body)
	if err := requestBody.Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecAccessTokensGenerateFailed, ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			WriteError(w, http.StatusUnauthorized, ErrSpecAccessTokensGenerateFailed, ErrDetailInvalidCredentials)
			return
		}

		WriteError(w, http.StatusInternalServerError, ErrSpecAccessTokensGenerateFailed, ErrDetailFailedUserLookup)
		return
	}

	t, err := h.accessTokenService.Generate(ctx, u.ID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecAccessTokensGenerateFailed)
		return
	}

	WriteJSON(w, http.StatusOK, AccessTokenResponse{
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
//	@Failure		401	{object}	ErrResponse		"Missing or invalid access token"
//	@Failure		500	{object}	ErrResponse		"Failed to revoke access token"
//	@Router			/access-token/current [delete]
func (h *AccessTokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token, ok := ctx.Value(AccessTokenContextKey).(string)
	if !ok || token == "" {
		WriteError(w, http.StatusUnauthorized, ErrSpecAccessTokensRevokeFailed, ErrDetailMissingOrInvalidAccessToken)
		return
	}

	err := h.accessTokenService.Revoke(ctx, token)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecAccessTokensRevokeFailed)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}
