package rest

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDecodeTodoRequestUsesStrictJSONRules(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		allowEmpty bool
		wantError  bool
	}{
		{name: "valid object", body: `{"item_ids":["item-1"]}`},
		{name: "unknown field", body: `{"item_ids":["item-1"],"unexpected":true}`, wantError: true},
		{name: "null object", body: `null`, wantError: true},
		{name: "trailing value", body: `{"item_ids":["item-1"]}{}`, wantError: true},
		{name: "trailing garbage", body: `{"item_ids":["item-1"]} trailing`, wantError: true},
		{name: "empty complete body", body: " \n", allowEmpty: true},
		{name: "empty reorder body", body: "", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			if err != nil {
				t.Fatal(err)
			}
			request.Body = io.NopCloser(strings.NewReader(test.body))
			var target TodoReorderRequest
			err = decodeTodoRequest(request, &target, test.allowEmpty)
			if (err != nil) != test.wantError {
				t.Fatalf("decodeTodoRequest() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}
