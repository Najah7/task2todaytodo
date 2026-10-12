package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	authdomain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

const bearerPrefix = "Bearer "

func NewAuthMiddleware(authenticate *authusecase.AuthenticateUseCase) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				rest.WriteError(w, http.StatusUnauthorized, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailMissingPersonalAccessToken)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
			if token == "" {
				rest.WriteError(w, http.StatusUnauthorized, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailMissingPersonalAccessToken)
				return
			}

			ctx := r.Context()
			userID, err := authenticate.Execute(ctx, token)
			if err != nil {
				if invalidPersonalAccessToken(err) {
					rest.WriteError(w, http.StatusUnauthorized, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailInvalidPersonalAccessToken)
					return
				}
				rest.WriteError(w, http.StatusInternalServerError, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailInternalServerError)
				return
			}

			ctx = context.WithValue(ctx, rest.UserIDContextKey, string(userID))
			ctx = context.WithValue(ctx, rest.PersonalAccessTokenContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func invalidPersonalAccessToken(err error) bool {
	return telemetry.IsNotFound(err) ||
		errors.Is(err, authdomain.ErrPersonalAccessTokenEmpty) ||
		errors.Is(err, authdomain.ErrPersonalAccessTokenUserIDEmpty) ||
		errors.Is(err, authdomain.ErrPersonalAccessTokenExpiresAtInvalid) ||
		errors.Is(err, authdomain.ErrPersonalAccessTokenRevoked) ||
		errors.Is(err, authdomain.ErrPersonalAccessTokenExpired)
}
