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

func scheduleFlowPool(t *testing.T) *pgxpool.Pool { return testdb.Open(t) }

type scheduleFlowIDs struct{}

func (scheduleFlowIDs) Generate() string { return ulid.Make().String() }

func TestRESTProjectScheduleOccurrenceHistoryFlow(t *testing.T) {
	pool := scheduleFlowPool(t)
	ctx := t.Context()
	ownerID, outsiderID := ulid.Make().String(), ulid.Make().String()
	for _, userID := range []string{ownerID, outsiderID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'REST','Audit',$2,'unused','Asia/Tokyo')`, userID, userID+"@rest-audit.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	var projectID, blueprintID, occurrenceID string
	t.Cleanup(func() {
		if blueprintID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE schedule_id IN (SELECT id FROM schedules WHERE blueprint_id=$1)`, blueprintID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE blueprint_id=$1`, blueprintID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_backfill_checkpoints WHERE revision_id IN (SELECT id FROM schedule_blueprint_revisions WHERE blueprint_id=$1)`, blueprintID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_project_pause_intervals WHERE blueprint_id=$1`, blueprintID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_blueprint_revisions WHERE blueprint_id=$1`, blueprintID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_blueprints WHERE id=$1`, blueprintID)
		}
		if projectID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::text[])`, []string{ownerID, outsiderID})
	})

	ids := scheduleFlowIDs{}
	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule)
	usecases, stopUseCases := application.NewUseCase(store.Auth, store.Project, store.Task, store.Admin, uow.Project, uow.Task, uow.Schedule, ids, nil)
	testdb.RegisterShutdown(t, stopUseCases)
	pageTokens, err := pagination.NewCodec([]byte(strings.Repeat("x", 32)))
	if err != nil {
		t.Fatal(err)
	}
	projectHandler := restapi.NewProjectHandler(usecases.Project, usecases.Task, ids, pageTokens)
	scheduleHandler := restapi.NewScheduleHandler(usecases.Schedule, pageTokens)
	router := chi.NewRouter()
	router.Post("/projects", projectHandler.Create)
	router.Post("/projects/{id}/schedules", scheduleHandler.CreateProjectBlueprint)
	router.Get("/projects/{id}/schedules", scheduleHandler.ListProjectOccurrences)
	router.Get("/schedules", scheduleHandler.ListOccurrences)
	router.Get("/schedules/{blueprint_id}/{occurrence_date}", scheduleHandler.GetOccurrence)
	router.Get("/schedules/{blueprint_id}/{occurrence_date}/revisions", scheduleHandler.ListOccurrenceRevisions)
	router.Patch("/schedules/{blueprint_id}/{occurrence_date}", scheduleHandler.UpdateOccurrence)
	request := func(method, path, body, actor string, headers ...[2]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), restapi.UserIDContextKey, actor))
		for _, header := range headers {
			req.Header.Set(header[0], header[1])
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder
	}

	projectResponse := request(http.MethodPost, "/projects", `{"title":"REST Schedule Project"}`, ownerID)
	if projectResponse.Code != http.StatusCreated {
		t.Fatalf("POST /projects status/body=%d/%s", projectResponse.Code, projectResponse.Body.String())
	}
	var project restapi.ProjectResponse
	if err := json.Unmarshal(projectResponse.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	projectID = project.ID
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	firstDate := time.Now().In(location).AddDate(0, 0, 1)
	start := time.Date(firstDate.Year(), firstDate.Month(), firstDate.Day(), 9, 0, 0, 0, location)
	weekdayCodes := [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	createBody := fmt.Sprintf(`{"title":"Project Schedule","timezone":"Asia/Tokyo","local_start":%q,"local_end":%q,"frequency":"weekly","interval":1,"weekdays":[%q]}`,
		start.Format("2006-01-02T15:04:05"), start.Add(time.Hour).Format("2006-01-02T15:04:05"), weekdayCodes[start.Weekday()])
	blueprintResponse := request(http.MethodPost, "/projects/"+projectID+"/schedules", createBody, ownerID)
	if blueprintResponse.Code != http.StatusCreated {
		t.Fatalf("POST /projects/{id}/schedules status/body=%d/%s", blueprintResponse.Code, blueprintResponse.Body.String())
	}
	var blueprint restapi.ScheduleBlueprintResponse
	if err := json.Unmarshal(blueprintResponse.Body.Bytes(), &blueprint); err != nil {
		t.Fatal(err)
	}
	blueprintID = blueprint.ID
	occurrenceDate := start.Format("2006-01-02")
	updatePath := "/schedules/" + blueprintID + "/" + occurrenceDate
	updateResponse := request(http.MethodPatch, updatePath, `{"title":"Saved occurrence"}`, ownerID, [2]string{"If-Match", `"0"`})
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PATCH occurrence status/body=%d/%s", updateResponse.Code, updateResponse.Body.String())
	}
	var occurrence restapi.ScheduleOccurrenceResponse
	if err := json.Unmarshal(updateResponse.Body.Bytes(), &occurrence); err != nil {
		t.Fatal(err)
	}
	if occurrence.ID == nil || !occurrence.Persisted || !occurrence.IsOverride || occurrence.BlueprintID != blueprintID {
		t.Fatalf("saved occurrence response=%+v", occurrence)
	}
	occurrenceID = *occurrence.ID

	revisionResponse := request(http.MethodGet, updatePath+"/revisions", "", ownerID)
	var revisions struct {
		Items         []restapi.ScheduleOccurrenceRevisionResponse `json:"items"`
		NextPageToken string                                       `json:"next_page_token"`
	}
	if revisionResponse.Code != http.StatusOK || json.Unmarshal(revisionResponse.Body.Bytes(), &revisions) != nil || len(revisions.Items) != 1 || revisions.Items[0].Title != "Saved occurrence" {
		t.Fatalf("GET occurrence revisions status/body=%d/%s", revisionResponse.Code, revisionResponse.Body.String())
	}

	from := time.Date(firstDate.Year(), firstDate.Month(), firstDate.Day(), 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 8)
	listPath := fmt.Sprintf("/schedules?from=%s&to=%s&page_size=10", from.Format(time.RFC3339), to.Format(time.RFC3339))
	listResponse := request(http.MethodGet, listPath, "", ownerID)
	var page restapi.ScheduleOccurrenceListResponse
	if listResponse.Code != http.StatusOK || json.Unmarshal(listResponse.Body.Bytes(), &page) != nil {
		t.Fatalf("GET schedules status/body=%d/%s", listResponse.Code, listResponse.Body.String())
	}
	foundSaved := false
	for _, item := range page.Items {
		if item.BlueprintID == blueprintID && item.OccurrenceDate == occurrenceDate {
			foundSaved = item.ID != nil && *item.ID == occurrenceID && item.Title == "Saved occurrence"
		}
	}
	if !foundSaved {
		t.Fatalf("schedule list did not return the saved occurrence: %+v", page.Items)
	}

	outsiderResponse := request(http.MethodGet, updatePath, "", outsiderID)
	if outsiderResponse.Code != http.StatusNotFound {
		t.Fatalf("outsider GET occurrence status/body=%d/%s; want safe 404", outsiderResponse.Code, outsiderResponse.Body.String())
	}
}
