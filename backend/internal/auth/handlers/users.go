package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	auth "github.com/Najah7/task2todaytodo/internal/auth/domain"
	authusecase "github.com/Najah7/task2todaytodo/internal/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/shared"
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
)

type UserHandler struct {
	svc   *authusecase.UserService
	idGen shared.IDGenerator
}

func NewUserHandler(svc *authusecase.UserService, idGen shared.IDGenerator) *UserHandler {
	return &UserHandler{
		svc:   svc,
		idGen: idGen,
	}
}

type UserResponse struct {
	UserID   string `json:"user_id"`
	UserName string `json:"user_name,omitempty"`
	Email    string `json:"email"`
}

type UserListResponse struct {
	users []UserResponse
}

func newUserResponse(u auth.User) UserResponse {
	return UserResponse{
		UserID:   string(u.ID),
		UserName: u.FullName(),
		Email:    u.Email.String(),
	}
}

// Get godoc
//
//	@Summary		Get current user
//	@Description	Returns the authenticated user's profile.
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	UserResponse
//	@Failure		401	{object}	sharedhandlers.ErrResponse	"Unauthorized"
//	@Failure		500	{object}	sharedhandlers.ErrResponse	"Failed to get user"
//	@Router			/users/me [get]
func (h UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := ctx.Value(sharedhandlers.UserIDContextKey).(auth.UserID)

	u, err := h.svc.GetUser(ctx, userID)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusInternalServerError, sharedhandlers.ErrSpecUsersGetFailed)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, newUserResponse(u))
}

type UserCreateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	//TODO: AuthCode string `json:"auth_code"` with invitation email
}

// Create godoc
//
//	@Summary		Create user
//	@Description	Creates a user with an email address and password.
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		UserCreateRequest	true	"User create request"
//	@Success		201		{object}	UserResponse
//	@Failure		400		{object}	sharedhandlers.ErrResponse	"Invalid request body"
//	@Failure		500		{object}	sharedhandlers.ErrResponse	"Failed to create user"
//	@Router			/users [post]
func (h UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UserCreateRequest
	requestBody := json.NewDecoder(r.Body)
	err := requestBody.Decode(&req)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, sharedhandlers.ErrSpecUsersCreateFailed, sharedhandlers.ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.svc.CreateUser(ctx, h.idGen.Generate, req.Email, req.Password)
	if err != nil {
		status, detail := errToErrResponse(err, "password")
		sharedhandlers.WriteError(w, status, sharedhandlers.ErrSpecUsersCreateFailed, detail)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusCreated, newUserResponse(u))
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
//	@Failure		400		{object}	sharedhandlers.ErrResponse	"Invalid request body"
//	@Failure		401		{object}	sharedhandlers.ErrResponse	"Unauthorized"
//	@Failure		500		{object}	sharedhandlers.ErrResponse	"Failed to update user"
//	@Router			/users/me [patch]
func (h UserHandler) UpdateBasicInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(sharedhandlers.UserIDContextKey).(auth.UserID)
	if !ok {
		sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecUsersUpdateBasicInfoFailed, sharedhandlers.ErrDetailUnauthorized)
		return
	}

	var req UserInfoUpdateRequest
	requestBody := json.NewDecoder(r.Body)
	err := requestBody.Decode(&req)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, sharedhandlers.ErrSpecUsersUpdateBasicInfoFailed, sharedhandlers.ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.svc.UpdateUserName(ctx, userID, req.FirstName, req.LastName)
	if err != nil {
		status, detail := errToErrResponse(err, "")
		sharedhandlers.WriteError(w, status, sharedhandlers.ErrSpecUsersUpdateBasicInfoFailed, detail)
		return
	}

	sharedhandlers.WriteJSON(w, http.StatusOK, newUserResponse(u))
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
//	@Failure		400		{object}	sharedhandlers.ErrResponse					"Invalid request body"
//	@Failure		401		{object}	sharedhandlers.ErrResponse					"Unauthorized"
//	@Failure		500		{object}	sharedhandlers.ErrResponse					"Failed to update password"
//	@Router			/users/me/password [patch]
func (h UserHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctx.Value(sharedhandlers.UserIDContextKey).(auth.UserID)
	if !ok {
		sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecUsersUpdatePasswordFailed, sharedhandlers.ErrDetailUnauthorized)
		return
	}

	var req UserPasswordUpdateRequest
	requestBody := json.NewDecoder(r.Body)
	err := requestBody.Decode(&req)
	if err != nil {
		sharedhandlers.WriteError(w, http.StatusBadRequest, sharedhandlers.ErrSpecUsersUpdatePasswordFailed, sharedhandlers.ErrDetailInvalidRequestBody)
		return
	}

	err = h.svc.UpdateUserPassword(ctx, userID, req.NewPassword)
	if err != nil {
		status, detail := errToErrResponse(err, "new_password")
		sharedhandlers.WriteError(w, status, sharedhandlers.ErrSpecUsersUpdatePasswordFailed, detail)
		return
	}

	sharedhandlers.WriteMessage(w, http.StatusOK, "OK")
}

func errToErrResponse(err error, passwordField string) (int, sharedhandlers.ErrDetail) {
	switch {
	case errors.Is(err, authusecase.ErrUserEmailAlreadyExists):
		return http.StatusConflict, sharedhandlers.NewErrDetail("email", "email_already_exists", "Email is already registered")
	case sharedhandlers.IsUniqueConstraint(err, "users_email_key"):
		return http.StatusConflict, sharedhandlers.NewErrDetail("email", "email_already_exists", "Email is already registered")
	case errors.Is(err, authusecase.ErrUserIDAlreadyExists):
		return http.StatusConflict, sharedhandlers.NewErrDetail("user_id", "user_id_already_exists", "User ID is already registered")
	case sharedhandlers.IsUniqueConstraint(err, "users_pkey"):
		return http.StatusConflict, sharedhandlers.NewErrDetail("user_id", "user_id_already_exists", "User ID is already registered")
	case errors.Is(err, auth.ErrEmailEmpty), errors.Is(err, auth.ErrInvalidEmailFormat):
		return http.StatusBadRequest, sharedhandlers.NewErrDetail("email", "invalid_email", "Email must be a valid email address")
	case errors.Is(err, auth.ErrPasswordEmpty),
		errors.Is(err, auth.ErrPasswordTooShort),
		errors.Is(err, auth.ErrPasswordMissingLowercase),
		errors.Is(err, auth.ErrPasswordMissingUppercase),
		errors.Is(err, auth.ErrPasswordMissingDigit),
		errors.Is(err, auth.ErrPasswordMissingSpecial):
		return http.StatusBadRequest, sharedhandlers.NewErrDetail(passwordField, "invalid_password", "Password does not meet the required format")
	case errors.Is(err, auth.ErrFirstNameRequired):
		return http.StatusBadRequest, sharedhandlers.NewErrDetail("first_name", "first_name_required", "First name is required")
	default:
		return http.StatusInternalServerError, sharedhandlers.ErrDetailInternalServerError
	}
}
