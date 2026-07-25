package middlewares

import (
	"context"
	"net/http"
	"strings"

	"github.com/Najah7/task2todaytodo/internal/domain/auth"
	"github.com/Najah7/task2todaytodo/internal/handlers"
)

const bearerPrefix = "Bearer "

func AuthMiddleware(accessTokenService auth.AccessTokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				handlers.WriteError(w, http.StatusUnauthorized, handlers.ErrSpecAuthAuthenticateFailed, handlers.ErrDetailMissingAccessToken)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
			if token == "" {
				handlers.WriteError(w, http.StatusUnauthorized, handlers.ErrSpecAuthAuthenticateFailed, handlers.ErrDetailMissingAccessToken)
				return
			}

			ctx := r.Context()
			t, err := accessTokenService.GetByToken(ctx, token)
			if err != nil {
				handlers.WriteError(w, http.StatusUnauthorized, handlers.ErrSpecAuthAuthenticateFailed, handlers.ErrDetailInvalidAccessToken)
				return
			}

			ctx = context.WithValue(ctx, handlers.UserIDContextKey, t.UserID)
			ctx = context.WithValue(ctx, handlers.AccessTokenContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
