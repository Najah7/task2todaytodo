package middleware

import (
	"context"
	"net/http"
	"strings"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
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
				rest.WriteError(w, http.StatusUnauthorized, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailInvalidPersonalAccessToken)
				return
			}

			ctx = context.WithValue(ctx, rest.UserIDContextKey, string(userID))
			ctx = context.WithValue(ctx, rest.PersonalAccessTokenContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
