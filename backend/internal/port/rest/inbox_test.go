package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	inboxdao "github.com/Najah7/task2todaytodo/internal/application/inbox/dao"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

type inboxReaderFake struct {
	timezone string
	items    []inboxdao.Item
	err      error
	lastUser string
	snapshot inboxusecase.Snapshot
}

func (f *inboxReaderFake) Timezone(_ context.Context, actor string) (string, error) {
	f.lastUser = actor
	return f.timezone, nil
}

func (f *inboxReaderFake) ReadCandidates(_ context.Context, actor string, snapshot inboxusecase.Snapshot, _ inboxusecase.CandidateRange) ([]inboxdao.Item, error) {
	f.lastUser = actor
	f.snapshot = snapshot
	if f.err != nil {
		return nil, f.err
	}
	return append([]inboxdao.Item(nil), f.items...), nil
}

func inboxTestHandler(t *testing.T, reader *inboxReaderFake) *InboxHandler {
	t.Helper()
	codec, err := pagination.NewCodec([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	list := inboxusecase.NewListUseCase(reader, nil, func() time.Time {
		return time.Date(2026, 10, 12, 0, 30, 0, 0, time.UTC)
	})
	return NewInboxHandler(inboxusecase.InboxUseCases{List: list}, codec)
}

func inboxTestItems() []inboxdao.Item {
	return []inboxdao.Item{
		{
			Type: inboxdao.ItemTypeTask, ID: "task-1", Title: "Write plan", ProjectID: "project-1", ProjectName: "Today",
			CreatedAt: "2026-10-11T10:00:00Z", Task: &inboxdao.TaskPayload{TaskID: "task-1", Revision: 7, CanUpdate: true, CanAssign: true},
		},
		{
			Type: inboxdao.ItemTypeSchedule, ID: "rule-1/2026-10-12", Title: "Daily review",
			CreatedAt: "2026-10-10T10:00:00Z", Schedule: &inboxdao.SchedulePayload{
				RuleID: "rule-1", OccurrenceDate: "2026-10-12",
				Timezone: "Asia/Tokyo", StartAt: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC).Unix(), EndAt: time.Date(2026, 10, 12, 1, 0, 0, 0, time.UTC).Unix(), CanUpdate: true, CanAssign: true, CanUpdateFollowing: true,
			},
		},
	}
}

func inboxRequest(handler *InboxHandler, query string, actor string) *httptest.ResponseRecorder {
	request := httptest.NewRequest("GET", "/api/inbox"+query, nil)
	if actor != "" {
		request = request.WithContext(context.WithValue(request.Context(), UserIDContextKey, actor))
	}
	response := httptest.NewRecorder()
	handler.List(response, request)
	return response
}

func TestInboxListReturnsMixedNullableProjectionAndCounts(t *testing.T) {
	reader := &inboxReaderFake{timezone: "Asia/Tokyo", items: inboxTestItems()}
	handler := inboxTestHandler(t, reader)
	response := inboxRequest(handler, "?type=task&page_size=10", "user-1")
	if response.Code != 200 {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var body InboxListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Type != "task" || body.Items[0].Task == nil || body.Items[0].Schedule != nil {
		t.Fatalf("mixed item = %+v", body.Items)
	}
	if body.Items[0].Task.Revision != 7 {
		t.Fatalf("task revision = %d", body.Items[0].Task.Revision)
	}
	if body.Items[0].ProjectID == nil || *body.Items[0].ProjectID != "project-1" || body.Items[0].ProjectName == nil {
		t.Fatalf("project fields = %+v", body.Items[0])
	}
	if body.Counts != (InboxCountsResponse{All: 2, Task: 1, Schedule: 1}) {
		t.Fatalf("counts = %+v", body.Counts)
	}
	allResponse := inboxRequest(handler, "", "user-1")
	var allBody InboxListResponse
	if err := json.Unmarshal(allResponse.Body.Bytes(), &allBody); err != nil {
		t.Fatal(err)
	}
	if len(allBody.Items) != 2 || allBody.Items[1].Schedule == nil || !allBody.Items[1].Schedule.CanAssign || !allBody.Items[1].Schedule.CanUpdateFollowing {
		t.Fatalf("schedule permissions = %+v", allBody.Items)
	}
	if reader.lastUser != "user-1" {
		t.Fatalf("actor passed to reader = %q", reader.lastUser)
	}
}

func TestInboxListRejectsMissingAuthMalformedRepeatedAndInvalidFilters(t *testing.T) {
	handler := inboxTestHandler(t, &inboxReaderFake{timezone: "UTC", items: inboxTestItems()})
	for _, test := range []struct {
		name  string
		query string
		actor string
	}{
		{name: "missing auth"},
		{name: "repeated type", query: "?type=task&type=schedule", actor: "user-1"},
		{name: "invalid type", query: "?type=todo", actor: "user-1"},
		{name: "repeated title", query: "?title=first&title=second", actor: "user-1"},
		{name: "invalid page size", query: "?page_size=-1", actor: "user-1"},
		{name: "malformed token", query: "?page_token=broken", actor: "user-1"},
		{name: "invalid field mask", query: "?fields=not_a_field", actor: "user-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := inboxRequest(handler, test.query, test.actor)
			want := 400
			if test.actor == "" {
				want = 401
			}
			if response.Code != want {
				t.Fatalf("status = %d, want %d: %s", response.Code, want, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "broken") {
				t.Fatalf("response exposed supplied token: %s", response.Body.String())
			}
		})
	}
}

func TestInboxPageTokenBindsFiltersAndFieldMaskAndCarriesSnapshot(t *testing.T) {
	reader := &inboxReaderFake{timezone: "Asia/Tokyo", items: inboxTestItems()}
	handler := inboxTestHandler(t, reader)
	first := inboxRequest(handler, "?page_size=1&title=", "user-1")
	if first.Code != 200 {
		t.Fatalf("first page status = %d: %s", first.Code, first.Body.String())
	}
	var firstBody InboxListResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatal(err)
	}
	if firstBody.NextPageToken == "" {
		t.Fatal("first page missing next token")
	}
	if got := reader.snapshot; got.Timezone != "Asia/Tokyo" || got.Today != "2026-10-12" || got.AsOf.IsZero() {
		t.Fatalf("snapshot = %+v", got)
	}
	for _, query := range []string{
		"?page_size=1&page_token=" + urlQueryEscape(firstBody.NextPageToken) + "&type=task",
		"?page_size=1&page_token=" + urlQueryEscape(firstBody.NextPageToken) + "&fields=items(id)",
		"?page_size=1&page_token=" + urlQueryEscape(firstBody.NextPageToken) + "&project_id=no-project",
		"?page_size=1&page_token=" + urlQueryEscape(firstBody.NextPageToken) + "&title=changed",
	} {
		response := inboxRequest(handler, query, "user-1")
		if response.Code != 400 {
			t.Fatalf("scope change status = %d, want 400: %s", response.Code, response.Body.String())
		}
	}
	otherActor := inboxRequest(handler, "?page_size=1&page_token="+urlQueryEscape(firstBody.NextPageToken), "user-2")
	if otherActor.Code != 400 {
		t.Fatalf("actor scope change status = %d, want 400", otherActor.Code)
	}
	second := inboxRequest(handler, "?page_size=1&page_token="+urlQueryEscape(firstBody.NextPageToken), "user-1")
	if second.Code != 200 {
		t.Fatalf("second page status = %d: %s", second.Code, second.Body.String())
	}
	if reader.snapshot.Today != "2026-10-12" || !reader.snapshot.AsOf.Equal(time.Date(2026, 10, 12, 0, 30, 0, 0, time.UTC)) {
		t.Fatalf("second page did not restore frozen snapshot: %+v", reader.snapshot)
	}
}

func TestInboxListClampsPageSizeAndMapsDriverErrorsSafely(t *testing.T) {
	items := make([]inboxdao.Item, 101)
	for i := range items {
		items[i] = inboxdao.Item{Type: inboxdao.ItemTypeTask, ID: fmt.Sprintf("task-%03d", i), Title: "Task", CreatedAt: "2026-10-12T00:00:00Z", Task: &inboxdao.TaskPayload{TaskID: fmt.Sprintf("task-%03d", i)}}
	}
	reader := &inboxReaderFake{timezone: "UTC", items: items}
	handler := inboxTestHandler(t, reader)
	response := inboxRequest(handler, "?page_size=999999", "user-1")
	if response.Code != 200 {
		t.Fatalf("large page status = %d: %s", response.Code, response.Body.String())
	}
	var body InboxListResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 100 {
		t.Fatalf("large page returned %d items, want clamped 100", len(body.Items))
	}
	reader.err = errors.New("ERROR: password=hunter2 SQL SELECT * FROM secret")
	response = inboxRequest(handler, "", "user-1")
	if response.Code != 500 {
		t.Fatalf("driver error status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "hunter2") || strings.Contains(response.Body.String(), "SELECT") {
		t.Fatalf("driver detail leaked: %s", response.Body.String())
	}
}

func urlQueryEscape(value string) string { return url.QueryEscape(value) }
