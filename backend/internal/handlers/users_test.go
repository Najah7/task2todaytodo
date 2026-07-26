package handlers

import (
	"errors"
	"net/http"
	"testing"

	auth "github.com/Najah7/task2todaytodo/internal/auth/domain"
	authusecase "github.com/Najah7/task2todaytodo/internal/auth/usecase"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUserServiceErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		field      string
		wantStatus int
		wantDetail ErrDetail
	}{
		{
			name:       "duplicate email",
			err:        authusecase.ErrUserEmailAlreadyExists,
			field:      "password",
			wantStatus: http.StatusConflict,
			wantDetail: NewErrDetail("email", "email_already_exists", "Email is already registered"),
		},
		{
			name:       "duplicate email from database",
			err:        &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"},
			field:      "password",
			wantStatus: http.StatusConflict,
			wantDetail: NewErrDetail("email", "email_already_exists", "Email is already registered"),
		},
		{
			name:       "invalid password",
			err:        auth.ErrPasswordTooShort,
			field:      "new_password",
			wantStatus: http.StatusBadRequest,
			wantDetail: NewErrDetail("new_password", "invalid_password", "Password does not meet the required format"),
		},
		{
			name:       "unknown error",
			err:        errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantDetail: ErrDetailInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotDetail := errToErrResponse(tt.err, tt.field)
			if gotStatus != tt.wantStatus {
				t.Errorf("status = %d, want %d", gotStatus, tt.wantStatus)
			}
			if gotDetail != tt.wantDetail {
				t.Errorf("detail = %+v, want %+v", gotDetail, tt.wantDetail)
			}
		})
	}
}
