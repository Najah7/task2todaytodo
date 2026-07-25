package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Najah7/task2schedule/internal/domain/auth"
	"github.com/Najah7/task2schedule/internal/domain/shared"
	"github.com/jackc/pgx/v5/pgconn"
)

const UserIDContextKey = "userID"

type UserHandler struct {
	svc   *auth.UserService
	idGen shared.IDGenerator
}

func NewUserHandler(svc *auth.UserService, idGen shared.IDGenerator) *UserHandler {
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
//	@Failure		401	{object}	ErrResponse	"Unauthorized"
//	@Failure		500	{object}	ErrResponse	"Failed to get user"
//	@Router			/users/me [get]
func (h UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := ctx.Value(UserIDContextKey).(auth.UserID)

	u, err := h.svc.GetUser(ctx, userID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, ErrSpecUsersGetFailed)
		return
	}

	WriteJSON(w, http.StatusOK, newUserResponse(u))
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
//	@Failure		400		{object}	ErrResponse	"Invalid request body"
//	@Failure		500		{object}	ErrResponse	"Failed to create user"
//	@Router			/users [post]
func (h UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UserCreateRequest
	requestBody := json.NewDecoder(r.Body)
	err := requestBody.Decode(&req)
	if err != nil {
		WriteError(w, http.StatusBadRequest, ErrSpecUsersCreateFailed, ErrDetailInvalidRequestBody)
		return
	}

	u, err := h.svc.CreateUser(ctx, h.idGen.Generate, req.Email, req.Password)
	if err != nil {
		status, detail := errToErrResponse(err, "password")
		WriteError(w, status, ErrSpecUsersCreateFailed, detail)
		return
	}

	WriteJSON(w, http.StatusCreated, newUserResponse(u))
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
	userID, ok := ctx.Value(UserIDContextKey).(auth.UserID)
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

	u, err := h.svc.UpdateUserName(ctx, userID, req.FirstName, req.LastName)
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
	userID, ok := ctx.Value(UserIDContextKey).(auth.UserID)
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

	err = h.svc.UpdateUserPassword(ctx, userID, req.NewPassword)
	if err != nil {
		status, detail := errToErrResponse(err, "new_password")
		WriteError(w, status, ErrSpecUsersUpdatePasswordFailed, detail)
		return
	}

	WriteMessage(w, http.StatusOK, "OK")
}

func errToErrResponse(err error, passwordField string) (int, ErrDetail) {
	switch {
	case errors.Is(err, auth.ErrUserEmailAlreadyExists):
		return http.StatusConflict, NewErrDetail("email", "email_already_exists", "Email is already registered")
	case isUniqueConstraint(err, "users_email_key"):
		return http.StatusConflict, NewErrDetail("email", "email_already_exists", "Email is already registered")
	case errors.Is(err, auth.ErrUserIDAlreadyExists):
		return http.StatusConflict, NewErrDetail("user_id", "user_id_already_exists", "User ID is already registered")
	case isUniqueConstraint(err, "users_pkey"):
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
	default:
		return http.StatusInternalServerError, ErrDetailInternalServerError
	}
}

func isUniqueConstraint(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
