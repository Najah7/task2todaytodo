//go:build integration

package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type taskEditIDs struct{}

func (taskEditIDs) Generate() string { return ulid.Make().String() }

type taskEditAPI struct {
	pool   *pgxpool.Pool
	userID string
	router *chi.Mux
}

func newTaskEditAPI(t *testing.T) *taskEditAPI {
	t.Helper()
	pool := testdb.Open(t)
	userID := ulid.Make().String()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'Task','Edit',$2,'unused','UTC')`, userID, userID+"@task-edit.test"); err != nil {
		t.Fatalf("insert Task edit user: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM action_items WHERE task_id IN (SELECT id FROM tasks WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM task_revisions WHERE id IN (SELECT id FROM tasks WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM tasks WHERE user_id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM project_revisions WHERE id IN (SELECT id FROM projects WHERE user_id=$1)`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE user_id=$1`, userID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	ids := taskEditIDs{}
	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule)
	usecases := application.NewUseCase(store.Auth, store.Project, store.Task, store.Schedule, store.Tag, uow.Project, uow.Task, uow.Schedule, ids, nil)
	pageTokens, err := pagination.NewCodec([]byte(strings.Repeat("e", 32)))
	if err != nil {
		t.Fatal(err)
	}
	tasks := restapi.NewTaskHandler(usecases.Task, ids, pageTokens)
	actionItems := restapi.NewActionItemHandler(usecases.ActionItem, ids, pageTokens)
	projects := restapi.NewProjectHandler(usecases.Project, usecases.Task, ids, pageTokens)
	router := chi.NewRouter()
	router.Post("/tasks", tasks.Create)
	router.Patch("/tasks/{id}", tasks.Update)
	router.Get("/tasks/{id}", tasks.Get)
	router.Post("/tasks/{taskId}/action-items", actionItems.Create)
	router.Get("/tasks/{taskId}/action-items", actionItems.List)
	router.Patch("/tasks/{taskId}/action-items/{id}", actionItems.Update)
	router.Post("/tasks/{taskId}/action-items/{id}:skip", actionItems.Skip)
	router.Post("/tasks/{taskId}/action-items/{id}:complete", actionItems.Complete)
	router.Delete("/tasks/{taskId}/action-items/{id}", actionItems.Delete)
	router.Post("/projects", projects.Create)
	router.Post("/projects/{id}/tasks", projects.CreateTask)
	return &taskEditAPI{pool: pool, userID: userID, router: router}
}

func (api *taskEditAPI) request(actor, method, path, body, ifMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), restapi.UserIDContextKey, actor))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	rec := httptest.NewRecorder()
	api.router.ServeHTTP(rec, req)
	return rec
}

type taskEditTask struct {
	ID                       string  `json:"id"`
	Revision                 int32   `json:"revision"`
	Title                    string  `json:"title"`
	Priority                 string  `json:"priority"`
	ProjectID                *string `json:"project_id"`
	CanUpdate                bool    `json:"can_update"`
	EstimatedMinutes         *int    `json:"estimated_minutes"`
	ActionItemCount          int     `json:"action_item_count"`
	ActionItemCompletedCount int     `json:"action_item_completed_count"`
}

type taskEditChild struct {
	ID               string `json:"id"`
	SeriesID         string `json:"series_id"`
	OccurrenceDate   string `json:"occurrence_date"`
	Title            string `json:"title"`
	Priority         string `json:"priority"`
	EstimatedMinutes *int   `json:"estimated_minutes"`
	Completed        bool   `json:"completed"`
}

type taskEditChildList struct {
	Items         []taskEditChild `json:"items"`
	NextPageToken string          `json:"next_page_token"`
}

func (api *taskEditAPI) createTask(t *testing.T, body string) taskEditTask {
	t.Helper()
	rec := api.request(api.userID, http.MethodPost, "/tasks", body, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /tasks status/body=%d/%s", rec.Code, rec.Body.String())
	}
	return decodeTaskEditTask(t, rec.Body.Bytes())
}

func decodeTaskEditTask(t *testing.T, body []byte) taskEditTask {
	t.Helper()
	var task taskEditTask
	if err := json.Unmarshal(body, &task); err != nil {
		t.Fatalf("decode Task: %v; body=%s", err, body)
	}
	return task
}

func (api *taskEditAPI) getTask(t *testing.T, actor, taskID string) taskEditTask {
	t.Helper()
	rec := api.request(actor, http.MethodGet, "/tasks/"+taskID, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET Task status/body=%d/%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Task taskEditTask `json:"task"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode Task detail: %v; body=%s", err, rec.Body.String())
	}
	return result.Task
}

func (api *taskEditAPI) listActionItems(t *testing.T, taskID string) []taskEditChild {
	t.Helper()
	var all []taskEditChild
	path := fmt.Sprintf("/tasks/%s/action-items?page_size=50", taskID)
	for {
		rec := api.request(api.userID, http.MethodGet, path, "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET ActionItems status/body=%d/%s", rec.Code, rec.Body.String())
		}
		var page taskEditChildList
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatalf("decode ActionItem page: %v; body=%s", err, rec.Body.String())
		}
		all = append(all, page.Items...)
		if page.NextPageToken == "" {
			return all
		}
		path = fmt.Sprintf("/tasks/%s/action-items?page_size=50&page_token=%s", taskID, page.NextPageToken)
	}
}

func TestRESTTaskEndpointsRejectNestedActionItems(t *testing.T) {
	api := newTaskEditAPI(t)
	create := api.request(api.userID, http.MethodPost, "/tasks", `{"title":"Rejected nested create","action_items":[{"title":"Child"}]}`, "")
	if create.Code != http.StatusBadRequest {
		t.Fatalf("nested Task create status/body=%d/%s; want 400", create.Code, create.Body.String())
	}
	task := api.createTask(t, `{"title":"Standalone Task","priority":"low"}`)
	child := api.request(api.userID, http.MethodPost, "/tasks/"+task.ID+"/action-items", `{"title":"Standalone child","estimated_minutes":20,"priority":"medium"}`, "")
	if child.Code != http.StatusCreated {
		t.Fatalf("POST ActionItem status/body=%d/%s", child.Code, child.Body.String())
	}
	items := api.listActionItems(t, task.ID)
	if len(items) != 1 {
		t.Fatalf("ActionItems=%+v; want one", items)
	}
	patch := api.request(api.userID, http.MethodPatch, "/tasks/"+task.ID, `{"title":"Should not persist","priority":"high","action_items":[{"operation":"delete"}]}`, fmt.Sprintf(`"%d"`, task.Revision))
	if patch.Code != http.StatusBadRequest {
		t.Fatalf("nested Task update status/body=%d/%s; want 400", patch.Code, patch.Body.String())
	}
	if got := api.getTask(t, api.userID, task.ID); got.Title != "Standalone Task" || got.Priority != "low" {
		t.Fatalf("rejected Task update persisted: %+v", got)
	}
	if got := api.listActionItems(t, task.ID); len(got) != 1 || got[0].Title != "Standalone child" {
		t.Fatalf("rejected Task update changed ActionItems: %+v", got)
	}
	project := api.request(api.userID, http.MethodPost, "/projects", `{"type":"other","title":"Project"}`, "")
	if project.Code != http.StatusCreated {
		t.Fatalf("POST Project status/body=%d/%s", project.Code, project.Body.String())
	}
	var projectResult struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(project.Body.Bytes(), &projectResult); err != nil {
		t.Fatal(err)
	}
	projectTask := api.request(api.userID, http.MethodPost, "/projects/"+projectResult.ID+"/tasks", `{"title":"Nested project task","action_items":[{"title":"Child"}]}`, "")
	if projectTask.Code != http.StatusBadRequest {
		t.Fatalf("nested ProjectTask create status/body=%d/%s; want 400", projectTask.Code, projectTask.Body.String())
	}
}

func TestRESTActionItemsUpdateIndividuallyAndPreserveTaskIfMatch(t *testing.T) {
	api := newTaskEditAPI(t)
	task := api.createTask(t, `{"title":"Revision task","priority":"low"}`)
	created := api.request(api.userID, http.MethodPost, "/tasks/"+task.ID+"/action-items", `{"title":"First child","estimated_minutes":10,"priority":"medium"}`, "")
	if created.Code != http.StatusCreated {
		t.Fatalf("POST ActionItem status/body=%d/%s", created.Code, created.Body.String())
	}
	child := decodeTaskEditChild(t, created.Body.Bytes())
	seriesID := child.SeriesID
	if seriesID == "" {
		seriesID = child.ID
	}
	updated := api.request(api.userID, http.MethodPatch, fmt.Sprintf("/tasks/%s/action-items/%s", task.ID, seriesID),
		fmt.Sprintf(`{"scope":"current","occurrence_date":%q,"title":"Updated child","estimated_minutes":25,"priority":"high"}`, child.OccurrenceDate), "")
	if updated.Code != http.StatusOK {
		t.Fatalf("PATCH ActionItem status/body=%d/%s", updated.Code, updated.Body.String())
	}
	if got := api.getTask(t, api.userID, task.ID); got.Revision <= task.Revision {
		t.Fatalf("ActionItem edit did not advance Task revision: before=%d after=%d", task.Revision, got.Revision)
	}
	stale := api.request(api.userID, http.MethodPatch, "/tasks/"+task.ID, `{"title":"Stale overwrite"}`, fmt.Sprintf(`"%d"`, task.Revision))
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale Task update status/body=%d/%s; want 409", stale.Code, stale.Body.String())
	}
	if field := taskEditFirstErrorField(t, stale.Body.Bytes()); field != "If-Match" {
		t.Fatalf("stale Task error field=%q; want If-Match", field)
	}
	if got := api.getTask(t, api.userID, task.ID); got.Title != "Revision task" {
		t.Fatalf("stale Task update changed title: %+v", got)
	}
	if got := api.listActionItems(t, task.ID); len(got) != 1 || got[0].Title != "Updated child" || got[0].Priority != "high" || got[0].EstimatedMinutes == nil || *got[0].EstimatedMinutes != 25 {
		t.Fatalf("independent ActionItem update=%+v", got)
	}
}

func TestRESTCurrentRecurringActionItemUpdateAndSkip(t *testing.T) {
	api := newTaskEditAPI(t)
	task := api.createTask(t, `{"title":"Recurring task","priority":"high"}`)
	today := time.Now().UTC().Format("2006-01-02")
	weekday := [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}[time.Now().UTC().Weekday()]
	created := api.request(api.userID, http.MethodPost, "/tasks/"+task.ID+"/action-items", fmt.Sprintf(`{"title":"Weekly item","estimated_minutes":60,"priority":"medium","interval_weeks":1,"frequencies":[%q]}`, weekday), "")
	if created.Code != http.StatusCreated {
		t.Fatalf("POST recurring ActionItem status/body=%d/%s", created.Code, created.Body.String())
	}
	root := decodeTaskEditChild(t, created.Body.Bytes())
	seriesID := root.SeriesID
	if seriesID == "" {
		seriesID = root.ID
	}
	rows := api.listActionItems(t, task.ID)
	var target, skipped, untouched string
	for _, row := range rows {
		if row.OccurrenceDate <= today {
			continue
		}
		if target == "" {
			target = row.OccurrenceDate
		} else if skipped == "" {
			skipped = row.OccurrenceDate
		} else {
			untouched = row.OccurrenceDate
		}
	}
	if target == "" || skipped == "" || untouched == "" {
		t.Fatalf("need three future occurrences: %+v", rows)
	}
	update := api.request(api.userID, http.MethodPatch, fmt.Sprintf("/tasks/%s/action-items/%s", task.ID, seriesID), fmt.Sprintf(`{"scope":"current","occurrence_date":%q,"title":"Current override","estimated_minutes":75,"priority":"low"}`, target), "")
	if update.Code != http.StatusOK {
		t.Fatalf("PATCH current occurrence status/body=%d/%s", update.Code, update.Body.String())
	}
	assertOccurrenceFields(t, api.listActionItems(t, task.ID), target, "Current override", "low", 75)
	skip := api.request(api.userID, http.MethodPost, fmt.Sprintf("/tasks/%s/action-items/%s:skip", task.ID, seriesID), fmt.Sprintf(`{"occurrence_date":%q}`, skipped), "")
	if skip.Code != http.StatusOK {
		t.Fatalf("skip current occurrence status/body=%d/%s", skip.Code, skip.Body.String())
	}
	after := api.listActionItems(t, task.ID)
	assertOccurrenceFields(t, after, target, "Current override", "low", 75)
	if findOccurrence(after, skipped) != nil {
		t.Fatalf("skipped occurrence remains visible: %+v", after)
	}
	assertOccurrenceFields(t, after, untouched, "Weekly item", "medium", 60)
}

func TestRESTTaskEditChangesProjectAndPriority(t *testing.T) {
	api := newTaskEditAPI(t)
	task := api.createTask(t, `{"title":"Move task","priority":"low"}`)
	projectResponse := api.request(api.userID, http.MethodPost, "/projects", `{"type":"other","title":"Destination project","priority":"high"}`, "")
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("POST /projects status/body=%d/%s", projectResponse.Code, projectResponse.Body.String())
	}
	var project struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(projectResponse.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode Project: %v", err)
	}
	updated := api.request(api.userID, http.MethodPatch, "/tasks/"+task.ID, fmt.Sprintf(`{"project_id":%q,"priority":"high"}`, project.ID), fmt.Sprintf(`"%d"`, task.Revision))
	if updated.Code != http.StatusOK {
		t.Fatalf("attach Task to Project status/body=%d/%s", updated.Code, updated.Body.String())
	}
	attached := decodeTaskEditTask(t, updated.Body.Bytes())
	if attached.ProjectID == nil || *attached.ProjectID != project.ID || attached.Priority != "high" {
		t.Fatalf("attached Task=%+v", attached)
	}
	detached := api.request(api.userID, http.MethodPatch, "/tasks/"+task.ID, `{"project_id":null,"priority":"low"}`, fmt.Sprintf(`"%d"`, attached.Revision))
	if detached.Code != http.StatusOK {
		t.Fatalf("detach Task status/body=%d/%s", detached.Code, detached.Body.String())
	}
	detachedTask := decodeTaskEditTask(t, detached.Body.Bytes())
	if detachedTask.ProjectID != nil || detachedTask.Priority != "low" {
		t.Fatalf("detached Task=%+v", detachedTask)
	}
}

func taskEditFirstErrorField(t *testing.T, body []byte) string {
	t.Helper()
	var response struct {
		Error struct {
			Details []struct {
				Field string `json:"field"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, body)
	}
	if len(response.Error.Details) == 0 {
		t.Fatalf("error response contains no field details: %s", body)
	}
	return response.Error.Details[0].Field
}

func decodeTaskEditChild(t *testing.T, body []byte) taskEditChild {
	t.Helper()
	var child taskEditChild
	if err := json.Unmarshal(body, &child); err != nil {
		t.Fatalf("decode ActionItem: %v; body=%s", err, body)
	}
	return child
}

func findOccurrence(rows []taskEditChild, date string) *taskEditChild {
	for i := range rows {
		if rows[i].OccurrenceDate == date {
			return &rows[i]
		}
	}
	return nil
}

func assertOccurrenceFields(t *testing.T, rows []taskEditChild, date, title, priority string, estimate int) {
	t.Helper()
	row := findOccurrence(rows, date)
	if row == nil || row.Title != title || row.Priority != priority || row.EstimatedMinutes == nil || *row.EstimatedMinutes != estimate {
		t.Fatalf("occurrence %s=%+v; want title=%q priority=%q estimate=%d", date, row, title, priority, estimate)
	}
}
