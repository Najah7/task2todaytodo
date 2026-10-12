package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdao "github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	authdomain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

type userMinimumGapRepository struct {
	authusecase.UserRepository
	user   authdao.User
	writes int
}

func (r *userMinimumGapRepository) Get(context.Context, authdomain.UserID) (authdao.User, error) {
	return r.user, nil
}

func (r *userMinimumGapRepository) UpdateMinimumGapMinutes(_ context.Context, _ authdomain.UserID, minutes int) error {
	r.user.MinimumGapMinutes = minutes
	r.writes++
	return nil
}

func TestUpdateMinimumGapPresenceRangeAndResponse(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want int
		code int
	}{
		{name: "zero is valid", body: `{"minimum_gap_minutes":0}`, want: 0, code: http.StatusOK},
		{name: "default", body: `{"minimum_gap_minutes":5}`, want: 5, code: http.StatusOK},
		{name: "maximum", body: `{"minimum_gap_minutes":1440}`, want: 1440, code: http.StatusOK},
		{name: "missing", body: `{}`, code: http.StatusBadRequest},
		{name: "null", body: `{"minimum_gap_minutes":null}`, code: http.StatusBadRequest},
		{name: "negative", body: `{"minimum_gap_minutes":-1}`, code: http.StatusBadRequest},
		{name: "too large", body: `{"minimum_gap_minutes":1441}`, code: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &userMinimumGapRepository{user: authdao.User{ID: "user-1", MinimumGapMinutes: 5}}
			handler := NewUserHandler(authusecase.UserUseCases{UpdateMinimumGap: authusecase.NewUpdateUserMinimumGapUseCase(repo, nil)})
			req := httptest.NewRequest(http.MethodPatch, "/users/me/minimum-gap", strings.NewReader(test.body))
			req = req.WithContext(context.WithValue(req.Context(), UserIDContextKey, "user-1"))
			response := httptest.NewRecorder()
			handler.UpdateMinimumGap(response, req)
			if response.Code != test.code {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.code, response.Body.String())
			}
			if test.code != http.StatusOK {
				if repo.writes != 0 {
					t.Fatalf("invalid value caused %d writes", repo.writes)
				}
				return
			}
			var result UserResponse
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.MinimumGapMinutes != test.want || repo.writes != 1 {
				t.Fatalf("response gap/write count = %d/%d, want %d/1", result.MinimumGapMinutes, repo.writes, test.want)
			}
		})
	}
}
