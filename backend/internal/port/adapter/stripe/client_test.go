package stripe

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestScheduleDowngradeReleasesCreatedScheduleWhenConfigurationFails(t *testing.T) {
	var paths []string
	client := NewClient("test-key", "price_pro", "price_team")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		status, body := http.StatusOK, "{}"
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v1/subscriptions/sub_test":
			body = `{"id":"sub_test","customer":"cus_test","status":"active","items":{"data":[{"id":"si_test","price":{"id":"price_team"}}]}}`
		case request.Method == http.MethodPost && request.URL.Path == "/v1/subscription_schedules":
			body = `{"id":"sched_test"}`
		case request.Method == http.MethodGet && request.URL.Path == "/v1/subscription_schedules/sched_test":
			body = `{"current_phase":{"start_date":100,"end_date":200},"phases":[{"start_date":100,"end_date":200,"items":[{"price":"price_team","quantity":1}]}]}`
		case request.Method == http.MethodPost && request.URL.Path == "/v1/subscription_schedules/sched_test":
			status, body = http.StatusBadRequest, `{"error":{"code":"invalid_request"}}`
		case request.Method == http.MethodPost && request.URL.Path == "/v1/subscription_schedules/sched_test/release":
			body = `{"id":"sched_test","status":"released"}`
		default:
			t.Errorf("unexpected Stripe request: %s %s", request.Method, request.URL.Path)
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})}

	_, err := client.ScheduleDowngrade(context.Background(), "sub_test", "price_team", "price_pro", "cus_test")
	if err == nil || !strings.Contains(err.Error(), "invalid_request") {
		t.Fatalf("ScheduleDowngrade() error = %v; want original configuration error", err)
	}
	if len(paths) == 0 || paths[len(paths)-1] != "POST /v1/subscription_schedules/sched_test/release" {
		t.Fatalf("last Stripe call = %v; want release compensation", paths)
	}
}
