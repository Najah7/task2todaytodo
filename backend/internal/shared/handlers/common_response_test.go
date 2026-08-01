package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorAddsInternalDetailWhenNoneIsProvided(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteError(recorder, http.StatusInternalServerError, ErrSpecUsersCreateFailed)

	var got ErrResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(got.Error.Details) != 1 || got.Error.Details[0] != ErrDetailInternalServerError {
		t.Errorf("details = %+v, want %+v", got.Error.Details, []ErrDetail{ErrDetailInternalServerError})
	}
}

func TestWriteErrorPreservesProvidedDetails(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteError(recorder, http.StatusBadRequest, ErrSpecUsersCreateFailed, ErrDetailInvalidRequestBody)

	var got ErrResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(got.Error.Details) != 1 || got.Error.Details[0] != ErrDetailInvalidRequestBody {
		t.Errorf("details = %+v, want %+v", got.Error.Details, []ErrDetail{ErrDetailInvalidRequestBody})
	}
}
