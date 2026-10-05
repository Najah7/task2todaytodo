package rest

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
)

func TestProjectMemberErrorsAreMappedToSafeClientResponses(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		status     int
		detailCode string
	}{
		{name: "unknown role", err: taskusecase.ErrProjectMemberRoleNotFound, status: 400, detailCode: "unknown_role"},
		{name: "unknown user", err: taskusecase.ErrProjectMemberUserNotFound, status: 404, detailCode: "not_found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			privateDBError := errors.New("private database detail")
			writeProjectMemberUseCaseError(response, projectMemberUpsertFailure, errors.Join(test.err, privateDBError))

			if response.Code != test.status {
				t.Fatalf("status = %d; want %d", response.Code, test.status)
			}
			var body ErrResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if len(body.Error.Details) != 1 || body.Error.Details[0].Code != test.detailCode {
				t.Fatalf("error details = %+v; want code %q", body.Error.Details, test.detailCode)
			}
			if response.Body.String() == "" || strings.Contains(response.Body.String(), privateDBError.Error()) {
				t.Fatalf("unsafe error response: %s", response.Body.String())
			}
		})
	}
}
