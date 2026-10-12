package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

type EmailAuthHandler struct{ usecases authusecase.EmailAuthUseCases }

func NewEmailAuthHandler(usecases authusecase.EmailAuthUseCases) *EmailAuthHandler {
	return &EmailAuthHandler{usecases: usecases}
}

type EmailAddressRequest struct {
	Email string `json:"email"`
}

type EmailVerificationConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type PasswordResetConfirmRequest struct {
	Token                string `json:"token"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type AuthLinkResult struct {
	Message string `json:"message"`
}

// ResendVerification sends another verification link when the address is eligible.
//
//	@Summary	Resend verification email
//	@Tags		Users
//	@Accept		json
//	@Produce	json
//	@Param		request	body		EmailAddressRequest	true	"Email address"
//	@Success	202		{object}	GenericEmailResponse
//	@Failure	400		{object}	ErrResponse
//	@Router		/email-verification/resend [post]
func (h *EmailAuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	setAuthResponseHeaders(w)
	var req EmailAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if err := h.usecases.ResendVerification.Execute(r.Context(), req.Email); err != nil {
		if errors.Is(err, authusecase.ErrInvalidEmail) {
			WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, NewErrDetail("email", "invalid_email", "Invalid email address"))
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecUsersCreateFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusAccepted, GenericEmailResponse{Message: genericEmailMessage})
}

// ConfirmVerification consumes a verification link after matching registration credentials.
//
//	@Summary	Confirm email address
//	@Tags		Users
//	@Accept		json
//	@Produce	json
//	@Param		request	body		EmailVerificationConfirmRequest	true	"Verification token and registration password"
//	@Success	200		{object}	AuthLinkResult
//	@Failure	400		{object}	ErrResponse
//	@Router		/email-verification/confirm [post]
func (h *EmailAuthHandler) ConfirmVerification(w http.ResponseWriter, r *http.Request) {
	setAuthResponseHeaders(w)
	var req EmailVerificationConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if err := h.usecases.ConfirmVerification.Execute(r.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, authusecase.ErrInvalidAuthLink) {
			WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, invalidAuthLinkDetail())
			return
		}
		if status, detail := errToErrResponse(err, "password"); detail.Code != "" {
			WriteError(w, status, ErrSpecUsersCreateFailed, detail)
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecUsersCreateFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, AuthLinkResult{Message: "Email address verified. Please log in."})
}

// RequestPasswordReset sends a reset link when the address is eligible.
//
//	@Summary	Request password reset
//	@Tags		Users
//	@Accept		json
//	@Produce	json
//	@Param		request	body		EmailAddressRequest	true	"Email address"
//	@Success	202		{object}	GenericEmailResponse
//	@Failure	400		{object}	ErrResponse
//	@Router		/password-reset/request [post]
func (h *EmailAuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	setAuthResponseHeaders(w)
	var req EmailAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if err := h.usecases.RequestPasswordReset.Execute(r.Context(), req.Email); err != nil {
		if errors.Is(err, authusecase.ErrInvalidEmail) {
			WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, NewErrDetail("email", "invalid_email", "Invalid email address"))
			return
		}
		WriteError(w, http.StatusInternalServerError, ErrSpecUsersCreateFailed, ErrDetailInternalServerError)
		return
	}
	WriteJSON(w, http.StatusAccepted, GenericEmailResponse{Message: genericEmailMessage})
}

// ConfirmPasswordReset replaces the password and revokes existing sessions and links.
//
//	@Summary	Confirm password reset
//	@Tags		Users
//	@Accept		json
//	@Produce	json
//	@Param		request	body		PasswordResetConfirmRequest	true	"Reset token and new password"
//	@Success	200		{object}	AuthLinkResult
//	@Failure	400		{object}	ErrResponse
//	@Router		/password-reset/confirm [post]
func (h *EmailAuthHandler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	setAuthResponseHeaders(w)
	var req PasswordResetConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersUpdatePasswordFailed, ErrDetailInvalidRequestBody)
		return
	}
	if req.Password != req.PasswordConfirmation {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersUpdatePasswordFailed, NewErrDetail("password_confirmation", "password_mismatch", "Passwords do not match"))
		return
	}
	if err := h.usecases.ConfirmPasswordReset.Execute(r.Context(), req.Token, req.Password); err != nil {
		if errors.Is(err, authusecase.ErrInvalidAuthLink) {
			WriteError(w, http.StatusBadRequest, ErrSpecUsersUpdatePasswordFailed, invalidAuthLinkDetail())
			return
		}
		status, detail := errToErrResponse(err, "password")
		WriteError(w, status, ErrSpecUsersUpdatePasswordFailed, detail)
		return
	}
	WriteJSON(w, http.StatusOK, AuthLinkResult{Message: "Password reset. Please log in."})
}

func invalidAuthLinkDetail() ErrDetail {
	return NewErrDetail("token", "invalid_or_expired_auth_link", "Link is invalid or expired")
}

func setAuthResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
