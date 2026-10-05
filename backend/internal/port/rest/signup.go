package rest

import (
	"encoding/json"
	"net/http"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
)

type SignupHandler struct {
	user authusecase.UserUseCases
	ID   shared.ID
}

func NewSignupHandler(user authusecase.UserUseCases, ID shared.ID) *SignupHandler {
	return &SignupHandler{user: user, ID: ID}
}

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	// TODO: AuthCode string `json:"auth_code"` with invitation email.
}

// Signup creates a user with an email address and password.
//
//	@Summary		Sign up
//	@Description	Creates a user with an email address and password.
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		SignupRequest	true	"Signup request"
//	@Success		201		{object}	UserResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		500		{object}	ErrResponse	"Failed to create user"
//	@Router			/signup [post]
func (h *SignupHandler) Signup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.user.Create.Execute(ctx, h.ID.Generate(), req.Email, req.Password)
	if err != nil {
		status, detail := errToErrResponse(err, "password")
		WriteError(w, status, ErrSpecUsersCreateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusCreated, newUserResponse(u))
}
