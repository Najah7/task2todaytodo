//go:build integration

package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type tasksPageIDs struct{}

func (tasksPageIDs) Generate() string { return ulid.Make().String() }

type tasksPageREST struct {
	pool   *pgxpool.Pool
	userID string
	router *chi.Mux
}

func newTasksPageREST(t *testing.T) *tasksPageREST {
	t.Helper()
	pool := testdb.Open(t)
	userID := ulid.Make().String()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Tasks','Page',$2,'unused','UTC')`, userID, userID+"@tasks-page.test"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM action_items WHERE task_id IN (SELECT id FROM tasks WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM task_revisions WHERE id IN (SELECT id FROM tasks WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM tasks WHERE user_id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})

	ids := tasksPageIDs{}
	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule)
	usecases := application.NewUseCase(store.Auth, store.Project, store.Task, store.Schedule, store.Tag, uow.Project, uow.Task, uow.Schedule, ids, nil)
	pageTokens, err := pagination.NewCodec([]byte(strings.Repeat("t", 32)))
	if err != nil {
		t.Fatal(err)
	}
	taskHandler := restapi.NewTaskHandler(usecases.Task, ids, pageTokens)
	actionItemHandler := restapi.NewActionItemHandler(usecases.ActionItem, ids, pageTokens)
	router := chi.NewRouter()
	router.Post("/tasks", taskHandler.Create)
	router.Get("/tasks", taskHandler.List)
	router.Get("/tasks/{id}", taskHandler.Get)
	router.Post("/tasks/{id}:complete", taskHandler.Complete)
	router.Get("/tasks/{taskId}/action-items", actionItemHandler.List)
	router.Post("/tasks/{taskId}/action-items", actionItemHandler.Create)
	router.Patch("/tasks/{taskId}/action-items/{id}", actionItemHandler.Update)
	router.Post("/tasks/{taskId}/action-items/{id}:complete", actionItemHandler.Complete)
	router.Delete("/tasks/{taskId}/action-items/{id}", actionItemHandler.Delete)
	return &tasksPageREST{pool: pool, userID: userID, router: router}
}

func (api *tasksPageREST) request(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), restapi.UserIDContextKey, api.userID))
	recorder := httptest.NewRecorder()
	api.router.ServeHTTP(recorder, req)
	return recorder
}

func tasksPageQuery(values url.Values) string {
	return "/tasks?" + values.Encode()
}

type tasksPageTask struct {
	ID                       string `json:"id"`
	Title                    string `json:"title"`
	Priority                 string `json:"priority"`
	Status                   string `json:"status"`
	Progress                 int    `json:"progress"`
	ActionItemCount          int    `json:"action_item_count"`
	ActionItemCompletedCount int    `json:"action_item_completed_count"`
	ManualEstimatedMinutes   *int   `json:"manual_estimated_minutes"`
	EstimatedMinutes         *int   `json:"estimated_minutes"`
	RemainingDays            *int   `json:"remaining_days"`
	EstimateSource           string `json:"estimate_source"`
	CanUpdate                bool   `json:"can_update"`
}

type tasksPageActionItem struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Priority         string `json:"priority"`
	EstimatedMinutes *int   `json:"estimated_minutes"`
	Completed        bool   `json:"completed"`
	SeriesID         string `json:"series_id"`
	OccurrenceDate   string `json:"occurrence_date"`
}

type tasksPageList struct {
	Items                    []tasksPageTask `json:"items"`
	NextPageToken            string          `json:"next_page_token"`
	PreviousPageToken        string          `json:"previous_page_token"`
	TotalCount               int             `json:"total_count"`
	StatusCounts             map[string]int  `json:"status_counts"`
	ActionItemTotalCount     int             `json:"action_item_total_count"`
	ActionItemCompletedCount int             `json:"action_item_completed_count"`
	EstimatedMinutesTotal    *int            `json:"estimated_minutes_total"`
}

func decodeTasksPageTask(t *testing.T, body []byte) tasksPageTask {
	t.Helper()
	var task tasksPageTask
	if err := json.Unmarshal(body, &task); err != nil {
		t.Fatalf("decode Task response: %v; body=%s", err, body)
	}
	return task
}

func decodeTasksPageList(t *testing.T, response *httptest.ResponseRecorder) tasksPageList {
	t.Helper()
	var page tasksPageList
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode Task page: %v; body=%s", err, response.Body.String())
	}
	return page
}

func taskPageEstimateValue(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func createTasksPageTask(t *testing.T, api *tasksPageREST, body string) tasksPageTask {
	t.Helper()
	var request map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatalf("decode Task fixture: %v", err)
	}
	var children []json.RawMessage
	if raw, ok := request["action_items"]; ok {
		if err := json.Unmarshal(raw, &children); err != nil {
			t.Fatalf("decode ActionItem fixtures: %v", err)
		}
		delete(request, "action_items")
	}
	taskBody, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := api.request(http.MethodPost, "/tasks", string(taskBody))
	if response.Code != http.StatusCreated {
		t.Fatalf("POST /tasks status/body=%d/%s", response.Code, response.Body.String())
	}
	task := decodeTasksPageTask(t, response.Body.Bytes())
	createdDetail := api.request(http.MethodGet, "/tasks/"+task.ID, "")
	if createdDetail.Code != http.StatusOK {
		t.Fatalf("GET Task after POST status/body=%d/%s", createdDetail.Code, createdDetail.Body.String())
	}
	var createdSnapshot struct {
		Task tasksPageTask `json:"task"`
	}
	if err := json.Unmarshal(createdDetail.Body.Bytes(), &createdSnapshot); err != nil {
		t.Fatalf("decode Task after POST: %v", err)
	}
	if !task.CanUpdate || task.CanUpdate != createdSnapshot.Task.CanUpdate || task.EstimateSource != createdSnapshot.Task.EstimateSource || taskPageEstimateValue(task.ManualEstimatedMinutes) != taskPageEstimateValue(createdSnapshot.Task.ManualEstimatedMinutes) {
		t.Fatalf("POST Task projection=%+v differs from GET projection=%+v; want can_update and estimate fields aligned", task, createdSnapshot.Task)
	}
	for _, child := range children {
		created := api.request(http.MethodPost, fmt.Sprintf("/tasks/%s/action-items", task.ID), string(child))
		if created.Code != http.StatusCreated {
			t.Fatalf("POST ActionItem fixture status/body=%d/%s", created.Code, created.Body.String())
		}
	}
	if len(children) > 0 {
		got := api.request(http.MethodGet, "/tasks/"+task.ID, "")
		if got.Code != http.StatusOK {
			t.Fatalf("GET Task after ActionItem fixtures status/body=%d/%s", got.Code, got.Body.String())
		}
		var detail struct {
			Task tasksPageTask `json:"task"`
		}
		if err := json.Unmarshal(got.Body.Bytes(), &detail); err != nil {
			t.Fatalf("decode Task after ActionItem fixtures: %v", err)
		}
		task = detail.Task
	}
	return task
}

func TestRESTTaskCreateRejectsNestedActionItemsAndUsesActionItemEstimateSum(t *testing.T) {
	api := newTasksPageREST(t)
	invalid := api.request(http.MethodPost, "/tasks", `{"title":"Nested task","action_items":[{"title":"Child"}]}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("nested POST /tasks status/body=%d/%s; want 400", invalid.Code, invalid.Body.String())
	}
	var taskCount, childCount int
	if err := api.pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM tasks WHERE user_id=$1`, api.userID).Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if err := api.pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM action_items ai JOIN tasks t ON t.id=ai.task_id WHERE t.user_id=$1`, api.userID).Scan(&childCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 0 || childCount != 0 {
		t.Fatalf("rejected nested create persisted %d Tasks and %d ActionItems", taskCount, childCount)
	}

	manual := 90
	task := createTasksPageTask(t, api, `{"title":"Sum task","priority":"high","manual_estimated_minutes":90,"action_items":[{"title":"First","estimated_minutes":35},{"title":"No estimate","estimated_minutes":null},{"title":"Zero estimate","estimated_minutes":0,"priority":"low"}]}`)
	if task.EstimatedMinutes == nil || *task.EstimatedMinutes != 35 || task.ManualEstimatedMinutes == nil || *task.ManualEstimatedMinutes != manual || task.EstimateSource != "action_items" {
		t.Fatalf("Task estimate projection=%+v; want manual 90 and effective ActionItem sum 35", task)
	}
	if task.ActionItemCount != 3 || task.ActionItemCompletedCount != 0 {
		t.Fatalf("Task ActionItem counts=%+v; want 3/0", task)
	}
	actionItemsResponse := api.request(http.MethodGet, fmt.Sprintf("/tasks/%s/action-items?page_size=10", task.ID), "")
	if actionItemsResponse.Code != http.StatusOK {
		t.Fatalf("GET ActionItems status/body=%d/%s", actionItemsResponse.Code, actionItemsResponse.Body.String())
	}
	var actionItems struct {
		Items []tasksPageActionItem `json:"items"`
	}
	if err := json.Unmarshal(actionItemsResponse.Body.Bytes(), &actionItems); err != nil {
		t.Fatalf("decode ActionItems: %v; body=%s", err, actionItemsResponse.Body.String())
	}
	if len(actionItems.Items) != 3 || actionItems.Items[0].Priority != "high" || actionItems.Items[1].Priority != "high" || actionItems.Items[2].Priority != "low" {
		t.Fatalf("ActionItem priorities/order=%+v; want inherited high, inherited high, explicit low", actionItems.Items)
	}
	if actionItems.Items[0].EstimatedMinutes == nil || *actionItems.Items[0].EstimatedMinutes != 35 || actionItems.Items[1].EstimatedMinutes != nil || actionItems.Items[2].EstimatedMinutes == nil || *actionItems.Items[2].EstimatedMinutes != 0 {
		t.Fatalf("ActionItem nullable estimates=%+v", actionItems.Items)
	}

	zero := createTasksPageTask(t, api, `{"title":"Manual zero","manual_estimated_minutes":0}`)
	if zero.EstimatedMinutes == nil || *zero.EstimatedMinutes != 0 || zero.EstimateSource != "manual" {
		t.Fatalf("manual zero estimate=%+v; want effective zero from manual source", zero)
	}
	allNull := createTasksPageTask(t, api, `{"title":"Null estimate","manual_estimated_minutes":90,"action_items":[{"title":"No estimate"}]}`)
	if allNull.EstimatedMinutes != nil || allNull.ManualEstimatedMinutes == nil || *allNull.ManualEstimatedMinutes != 90 || allNull.EstimateSource != "action_items" {
		t.Fatalf("all-null child estimate=%+v; want NULL effective estimate and retained manual 90", allNull)
	}
	noEstimate := createTasksPageTask(t, api, `{"title":"No manual estimate"}`)
	if noEstimate.EstimatedMinutes != nil || noEstimate.ManualEstimatedMinutes != nil || noEstimate.EstimateSource != "manual" {
		t.Fatalf("unset manual estimate=%+v; want nullable manual source", noEstimate)
	}
	zeroSummaryResponse := api.request(http.MethodGet, tasksPageQuery(url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {"Manual zero"}}), "")
	zeroSummary := decodeTasksPageList(t, zeroSummaryResponse)
	if zeroSummary.EstimatedMinutesTotal == nil || *zeroSummary.EstimatedMinutesTotal != 0 {
		t.Fatalf("zero-only estimate footer=%+v; want non-NULL zero", zeroSummary)
	}
	nullSummaryResponse := api.request(http.MethodGet, tasksPageQuery(url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {"No manual estimate"}}), "")
	nullSummary := decodeTasksPageList(t, nullSummaryResponse)
	if nullSummary.EstimatedMinutesTotal != nil {
		t.Fatalf("all-NULL estimate footer=%+v; want NULL", nullSummary)
	}
}

func TestRESTTaskListFiltersSummaryAcrossCursorPagesAndReflectsCompletion(t *testing.T) {
	api := newTasksPageREST(t)
	alpha := createTasksPageTask(t, api, `{"title":"Alpha task","manual_estimated_minutes":99,"action_items":[{"title":"One","estimated_minutes":10},{"title":"Two","estimated_minutes":20}]}`)
	if _, err := api.pool.Exec(t.Context(), `UPDATE tasks SET status='in_progress' WHERE id=$1`, alpha.ID); err != nil {
		t.Fatalf("set Alpha status for tab-count coverage: %v", err)
	}
	createTasksPageTask(t, api, `{"title":"Beta task","manual_estimated_minutes":15}`)
	createTasksPageTask(t, api, `{"title":"Gamma task","action_items":[{"title":"Zero","estimated_minutes":0}]}`)
	actionItemsResponse := api.request(http.MethodGet, fmt.Sprintf("/tasks/%s/action-items?page_size=10", alpha.ID), "")
	if actionItemsResponse.Code != http.StatusOK {
		t.Fatalf("GET Alpha ActionItems status/body=%d/%s", actionItemsResponse.Code, actionItemsResponse.Body.String())
	}
	var actionItems struct {
		Items []tasksPageActionItem `json:"items"`
	}
	if err := json.Unmarshal(actionItemsResponse.Body.Bytes(), &actionItems); err != nil || len(actionItems.Items) != 2 {
		t.Fatalf("decode Alpha ActionItems: err=%v body=%s", err, actionItemsResponse.Body.String())
	}
	completeFirst := api.request(http.MethodPost, fmt.Sprintf("/tasks/%s/action-items/%s:complete", alpha.ID, actionItems.Items[0].ID), `{}`)
	if completeFirst.Code != http.StatusOK {
		t.Fatalf("complete first ActionItem status/body=%d/%s", completeFirst.Code, completeFirst.Body.String())
	}

	firstResponse := api.request(http.MethodGet, "/tasks?status=all&due_filter=all&sort_by=title&sort_order=asc&page_size=1", "")
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("GET first Task page status/body=%d/%s", firstResponse.Code, firstResponse.Body.String())
	}
	first := decodeTasksPageList(t, firstResponse)
	if len(first.Items) != 1 || first.Items[0].ID != alpha.ID || first.Items[0].Progress != 50 || first.Items[0].ActionItemCount != 2 || first.Items[0].ActionItemCompletedCount != 1 || first.Items[0].EstimatedMinutes == nil || *first.Items[0].EstimatedMinutes != 30 {
		t.Fatalf("first Task row=%+v; want Alpha progress/counts and child estimate sum", first.Items)
	}
	if first.TotalCount != 3 || first.ActionItemTotalCount != 3 || first.ActionItemCompletedCount != 1 || first.EstimatedMinutesTotal == nil || *first.EstimatedMinutesTotal != 45 {
		t.Fatalf("first-page footer summary=%+v; want all 3 Tasks/3 items/1 completed/45 minutes", first)
	}
	if first.NextPageToken == "" {
		t.Fatalf("first Task page has no continuation token: %+v", first)
	}
	secondResponse := api.request(http.MethodGet, "/tasks?status=all&due_filter=all&sort_by=title&sort_order=asc&page_size=1&page_token="+first.NextPageToken, "")
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("GET second Task page status/body=%d/%s", secondResponse.Code, secondResponse.Body.String())
	}
	second := decodeTasksPageList(t, secondResponse)
	if len(second.Items) != 1 || second.Items[0].Title != "Beta task" || second.TotalCount != first.TotalCount || second.ActionItemTotalCount != first.ActionItemTotalCount || second.ActionItemCompletedCount != first.ActionItemCompletedCount || second.EstimatedMinutesTotal == nil || *second.EstimatedMinutesTotal != *first.EstimatedMinutesTotal {
		t.Fatalf("second page/summary=%+v; want next Task and same all-page footer", second)
	}

	filteredResponse := api.request(http.MethodGet, "/tasks?status=all&due_filter=all&title=Beta", "")
	if filteredResponse.Code != http.StatusOK {
		t.Fatalf("GET title-filtered Tasks status/body=%d/%s", filteredResponse.Code, filteredResponse.Body.String())
	}
	filtered := decodeTasksPageList(t, filteredResponse)
	if filtered.TotalCount != 1 || len(filtered.Items) != 1 || filtered.Items[0].Title != "Beta task" || filtered.ActionItemTotalCount != 0 {
		t.Fatalf("title-filtered Task page=%+v; want only Beta and zero child count", filtered)
	}
	openResponse := api.request(http.MethodGet, "/tasks?status=open&due_filter=all&title=task", "")
	if openResponse.Code != http.StatusOK {
		t.Fatalf("GET status-filtered Tasks status/body=%d/%s", openResponse.Code, openResponse.Body.String())
	}
	open := decodeTasksPageList(t, openResponse)
	if open.TotalCount != 2 || open.StatusCounts["open"] != 2 || open.StatusCounts["in_progress"] != 1 {
		t.Fatalf("status filter or status counts=%+v; want 2 open rows and counts excluding status filter", open)
	}

	completeSecond := api.request(http.MethodPost, fmt.Sprintf("/tasks/%s/action-items/%s:complete", alpha.ID, actionItems.Items[1].ID), `{}`)
	if completeSecond.Code != http.StatusOK {
		t.Fatalf("complete second ActionItem status/body=%d/%s", completeSecond.Code, completeSecond.Body.String())
	}
	alphaResponse := api.request(http.MethodGet, "/tasks?status=all&due_filter=all&title=Alpha", "")
	if alphaResponse.Code != http.StatusOK {
		t.Fatalf("GET completed Alpha page status/body=%d/%s", alphaResponse.Code, alphaResponse.Body.String())
	}
	completed := decodeTasksPageList(t, alphaResponse)
	if len(completed.Items) != 1 || completed.Items[0].Progress != 100 || completed.Items[0].ActionItemCompletedCount != 2 || completed.ActionItemCompletedCount != 2 {
		t.Fatalf("completed Task counts/progress=%+v; want 100%%, 2/2", completed)
	}
}

func TestRESTTaskListDueFiltersAndNullLastOrdering(t *testing.T) {
	api := newTasksPageREST(t)
	today := time.Now().UTC()
	dueDate := func(days int) string { return today.AddDate(0, 0, days).Format("2006-01-02") }
	createTasksPageTask(t, api, fmt.Sprintf(`{"title":"Due overdue","due_date":%q}`, dueDate(-1)))
	createTasksPageTask(t, api, fmt.Sprintf(`{"title":"Due today","due_date":%q}`, dueDate(0)))
	createTasksPageTask(t, api, fmt.Sprintf(`{"title":"Due soon","due_date":%q}`, dueDate(1)))
	createTasksPageTask(t, api, fmt.Sprintf(`{"title":"Due last soon","due_date":%q}`, dueDate(14)))
	createTasksPageTask(t, api, fmt.Sprintf(`{"title":"Due outside window","due_date":%q}`, dueDate(15)))
	createTasksPageTask(t, api, `{"title":"Due no date"}`)
	for _, test := range []struct {
		filter string
		want   int
	}{
		{"overdue", 1}, {"today", 1}, {"due_soon", 3}, {"no_due", 1},
	} {
		response := api.request(http.MethodGet, tasksPageQuery(url.Values{"status": {"all"}, "due_filter": {test.filter}, "title": {"Due"}}), "")
		if response.Code != http.StatusOK {
			t.Fatalf("GET due_filter=%s status/body=%d/%s", test.filter, response.Code, response.Body.String())
		}
		page := decodeTasksPageList(t, response)
		if page.TotalCount != test.want || len(page.Items) != test.want {
			t.Fatalf("due_filter=%s returned %d items / total %d; want %d", test.filter, len(page.Items), page.TotalCount, test.want)
		}
		if test.filter == "overdue" && (page.Items[0].RemainingDays == nil || *page.Items[0].RemainingDays >= 0) {
			t.Fatalf("overdue Task remaining_days=%v; want a negative value", taskPageEstimateValue(page.Items[0].RemainingDays))
		}
		if test.filter == "today" && (page.Items[0].RemainingDays == nil || *page.Items[0].RemainingDays != 0) {
			t.Fatalf("today Task remaining_days=%v; want zero", taskPageEstimateValue(page.Items[0].RemainingDays))
		}
		if test.filter == "no_due" && page.Items[0].RemainingDays != nil {
			t.Fatalf("undated Task remaining_days=%v; want null", taskPageEstimateValue(page.Items[0].RemainingDays))
		}
	}
	response := api.request(http.MethodGet, tasksPageQuery(url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {"Due"}, "sort_by": {"due_date"}, "sort_order": {"asc"}, "page_size": {"10"}}), "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET due-sorted Tasks status/body=%d/%s", response.Code, response.Body.String())
	}
	page := decodeTasksPageList(t, response)
	wantOrder := []string{"Due overdue", "Due today", "Due soon", "Due last soon", "Due outside window", "Due no date"}
	if len(page.Items) != len(wantOrder) {
		t.Fatalf("due-sorted Task count=%d; want %d", len(page.Items), len(wantOrder))
	}
	for index, title := range wantOrder {
		if page.Items[index].Title != title {
			t.Fatalf("due sort order[%d]=%q; want %q", index, page.Items[index].Title, title)
		}
	}

	createTasksPageTask(t, api, fmt.Sprintf(`{"title":"Same title","due_date":%q}`, dueDate(2)))
	createTasksPageTask(t, api, fmt.Sprintf(`{"title":"Same title","due_date":%q}`, dueDate(2)))
	for _, test := range []struct {
		title  string
		sortBy string
	}{
		{title: "Same title", sortBy: "title"},
		{title: "Same title", sortBy: "due_date"},
	} {
		seen := map[string]bool{}
		pageToken := ""
		for {
			query := url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {test.title}, "sort_by": {test.sortBy}, "sort_order": {"asc"}, "page_size": {"1"}}
			if pageToken != "" {
				query.Set("page_token", pageToken)
			}
			response := api.request(http.MethodGet, tasksPageQuery(query), "")
			if response.Code != http.StatusOK {
				t.Fatalf("GET tied %s sort status/body=%d/%s", test.sortBy, response.Code, response.Body.String())
			}
			page := decodeTasksPageList(t, response)
			if len(page.Items) != 1 || seen[page.Items[0].ID] {
				t.Fatalf("tied %s cursor page=%+v; duplicate or missing row", test.sortBy, page)
			}
			seen[page.Items[0].ID] = true
			if pageToken == "" && page.NextPageToken != "" {
				firstID := page.Items[0].ID
				nextQuery := url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {test.title}, "sort_by": {test.sortBy}, "sort_order": {"asc"}, "page_size": {"1"}, "page_token": {page.NextPageToken}}
				nextResponse := api.request(http.MethodGet, tasksPageQuery(nextQuery), "")
				if nextResponse.Code != http.StatusOK {
					t.Fatalf("GET tied %s next page status/body=%d/%s", test.sortBy, nextResponse.Code, nextResponse.Body.String())
				}
				nextPage := decodeTasksPageList(t, nextResponse)
				if len(nextPage.Items) != 1 || nextPage.PreviousPageToken == "" {
					t.Fatalf("tied %s next page lacks previous cursor: %+v", test.sortBy, nextPage)
				}
				previousQuery := url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {test.title}, "sort_by": {test.sortBy}, "sort_order": {"asc"}, "page_size": {"1"}, "page_token": {nextPage.PreviousPageToken}}
				previousResponse := api.request(http.MethodGet, tasksPageQuery(previousQuery), "")
				if previousResponse.Code != http.StatusOK {
					t.Fatalf("GET tied %s previous page status/body=%d/%s", test.sortBy, previousResponse.Code, previousResponse.Body.String())
				}
				previousPage := decodeTasksPageList(t, previousResponse)
				if len(previousPage.Items) != 1 || previousPage.Items[0].ID != firstID || previousPage.PreviousPageToken != "" {
					t.Fatalf("tied %s previous page=%+v; want row %s and no earlier cursor", test.sortBy, previousPage, firstID)
				}
			}
			pageToken = page.NextPageToken
			if pageToken == "" {
				break
			}
		}
		if len(seen) != 2 {
			t.Fatalf("tied %s pagination returned %d unique Tasks; want 2", test.sortBy, len(seen))
		}
	}
}

func TestRESTTaskEstimateMatchesAllPagedRecurringActionItemRows(t *testing.T) {
	api := newTasksPageREST(t)
	task := createTasksPageTask(t, api, `{"title":"Recurring estimate","manual_estimated_minutes":120,"priority":"high"}`)
	today := calendar.NormalizeCalendarDate(time.Now().UTC())
	todayText := today.Format("2006-01-02")
	weekday := strings.ToLower(today.Weekday().String()[:3])
	actionItemResponse := api.request(http.MethodPost, fmt.Sprintf("/tasks/%s/action-items", task.ID), fmt.Sprintf(`{"title":"Weekly item","due_date":%q,"estimated_minutes":17,"interval_weeks":1,"frequencies":[%q]}`, todayText, weekday))
	if actionItemResponse.Code != http.StatusCreated {
		t.Fatalf("create recurring ActionItem status/body=%d/%s", actionItemResponse.Code, actionItemResponse.Body.String())
	}
	var root tasksPageActionItem
	if err := json.Unmarshal(actionItemResponse.Body.Bytes(), &root); err != nil {
		t.Fatalf("decode recurring ActionItem: %v; body=%s", err, actionItemResponse.Body.String())
	}
	if root.SeriesID != root.ID || root.Priority != "high" || root.EstimatedMinutes == nil || *root.EstimatedMinutes != 17 {
		t.Fatalf("recurring root=%+v; want saved root with inherited priority/high and 17 minutes", root)
	}

	windowEnd := calendar.AddCalendarMonthClamped(today)
	outOfRangeDate := windowEnd.AddDate(0, 0, 1).Format("2006-01-02")
	if _, err := api.pool.Exec(t.Context(), `
		INSERT INTO action_items (id, task_id, title, description, due_date, completed, position, series_id, occurrence_date, timezone, is_exception, repeat_state, frequency_anchor_date, interval_weeks, estimated_minutes, priority)
		SELECT $1, task_id, title, description, due_date, false, position + 1, id, $2::date, timezone, true, NULL, NULL, 0, 999, 'low'
		FROM action_items WHERE id=$3 AND task_id=$4`, ulid.Make().String(), outOfRangeDate, root.ID, task.ID); err != nil {
		t.Fatalf("insert saved out-of-window recurring exception: %v", err)
	}
	pastDate := today.AddDate(0, 0, -7).Format("2006-01-02")
	if _, err := api.pool.Exec(t.Context(), `
		INSERT INTO action_items (id, task_id, title, description, due_date, completed, position, series_id, occurrence_date, timezone, is_exception, repeat_state, frequency_anchor_date, interval_weeks, estimated_minutes, priority)
		SELECT $1, task_id, title, description, due_date, true, position + 1, id, $2::date, timezone, true, NULL, NULL, 0, 17, 'high'
		FROM action_items WHERE id=$3 AND task_id=$4`, ulid.Make().String(), pastDate, root.ID, task.ID); err != nil {
		t.Fatalf("insert saved completed past occurrence: %v", err)
	}

	var projected []tasksPageActionItem
	pageToken := ""
	for {
		query := url.Values{"page_size": {"2"}}
		if pageToken != "" {
			query.Set("page_token", pageToken)
		}
		response := api.request(http.MethodGet, fmt.Sprintf("/tasks/%s/action-items?%s", task.ID, query.Encode()), "")
		if response.Code != http.StatusOK {
			t.Fatalf("list recurring ActionItems status/body=%d/%s", response.Code, response.Body.String())
		}
		var page struct {
			Items         []tasksPageActionItem `json:"items"`
			NextPageToken string                `json:"next_page_token"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode recurring ActionItem page: %v; body=%s", err, response.Body.String())
		}
		projected = append(projected, page.Items...)
		pageToken = page.NextPageToken
		if pageToken == "" {
			break
		}
	}
	if len(projected) != 6 {
		t.Fatalf("all ActionItem pages returned %d occurrences; want one saved past row plus 5 in-window weekly occurrences", len(projected))
	}
	var actionEstimate, completedCount int
	for _, item := range projected {
		if item.Priority != "high" || item.EstimatedMinutes == nil || *item.EstimatedMinutes != 17 {
			t.Fatalf("projected occurrence lost inherited planning values: %+v", item)
		}
		actionEstimate += *item.EstimatedMinutes
		if item.Completed {
			completedCount++
		}
	}
	if actionEstimate != 102 || completedCount != 1 {
		t.Fatalf("ActionItem list estimate/completed=%d/%d; want 102 minutes and one saved past completion", actionEstimate, completedCount)
	}
	listResponse := api.request(http.MethodGet, tasksPageQuery(url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {"Recurring estimate"}}), "")
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list Task estimate status/body=%d/%s", listResponse.Code, listResponse.Body.String())
	}
	list := decodeTasksPageList(t, listResponse)
	if len(list.Items) != 1 || list.Items[0].ActionItemCount != len(projected) || list.Items[0].ActionItemCompletedCount != completedCount || list.Items[0].Progress != 16 || list.Items[0].EstimatedMinutes == nil || *list.Items[0].EstimatedMinutes != actionEstimate || list.Items[0].EstimateSource != "action_items" || list.Items[0].Priority != "high" {
		t.Fatalf("Task list projection=%+v; want same all-page ActionItem count/sum and inherited priority", list.Items)
	}
	assertTaskDetailProjection := func(wantCompleted, wantProgress int) {
		detailResponse := api.request(http.MethodGet, "/tasks/"+task.ID, "")
		if detailResponse.Code != http.StatusOK {
			t.Fatalf("GET Task details status/body=%d/%s", detailResponse.Code, detailResponse.Body.String())
		}
		var detail struct {
			Task tasksPageTask `json:"task"`
		}
		if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
			t.Fatalf("decode Task details: %v; body=%s", err, detailResponse.Body.String())
		}
		if detail.Task.ActionItemCount != len(projected) || detail.Task.ActionItemCompletedCount != wantCompleted || detail.Task.Progress != wantProgress || detail.Task.EstimatedMinutes == nil || *detail.Task.EstimatedMinutes != actionEstimate || detail.Task.EstimateSource != "action_items" || detail.Task.Priority != "high" {
			t.Fatalf("Task detail projection=%+v; want same list count/sum, %d completed, and %d%% progress", detail.Task, wantCompleted, wantProgress)
		}
	}
	assertTaskDetailProjection(1, 16)

	var openOccurrence *tasksPageActionItem
	for i := range projected {
		if !projected[i].Completed {
			openOccurrence = &projected[i]
			break
		}
	}
	if openOccurrence == nil {
		t.Fatal("expected an incomplete projected occurrence")
	}
	complete := api.request(http.MethodPost, fmt.Sprintf("/tasks/%s/action-items/%s:complete", task.ID, openOccurrence.ID), fmt.Sprintf(`{"occurrence_date":%q}`, openOccurrence.OccurrenceDate))
	if complete.Code != http.StatusOK {
		t.Fatalf("complete recurring occurrence status/body=%d/%s", complete.Code, complete.Body.String())
	}
	afterCompleteResponse := api.request(http.MethodGet, fmt.Sprintf("/tasks/%s/action-items", task.ID), "")
	if afterCompleteResponse.Code != http.StatusOK {
		t.Fatalf("re-list ActionItems after completion status/body=%d/%s", afterCompleteResponse.Code, afterCompleteResponse.Body.String())
	}
	var afterComplete struct {
		Items []tasksPageActionItem `json:"items"`
	}
	if err := json.Unmarshal(afterCompleteResponse.Body.Bytes(), &afterComplete); err != nil {
		t.Fatalf("decode ActionItems after completion: %v; body=%s", err, afterCompleteResponse.Body.String())
	}
	var afterCompleteEstimate int
	for _, item := range afterComplete.Items {
		if item.EstimatedMinutes != nil {
			afterCompleteEstimate += *item.EstimatedMinutes
		}
	}
	if len(afterComplete.Items) != len(projected) || afterCompleteEstimate != actionEstimate {
		t.Fatalf("after-complete ActionItem projection=%+v sum=%d; want %d rows and unchanged %d-minute estimate", afterComplete.Items, afterCompleteEstimate, len(projected), actionEstimate)
	}
	completedResponse := api.request(http.MethodGet, tasksPageQuery(url.Values{"status": {"all"}, "due_filter": {"all"}, "title": {"Recurring estimate"}}), "")
	completed := decodeTasksPageList(t, completedResponse)
	if len(completed.Items) != 1 || completed.Items[0].ActionItemCount != len(projected) || completed.Items[0].ActionItemCompletedCount != 2 || completed.Items[0].Progress != 33 || completed.Items[0].EstimatedMinutes == nil || *completed.Items[0].EstimatedMinutes != actionEstimate || completed.Items[0].Priority != "high" {
		var gotEstimate *int
		if len(completed.Items) > 0 {
			gotEstimate = completed.Items[0].EstimatedMinutes
		}
		t.Fatalf("completed recurring Task projection=%+v estimate=%v; want 2/6, 33%% progress, %d minutes and high priority", completed.Items, taskPageEstimateValue(gotEstimate), actionEstimate)
	}
	assertTaskDetailProjection(2, 33)
}

func TestRESTActionItemCreateFailureLeavesTaskForIndependentRetry(t *testing.T) {
	api := newTasksPageREST(t)
	suffix := strings.ToLower(ulid.Make().String())
	functionName, triggerName := "fail_action_item_"+suffix, "fail_action_item_"+suffix
	if _, err := api.pool.Exec(t.Context(), fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.title = 'Force nested insert failure' THEN
				RAISE EXCEPTION 'forced ActionItem insert failure';
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER %s BEFORE INSERT ON action_items FOR EACH ROW EXECUTE FUNCTION %s();`, functionName, triggerName, functionName)); err != nil {
		t.Fatalf("create scoped failure trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = api.pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %s ON action_items; DROP FUNCTION IF EXISTS %s();`, triggerName, functionName))
	})

	task := createTasksPageTask(t, api, `{"title":"Independent item create"}`)
	ok := api.request(http.MethodPost, fmt.Sprintf("/tasks/%s/action-items", task.ID), `{"title":"First valid child","estimated_minutes":25}`)
	if ok.Code != http.StatusCreated {
		t.Fatalf("first ActionItem status/body=%d/%s", ok.Code, ok.Body.String())
	}
	response := api.request(http.MethodPost, fmt.Sprintf("/tasks/%s/action-items", task.ID), `{"title":"Force nested insert failure","estimated_minutes":35}`)
	if response.Code < http.StatusInternalServerError {
		t.Fatalf("ActionItem create failure status/body=%d/%s; want server error", response.Code, response.Body.String())
	}
	var taskCount, childCount int
	if err := api.pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM tasks WHERE user_id=$1 AND title='Independent item create'`, api.userID).Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if err := api.pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM action_items ai JOIN tasks t ON t.id=ai.task_id WHERE t.user_id=$1 AND t.title='Independent item create'`, api.userID).Scan(&childCount); err != nil {
		t.Fatal(err)
	}
	if taskCount != 1 || childCount != 1 {
		t.Fatalf("independent create state has %d Tasks and %d ActionItems; want Task and prior successful item preserved", taskCount, childCount)
	}
}

func TestRESTDeletingLastActionItemRestoresTaskManualEstimate(t *testing.T) {
	api := newTasksPageREST(t)
	task := createTasksPageTask(t, api, `{"title":"Manual fallback after delete","manual_estimated_minutes":90,"action_items":[{"title":"Only child","estimated_minutes":30}]}`)
	actionItemsResponse := api.request(http.MethodGet, fmt.Sprintf("/tasks/%s/action-items", task.ID), "")
	if actionItemsResponse.Code != http.StatusOK {
		t.Fatalf("list child before delete status/body=%d/%s", actionItemsResponse.Code, actionItemsResponse.Body.String())
	}
	var actionItems struct {
		Items []tasksPageActionItem `json:"items"`
	}
	if err := json.Unmarshal(actionItemsResponse.Body.Bytes(), &actionItems); err != nil {
		t.Fatalf("decode child list: %v; body=%s", err, actionItemsResponse.Body.String())
	}
	if len(actionItems.Items) != 1 {
		t.Fatalf("initial ActionItems=%+v; want one child", actionItems.Items)
	}
	deleted := api.request(http.MethodDelete, fmt.Sprintf("/tasks/%s/action-items/%s", task.ID, actionItems.Items[0].ID), "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete final ActionItem status/body=%d/%s", deleted.Code, deleted.Body.String())
	}
	detailResponse := api.request(http.MethodGet, "/tasks/"+task.ID, "")
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("GET Task after final child deletion status/body=%d/%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail struct {
		Task tasksPageTask `json:"task"`
	}
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode Task after final child deletion: %v; body=%s", err, detailResponse.Body.String())
	}
	if detail.Task.ActionItemCount != 0 || detail.Task.ManualEstimatedMinutes == nil || *detail.Task.ManualEstimatedMinutes != 90 || detail.Task.EstimatedMinutes == nil || *detail.Task.EstimatedMinutes != 90 || detail.Task.EstimateSource != "manual" {
		t.Fatalf("Task after last child deletion=%+v; want manual estimate 90 and no children", detail.Task)
	}
}
