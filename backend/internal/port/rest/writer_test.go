package rest

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorUsesResponseRequestID(t *testing.T) {
	response := httptest.NewRecorder()
	response.Header().Set("X-Request-ID", "123e4567-e89b-12d3-a456-426614174000")
	WriteError(response, 500, ErrSpecUsersGetFailed)
	var body struct {
		Error struct {
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("request_id = %q, want response header", body.Error.RequestID)
	}
}
