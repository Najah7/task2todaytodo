package middleware

import (
	"context"
	"net/http"
	"strings"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest"
)

// NewOptionalAuthMiddleware authenticates a supplied bearer token while
// allowing requests without an Authorization header to continue anonymously.
func NewOptionalAuthMiddleware(authenticate *authusecase.AuthenticateUseCase) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				rest.WriteError(w, http.StatusUnauthorized, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailMissingPersonalAccessToken)
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
			if token == "" {
				rest.WriteError(w, http.StatusUnauthorized, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailMissingPersonalAccessToken)
				return
			}
			userID, err := authenticate.Execute(r.Context(), token)
			if err != nil {
				if invalidPersonalAccessToken(err) {
					rest.WriteError(w, http.StatusUnauthorized, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailInvalidPersonalAccessToken)
					return
				}
				rest.WriteError(w, http.StatusInternalServerError, rest.ErrSpecAuthAuthenticateFailed, rest.ErrDetailInternalServerError)
				return
			}
			ctx := context.WithValue(r.Context(), rest.UserIDContextKey, string(userID))
			ctx = context.WithValue(ctx, rest.PersonalAccessTokenContextKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
