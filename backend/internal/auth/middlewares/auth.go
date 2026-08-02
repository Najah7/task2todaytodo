package middlewares

import (
	"context"
	"net/http"
	"strings"

	authusecase "github.com/Najah7/task2todaytodo/internal/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/shared"
	sharedhandlers "github.com/Najah7/task2todaytodo/internal/shared/handlers"
)

const bearerPrefix = "Bearer "

func AuthMiddleware(accessTokenService authusecase.AccessTokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecAuthAuthenticateFailed, sharedhandlers.ErrDetailMissingAccessToken)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
			if token == "" {
				sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecAuthAuthenticateFailed, sharedhandlers.ErrDetailMissingAccessToken)
				return
			}

			ctx := r.Context()
			t, err := accessTokenService.GetByToken(ctx, token)
			if err != nil {
				sharedhandlers.WriteError(w, http.StatusUnauthorized, sharedhandlers.ErrSpecAuthAuthenticateFailed, sharedhandlers.ErrDetailInvalidAccessToken)
				return
			}

			ctx = context.WithValue(ctx, sharedhandlers.UserIDContextKey, shared.ID(t.UserID))
			ctx = context.WithValue(ctx, sharedhandlers.AccessTokenContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
