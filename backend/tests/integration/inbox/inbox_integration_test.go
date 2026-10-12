//go:build integration

package inbox_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application"
	inboxdao "github.com/Najah7/task2todaytodo/internal/application/inbox/dao"
	inboxusecase "github.com/Najah7/task2todaytodo/internal/application/inbox/usecase"
	"github.com/Najah7/task2todaytodo/internal/port/adapter"
	restapi "github.com/Najah7/task2todaytodo/internal/port/rest"
	"github.com/Najah7/task2todaytodo/internal/port/rest/pagination"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type inboxREST struct {
	pool   *pgxpool.Pool
	actor  string
	router *chi.Mux
	list   *inboxusecase.ListUseCase
}

func newInboxREST(t *testing.T, timezone string) *inboxREST {
	t.Helper()
	pool := testdb.Open(t)
	actor := ulid.Make().String()
	insertInboxUser(t, pool, actor, timezone)

	store := application.NewStore(pool)
	uow := application.NewUOW(pool, store.Auth, store.Project, store.Task, store.Schedule, store.Allocation)
	usecases, stop := application.NewUseCase(store, uow, adapter.NewIDIssuer(), nil)
	testdb.RegisterShutdown(t, stop)
	codec, err := pagination.NewCodec([]byte(strings.Repeat("i", 32)))
	if err != nil {
		t.Fatal(err)
	}
	handler := restapi.NewInboxHandler(usecases.Inbox, codec)
	router := chi.NewRouter()
	router.Get("/inbox", handler.List)
	return &inboxREST{pool: pool, actor: actor, router: router, list: usecases.Inbox.List}
}

func (api *inboxREST) request(path, actor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if actor != "" {
		req = req.WithContext(context.WithValue(req.Context(), restapi.UserIDContextKey, actor))
	}
	recorder := httptest.NewRecorder()
	api.router.ServeHTTP(recorder, req)
	return recorder
}

type inboxCounts struct {
	All      int `json:"all"`
	Task     int `json:"task"`
	Schedule int `json:"schedule"`
}

type inboxTask struct {
	ID                       string  `json:"id"`
	AssigneeID               string  `json:"assignee_id"`
	DueDate                  *string `json:"due_date"`
	Revision                 int     `json:"revision"`
	ActionItemCount          int     `json:"action_item_count"`
	ActionItemCompletedCount int     `json:"action_item_completed_count"`
	CanUpdate                bool    `json:"can_update"`
	CanAssign                bool    `json:"can_assign"`
}

type inboxSchedule struct {
	OccurrenceID       *string `json:"occurrence_id"`
	RuleID             string  `json:"rule_id"`
	OccurrenceDate     string  `json:"occurrence_date"`
	Timezone           string  `json:"timezone"`
	AllDay             bool    `json:"all_day"`
	StartAt            string  `json:"start_at"`
	EndAt              string  `json:"end_at"`
	CanUpdate          bool    `json:"can_update"`
	CanUpdateFollowing bool    `json:"can_update_following"`
}

type inboxItem struct {
	Type        string         `json:"type"`
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	ProjectID   *string        `json:"project_id"`
	ProjectName *string        `json:"project_name"`
	CreatedAt   string         `json:"created_at"`
	Task        *inboxTask     `json:"task"`
	Schedule    *inboxSchedule `json:"schedule"`
}

type inboxPage struct {
	Items             []inboxItem `json:"items"`
	NextPageToken     string      `json:"next_page_token"`
	PreviousPageToken string      `json:"previous_page_token"`
	Counts            inboxCounts `json:"counts"`
}

func decodeInboxPage(t *testing.T, response *httptest.ResponseRecorder) inboxPage {
	t.Helper()
	var page inboxPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode inbox page: %v; body=%s", err, response.Body.String())
	}
	return page
}

func TestRESTInboxAppliesAssignmentProjectACLStateFiltersAndBidirectionalPagination(t *testing.T) {
	api := newInboxREST(t, "UTC")
	f := newInboxFixtures(t, api.pool, api.actor)
	owner := f.user("Asia/Tokyo")
	outsider := f.user("UTC")
	sharedProject := f.project(owner, "Shared project")
	privateProject := f.project(owner, "Private project")
	role := f.readRole()
	f.addMember(sharedProject, role, owner)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ownedTask := f.task(api.actor, api.actor, "Owned Alpha", "", now.Add(-2*time.Hour), nil, "open")
	sharedTask := f.task(owner, api.actor, "Project Beta", sharedProject, now.Add(-time.Hour), datePointer(now.AddDate(0, 0, -1)), "open")
	f.task(owner, api.actor, "Private assigned", privateProject, now, nil, "open")
	f.task(owner, outsider, "Wrong assignee", sharedProject, now, nil, "open")
	f.task(owner, api.actor, "Completed task", sharedProject, now, nil, "done")
	deleted := f.task(owner, api.actor, "Deleted task", sharedProject, now, nil, "open")
	if _, err := f.pool.Exec(t.Context(), `UPDATE tasks SET deleted_at=$2 WHERE id=$1`, deleted, now); err != nil {
		t.Fatalf("soft-delete task fixture: %v", err)
	}
	if _, err := f.pool.Exec(t.Context(), `INSERT INTO action_items(id,task_id,title,position,estimated_minutes,completed) VALUES($1,$2,'Open item',0,10,false),($3,$2,'Done item',1,10,true)`, ulid.Make().String(), ownedTask, ulid.Make().String()); err != nil {
		t.Fatalf("insert ActionItem fixtures: %v", err)
	}

	today := now.Format("2006-01-02")
	todayStart, err := time.ParseInLocation("2006-01-02", today, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	start := todayStart.Add(9 * time.Hour)
	sharedSchedule := f.schedule(owner, sharedProject, api.actor, "Project Gamma", start, start.Add(time.Hour), "once", false, false, false)
	if _, err := f.pool.Exec(t.Context(), `UPDATE schedule_rules SET created_at=$2 WHERE id=$1`, sharedSchedule.RuleID, now.Add(-3*time.Hour)); err != nil {
		t.Fatalf("set Schedule rule creation time: %v", err)
	}
	f.schedule(owner, sharedProject, api.actor, "Completed schedule", start.Add(time.Hour), start.Add(2*time.Hour), "once", true, false, false)
	f.schedule(owner, sharedProject, api.actor, "Skipped schedule", start.Add(2*time.Hour), start.Add(3*time.Hour), "once", false, true, false)
	f.schedule(owner, sharedProject, api.actor, "Deleted schedule", start.Add(3*time.Hour), start.Add(4*time.Hour), "once", false, false, true)
	f.schedule(owner, privateProject, api.actor, "Private schedule", start.Add(4*time.Hour), start.Add(5*time.Hour), "once", false, false, false)
	f.schedule(owner, sharedProject, outsider, "Wrong schedule assignee", start.Add(5*time.Hour), start.Add(6*time.Hour), "once", false, false, false)

	unauthorized := api.request("/inbox", "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("GET /inbox without actor status/body=%d/%s; want 401", unauthorized.Code, unauthorized.Body.String())
	}

	firstResponse := api.request("/inbox?page_size=1", api.actor)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("GET /inbox status/body=%d/%s", firstResponse.Code, firstResponse.Body.String())
	}
	first := decodeInboxPage(t, firstResponse)
	if first.Counts != (inboxCounts{All: 3, Task: 2, Schedule: 1}) || len(first.Items) != 1 || first.NextPageToken == "" {
		t.Fatalf("first inbox page counts/items/next=%+v/%+v/%q; want 3 total readable active items", first.Counts, first.Items, first.NextPageToken)
	}
	if first.Items[0].Type != "task" || first.Items[0].ID != sharedTask {
		t.Fatalf("newest inbox item=%+v; want shared assigned Task", first.Items[0])
	}
	if first.Items[0].Task == nil || first.Items[0].Schedule != nil || first.Items[0].Task.DueDate == nil || *first.Items[0].Task.DueDate != now.AddDate(0, 0, -1).Format("2006-01-02") {
		t.Fatalf("Task payload=%+v; want overdue due date and exactly one typed payload", first.Items[0])
	}
	if first.Items[0].Task.Revision < 1 || first.Items[0].CreatedAt != now.Add(-time.Hour).Format(time.RFC3339Nano) {
		t.Fatalf("Task revision/creation precision=%d/%q; want revision and microsecond creation time preserved", first.Items[0].Task.Revision, first.Items[0].CreatedAt)
	}

	secondResponse := api.request("/inbox?"+url.Values{"page_size": {"1"}, "page_token": {first.NextPageToken}}.Encode(), api.actor)
	second := decodeInboxPage(t, secondResponse)
	if secondResponse.Code != http.StatusOK || len(second.Items) != 1 || second.PreviousPageToken == "" {
		t.Fatalf("second inbox page status/items/previous=%d/%+v/%q body=%s", secondResponse.Code, second.Items, second.PreviousPageToken, secondResponse.Body.String())
	}
	thirdResponse := api.request("/inbox?"+url.Values{"page_size": {"1"}, "page_token": {second.NextPageToken}}.Encode(), api.actor)
	third := decodeInboxPage(t, thirdResponse)
	if thirdResponse.Code != http.StatusOK || len(third.Items) != 1 || third.Items[0].Type != "schedule" || third.Items[0].ID != sharedSchedule.RuleID+"/"+today {
		t.Fatalf("third inbox page status/items=%d/%+v body=%s", thirdResponse.Code, third.Items, thirdResponse.Body.String())
	}
	backResponse := api.request("/inbox?"+url.Values{"page_size": {"1"}, "page_token": {third.PreviousPageToken}}.Encode(), api.actor)
	back := decodeInboxPage(t, backResponse)
	if backResponse.Code != http.StatusOK || len(back.Items) != 1 || back.Items[0].ID != second.Items[0].ID {
		t.Fatalf("backward inbox page status/items=%d/%+v; want second item %q", backResponse.Code, back.Items, second.Items[0].ID)
	}
	if second.Items[0].ID == first.Items[0].ID || third.Items[0].ID == second.Items[0].ID {
		t.Fatalf("forward pages duplicated an item: %q, %q, %q", first.Items[0].ID, second.Items[0].ID, third.Items[0].ID)
	}
	if second.Items[0].ID != ownedTask {
		t.Fatalf("second forward page item=%+v; want owned Task %q", second.Items[0], ownedTask)
	}

	projectQuery := url.Values{"project_id": {sharedProject}, "type": {"schedule"}, "page_size": {"10"}}
	projectResponse := api.request("/inbox?"+projectQuery.Encode(), api.actor)
	projectPage := decodeInboxPage(t, projectResponse)
	if projectResponse.Code != http.StatusOK || projectPage.Counts != (inboxCounts{All: 2, Task: 1, Schedule: 1}) || len(projectPage.Items) != 1 || projectPage.Items[0].Type != "schedule" {
		t.Fatalf("project/type filtered page status/counts/items=%d/%+v/%+v body=%s", projectResponse.Code, projectPage.Counts, projectPage.Items, projectResponse.Body.String())
	}
	noProjectResponse := api.request("/inbox?project_id=no-project&type=task", api.actor)
	noProjectPage := decodeInboxPage(t, noProjectResponse)
	if noProjectResponse.Code != http.StatusOK || noProjectPage.Counts != (inboxCounts{All: 1, Task: 1}) || len(noProjectPage.Items) != 1 || noProjectPage.Items[0].ID != ownedTask {
		t.Fatalf("no-project Task filter status/counts/items=%d/%+v/%+v", noProjectResponse.Code, noProjectPage.Counts, noProjectPage.Items)
	}
	literalResponse := api.request("/inbox?"+url.Values{"title": {"OWNED ALPHA"}, "type": {"task"}}.Encode(), api.actor)
	literalPage := decodeInboxPage(t, literalResponse)
	if literalResponse.Code != http.StatusOK || literalPage.Counts != (inboxCounts{All: 1, Task: 1}) || len(literalPage.Items) != 1 || literalPage.Items[0].ID != ownedTask {
		t.Fatalf("case-insensitive title substring status/counts/items=%d/%+v/%+v", literalResponse.Code, literalPage.Counts, literalPage.Items)
	}
	wildcardTask := f.task(api.actor, api.actor, "Literal %_ Match", "", now.Add(-4*time.Hour), nil, "open")
	wildcardResponse := api.request("/inbox?"+url.Values{"title": {"%_"}, "type": {"task"}}.Encode(), api.actor)
	wildcardPage := decodeInboxPage(t, wildcardResponse)
	if wildcardResponse.Code != http.StatusOK || wildcardPage.Counts != (inboxCounts{All: 1, Task: 1}) || len(wildcardPage.Items) != 1 || wildcardPage.Items[0].ID != wildcardTask {
		t.Fatalf("literal wildcard title filter status/counts/items=%d/%+v/%+v", wildcardResponse.Code, wildcardPage.Counts, wildcardPage.Items)
	}
}

func TestInboxDSTWindowIncludesVirtualOvernightAndFloatingAllDayWithoutMaterializing(t *testing.T) {
	api := newInboxREST(t, "America/New_York")
	f := newInboxFixtures(t, api.pool, api.actor)
	owner := api.actor
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	today := "2026-03-07"
	asOf := time.Date(2026, time.March, 7, 12, 0, 0, 0, time.UTC)
	start := time.Date(2026, time.March, 7, 23, 30, 0, 0, newYork)
	firstEnd := time.Date(2026, time.March, 8, 1, 30, 0, 0, newYork)
	series := f.schedule(owner, "", api.actor, "Overnight daily", start, firstEnd, "daily", false, false, false)
	if _, err := f.pool.Exec(t.Context(), `UPDATE schedule_rules SET created_at=$2 WHERE id=$1`, series.RuleID, asOf.Add(-24*time.Hour)); err != nil {
		t.Fatalf("set recurrence rule creation time: %v", err)
	}

	kiritimati, err := time.LoadLocation("Pacific/Kiritimati")
	if err != nil {
		t.Fatal(err)
	}
	allDayStart := time.Date(2026, time.March, 8, 0, 0, 0, 0, kiritimati)
	allDay := f.schedule(owner, "", api.actor, "Floating all-day", allDayStart, allDayStart.AddDate(0, 0, 1), "once", false, false, false)
	if _, err := f.pool.Exec(t.Context(), `UPDATE schedules SET all_day=true WHERE id=$1`, allDay.ScheduleID); err != nil {
		t.Fatalf("mark all-day occurrence: %v", err)
	}
	outsideFloatingStart := time.Date(2026, time.March, 6, 0, 0, 0, 0, kiritimati)
	outsideFloating := f.schedule(owner, "", api.actor, "Yesterday all-day", outsideFloatingStart, outsideFloatingStart.AddDate(0, 0, 1), "once", false, false, false)
	if _, err := f.pool.Exec(t.Context(), `UPDATE schedules SET all_day=true WHERE id=$1`, outsideFloating.ScheduleID); err != nil {
		t.Fatalf("mark prior all-day occurrence: %v", err)
	}

	beforeSchedules, beforeRevisions := f.scheduleRowCounts(owner)
	page, err := api.list.Execute(t.Context(), api.actor, inboxusecase.ListRequest{
		Size:     100,
		Snapshot: inboxusecase.Snapshot{AsOf: asOf, Timezone: "America/New_York", Today: today},
	})
	if err != nil {
		t.Fatalf("list historical inbox snapshot: %v", err)
	}
	afterSchedules, afterRevisions := f.scheduleRowCounts(owner)
	if beforeSchedules != afterSchedules || beforeRevisions != afterRevisions {
		t.Fatalf("Inbox read materialized schedules or revisions: schedules %d->%d, revisions %d->%d", beforeSchedules, afterSchedules, beforeRevisions, afterRevisions)
	}

	virtualDates := make(map[string]bool)
	allDayFound, priorAllDayFound := false, false
	for _, item := range page.Items {
		if item.Type != inboxdao.ItemTypeSchedule || item.Schedule == nil {
			continue
		}
		if item.Schedule.RuleID == series.RuleID {
			if item.Schedule.OccurrenceDate == today && item.Schedule.OccurrenceID == nil {
				t.Fatalf("origin occurrence %s unexpectedly virtual", item.Schedule.OccurrenceDate)
			}
			if item.Schedule.OccurrenceDate != today && item.Schedule.OccurrenceID != nil {
				t.Fatalf("recurrence date %s unexpectedly persisted before read", item.Schedule.OccurrenceDate)
			}
			virtualDates[item.Schedule.OccurrenceDate] = true
		}
		if item.Schedule.RuleID == allDay.RuleID {
			allDayFound = item.Schedule.AllDay && item.Schedule.OccurrenceDate == "2026-03-08"
		}
		if item.Schedule.RuleID == outsideFloating.RuleID {
			priorAllDayFound = true
		}
	}
	if len(virtualDates) != 7 {
		t.Fatalf("daily overnight occurrence dates across spring DST=%v; want all seven local dates March 7-13", virtualDates)
	}
	for offset := 0; offset < 7; offset++ {
		date := time.Date(2026, time.March, 7, 0, 0, 0, 0, newYork).AddDate(0, 0, offset).Format("2006-01-02")
		if !virtualDates[date] {
			t.Errorf("missing occurrence on local date %s", date)
		}
	}
	if !allDayFound {
		t.Fatal("floating all-day occurrence on March 8 in Pacific/Kiritimati was not included by its local date")
	}
	if priorAllDayFound {
		t.Fatal("prior floating all-day date was included only because its UTC interval overlaps the Inbox boundary")
	}

	beforeOrder := inboxItemKeys(page.Items)
	materializedDate := "2026-03-08"
	materializeDailyOccurrence(t, f.pool, series, owner, api.actor, materializedDate, newYork)
	materializedPage, err := api.list.Execute(t.Context(), api.actor, inboxusecase.ListRequest{
		Size:     100,
		Snapshot: inboxusecase.Snapshot{AsOf: asOf, Timezone: "America/New_York", Today: today},
	})
	if err != nil {
		t.Fatalf("list Inbox after ordinary occurrence materialization: %v", err)
	}
	if got := inboxItemKeys(materializedPage.Items); !equalStrings(beforeOrder, got) {
		t.Fatalf("materializing a virtual occurrence changed Inbox order: before=%v after=%v", beforeOrder, got)
	}
	for _, item := range materializedPage.Items {
		if item.Type == inboxdao.ItemTypeSchedule && item.Schedule != nil && item.Schedule.RuleID == series.RuleID && item.Schedule.OccurrenceDate == materializedDate && item.Schedule.OccurrenceID == nil {
			t.Fatal("materialized recurrence still has a nil occurrence ID")
		}
	}
}

func TestRESTInboxSameOccurrencesPageForwardAndBackward(t *testing.T) {
	api := newInboxREST(t, "UTC")
	f := newInboxFixtures(t, api.pool, api.actor)
	owner := api.actor
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.Add(9 * time.Hour)
	series := f.schedule(owner, "", api.actor, "Daily series", start, start.Add(time.Hour), "daily", false, false, false)

	pages := make([]inboxPage, 0, 7)
	path := "/inbox?page_size=1"
	for offset := 0; offset < 7; offset++ {
		response := api.request(path, api.actor)
		if response.Code != http.StatusOK {
			t.Fatalf("forward page %d status/body=%d/%s", offset+1, response.Code, response.Body.String())
		}
		page := decodeInboxPage(t, response)
		if len(page.Items) != 1 || page.Items[0].Type != "schedule" || page.Items[0].Schedule == nil {
			t.Fatalf("forward page %d items=%+v; want one Schedule occurrence", offset+1, page.Items)
		}
		wantDate := today.AddDate(0, 0, offset).Format("2006-01-02")
		if page.Items[0].Schedule.RuleID != series.RuleID || page.Items[0].Schedule.OccurrenceDate != wantDate {
			t.Fatalf("forward page %d occurrence=%+v; want rule/date %s/%s", offset+1, page.Items[0].Schedule, series.RuleID, wantDate)
		}
		pages = append(pages, page)
		if offset < 6 {
			if page.NextPageToken == "" {
				t.Fatalf("forward page %d has no next token", offset+1)
			}
			path = "/inbox?" + url.Values{"page_size": {"1"}, "page_token": {page.NextPageToken}}.Encode()
		}
	}
	if pages[6].NextPageToken != "" || pages[6].PreviousPageToken == "" {
		t.Fatalf("last page next/previous token=%q/%q", pages[6].NextPageToken, pages[6].PreviousPageToken)
	}
	back := api.request("/inbox?"+url.Values{"page_size": {"1"}, "page_token": {pages[6].PreviousPageToken}}.Encode(), api.actor)
	if back.Code != http.StatusOK {
		t.Fatalf("backward same-rule page status/body=%d/%s", back.Code, back.Body.String())
	}
	backPage := decodeInboxPage(t, back)
	if len(backPage.Items) != 1 || backPage.Items[0].ID != pages[5].Items[0].ID || backPage.Items[0].Schedule.OccurrenceDate != pages[5].Items[0].Schedule.OccurrenceDate {
		t.Fatalf("backward page item=%+v; want preceding occurrence %+v", backPage.Items, pages[5].Items[0])
	}
}

func inboxItemKeys(items []inboxdao.Item) []string {
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, string(item.Type)+":"+item.ID+":"+item.CreatedAt)
	}
	return keys
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func materializeDailyOccurrence(t *testing.T, pool *pgxpool.Pool, fixture inboxScheduleFixture, ownerID, actorID, date string, location *time.Location) {
	t.Helper()
	localDate, err := time.ParseInLocation("2006-01-02", date, location)
	if err != nil {
		t.Fatal(err)
	}
	localStart := time.Date(localDate.Year(), localDate.Month(), localDate.Day(), fixture.StartAt.In(location).Hour(), fixture.StartAt.In(location).Minute(), 0, 0, location)
	localEnd := localStart.Add(2 * time.Hour)
	if _, err := pool.Exec(t.Context(), `
		INSERT INTO schedules(id,rule_id,source_revision_id,user_id,occurrence_date,start_at,end_at,title,description,
			location,project_id,assignee_id,timezone,completed,changed_by)
		VALUES($1,$2,$3,$4,$5::date,$6,$7,$8,'','',$9,$10,$11,false,$4)
	`, ulid.Make().String(), fixture.RuleID, fixture.RevisionID, ownerID, date, localStart, localEnd,
		fixture.Title, nil, actorID, "America/New_York"); err != nil {
		t.Fatalf("materialize recurrence occurrence %s: %v", date, err)
	}
}

func insertInboxUser(t *testing.T, pool *pgxpool.Pool, id, timezone string) {
	t.Helper()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,first_name,last_name,email,password_hash,email_verified_at,timezone) VALUES($1,'Inbox','Test',$2,'unused',now(),$3)`, id, id+"@inbox.test", timezone); err != nil {
		t.Fatalf("insert inbox user: %v", err)
	}
}

type inboxFixtures struct {
	t        *testing.T
	pool     *pgxpool.Pool
	users    []string
	projects []string
	rules    []string
	roles    []string
}

func newInboxFixtures(t *testing.T, pool *pgxpool.Pool, actorID string) *inboxFixtures {
	t.Helper()
	f := &inboxFixtures{t: t, pool: pool, users: []string{actorID}}
	t.Cleanup(f.cleanup)
	return f
}

func (f *inboxFixtures) user(timezone string) string {
	f.t.Helper()
	id := ulid.Make().String()
	insertInboxUser(f.t, f.pool, id, timezone)
	f.users = append(f.users, id)
	return id
}

func (f *inboxFixtures) project(ownerID, title string) string {
	f.t.Helper()
	id := ulid.Make().String()
	if _, err := f.pool.Exec(f.t.Context(), `INSERT INTO projects(id,user_id,type,title,priority,status,changed_by) VALUES($1,$2,'other',$3,'low','open',$2)`, id, ownerID, title); err != nil {
		f.t.Fatalf("insert Project fixture: %v", err)
	}
	f.projects = append(f.projects, id)
	return id
}

func (f *inboxFixtures) readRole() string {
	f.t.Helper()
	role := "inbox_read_" + strings.ToLower(ulid.Make().String())
	if _, err := f.pool.Exec(f.t.Context(), `INSERT INTO roles(role_id,name) VALUES($1,$1)`, role); err != nil {
		f.t.Fatalf("insert inbox ACL role: %v", err)
	}
	for _, resource := range []string{"project", "task", "schedule"} {
		if _, err := f.pool.Exec(f.t.Context(), `INSERT INTO role_permissions(role_id,permission_id) SELECT $1,permission_id FROM permissions WHERE resource_id=$2 AND action='read' AND effect='allow'`, role, resource); err != nil {
			f.t.Fatalf("grant %s read to inbox role: %v", resource, err)
		}
	}
	f.roles = append(f.roles, role)
	return role
}

func (f *inboxFixtures) addMember(projectID, roleID, addedBy string) {
	f.t.Helper()
	if _, err := f.pool.Exec(f.t.Context(), `INSERT INTO project_members(project_id,user_id,role_id,added_by) VALUES($1,$2,$3,$4)`, projectID, f.users[0], roleID, addedBy); err != nil {
		f.t.Fatalf("add inbox Project member: %v", err)
	}
}

func (f *inboxFixtures) task(ownerID, assigneeID, title, projectID string, createdAt time.Time, dueDate any, status string) string {
	f.t.Helper()
	id := ulid.Make().String()
	var project any
	if projectID != "" {
		project = projectID
	}
	if _, err := f.pool.Exec(f.t.Context(), `
		INSERT INTO tasks(id,user_id,project_id,assignee_id,title,description,due_date,priority,status,changed_by,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,'',$6,'high',$7,$2,$8,$8)
	`, id, ownerID, project, assigneeID, title, dueDate, status, createdAt); err != nil {
		f.t.Fatalf("insert Task fixture %q: %v", title, err)
	}
	return id
}

type inboxScheduleFixture struct {
	RuleID     string
	RevisionID string
	ScheduleID string
	Title      string
	StartAt    time.Time
}

func (f *inboxFixtures) schedule(ownerID, projectID, assigneeID, title string, startAt, endAt time.Time, frequency string, completed, skipped, deleted bool) inboxScheduleFixture {
	f.t.Helper()
	ruleID, revisionID, scheduleID := ulid.Make().String(), ulid.Make().String(), ulid.Make().String()
	f.rules = append(f.rules, ruleID)
	if err := testdb.InsertScheduleFixture(f.t.Context(), f.pool, testdb.ScheduleFixture{
		RuleID: ruleID, RevisionID: revisionID, ScheduleID: scheduleID,
		UserID: ownerID, ProjectID: projectID, AssigneeID: assigneeID, Title: title,
		Timezone: startAt.Location().String(), Frequency: frequency, Interval: 1,
		StartAt: startAt, EndAt: endAt, Completed: completed, Skipped: skipped, Deleted: deleted,
	}); err != nil {
		f.t.Fatalf("insert Schedule fixture %q: %v", title, err)
	}
	return inboxScheduleFixture{RuleID: ruleID, RevisionID: revisionID, ScheduleID: scheduleID, Title: title, StartAt: startAt}
}

func (f *inboxFixtures) scheduleRowCounts(ownerID string) (schedules, revisions int) {
	f.t.Helper()
	if err := f.pool.QueryRow(f.t.Context(), `SELECT COUNT(*) FROM schedules WHERE user_id=$1`, ownerID).Scan(&schedules); err != nil {
		f.t.Fatalf("count Schedule fixtures: %v", err)
	}
	if err := f.pool.QueryRow(f.t.Context(), `SELECT COUNT(*) FROM schedule_revisions WHERE user_id=$1`, ownerID).Scan(&revisions); err != nil {
		f.t.Fatalf("count Schedule revisions: %v", err)
	}
	return schedules, revisions
}

func (f *inboxFixtures) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, ruleID := range f.rules {
		if err := testdb.DeleteScheduleFixture(ctx, f.pool, ruleID); err != nil {
			f.t.Errorf("delete Schedule fixture %s: %v", ruleID, err)
		}
	}
	_, _ = f.pool.Exec(ctx, `DELETE FROM action_items WHERE task_id IN (SELECT id FROM tasks WHERE user_id=ANY($1::text[]))`, f.users)
	_, _ = f.pool.Exec(ctx, `DELETE FROM task_revisions WHERE id IN (SELECT id FROM tasks WHERE user_id=ANY($1::text[]))`, f.users)
	_, _ = f.pool.Exec(ctx, `DELETE FROM tasks WHERE user_id=ANY($1::text[])`, f.users)
	_, _ = f.pool.Exec(ctx, `DELETE FROM project_members WHERE project_id=ANY($1::text[])`, f.projects)
	_, _ = f.pool.Exec(ctx, `DELETE FROM project_revisions WHERE id=ANY($1::text[])`, f.projects)
	_, _ = f.pool.Exec(ctx, `DELETE FROM projects WHERE id=ANY($1::text[])`, f.projects)
	for _, role := range f.roles {
		_, _ = f.pool.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, role)
		_, _ = f.pool.Exec(ctx, `DELETE FROM roles WHERE role_id=$1`, role)
	}
	_, _ = f.pool.Exec(ctx, `DELETE FROM users WHERE id=ANY($1::text[])`, f.users)
}

func datePointer(value time.Time) *string {
	date := value.Format("2006-01-02")
	return &date
}
