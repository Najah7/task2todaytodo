package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	authdomain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
)

type ProjectInvitationAcceptanceHandler struct {
	accept *projectusecase.AcceptProjectInvitationUseCase
	newID  func() string
}

func NewProjectInvitationAcceptanceHandler(accept *projectusecase.AcceptProjectInvitationUseCase, newID func() string) *ProjectInvitationAcceptanceHandler {
	return &ProjectInvitationAcceptanceHandler{accept: accept, newID: newID}
}

type ProjectInvitationAcceptanceRequest struct {
	Token    string `json:"token"`
	Password string `json:"password,omitempty"`
}

// AcceptProjectInvitation consumes an invitation after an explicit user action.
//
//	@Summary	Accept project invitation
//	@Tags		Project Invitations
//	@Accept		json
//	@Produce	json
//	@Param		request	body		ProjectInvitationAcceptanceRequest	true	"Invitation token and, for a new or unverified account, password"
//	@Success	200		{object}	PersonalAccessTokenResponse			"Account created or verified and signed in"
//	@Success	204		"Verified recipient joined project"
//	@Failure	400		{object}	ErrResponse
//	@Failure	401		{object}	ErrResponse
//	@Failure	403		{object}	ErrResponse
//	@Failure	409		{object}	ErrResponse
//	@Failure	500		{object}	ErrResponse
//	@Router		/invitations/accept [post]
func (h *ProjectInvitationAcceptanceHandler) Accept(w http.ResponseWriter, r *http.Request) {
	setAuthResponseHeaders(w)
	var request ProjectInvitationAcceptanceRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, ErrDetailInvalidRequestBody)
		return
	}
	if request.Token == "" {
		WriteError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, NewErrDetail("token", "required", "Invitation token is required"))
		return
	}
	userID, _ := projectUserID(r.Context())
	newID := ""
	if h.newID != nil {
		newID = h.newID()
	}
	result, err := h.accept.Execute(r.Context(), request.Token, request.Password, string(userID), newID)
	if err != nil {
		switch {
		case errors.Is(err, projectusecase.ErrInvalidProjectInvitation):
			WriteError(w, http.StatusBadRequest, ErrSpecProjectsUpdateFailed, NewErrDetail("token", "invalid_or_expired_invitation", "Invitation is invalid, expired, or no longer available"))
		case errors.Is(err, projectusecase.ErrInvitationRecipientAuthenticationRequired):
			WriteError(w, http.StatusForbidden, ErrSpecProjectsUpdateFailed, NewErrDetail("email", "account_mismatch", "Sign in with the account invited by this project"))
		case errors.Is(err, projectusecase.ErrProjectInvitationMemberExists):
			WriteError(w, http.StatusConflict, ErrSpecProjectsUpdateFailed, NewErrDetail("project_id", "already_member", "This account is already a project member"))
		case errors.Is(err, authusecase.ErrUserEmailAlreadyExists):
			WriteError(w, http.StatusConflict, ErrSpecProjectsUpdateFailed, NewErrDetail("email", "account_exists", "An account with this email already exists. Sign in with that account and accept again."))
		case passwordValidationError(err):
			status, detail := errToErrResponse(err, "password")
			WriteError(w, status, ErrSpecProjectsUpdateFailed, detail)
		default:
			WriteError(w, http.StatusInternalServerError, ErrSpecProjectsUpdateFailed, ErrDetailInternalServerError)
		}
		return
	}
	if !result.NeedsAutoLogin {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	WriteJSON(w, http.StatusOK, PersonalAccessTokenResponse{
		PersonalAccessToken: result.PersonalAccessToken,
		ExpiresAt:           time.Unix(result.ExpiresAt, 0).In(time.FixedZone("JST", 9*60*60)).Format("2006-01-02 15:04:05"),
	})
}

func passwordValidationError(err error) bool {
	return errors.Is(err, authdomain.ErrPasswordEmpty) ||
		errors.Is(err, authdomain.ErrPasswordTooShort) ||
		errors.Is(err, authdomain.ErrPasswordMissingLowercase) ||
		errors.Is(err, authdomain.ErrPasswordMissingUppercase) ||
		errors.Is(err, authdomain.ErrPasswordMissingDigit) ||
		errors.Is(err, authdomain.ErrPasswordMissingSpecial)
}
