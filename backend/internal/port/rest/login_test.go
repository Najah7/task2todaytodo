package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authusecase "github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
)

func TestLoginHandlerRejectsInvalidJSON(t *testing.T) {
	handler := NewLoginHandler(authusecase.PersonalAccessTokenUseCases{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{"))

	handler.Login(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
