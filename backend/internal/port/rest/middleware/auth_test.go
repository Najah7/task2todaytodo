package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	authdomain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
)

type authTokenRepositoryStub struct {
	token dao.PersonalAccessToken
	err   error
}

func (r authTokenRepositoryStub) GetByToken(context.Context, string) (dao.PersonalAccessToken, error) {
	return r.token, r.err
}
func (authTokenRepositoryStub) Create(context.Context, authdomain.PersonalAccessToken) (dao.PersonalAccessToken, error) {
	return dao.PersonalAccessToken{}, errors.New("unexpected Create")
}
func (authTokenRepositoryStub) Revoke(context.Context, string) error {
	return errors.New("unexpected Revoke")
}

func TestAuthMiddlewareOnlyMapsInvalidTokensToUnauthorized(t *testing.T) {
	tests := []struct {
		name       string
		repo       authTokenRepositoryStub
		status     int
		detailCode string
	}{
		{
			name:       "unknown token",
			repo:       authTokenRepositoryStub{err: sql.ErrNoRows},
			status:     http.StatusUnauthorized,
			detailCode: rest.DetailInvalidTokenCode,
		},
		{
			name: "revoked token",
			repo: authTokenRepositoryStub{token: dao.PersonalAccessToken{
				Token: "secret", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour).Unix(), RevokedAt: time.Now().Unix(),
			}},
			status: http.StatusUnauthorized, detailCode: rest.DetailInvalidTokenCode,
		},
		{
			name: "expired token",
			repo: authTokenRepositoryStub{token: dao.PersonalAccessToken{
				Token: "secret", UserID: "user-1", ExpiresAt: time.Now().Add(-time.Hour).Unix(),
			}},
			status: http.StatusUnauthorized, detailCode: rest.DetailInvalidTokenCode,
		},
		{
			name:       "pool deadline",
			repo:       authTokenRepositoryStub{err: context.DeadlineExceeded},
			status:     http.StatusInternalServerError,
			detailCode: rest.DetailInternalErrorCode,
		},
		{
			name:       "poll deadline",
			repo:       authTokenRepositoryStub{err: os.ErrDeadlineExceeded},
			status:     http.StatusInternalServerError,
			detailCode: rest.DetailInternalErrorCode,
		},
		{
			name:       "lookup canceled while request remains active",
			repo:       authTokenRepositoryStub{err: context.Canceled},
			status:     http.StatusInternalServerError,
			detailCode: rest.DetailInternalErrorCode,
		},
		{
			name:       "database unavailable",
			repo:       authTokenRepositoryStub{err: errors.New("database unavailable")},
			status:     http.StatusInternalServerError,
			detailCode: rest.DetailInternalErrorCode,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticate := authusecase.NewAuthenticateUseCase(tt.repo, nil)
			handlerCalled := false
			handler := NewAuthMiddleware(authenticate)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/timezones", nil)
			req.Header.Set("Authorization", "Bearer secret")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tt.status {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, tt.status, res.Body.String())
			}
			if handlerCalled {
				t.Fatal("protected handler ran after authentication failed")
			}
			var body rest.ErrResponse
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if len(body.Error.Details) != 1 || body.Error.Details[0].Code != tt.detailCode {
				t.Fatalf("error details = %+v, want code %q", body.Error.Details, tt.detailCode)
			}
		})
	}
}

func TestAuthMiddlewareAllowsValidToken(t *testing.T) {
	authenticate := authusecase.NewAuthenticateUseCase(authTokenRepositoryStub{token: dao.PersonalAccessToken{
		Token: "secret", UserID: "user-1", ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}}, nil)
	handlerCalled := false
	handler := NewAuthMiddleware(authenticate)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/timezones", nil)
	req.Header.Set("Authorization", "Bearer secret")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if !handlerCalled {
		t.Fatal("protected handler was not called for a valid token")
	}
}
