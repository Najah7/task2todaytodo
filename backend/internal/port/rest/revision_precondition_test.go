package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExpectedCollectionRevision(t *testing.T) {
	valid := strings.Repeat("a", 64)
	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantToken  string
	}{
		{name: "quoted sha256", header: `"` + valid + `"`, wantStatus: http.StatusOK, wantToken: valid},
		{name: "missing", wantStatus: http.StatusPreconditionRequired},
		{name: "unquoted", header: valid, wantStatus: http.StatusBadRequest},
		{name: "empty", header: `""`, wantStatus: http.StatusBadRequest},
		{name: "list", header: `"` + valid + `", "` + valid + `"`, wantStatus: http.StatusBadRequest},
		{name: "uppercase", header: `"` + strings.Repeat("A", 64) + `"`, wantStatus: http.StatusBadRequest},
		{name: "weak", header: `W/"` + valid + `"`, wantStatus: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/", nil)
			if test.header != "" {
				request.Header.Set(ifMatchHeader, test.header)
			}
			response := httptest.NewRecorder()
			token, ok := expectedCollectionRevision(response, request, ErrSpecAllocationsUpdateFailed)
			if test.wantStatus == http.StatusOK {
				if !ok || token != test.wantToken {
					t.Fatalf("parsed token = %q, %v; want %q, true", token, ok, test.wantToken)
				}
				return
			}
			if ok || response.Code != test.wantStatus {
				t.Fatalf("parse result = %q, %v, status %d; want false, %d", token, ok, response.Code, test.wantStatus)
			}
		})
	}
}
