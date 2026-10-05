package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	auth "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

type UserHandler struct {
	user authusecase.UserUseCases
}

func NewUserHandler(user authusecase.UserUseCases) *UserHandler {
	return &UserHandler{user: user}
}

type UserResponse struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name,omitempty"`
	Email    string `json:"email"`
	Timezone string `json:"timezone"`
}

type UserListResponse struct {
	users []UserResponse
}

func newUserResponse(u dao.User) UserResponse {
	return UserResponse{
		UserID:   u.ID,
		UserName: auth.NewUserName(u.FirstName, u.LastName).FullName(),
		Email:    u.Email,
		Timezone: u.Timezone,
	}
}

type UserTimezoneUpdateRequest struct {
	Timezone string `json:"timezone"`
}

// UpdateTimezone updates the authenticated user's IANA timezone.
//
//	@Summary		Update current user timezone
//	@Description	Updates the authenticated user's IANA timezone.
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UserTimezoneUpdateRequest	true	"User timezone update request"
//	@Success		200		{object}	UserResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body or timezone"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		500		{object}	ErrResponse	"Failed to update user timezone"
//	@Router			/users/me/timezone [patch]
func (h UserHandler) UpdateTimezone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(UserIDContextKey).(string)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecUsersUpdateTimezoneFailed, ErrDetailUnauthorized)
		return
	}

	var req UserTimezoneUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersUpdateTimezoneFailed, ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.user.UpdateTimezone.Execute(ctx, auth.UserID(userID), req.Timezone)
	if err != nil {
		status, detail := errToErrResponse(err, "")
		WriteError(w, status, ErrSpecUsersUpdateTimezoneFailed, detail)
		return
	}
	WriteJSON(w, http.StatusOK, newUserResponse(u))
}

// Get godoc
//
//	@Summary		Get current user
//	@Description	Returns the authenticated user's profile.
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UserResponse
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		500	{object}	ErrResponse	"Failed to get user"
//	@Router			/users/me [get]
func (h UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(UserIDContextKey).(string)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecUsersGetFailed, ErrDetailUnauthorized)
		return
	}

	u, err := h.user.Get.Execute(ctx, auth.UserID(userID))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecUsersGetFailed)
		return
	}

	WriteJSON(w, http.StatusOK, newUserResponse(u))
}

type UserInfoUpdateRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// UpdateBasicInfo godoc
//
//	@Summary		Update current user basic info
//	@Description	Updates the authenticated user's first and last name.
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			request	body		UserInfoUpdateRequest	true	"User basic info update request"
//	@Success		200		{object}	UserResponse
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		401		{object}	ErrResponse	"Unauthorized"
//	@Failure		500		{object}	ErrResponse	"Failed to update user"
//	@Router			/users/me [patch]
func (h UserHandler) UpdateBasicInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(UserIDContextKey).(string)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecUsersUpdateBasicInfoFailed, ErrDetailUnauthorized)
		return
	}

	var req UserInfoUpdateRequest
	requestBody := json.NewDecoder(r.Body)
	err := requestBody.Decode(&req)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersUpdateBasicInfoFailed, ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.user.UpdateName.Execute(ctx, auth.UserID(userID), req.FirstName, req.LastName)
	if err != nil {
		status, detail := errToErrResponse(err, "")
		WriteError(w, status, ErrSpecUsersUpdateBasicInfoFailed, detail)
		return
	}

	WriteJSON(w, http.StatusOK, newUserResponse(u))
}

type UserPasswordUpdateRequest struct {
	NewPassword string `json:"new_password"`
}

// UpdatePassword godoc
//
//	@Summary		Update current user password
//	@Description	Updates the authenticated user's password.
//	@Tags			Users
//	@Accept			json
//	@Security		BearerAuth
//	@Param			request	body		UserPasswordUpdateRequest	true	"User password update request"
//	@Success		200		{object}	MessageResponse				"OK"
//	@Failure		400		{object}	ErrResponse					"Invalid request body"
//	@Failure		401		{object}	ErrResponse					"Unauthorized"
//	@Failure		500		{object}	ErrResponse					"Failed to update password"
//	@Router			/users/me/password [patch]
func (h UserHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(UserIDContextKey).(string)
	if !ok {
		WriteError(w, http.StatusUnauthorized, ErrSpecUsersUpdatePasswordFailed, ErrDetailUnauthorized)
		return
	}

	var req UserPasswordUpdateRequest
	requestBody := json.NewDecoder(r.Body)
	err := requestBody.Decode(&req)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersUpdatePasswordFailed, ErrDetailInvalidRequestBody)
		return
	}

	err = h.user.UpdatePassword.Execute(ctx, auth.UserID(userID), req.NewPassword)
	if err != nil {
		status, detail := errToErrResponse(err, "new_password")
		WriteError(w, status, ErrSpecUsersUpdatePasswordFailed, detail)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}

func errToErrResponse(err error, passwordField string) (int, ErrDetail) {
	switch {
	case errors.Is(err, authusecase.ErrUserEmailAlreadyExists):
		return http.StatusConflict, NewErrDetail("email", "email_already_exists", "Email is already registered")
	case IsUniqueConstraint(err, "users_email_key"):
		return http.StatusConflict, NewErrDetail("email", "email_already_exists", "Email is already registered")
	case errors.Is(err, authusecase.ErrUserIDAlreadyExists):
		return http.StatusConflict, NewErrDetail("user_id", "user_id_already_exists", "User ID is already registered")
	case IsUniqueConstraint(err, "users_pkey"):
		return http.StatusConflict, NewErrDetail("user_id", "user_id_already_exists", "User ID is already registered")
	case errors.Is(err, auth.ErrEmailEmpty), errors.Is(err, auth.ErrInvalidEmailFormat):
		return http.StatusBadRequest, NewErrDetail("email", "invalid_email", "Email must be a valid email address")
	case errors.Is(err, auth.ErrPasswordEmpty),
		errors.Is(err, auth.ErrPasswordTooShort),
		errors.Is(err, auth.ErrPasswordMissingLowercase),
		errors.Is(err, auth.ErrPasswordMissingUppercase),
		errors.Is(err, auth.ErrPasswordMissingDigit),
		errors.Is(err, auth.ErrPasswordMissingSpecial):
		return http.StatusBadRequest, NewErrDetail(passwordField, "invalid_password", "Password does not meet the required format")
	case errors.Is(err, auth.ErrFirstNameRequired):
		return http.StatusBadRequest, NewErrDetail("first_name", "first_name_required", "First name is required")
	case errors.Is(err, auth.ErrInvalidUserTimezone):
		return http.StatusBadRequest, NewErrDetail("timezone", "invalid_timezone", "Timezone must be a valid IANA timezone")
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}
