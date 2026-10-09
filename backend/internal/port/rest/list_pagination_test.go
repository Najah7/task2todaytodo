package rest

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	projectusecase "github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/rest/fieldmask"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
)

func listTestCodec() *pagination.Codec {
	codec, err := pagination.NewCodec([]byte("01234567890123456789012345678901"))
	if err != nil {
		panic(err)
	}
	return codec
}

func TestParseListRequestBindsTokenToListScopeAndMask(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	codec, err := pagination.NewCodec(key)
	if err != nil {
		t.Fatal(err)
	}
	schema := listEnvelope[TaskResponse]{}
	first := httptest.NewRequest("GET", "/tasks?page_size=7&fields=items(id,title),next_page_token", nil)
	parsed, err := parseListRequest(first, codec, "user-1", "tasks", "", "created_at_desc_id_desc", schema)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Size != 7 || parsed.Mask.Canonical() == "" {
		t.Fatalf("request parse = %+v", parsed)
	}
	token, err := pagination.Encode(codec, parsed.Scope, listAnchor{At: "2026-10-04T01:02:03.000004Z", ID: "cursor-id"})
	if err != nil {
		t.Fatal(err)
	}
	continued := httptest.NewRequest("GET", "/tasks?page_size=25&page_token="+token+"&fields=items(id,title),next_page_token", nil)
	continuedRequest, err := parseListRequest(continued, codec, "user-1", "tasks", "", "created_at_desc_id_desc", schema)
	if err != nil || continuedRequest.Anchor == nil || continuedRequest.Anchor.ID != "cursor-id" || continuedRequest.Size != 25 {
		t.Fatalf("continuation=%+v err=%v", continuedRequest, err)
	}
	for _, scope := range []struct{ user, list, parent, order string }{
		{"user-2", "tasks", "", "created_at_desc_id_desc"},
		{"user-1", "projects", "", "created_at_desc_id_desc"},
		{"user-1", "tasks", "parent", "created_at_desc_id_desc"},
		{"user-1", "tasks", "", "name_asc_id_asc"},
	} {
		request := httptest.NewRequest("GET", "/tasks?page_token="+token+"&fields=items(id,title),next_page_token", nil)
		if _, err := parseListRequest(request, codec, scope.user, scope.list, scope.parent, scope.order, schema); err == nil {
			t.Errorf("token accepted different scope: %+v", scope)
		}
	}
	wrongMask := httptest.NewRequest("GET", "/tasks?page_token="+token+"&fields=items(id),next_page_token", nil)
	if _, err := parseListRequest(wrongMask, codec, "user-1", "tasks", "", "created_at_desc_id_desc", schema); err == nil {
		t.Error("token accepted different field mask")
	}
}

func TestParseListRequestBindsFromDateAndCarriesFrozenAsOf(t *testing.T) {
	codec := listTestCodec()
	schema := listEnvelope[ActionItemResponse]{}
	first := httptest.NewRequest("GET", "/tasks/task-1/action-items?from_date=2026-10-04", nil)
	parsed, err := parseListRequestWithFromDate(first, codec, "user-1", "task_action_items", "task-1", "occurrence_date_asc_position_asc_series_id_asc", schema, true)
	if err != nil || parsed.FromDate != "2026-10-04" {
		t.Fatalf("first page=%+v err=%v", parsed, err)
	}
	anchor := listAnchor{Position: 2, OccurrenceDate: "2026-10-05", SeriesID: "series-1", AsOf: "2026-10-04T00:00:00.123456789Z"}
	token, err := pagination.Encode(codec, parsed.Scope, anchor)
	if err != nil {
		t.Fatal(err)
	}
	continued := httptest.NewRequest("GET", "/tasks/task-1/action-items?from_date=2026-10-04&page_token="+token, nil)
	request, err := parseListRequestWithFromDate(continued, codec, "user-1", "task_action_items", "task-1", "occurrence_date_asc_position_asc_series_id_asc", schema, true)
	if err != nil || request.Anchor == nil || request.AsOf.IsZero() || request.Anchor.SeriesID != "series-1" {
		t.Fatalf("continuation=%+v err=%v", request, err)
	}
	changedDate := httptest.NewRequest("GET", "/tasks/task-1/action-items?from_date=2026-10-05&page_token="+token, nil)
	if _, err := parseListRequestWithFromDate(changedDate, codec, "user-1", "task_action_items", "task-1", "occurrence_date_asc_position_asc_series_id_asc", schema, true); err == nil {
		t.Error("token accepted a different from_date")
	}
}

func TestProjectListCursorUsesFreshAsOfAndDoesNotPersistIt(t *testing.T) {
	codec := listTestCodec()
	firstRequest := httptest.NewRequest("GET", "/projects?sort_by=progress&sort_order=asc", nil)
	first, _, err := parseProjectListRequest(firstRequest, codec, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	oldAsOf := "2020-01-01T00:00:00Z"
	token, err := pagination.Encode(codec, first.Scope, listAnchor{
		ID: "anchor-project", AsOf: oldAsOf, Direction: "forward", Progress: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().UTC()
	continued := httptest.NewRequest("GET", "/projects?sort_by=progress&sort_order=asc&page_token="+token, nil)
	parsed, request, err := parseProjectListRequest(continued, codec, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if request.AsOf.IsZero() || request.AsOf.Before(before) || request.AsOf.Format(time.RFC3339Nano) == oldAsOf {
		t.Errorf("Project list continuation AsOf=%s; want fresh current time instead of cursor AsOf %s", request.AsOf, oldAsOf)
	}
	if parsed.Anchor == nil {
		t.Fatal("Project cursor did not decode")
	}
	if parsed.Anchor.AsOf != "" {
		t.Errorf("decoded Project cursor retained AsOf=%q; want only the live keyset tuple", parsed.Anchor.AsOf)
	}
	encoded, err := encodeProjectCursor(codec, parsed.Scope, projectusecase.CursorAnchor{
		ID: "next-project", AsOf: oldAsOf, Direction: "forward", Progress: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := pagination.Decode[listAnchor](codec, encoded, parsed.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.AsOf != "" {
		t.Errorf("encoded Project cursor AsOf=%q; want no frozen evaluation time", decoded.AsOf)
	}
}

func TestParseListRequestRejectsLegacyAndInvalidParameters(t *testing.T) {
	codec, _ := pagination.NewCodec([]byte("01234567890123456789012345678901"))
	schema := listEnvelope[TaskResponse]{}
	for _, tc := range []struct{ uri, field string }{
		{"/tasks?limit=10", "limit"}, {"/tasks?offset=2", "offset"}, {"/tasks?page_size=-1", "page_size"}, {"/tasks?page_size=1&page_size=2", "page_size"},
		{"/tasks?fields=items(unknown)", "fields"}, {"/tasks?fields=", "fields"}, {"/tasks?page_token=broken", "page_token"},
	} {
		request := httptest.NewRequest("GET", tc.uri, nil)
		if _, err := parseListRequest(request, codec, "user-1", "tasks", "", "created_at_desc_id_desc", schema); err == nil {
			t.Errorf("accepted invalid request %s", tc.uri)
		} else if field := invalidListField(err); field != tc.field {
			t.Errorf("invalid request %s error field=%q, want %q", tc.uri, field, tc.field)
		}
	}
}

func TestWriteListErrorReportsInvalidQueryField(t *testing.T) {
	request := httptest.NewRequest("GET", "/tasks?page_size=-1", nil)
	_, err := parseListRequest(request, listTestCodec(), "user-1", "tasks", "", "created_at_desc_id_desc", listEnvelope[TaskResponse]{})
	if err == nil {
		t.Fatal("expected invalid page_size")
	}
	response := httptest.NewRecorder()
	writeListError(response, errSpecTasksListFailed, err)
	var body struct {
		Error struct {
			Details []struct {
				Field string `json:"field"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 400 || len(body.Error.Details) != 1 || body.Error.Details[0].Field != "page_size" {
		t.Fatalf("error response=%d %+v", response.Code, body)
	}
}

func TestWriteListResponseAppliesMaskAndReturnsArray(t *testing.T) {
	mask, err := fieldmask.Parse("items(id,title),next_page_token", listEnvelope[TaskResponse]{})
	if err != nil {
		t.Fatal(err)
	}
	request := listRequest{Mask: mask, Scope: pagination.Scope{UserID: "user-1", List: "tasks", Order: "order", Fields: mask.Canonical()}}
	response := httptest.NewRecorder()
	writeListResponse(response, []TaskResponse{{ID: "task-1", Title: "Selected", Description: "hidden"}}, []listAnchor{{ID: "task-1"}}, false, request, nil, tagListFailure)
	if response.Code != 200 || response.Body.String() == "" {
		t.Fatalf("response status/body=%d/%s", response.Code, response.Body.String())
	}
	if got := response.Body.String(); got != "{\"items\":[{\"id\":\"task-1\",\"title\":\"Selected\"}],\"next_page_token\":\"\"}" {
		t.Fatalf("masked JSON=%s", got)
	}
}
