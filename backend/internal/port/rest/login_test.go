package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdao "github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
)

type loginLookupUserRepository struct {
	authusecase.UserRepository
	err error
}

func (r loginLookupUserRepository) GetByEmail(context.Context, string) (authdao.User, error) {
	return authdao.User{}, r.err
}

func loginHandlerWithLookupError(err error) *LoginHandler {
	login := authusecase.NewLoginUserUseCase(loginLookupUserRepository{err: err}, nil)
	return NewLoginHandler(authusecase.PersonalAccessTokenUseCases{Login: login})
}

func TestLoginHandlerRejectsInvalidJSON(t *testing.T) {
	handler := NewLoginHandler(authusecase.PersonalAccessTokenUseCases{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{"))

	handler.Login(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLoginHandlerReturnsUnauthorizedForUnknownEmail(t *testing.T) {
	driverErr := fmt.Errorf("get user by email: %w", pgx.ErrNoRows)
	handler := loginHandlerWithLookupError(driverErr)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"unknown@example.com","password":"SecretPassword1!"}`))

	handler.Login(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; response: %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
	for _, credential := range []string{"unknown@example.com", "SecretPassword1!"} {
		if strings.Contains(recorder.Body.String(), credential) {
			t.Errorf("response contains credential %q: %s", credential, recorder.Body.String())
		}
	}
}

func TestLoginHandlerKeepsLookupFailuresInternal(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "database failure", err: errors.New("database unavailable")},
		{name: "timeout", err: context.DeadlineExceeded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := loginHandlerWithLookupError(tt.err)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"email":"user@example.com","password":"SecretPassword1!"}`))

			handler.Login(recorder, request)

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want %d; response: %s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
			}
		})
	}
}
