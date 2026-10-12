package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

type SignupHandler struct {
	register *authusecase.RegisterUserUseCase
	issueID  func() string
}

func NewSignupHandler(register *authusecase.RegisterUserUseCase, issueID func() string) *SignupHandler {
	return &SignupHandler{register: register, issueID: issueID}
}

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type GenericEmailResponse struct {
	Message string `json:"message"`
}

const genericEmailMessage = "If the address can receive email, instructions have been sent."

// Signup registers an email and sends confirmation when the address is eligible.
//
//	@Summary		Sign up
//	@Description	Registers an unverified user and sends a confirmation email when eligible.
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		SignupRequest	true	"Signup request"
//	@Success		202		{object}	GenericEmailResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request"
//	@Router			/signup [post]
func (h *SignupHandler) Signup(w http.ResponseWriter, r *http.Request) {
	setAuthResponseHeaders(w)
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if err := h.register.Execute(r.Context(), h.issueID(), req.Email, req.Password); err != nil {
		if errors.Is(err, authusecase.ErrInvalidEmail) {
			WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, NewErrDetail("email", "invalid_email", "Invalid email address"))
			return
		}
		status, detail := errToErrResponse(err, "password")
		WriteError(w, status, ErrSpecUsersCreateFailed, detail)
		return
	}
	WriteJSON(w, http.StatusAccepted, GenericEmailResponse{Message: genericEmailMessage})
}
