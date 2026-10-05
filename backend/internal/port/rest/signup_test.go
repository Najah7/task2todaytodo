package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

func TestSignupHandlerRejectsInvalidJSON(t *testing.T) {
	handler := NewSignupHandler(authusecase.UserUseCases{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/signup", strings.NewReader("{"))

	handler.Signup(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
