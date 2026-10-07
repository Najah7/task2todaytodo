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
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

func scheduleTagFlowPool(t *testing.T) *pgxpool.Pool {
	return testdb.Open(t)
}

type scheduleTagFlowIDs struct{}

func (scheduleTagFlowIDs) Generate() string { return ulid.Make().String() }

const scheduleVirtualOccurrenceID = "TASK2TODAYTODO000000000000"

func TestRESTProjectScheduleTagHistoryAndPermissionFlow(t *testing.T) {
	pool := scheduleTagFlowPool(t)
	ctx := t.Context()
	ownerID, outsiderID := ulid.Make().String(), ulid.Make().String()
	for _, userID := range []string{ownerID, outsiderID} {
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password,timezone) VALUES($1,'REST','Audit',$2,'unused','Asia/Tokyo')`, userID, userID+"@rest-audit.test"); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	var projectID, tagID, scheduleID, overrideID string
	t.Cleanup(func() {
		if scheduleID != "" || overrideID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_revisions WHERE id = ANY($1::text[])`, []string{scheduleID, overrideID})
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedule_tag_assignments WHERE schedule_id = ANY($1::text[])`, []string{scheduleID, overrideID})
			_, _ = pool.Exec(context.Background(), `DELETE FROM schedules WHERE id = ANY($1::text[])`, []string{scheduleID, overrideID})
		}
		if tagID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM tags WHERE id=$1`, tagID)
		}
		if projectID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM project_revisions WHERE id=$1`, projectID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::text[])`, []string{ownerID, outsiderID})
	})

	ids := scheduleTagFlowIDs{}
	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule)
	usecases := application.NewUseCase(store.Auth, store.Project, store.Task, store.Schedule, store.Tag, uow.Project, uow.Task, uow.Schedule, ids, nil)
	pageTokens, err := pagination.NewCodec([]byte(strings.Repeat("x", 32)))
	if err != nil {
		t.Fatal(err)
	}
	projectHandler := restapi.NewProjectHandler(usecases.Project, usecases.Task, ids, pageTokens)
	tagHandler := restapi.NewTagHandler(usecases.Tag, ids, pageTokens)
	scheduleHandler := restapi.NewScheduleHandler(usecases.Schedule, ids, pageTokens)
	router := chi.NewRouter()
	router.Post("/projects", projectHandler.Create)
	router.Post("/tags", tagHandler.Create)
	router.Post("/projects/{id}/schedules", scheduleHandler.CreateForProject)
	router.Get("/projects/{id}/schedules", scheduleHandler.ListProject)
	router.Get("/schedules", scheduleHandler.List)
	router.Get("/schedules/{id}", scheduleHandler.Get)
	router.Patch("/schedules/{id}", scheduleHandler.Update)
	router.Get("/schedules/{id}/revisions", scheduleHandler.ListRevisions)
	router.Post("/schedules/{id}/tags:add", scheduleHandler.AddTag)
	router.Post("/schedules/{id}/tags:remove", scheduleHandler.RemoveTag)
	request := func(method, path, body, actor string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), restapi.UserIDContextKey, actor))
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
	tagResponse := request(http.MethodPost, "/tags", `{"name":"Planning"}`, ownerID)
	if tagResponse.Code != http.StatusCreated {
		t.Fatalf("POST /tags status/body=%d/%s", tagResponse.Code, tagResponse.Body.String())
	}
	var tag restapi.TagResponse
	if err := json.Unmarshal(tagResponse.Body.Bytes(), &tag); err != nil {
		t.Fatal(err)
	}
	tagID = tag.ID
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	firstDate := time.Now().In(location).Truncate(24 * time.Hour)
	start := firstDate.Add(9 * time.Hour)
	weekdayCodes := [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	createScheduleBody, _ := json.Marshal(restapi.ScheduleCreateRequest{
		Title: "Project Schedule", StartAt: stringPointer(start.Format(time.RFC3339)), EndAt: stringPointer(start.Add(time.Hour).Format(time.RFC3339)),
		IntervalWeeks: 1, Frequencies: []string{weekdayCodes[start.Weekday()]},
	})
	scheduleResponse := request(http.MethodPost, "/projects/"+projectID+"/schedules", string(createScheduleBody), ownerID)
	if scheduleResponse.Code != http.StatusCreated {
		t.Fatalf("POST /projects/{id}/schedules status/body=%d/%s", scheduleResponse.Code, scheduleResponse.Body.String())
	}
	var schedule restapi.ScheduleResponse
	if err := json.Unmarshal(scheduleResponse.Body.Bytes(), &schedule); err != nil {
		t.Fatal(err)
	}
	scheduleID = schedule.ID
	addResponse := request(http.MethodPost, "/schedules/"+scheduleID+"/tags:add", fmt.Sprintf(`{"tag_id":%q}`, tagID), ownerID)
	if addResponse.Code != http.StatusOK {
		t.Fatalf("POST /schedules/{id}/tags:add status/body=%d/%s", addResponse.Code, addResponse.Body.String())
	}
	occurrenceDate := firstDate.AddDate(0, 0, 7).Format("2006-01-02")
	updateResponse := request(http.MethodPatch, "/schedules/"+scheduleID, fmt.Sprintf(`{"occurrence_date":%q,"title":"Saved occurrence"}`, occurrenceDate), ownerID)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PATCH /schedules/{id} occurrence status/body=%d/%s", updateResponse.Code, updateResponse.Body.String())
	}
	var override restapi.ScheduleResponse
	if err := json.Unmarshal(updateResponse.Body.Bytes(), &override); err != nil {
		t.Fatal(err)
	}
	if !override.IsException || override.ID == scheduleID || override.SeriesID != scheduleID {
		t.Fatalf("PATCH occurrence response=%+v; want saved override in series %q", override, scheduleID)
	}
	overrideID = override.ID
	removeViaOverride := request(http.MethodPost, "/schedules/"+overrideID+"/tags:remove", fmt.Sprintf(`{"tag_id":%q}`, tagID), ownerID)
	if removeViaOverride.Code != http.StatusOK {
		t.Fatalf("POST /schedules/{override}/tags:remove status/body=%d/%s", removeViaOverride.Code, removeViaOverride.Body.String())
	}
	addViaOverride := request(http.MethodPost, "/schedules/"+overrideID+"/tags:add", fmt.Sprintf(`{"tag_id":%q}`, tagID), ownerID)
	if addViaOverride.Code != http.StatusOK {
		t.Fatalf("POST /schedules/{override}/tags:add status/body=%d/%s", addViaOverride.Code, addViaOverride.Body.String())
	}
	getResponse := request(http.MethodGet, "/schedules/"+scheduleID, "", ownerID)
	var tagged restapi.ScheduleResponse
	if getResponse.Code != http.StatusOK || json.Unmarshal(getResponse.Body.Bytes(), &tagged) != nil || len(tagged.Tags) != 1 || tagged.Tags[0].ID != tagID {
		t.Fatalf("GET schedule after tag add status/body=%d/%s", getResponse.Code, getResponse.Body.String())
	}
	getOverrideResponse := request(http.MethodGet, "/schedules/"+overrideID, "", ownerID)
	var taggedOverride restapi.ScheduleResponse
	if getOverrideResponse.Code != http.StatusOK || json.Unmarshal(getOverrideResponse.Body.Bytes(), &taggedOverride) != nil || len(taggedOverride.Tags) != 1 || taggedOverride.Tags[0].ID != tagID {
		t.Fatalf("GET saved override tags status/body=%d/%s", getOverrideResponse.Code, getOverrideResponse.Body.String())
	}
	fromDate := url.QueryEscape(firstDate.Format("2006-01-02"))
	for _, listPath := range []string{"/schedules?from_date=" + fromDate + "&page_size=10", "/projects/" + projectID + "/schedules?from_date=" + fromDate + "&page_size=10"} {
		listResponse := request(http.MethodGet, listPath, "", ownerID)
		var page restapi.ScheduleListResponse
		if listResponse.Code != http.StatusOK || json.Unmarshal(listResponse.Body.Bytes(), &page) != nil {
			t.Fatalf("GET %s status/body=%d/%s", listPath, listResponse.Code, listResponse.Body.String())
		}
		foundOverride, foundVirtual := false, false
		for _, item := range page.Items {
			if item.SeriesID != scheduleID {
				continue
			}
			if len(item.Tags) != 1 || item.Tags[0].ID != tagID {
				t.Errorf("GET %s Schedule row %s (%s) tags=%+v; want series tag", listPath, item.ID, item.OccurrenceDate, item.Tags)
			}
			foundOverride = foundOverride || item.IsException
			foundVirtual = foundVirtual || item.ID == scheduleVirtualOccurrenceID
		}
		if !foundOverride || !foundVirtual {
			t.Errorf("GET %s rows=%+v; want tagged saved override and virtual occurrence", listPath, page.Items)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE schedules SET deleted_at = now(), changed_by = $2 WHERE id = $1`, scheduleID, ownerID); err != nil {
		t.Fatalf("soft-delete series root while retaining override: %v", err)
	}
	getLiveOverrideResponse := request(http.MethodGet, "/schedules/"+overrideID, "", ownerID)
	var liveOverride restapi.ScheduleResponse
	if getLiveOverrideResponse.Code != http.StatusOK || json.Unmarshal(getLiveOverrideResponse.Body.Bytes(), &liveOverride) != nil || len(liveOverride.Tags) != 1 || liveOverride.Tags[0].ID != tagID {
		t.Fatalf("GET override after root deletion status/body=%d/%s; want live override with series tags", getLiveOverrideResponse.Code, getLiveOverrideResponse.Body.String())
	}
	revisionResponse := request(http.MethodGet, "/schedules/"+scheduleID+"/revisions", "", ownerID)
	var revisions restapi.ScheduleRevisionListResponse
	if revisionResponse.Code != http.StatusOK || json.Unmarshal(revisionResponse.Body.Bytes(), &revisions) != nil || len(revisions.Items) != 2 {
		t.Fatalf("GET Schedule revisions after tag add status/body=%d/%s; want the two pre-tag Schedule revisions unchanged", revisionResponse.Code, revisionResponse.Body.String())
	}
	removeResponse := request(http.MethodPost, "/schedules/"+overrideID+"/tags:remove", fmt.Sprintf(`{"tag_id":%q}`, tagID), ownerID)
	if removeResponse.Code != http.StatusOK {
		t.Fatalf("POST /schedules/{override}/tags:remove after root deletion status/body=%d/%s", removeResponse.Code, removeResponse.Body.String())
	}
	outsiderResponse := request(http.MethodGet, "/schedules/"+scheduleID, "", outsiderID)
	if outsiderResponse.Code != http.StatusNotFound {
		t.Fatalf("outsider GET Schedule status/body=%d/%s; want safe 404", outsiderResponse.Code, outsiderResponse.Body.String())
	}
	outsiderAddResponse := request(http.MethodPost, "/schedules/"+scheduleID+"/tags:add", fmt.Sprintf(`{"tag_id":%q}`, tagID), outsiderID)
	if outsiderAddResponse.Code != http.StatusNotFound {
		t.Fatalf("outsider add Tag status/body=%d/%s; want safe 404", outsiderAddResponse.Code, outsiderAddResponse.Body.String())
	}
}

func stringPointer(value string) *string { return &value }
