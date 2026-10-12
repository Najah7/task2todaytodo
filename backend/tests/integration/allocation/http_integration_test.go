//go:build integration

package allocation_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/port/rest"
)

func TestAllocationHTTPSleepExceptionCanBeSavedAndReset(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	if response := api.request(t, userID, http.MethodPatch, "/api/users/me/sleep", `{"wake_up_time":"07:00","bedtime":"23:00"}`); response.Code != http.StatusOK {
		t.Fatalf("PATCH /users/me/sleep status/body=%d/%s", response.Code, response.Body.String())
	}

	date := time.Now().In(mustLocation(t, "Asia/Tokyo")).AddDate(0, 0, 1)
	dateKey := date.Format("2006-01-02")
	put := api.request(t, userID, http.MethodPut, "/api/daily-sleep-exceptions/"+dateKey,
		fmt.Sprintf(`{"wake_local":%q,"bedtime_local":%q,"timezone":"Asia/Tokyo"}`,
			dateKey+"T08:00", dateKey+"T22:00"), [2]string{"If-Match", `"0"`})
	if put.Code != http.StatusOK {
		t.Fatalf("PUT daily sleep exception status/body=%d/%s", put.Code, put.Body.String())
	}
	var exception rest.SleepExceptionResponse
	if err := json.Unmarshal(put.Body.Bytes(), &exception); err != nil {
		t.Fatal(err)
	}
	if !exception.HasException || exception.Timezone != "Asia/Tokyo" {
		t.Fatalf("saved sleep exception=%+v; expected a persisted Tokyo pair", exception)
	}

	weekStart := mondayUTC(date)
	week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+weekStart.Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET allocation week status/body=%d/%s", week.Code, week.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, day := range projection.Days {
		if day.Date == dateKey {
			found = day.HasSleepException
		}
	}
	if !found {
		t.Fatalf("week projection did not include saved sleep exception for %s: %+v", dateKey, projection.Days)
	}

	reset := api.request(t, userID, http.MethodDelete, "/api/daily-sleep-exceptions/"+dateKey, "", [2]string{"If-Match", fmt.Sprintf(`"%d"`, exception.Revision)})
	if reset.Code != http.StatusNoContent {
		t.Fatalf("DELETE daily sleep exception status/body=%d/%s", reset.Code, reset.Body.String())
	}
	week = api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+weekStart.Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET allocation week after reset status/body=%d/%s", week.Code, week.Body.String())
	}
	if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	for _, day := range projection.Days {
		if day.Date == dateKey && day.HasSleepException {
			t.Fatalf("reset projection still marks exception for %s: %+v", dateKey, day)
		}
	}
}

func TestAllocationHTTPRejectsSleepOverlap(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	if response := api.request(t, userID, http.MethodPatch, "/api/users/me/sleep", `{"wake_up_time":"07:00","bedtime":"23:00"}`); response.Code != http.StatusOK {
		t.Fatalf("PATCH /users/me/sleep status/body=%d/%s", response.Code, response.Body.String())
	}
	projectID := createAllocationProject(t, api, userID, "Sleep overlap")
	location := mustLocation(t, "Asia/Tokyo")
	date := time.Now().In(location).AddDate(0, 0, 2)
	dateKey := date.Format("2006-01-02")
	start := time.Date(date.Year(), date.Month(), date.Day(), 10, 0, 0, 0, location)
	allocation := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":false}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if allocation.Code != http.StatusCreated {
		t.Fatalf("POST allocation status/body=%d/%s", allocation.Code, allocation.Body.String())
	}
	profileAdjacent := api.request(t, userID, http.MethodPatch, "/api/users/me/sleep", `{"wake_up_time":"10:00","bedtime":"12:00"}`)
	if profileAdjacent.Code != http.StatusOK {
		t.Fatalf("sleep profile adjacent to allocation status/body=%d/%s; want no conflict", profileAdjacent.Code, profileAdjacent.Body.String())
	}
	profileConflict := api.request(t, userID, http.MethodPatch, "/api/users/me/sleep", `{"wake_up_time":"10:30","bedtime":"12:00"}`)
	if profileConflict.Code != http.StatusConflict {
		t.Fatalf("sleep profile overlapping allocation status/body=%d/%s; want conflict", profileConflict.Code, profileConflict.Body.String())
	}
	response := api.request(t, userID, http.MethodPut, "/api/daily-sleep-exceptions/"+dateKey,
		fmt.Sprintf(`{"wake_local":%q,"bedtime_local":%q,"timezone":"Asia/Tokyo"}`,
			dateKey+"T12:00", dateKey+"T22:00"), [2]string{"If-Match", `"0"`})
	if response.Code != http.StatusConflict {
		t.Fatalf("sleep exception overlapping allocation status/body=%d/%s; want conflict", response.Code, response.Body.String())
	}
}

func TestAllocationHTTPSleepUpdateAllowsStartedAllocation(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Started allocation")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 1)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":false}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var allocation rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &allocation); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now().UTC().Truncate(time.Minute).Add(-15 * time.Minute)
	endedAt := startedAt.Add(time.Hour)
	if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocations SET start_at=$1, end_at=$2 WHERE rule_id=$3 AND occurrence_date=$4`,
		startedAt, endedAt, allocation.RuleID, allocation.OccurrenceDate); err != nil {
		t.Fatalf("make allocation currently running: %v", err)
	}
	if _, err := api.pool.Exec(t.Context(), `INSERT INTO project_time_allocation_exceptions
		(rule_id, occurrence_date, source_revision_id, user_id, project_id, start_at, end_at, timezone, changed_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$4)`, allocation.RuleID, allocation.OccurrenceDate,
		allocation.SourceRevisionID, userID, projectID, startedAt, endedAt, allocation.Timezone); err != nil {
		t.Fatalf("save current allocation snapshot: %v", err)
	}
	hour := time.Now().In(location).Hour()
	wake, bedtime := 0, 1
	if hour < 21 {
		wake, bedtime = hour+2, hour+3
	}
	profile := api.request(t, userID, http.MethodPatch, "/api/users/me/sleep", fmt.Sprintf(`{"wake_up_time":"%02d:00","bedtime":"%02d:00"}`, wake, bedtime))
	if profile.Code != http.StatusOK {
		t.Fatalf("sleep profile update overlapping already-started allocation status/body=%d/%s; want allowed", profile.Code, profile.Body.String())
	}
}

func TestAllocationHTTPCurrentMoveConflictsWithSameRuleVirtualOccurrence(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Same-rule virtual conflict")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 1)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var first rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	firstDate, err := time.Parse("2006-01-02", first.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	targetDate := firstDate.AddDate(0, 0, 7)
	targetWeek := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(targetDate).Format("2006-01-02"), "")
	if targetWeek.Code != http.StatusOK {
		t.Fatalf("GET target virtual week status/body=%d/%s", targetWeek.Code, targetWeek.Body.String())
	}
	targetStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 9, 0, 0, 0, location)
	moved := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, first.OccurrenceDate),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`, projectID, targetStart.Format(time.RFC3339), targetStart.Add(time.Hour).Format(time.RFC3339)),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, first.Revision)})
	if moved.Code != http.StatusConflict {
		t.Fatalf("PATCH current occurrence onto same-rule virtual slot status/body=%d/%s; want conflict", moved.Code, moved.Body.String())
	}
}

func TestAllocationHTTPRejectsRulesThatOverlapSleep(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "plan sleep conflict")
	weekday := time.Now().In(mustLocation(t, "Asia/Tokyo")).AddDate(0, 0, 2).Weekday()
	rules := getAllocationRules(t, api, userID)
	response := putAllocationRules(t, api, userID, rules.Revision, []allocationRuleWriteItem{{
		ProjectID: projectID, Weekday: int(weekday), StartTime: "02:00", EndTime: "03:00", Timezone: "Asia/Tokyo",
	}})
	if response.Code != http.StatusConflict {
		t.Fatalf("PUT allocation plan overlapping sleep status/body=%d/%s; want conflict", response.Code, response.Body.String())
	}
	persisted := getAllocationRules(t, api, userID)
	if persisted.Revision != rules.Revision || len(persisted.Items) != 0 {
		t.Fatalf("rejected Rules write leaked state: %+v", persisted)
	}
}

func TestAllocationHTTPOneOffOverlapIsRejected(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	firstProject := createAllocationProject(t, api, userID, "First one-off")
	secondProject := createAllocationProject(t, api, userID, "Second one-off")
	location := mustLocation(t, "Asia/Tokyo")
	date := time.Now().In(location).AddDate(0, 0, 2)
	start := time.Date(date.Year(), date.Month(), date.Day(), 10, 0, 0, 0, location)
	create := func(projectID string) *httptest.ResponseRecorder {
		t.Helper()
		return api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
			`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":false}`,
			projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	}
	if response := create(firstProject); response.Code != http.StatusCreated {
		t.Fatalf("POST first one-off status/body=%d/%s", response.Code, response.Body.String())
	}
	if response := create(secondProject); response.Code != http.StatusConflict {
		t.Fatalf("POST overlapping one-off status/body=%d/%s; want conflict", response.Code, response.Body.String())
	}
}

func TestAllocationHTTPMovesVirtualFutureOriginBeforePrefill(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	api.grantSystemAdmin(t, userID)
	projectID := createAllocationProject(t, api, userID, "Virtual future")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 1)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var first rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	firstDate, err := time.Parse("2006-01-02", first.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	virtualDate := firstDate.AddDate(0, 0, 7)
	if count := allocationCountForOrigin(t, api, first.RuleID, virtualDate); count != 0 {
		t.Fatalf("future origin has %d concrete rows before prefill; want virtual only", count)
	}
	week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(virtualDate).Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET virtual occurrence week status/body=%d/%s", week.Code, week.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	var virtual rest.AllocationOccurrenceResponse
	for _, item := range projection.Allocations {
		if item.RuleID == first.RuleID && item.OccurrenceDate == virtualDate.Format("2006-01-02") {
			virtual = item
			break
		}
	}
	if virtual.RuleID == "" || virtual.Revision != 0 {
		t.Fatalf("future occurrence was not projected as virtual: %+v", virtual)
	}
	movedStart := time.Date(virtualDate.Year(), virtualDate.Month(), virtualDate.Day(), 13, 0, 0, 0, location)
	moved := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, virtualDate.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`,
			projectID, movedStart.Format(time.RFC3339), movedStart.Add(time.Hour).Format(time.RFC3339)),
		[2]string{"If-Match", `"0"`})
	if moved.Code != http.StatusOK {
		t.Fatalf("PATCH virtual future occurrence status/body=%d/%s", moved.Code, moved.Body.String())
	}
	var saved rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(moved.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.IsException || saved.OccurrenceDate != virtualDate.Format("2006-01-02") || !saved.StartAt.Equal(movedStart) {
		t.Fatalf("virtual edit did not preserve original identity/full snapshot: %+v", saved)
	}
	runAllocationMaterialization(t, api, userID)
	if count := allocationCountForOrigin(t, api, first.RuleID, virtualDate); count != 1 {
		t.Fatalf("materialization created %d rows for saved future origin; want exactly one", count)
	}
	var persistedStart time.Time
	if err := api.pool.QueryRow(t.Context(), `SELECT start_at FROM project_time_allocation_exceptions WHERE rule_id=$1 AND occurrence_date=$2`, first.RuleID, virtualDate).Scan(&persistedStart); err != nil {
		t.Fatalf("read virtual exception after prefill: %v", err)
	}
	if !persistedStart.Equal(movedStart.UTC()) {
		t.Fatalf("prefill changed virtual exception start=%s; want %s", persistedStart, movedStart.UTC())
	}
}

func TestAllocationHTTPFollowingEditRejectsCollisionWithMovedSameRuleOrigin(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Moved origin collision")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 1)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var first rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	firstDate, err := time.Parse("2006-01-02", first.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	movedOrigin := firstDate.AddDate(0, 0, 7)
	// Move the second Tuesday-origin occurrence onto the Wednesday that the
	// following edit will create from the fourth occurrence onward.
	movedInto := firstDate.AddDate(0, 0, 15)
	getOccurrence := func(originDate, weekDate time.Time) rest.AllocationOccurrenceResponse {
		t.Helper()
		week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(weekDate).Format("2006-01-02"), "")
		if week.Code != http.StatusOK {
			t.Fatalf("GET occurrence week status/body=%d/%s", week.Code, week.Body.String())
		}
		var projection rest.AllocationWeekResponse
		if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
			t.Fatal(err)
		}
		for _, item := range projection.Allocations {
			if item.RuleID == first.RuleID && item.OccurrenceDate == originDate.Format("2006-01-02") {
				return item
			}
		}
		t.Fatalf("origin %s is absent from week %s projection: %+v", originDate.Format("2006-01-02"), weekDate.Format("2006-01-02"), projection.Allocations)
		return rest.AllocationOccurrenceResponse{}
	}
	moveStart := time.Date(movedInto.Year(), movedInto.Month(), movedInto.Day(), 9, 0, 0, 0, location)
	toMove := getOccurrence(movedOrigin, movedOrigin)
	moved := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, movedOrigin.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`, projectID, moveStart.Format(time.RFC3339), moveStart.Add(time.Hour).Format(time.RFC3339)),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, toMove.Revision)})
	if moved.Code != http.StatusOK {
		t.Fatalf("PATCH moved origin status/body=%d/%s", moved.Code, moved.Body.String())
	}
	selected := getOccurrence(movedOrigin, movedInto)
	if !selected.IsException || !selected.StartAt.Equal(moveStart) {
		t.Fatalf("moved origin not projected in its new week: %+v", selected)
	}
	followingDate := firstDate.AddDate(0, 0, 1)
	followingStart := time.Date(followingDate.Year(), followingDate.Month(), followingDate.Day(), 9, 0, 0, 0, location)
	updated := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, firstDate.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"following","rule_revision":%d}`,
			projectID, followingStart.Format(time.RFC3339), followingStart.Add(time.Hour).Format(time.RFC3339), first.RuleRevision),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, first.Revision)})
	if updated.Code != http.StatusConflict {
		t.Fatalf("PATCH following rule colliding with moved same-rule origin status/body=%d/%s; want conflict", updated.Code, updated.Body.String())
	}
}

func TestAllocationHTTPFollowingDeleteKeepsEarlierAndTombstonesLaterException(t *testing.T) {
	for _, materializeBeforeDelete := range []bool{false, true} {
		name := "virtual future"
		if materializeBeforeDelete {
			name = "concrete future"
		}
		t.Run(name, func(t *testing.T) {
			api := newAllocationHTTP(t)
			userID := api.seedUser(t, "Asia/Tokyo")
			api.grantSystemAdmin(t, userID)
			projectID := createAllocationProject(t, api, userID, "Following delete "+name)
			location := mustLocation(t, "Asia/Tokyo")
			start := time.Now().In(location).AddDate(0, 0, 1)
			start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
			created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
				`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
				projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
			if created.Code != http.StatusCreated {
				t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
			}
			var first rest.AllocationOccurrenceResponse
			if err := json.Unmarshal(created.Body.Bytes(), &first); err != nil {
				t.Fatal(err)
			}
			firstDate, err := time.Parse("2006-01-02", first.OccurrenceDate)
			if err != nil {
				t.Fatal(err)
			}
			priorDate := firstDate.AddDate(0, 0, 7)
			selectedDate := firstDate.AddDate(0, 0, 14)
			laterDate := firstDate.AddDate(0, 0, 21)
			if materializeBeforeDelete {
				runAllocationMaterialization(t, api, userID)
			}
			getOccurrenceInWeek := func(originDate, weekDate time.Time) rest.AllocationOccurrenceResponse {
				t.Helper()
				week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(weekDate).Format("2006-01-02"), "")
				if week.Code != http.StatusOK {
					t.Fatalf("GET occurrence week status/body=%d/%s", week.Code, week.Body.String())
				}
				var projection rest.AllocationWeekResponse
				if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
					t.Fatal(err)
				}
				for _, item := range projection.Allocations {
					if item.RuleID == first.RuleID && item.OccurrenceDate == originDate.Format("2006-01-02") {
						return item
					}
				}
				t.Fatalf("origin %s is absent from projection: %+v", originDate.Format("2006-01-02"), projection.Allocations)
				return rest.AllocationOccurrenceResponse{}
			}
			getOccurrence := func(originDate time.Time) rest.AllocationOccurrenceResponse {
				return getOccurrenceInWeek(originDate, originDate)
			}

			later := getOccurrence(laterDate)
			laterStart := time.Date(laterDate.Year(), laterDate.Month(), laterDate.Day(), 14, 0, 0, 0, location)
			saved := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, laterDate.Format("2006-01-02")),
				fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`,
					projectID, laterStart.Format(time.RFC3339), laterStart.Add(time.Hour).Format(time.RFC3339)),
				[2]string{"If-Match", fmt.Sprintf(`"%d"`, later.Revision)})
			if saved.Code != http.StatusOK {
				t.Fatalf("PATCH later exception status/body=%d/%s", saved.Code, saved.Body.String())
			}
			// Preserve an earlier origin even when its full exception snapshot has
			// moved beyond the following-delete cutoff.
			movedOutDate := laterDate.AddDate(0, 0, 7)
			target := getOccurrence(movedOutDate)
			removeTarget := api.request(t, userID, http.MethodDelete, fmt.Sprintf("/api/allocations/%s/%s?scope=current", first.RuleID, movedOutDate.Format("2006-01-02")), "",
				[2]string{"If-Match", fmt.Sprintf(`"%d"`, target.Revision)})
			if removeTarget.Code != http.StatusNoContent {
				t.Fatalf("DELETE move destination origin status/body=%d/%s", removeTarget.Code, removeTarget.Body.String())
			}
			prior := getOccurrence(priorDate)
			movedOutStart := time.Date(movedOutDate.Year(), movedOutDate.Month(), movedOutDate.Day(), 16, 0, 0, 0, location)
			moveEarlier := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, priorDate.Format("2006-01-02")),
				fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`, projectID, movedOutStart.Format(time.RFC3339), movedOutStart.Add(time.Hour).Format(time.RFC3339)),
				[2]string{"If-Match", fmt.Sprintf(`"%d"`, prior.Revision)})
			if moveEarlier.Code != http.StatusOK {
				t.Fatalf("move earlier origin beyond following-delete boundary status/body=%d/%s", moveEarlier.Code, moveEarlier.Body.String())
			}
			selected := getOccurrence(selectedDate)
			deleted := api.request(t, userID, http.MethodDelete, fmt.Sprintf("/api/allocations/%s/%s?scope=following&rule_revision=%d", first.RuleID, selectedDate.Format("2006-01-02"), selected.RuleRevision), "",
				[2]string{"If-Match", fmt.Sprintf(`"%d"`, selected.Revision)})
			if deleted.Code != http.StatusNoContent {
				t.Fatalf("DELETE following allocations status/body=%d/%s", deleted.Code, deleted.Body.String())
			}
			if count := allocationCountForOrigin(t, api, first.RuleID, priorDate); count != 1 {
				t.Fatalf("earlier concrete future origin rows=%d; want 1", count)
			}
			if count := allocationCountForOrigin(t, api, first.RuleID, selectedDate); count != 0 {
				t.Fatalf("deleted selected origin has %d live concrete rows; want 0", count)
			}
			var deletedExceptions int
			if err := api.pool.QueryRow(t.Context(), `SELECT count(*) FROM project_time_allocation_exceptions
				WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NOT NULL`, first.RuleID, laterDate.Format("2006-01-02")).Scan(&deletedExceptions); err != nil {
				t.Fatalf("read later exception tombstone: %v", err)
			}
			if deletedExceptions != 1 {
				t.Fatalf("later exception tombstones=%d; want 1", deletedExceptions)
			}
			var priorExceptionDeleted bool
			if err := api.pool.QueryRow(t.Context(), `SELECT deleted_at IS NOT NULL FROM project_time_allocation_exceptions
				WHERE rule_id=$1 AND occurrence_date=$2`, first.RuleID, priorDate.Format("2006-01-02")).Scan(&priorExceptionDeleted); err != nil {
				t.Fatalf("read earlier moved exception: %v", err)
			}
			if priorExceptionDeleted {
				t.Fatal("following delete tombstoned an earlier-origin exception moved to a later actual date")
			}

			runAllocationMaterialization(t, api, userID)
			if count := allocationCountForOrigin(t, api, first.RuleID, priorDate); count != 1 {
				t.Fatalf("earlier exception projection rows=%d after materialization; want exactly one", count)
			}
			if count := allocationCountForOrigin(t, api, first.RuleID, laterDate); count != 0 {
				t.Fatalf("later deleted exception was recreated with %d live rows", count)
			}
			movedWeek := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(movedOutDate).Format("2006-01-02"), "")
			if movedWeek.Code != http.StatusOK {
				t.Fatalf("GET moved earlier exception week status/body=%d/%s", movedWeek.Code, movedWeek.Body.String())
			}
			var movedProjection rest.AllocationWeekResponse
			if err := json.Unmarshal(movedWeek.Body.Bytes(), &movedProjection); err != nil {
				t.Fatal(err)
			}
			foundMovedEarlier := false
			for _, item := range movedProjection.Allocations {
				if item.RuleID == first.RuleID && item.OccurrenceDate == priorDate.Format("2006-01-02") && item.StartAt.Equal(movedOutStart) {
					foundMovedEarlier = true
				}
			}
			if !foundMovedEarlier {
				t.Fatalf("earlier-origin moved exception disappeared from actual week: %+v", movedProjection.Allocations)
			}
		})
	}
}

func TestAllocationHTTPFollowingDeletePreservesStartedLaterOrigin(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	api.grantSystemAdmin(t, userID)
	projectID := createAllocationProject(t, api, userID, "Started later origin")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 1)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var selected rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &selected); err != nil {
		t.Fatal(err)
	}
	firstDate, err := time.Parse("2006-01-02", selected.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	selectedDate := time.Now().In(location)
	selectedDate = time.Date(selectedDate.Year(), selectedDate.Month(), selectedDate.Day(), 0, 0, 0, 0, location)
	laterOrigin := firstDate.AddDate(0, 0, 7)
	if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocation_rule_revisions SET effective_from=$1 WHERE rule_id=$2`, selectedDate.Format("2006-01-02"), selected.RuleID); err != nil {
		t.Fatalf("move recurrence origin window into past: %v", err)
	}
	if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocations SET occurrence_date=$1 WHERE rule_id=$2 AND occurrence_date=$3`,
		selectedDate.Format("2006-01-02"), selected.RuleID, firstDate.Format("2006-01-02")); err != nil {
		t.Fatalf("move selected projection identity into past: %v", err)
	}
	runAllocationMaterialization(t, api, userID)
	now := time.Now().UTC().Truncate(time.Minute)
	selectedStart := now.Add(-2 * time.Hour)
	if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocations SET start_at=$1,end_at=$2 WHERE rule_id=$3 AND occurrence_date=$4`,
		selectedStart, selectedStart.Add(time.Hour), selected.RuleID, selectedDate.Format("2006-01-02")); err != nil {
		t.Fatalf("make selected origin a past occurrence: %v", err)
	}
	laterStart := now.Add(-30 * time.Minute)
	if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocations SET start_at=$1,end_at=$2 WHERE rule_id=$3 AND occurrence_date=$4`,
		laterStart, now.Add(30*time.Minute), selected.RuleID, laterOrigin.Format("2006-01-02")); err != nil {
		t.Fatalf("make later origin already started: %v", err)
	}
	week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(selectedDate).Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET selected origin week status/body=%d/%s", week.Code, week.Body.String())
	}
	var selectedWeek rest.AllocationWeekResponse
	if err := json.Unmarshal(week.Body.Bytes(), &selectedWeek); err != nil {
		t.Fatal(err)
	}
	foundSelected := false
	for _, occurrence := range selectedWeek.Allocations {
		if occurrence.RuleID == selected.RuleID && occurrence.OccurrenceDate == selectedDate.Format("2006-01-02") {
			selected = occurrence
			foundSelected = true
		}
	}
	if !foundSelected {
		t.Fatalf("selected origin %s missing from public weekly projection: %+v", selectedDate.Format("2006-01-02"), selectedWeek.Allocations)
	}
	deleteResponse := api.request(t, userID, http.MethodDelete,
		fmt.Sprintf("/api/allocations/%s/%s?scope=following&rule_revision=%d", selected.RuleID, selectedDate.Format("2006-01-02"), selected.RuleRevision), "",
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, selected.Revision)})
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("DELETE following past origin status/body=%d/%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	var deletedAt bool
	if err := api.pool.QueryRow(t.Context(), `SELECT deleted_at IS NOT NULL FROM project_time_allocations WHERE rule_id=$1 AND occurrence_date=$2`,
		selected.RuleID, laterOrigin.Format("2006-01-02")).Scan(&deletedAt); err != nil {
		t.Fatalf("read later started origin: %v", err)
	}
	if deletedAt {
		t.Fatal("following delete removed a later-origin projection whose actual occurrence had already started")
	}
	currentWeekResponse := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(now.In(location)).Format("2006-01-02"), "")
	if currentWeekResponse.Code != http.StatusOK {
		t.Fatalf("GET current week after following delete status/body=%d/%s", currentWeekResponse.Code, currentWeekResponse.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(currentWeekResponse.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range projection.Allocations {
		if item.RuleID == selected.RuleID && item.OccurrenceDate == laterOrigin.Format("2006-01-02") && item.StartAt.Equal(laterStart) {
			found = true
		}
	}
	if !found {
		t.Fatalf("started later-origin concrete frame disappeared after following delete: %+v", projection.Allocations)
	}
}

func TestAllocationHTTPSharedProjectKeepsEachOwnersAllocationsPrivate(t *testing.T) {
	api := newAllocationHTTP(t)
	ownerID := api.seedUser(t, "Asia/Tokyo")
	memberID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, ownerID, "Shared allocation project")
	api.seedProjectMember(t, projectID, memberID, ownerID, "viewer")
	project := api.request(t, memberID, http.MethodGet, "/api/projects/"+projectID, "")
	if project.Code != http.StatusOK {
		t.Fatalf("shared member cannot read Project status/body=%d/%s", project.Code, project.Body.String())
	}

	location := mustLocation(t, "Asia/Tokyo")
	date := time.Now().In(location).AddDate(0, 0, 2)
	start := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, location)
	ownerAllocation := api.request(t, ownerID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":false}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if ownerAllocation.Code != http.StatusCreated {
		t.Fatalf("owner POST allocation status/body=%d/%s", ownerAllocation.Code, ownerAllocation.Body.String())
	}
	memberWeek := api.request(t, memberID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(date).Format("2006-01-02"), "")
	if memberWeek.Code != http.StatusOK {
		t.Fatalf("shared member GET allocation week status/body=%d/%s", memberWeek.Code, memberWeek.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(memberWeek.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	for _, item := range projection.Allocations {
		if item.ProjectID == projectID {
			t.Fatalf("shared member saw another owner's allocation: %+v", item)
		}
	}

	memberStart := start.Add(2 * time.Hour)
	memberAllocation := api.request(t, memberID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":false}`,
		projectID, memberStart.Format(time.RFC3339), memberStart.Add(time.Hour).Format(time.RFC3339)))
	if memberAllocation.Code != http.StatusCreated {
		t.Fatalf("read member cannot create own Project allocation status/body=%d/%s", memberAllocation.Code, memberAllocation.Body.String())
	}
	var own rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(memberAllocation.Body.Bytes(), &own); err != nil {
		t.Fatal(err)
	}
	if own.ProjectID != projectID {
		t.Fatalf("member's own allocation response=%+v", own)
	}
}

func TestAllocationHTTPMaterializationIsIdempotentAndRespectsExceptions(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	api.grantSystemAdmin(t, userID)
	projectID := createAllocationProject(t, api, userID, "Materialized allocations")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 1)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var first rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.RuleID == "" || first.OccurrenceDate == "" {
		t.Fatalf("created weekly allocation response=%+v", first)
	}

	movedStart := start.Add(3 * time.Hour)
	moved := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, first.OccurrenceDate),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`,
			projectID, movedStart.Format(time.RFC3339), movedStart.Add(time.Hour).Format(time.RFC3339)),
		[2]string{"If-Match", `"1"`})
	if moved.Code != http.StatusOK {
		t.Fatalf("PATCH allocation occurrence status/body=%d/%s", moved.Code, moved.Body.String())
	}
	var saved rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(moved.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.IsException || saved.OccurrenceDate != first.OccurrenceDate || !saved.StartAt.Equal(movedStart) {
		t.Fatalf("moved allocation lost exception identity/snapshot: %+v", saved)
	}

	runAllocationMaterialization(t, api, userID)
	runAllocationMaterialization(t, api, userID)
	var currentStart time.Time
	if err := api.pool.QueryRow(t.Context(), `SELECT start_at FROM project_time_allocation_exceptions WHERE rule_id=$1 AND occurrence_date=$2`, first.RuleID, first.OccurrenceDate).Scan(&currentStart); err != nil {
		t.Fatalf("read saved moved allocation exception: %v", err)
	}
	if !currentStart.Equal(movedStart.UTC()) {
		t.Fatalf("materialization overwrote moved exception start=%s; want %s", currentStart, movedStart.UTC())
	}
	var occurrences int
	if err := api.pool.QueryRow(t.Context(), `SELECT count(*) FROM project_time_allocations WHERE rule_id=$1 AND deleted_at IS NULL`, first.RuleID).Scan(&occurrences); err != nil {
		t.Fatal(err)
	}
	if occurrences != 3 {
		t.Fatalf("materialized recurring rows=%d; want current and next two weeks", occurrences)
	}

	firstDate, err := time.Parse("2006-01-02", first.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	deletedDate := firstDate.AddDate(0, 0, 7)
	deletedWeek := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(deletedDate).Format("2006-01-02"), "")
	if deletedWeek.Code != http.StatusOK {
		t.Fatalf("GET next allocation week status/body=%d/%s", deletedWeek.Code, deletedWeek.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(deletedWeek.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	var next rest.AllocationOccurrenceResponse
	for _, item := range projection.Allocations {
		if item.RuleID == first.RuleID && item.OccurrenceDate == deletedDate.Format("2006-01-02") {
			next = item
			break
		}
	}
	if next.RuleID == "" {
		t.Fatalf("next materialized allocation missing from week: %+v", projection.Allocations)
	}
	deleted := api.request(t, userID, http.MethodDelete, fmt.Sprintf("/api/allocations/%s/%s?scope=current", next.RuleID, deletedDate.Format("2006-01-02")), "",
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, next.Revision)})
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("DELETE allocation occurrence status/body=%d/%s", deleted.Code, deleted.Body.String())
	}
	runAllocationMaterialization(t, api, userID)
	var tombstones int
	if err := api.pool.QueryRow(t.Context(), `SELECT count(*) FROM project_time_allocation_exceptions WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NOT NULL`, first.RuleID, deletedDate).Scan(&tombstones); err != nil {
		t.Fatal(err)
	}
	if tombstones != 1 {
		t.Fatalf("deleted occurrence exception tombstones=%d; want 1", tombstones)
	}
	var liveDeletedDate int
	if err := api.pool.QueryRow(t.Context(), `SELECT count(*) FROM project_time_allocations WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NULL`, first.RuleID, deletedDate).Scan(&liveDeletedDate); err != nil {
		t.Fatal(err)
	}
	if liveDeletedDate != 0 {
		t.Fatalf("deleted occurrence was recreated by materialization: rows=%d", liveDeletedDate)
	}
}

func TestAllocationHTTPFollowingRuleEditPreservesEarlierAndSavedExceptions(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	api.grantSystemAdmin(t, userID)
	projectID := createAllocationProject(t, api, userID, "Following edit allocations")
	location := mustLocation(t, "Asia/Tokyo")
	// Start with an already-ended occurrence so the following edit must retain
	// a started historical frame while refreshing only its future cadence.
	start := time.Now().In(location).AddDate(0, 0, -2)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var first rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	runAllocationMaterialization(t, api, userID)
	firstDate, err := time.Parse("2006-01-02", first.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	selectedDate := firstDate.AddDate(0, 0, 7)
	exceptionDate := firstDate.AddDate(0, 0, 14)
	exceptionStart := time.Date(exceptionDate.Year(), exceptionDate.Month(), exceptionDate.Day(), 13, 0, 0, 0, location)
	saved := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, exceptionDate.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`,
			projectID, exceptionStart.Format(time.RFC3339), exceptionStart.Add(time.Hour).Format(time.RFC3339)),
		[2]string{"If-Match", `"1"`})
	if saved.Code != http.StatusOK {
		t.Fatalf("PATCH saved future exception status/body=%d/%s", saved.Code, saved.Body.String())
	}
	var selected rest.AllocationOccurrenceResponse
	week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(selectedDate).Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET selected allocation week status/body=%d/%s", week.Code, week.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	for _, item := range projection.Allocations {
		if item.RuleID == first.RuleID && item.OccurrenceDate == selectedDate.Format("2006-01-02") {
			selected = item
			break
		}
	}
	if selected.RuleID == "" {
		t.Fatalf("selected occurrence not materialized: %+v", projection.Allocations)
	}
	newStart := time.Date(selectedDate.Year(), selectedDate.Month(), selectedDate.Day(), 10, 0, 0, 0, location)
	updated := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, selectedDate.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"following","rule_revision":%d}`,
			projectID, newStart.Format(time.RFC3339), newStart.Add(time.Hour).Format(time.RFC3339), selected.RuleRevision),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, selected.Revision)})
	if updated.Code != http.StatusOK {
		t.Fatalf("PATCH following allocation rule status/body=%d/%s", updated.Code, updated.Body.String())
	}

	assertAllocationStart := func(date time.Time, want time.Time) {
		t.Helper()
		var got time.Time
		if err := api.pool.QueryRow(t.Context(), `SELECT start_at FROM project_time_allocations WHERE rule_id=$1 AND occurrence_date=$2`, first.RuleID, date.Format("2006-01-02")).Scan(&got); err != nil {
			t.Fatalf("read occurrence %s: %v", date.Format("2006-01-02"), err)
		}
		if !got.Equal(want.UTC()) {
			t.Fatalf("occurrence %s start=%s; want %s", date.Format("2006-01-02"), got, want.UTC())
		}
	}
	assertAllocationStart(firstDate, start)
	assertAllocationStart(selectedDate, newStart)
	assertAllocationStart(exceptionDate, exceptionStart)
}

func TestAllocationHTTPProjectReopenKeepsConflictingRulePausedUntilEdited(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	firstProject := createAllocationProject(t, api, userID, "Paused allocation")
	secondProject := createAllocationProject(t, api, userID, "Conflicting allocation")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 2)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	createWeekly := func(projectID string) rest.AllocationOccurrenceResponse {
		t.Helper()
		response := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
			`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
			projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
		if response.Code != http.StatusCreated {
			t.Fatalf("POST allocation for %s status/body=%d/%s", projectID, response.Code, response.Body.String())
		}
		var occurrence rest.AllocationOccurrenceResponse
		if err := json.Unmarshal(response.Body.Bytes(), &occurrence); err != nil {
			t.Fatal(err)
		}
		return occurrence
	}
	pausedRule := createWeekly(firstProject)
	rulesBeforePause := getAllocationRules(t, api, userID)
	plansBeforePause := getAllocationPlans(t, api, userID)

	complete := api.request(t, userID, http.MethodPost, "/api/projects/"+firstProject+":complete", "", [2]string{"If-Match", `"1"`})
	if complete.Code != http.StatusOK {
		t.Fatalf("complete Project status/body=%d/%s", complete.Code, complete.Body.String())
	}
	rulesAfterPause := getAllocationRules(t, api, userID)
	if rulesAfterPause.Revision == rulesBeforePause.Revision {
		t.Fatal("Project pause did not invalidate Rules token")
	}
	if got := getAllocationPlans(t, api, userID); got.Revision != plansBeforePause.Revision {
		t.Fatalf("Project pause changed Plans token: before=%q after=%q", plansBeforePause.Revision, got.Revision)
	}
	if response := createWeekly(secondProject); response.RuleID == "" {
		t.Fatalf("create active conflicting allocation response=%+v", response)
	}
	reopen := api.request(t, userID, http.MethodPost, "/api/projects/"+firstProject+":reopen", "", [2]string{"If-Match", `"2"`})
	if reopen.Code != http.StatusOK {
		t.Fatalf("reopen Project status/body=%d/%s", reopen.Code, reopen.Body.String())
	}

	rulesBeforeReopen := getAllocationRules(t, api, userID)
	plansBeforeReopen := getAllocationPlans(t, api, userID)
	pausedReason := ""
	for _, rule := range rulesBeforeReopen.Items {
		if rule.ProjectID != firstProject {
			continue
		}
		if rule.ID == pausedRule.RuleID {
			pausedReason = rule.PausedReason
		}
	}
	if pausedReason == "" {
		t.Fatalf("reopened conflicting rule was not retained as paused: %+v", rulesBeforeReopen.Items)
	}
	if rulesBeforeReopen.Revision == "" || plansBeforeReopen.Revision == "" {
		t.Fatal("missing collection revision")
	}

	newStart := time.Date(start.Year(), start.Month(), start.Day(), 12, 0, 0, 0, location)
	edit := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", pausedRule.RuleID, pausedRule.OccurrenceDate),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"following","rule_revision":%d}`,
			firstProject, newStart.Format(time.RFC3339), newStart.Add(time.Hour).Format(time.RFC3339), pausedRule.RuleRevision),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, pausedRule.Revision)})
	if edit.Code != http.StatusOK {
		t.Fatalf("edit paused allocation to resolve conflict status/body=%d/%s", edit.Code, edit.Body.String())
	}
	updatedRules := getAllocationRules(t, api, userID)
	if updatedRules.Revision == rulesBeforeReopen.Revision {
		t.Fatal("Project pause/reopen or following edit did not change Rules token")
	}
	for _, rule := range updatedRules.Items {
		if rule.ProjectID != firstProject {
			continue
		}
		if rule.ID == pausedRule.RuleID && rule.PausedReason != "" {
			t.Fatalf("resolving rule edit left it paused: %+v", rule)
		}
	}
	if plansAfter := getAllocationPlans(t, api, userID); plansAfter.Revision != plansBeforeReopen.Revision {
		t.Fatalf("Project/rule lifecycle changed Plans token: before=%q after=%q", plansBeforeReopen.Revision, plansAfter.Revision)
	}
}

func TestAllocationHTTPProjectReopenValidatesSavedOccurrenceSnapshots(t *testing.T) {
	for _, scenario := range []struct {
		name               string
		exceptionHour      int
		moveOriginIntoPast bool
		wantReason         string
	}{
		{name: "saved override clears original conflict", exceptionHour: 12},
		{name: "saved override conflicts", exceptionHour: 10, wantReason: "allocation_conflict"},
		{name: "older origin moved forward still conflicts", exceptionHour: 10, moveOriginIntoPast: true, wantReason: "allocation_conflict"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			api := newAllocationHTTP(t)
			userID := api.seedUser(t, "Asia/Tokyo")
			projectID := createAllocationProject(t, api, userID, "Resume snapshot source "+scenario.name)
			conflictProjectID := createAllocationProject(t, api, userID, "Resume snapshot conflict "+scenario.name)
			location := mustLocation(t, "Asia/Tokyo")
			date := time.Now().In(location).AddDate(0, 0, 2)
			date = time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, location)
			create := func(projectID string, start time.Time) rest.AllocationOccurrenceResponse {
				t.Helper()
				response := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
					`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":false}`,
					projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
				if response.Code != http.StatusCreated {
					t.Fatalf("POST one-off allocation status/body=%d/%s", response.Code, response.Body.String())
				}
				var occurrence rest.AllocationOccurrenceResponse
				if err := json.Unmarshal(response.Body.Bytes(), &occurrence); err != nil {
					t.Fatal(err)
				}
				return occurrence
			}
			source := create(projectID, date)
			exceptionStart := time.Date(date.Year(), date.Month(), date.Day(), scenario.exceptionHour, 0, 0, 0, location)
			updated := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", source.RuleID, source.OccurrenceDate),
				fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`, projectID, exceptionStart.Format(time.RFC3339), exceptionStart.Add(time.Hour).Format(time.RFC3339)),
				[2]string{"If-Match", fmt.Sprintf(`"%d"`, source.Revision)})
			if updated.Code != http.StatusOK {
				t.Fatalf("PATCH source exception status/body=%d/%s", updated.Code, updated.Body.String())
			}
			if scenario.moveOriginIntoPast {
				pastOrigin := date.AddDate(0, 0, -14)
				if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocations SET occurrence_date=$1 WHERE rule_id=$2 AND occurrence_date=$3`,
					pastOrigin.Format("2006-01-02"), source.RuleID, source.OccurrenceDate); err != nil {
					t.Fatalf("move exception projection to an older identity: %v", err)
				}
				if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocation_exceptions SET occurrence_date=$1 WHERE rule_id=$2 AND occurrence_date=$3`,
					pastOrigin.Format("2006-01-02"), source.RuleID, source.OccurrenceDate); err != nil {
					t.Fatalf("move exception identity outside resume origin window: %v", err)
				}
			}
			complete := api.request(t, userID, http.MethodPost, "/api/projects/"+projectID+":complete", "", [2]string{"If-Match", `"1"`})
			if complete.Code != http.StatusOK {
				t.Fatalf("complete source Project status/body=%d/%s", complete.Code, complete.Body.String())
			}
			conflictStart := date
			if scenario.wantReason != "" {
				conflictStart = exceptionStart
			}
			create(conflictProjectID, conflictStart)
			reopened := api.request(t, userID, http.MethodPost, "/api/projects/"+projectID+":reopen", "", [2]string{"If-Match", `"2"`})
			if reopened.Code != http.StatusOK {
				t.Fatalf("reopen source Project status/body=%d/%s", reopened.Code, reopened.Body.String())
			}
			var gotReason sql.NullString
			if err := api.pool.QueryRow(t.Context(), `SELECT pause_reason FROM project_time_allocation_rules WHERE id=$1`, source.RuleID).Scan(&gotReason); err != nil {
				t.Fatalf("read reopened rule pause reason: %v", err)
			}
			if gotReason.String != scenario.wantReason {
				t.Fatalf("reopened rule pause reason=%q; want %q", gotReason.String, scenario.wantReason)
			}
		})
	}
}

func TestAllocationHTTPWritesOnDoneProjectRemainPaused(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	createProject := createAllocationProject(t, api, userID, "Done Project allocation create")
	planProject := createAllocationProject(t, api, userID, "Done Project allocation plan")
	for _, projectID := range []string{createProject, planProject} {
		completed := api.request(t, userID, http.MethodPost, "/api/projects/"+projectID+":complete", "", [2]string{"If-Match", `"1"`})
		if completed.Code != http.StatusOK {
			t.Fatalf("complete Project %s status/body=%d/%s", projectID, completed.Code, completed.Body.String())
		}
	}
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 2)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		createProject, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST allocation for done Project status/body=%d/%s", created.Code, created.Body.String())
	}
	var createOccurrence rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &createOccurrence); err != nil {
		t.Fatal(err)
	}

	rulesBefore := getAllocationRules(t, api, userID)
	var createdRule allocationRuleItem
	for _, rule := range rulesBefore.Items {
		if rule.ProjectID == createProject {
			createdRule = rule
		}
	}
	if createdRule.ID == "" {
		t.Fatalf("Create did not register rule for done Project: %+v", rulesBefore.Items)
	}
	weekday := int(start.Weekday())
	put := putAllocationRules(t, api, userID, rulesBefore.Revision, []allocationRuleWriteItem{
		{ID: createdRule.ID, Revision: createdRule.Revision, ProjectID: createProject, Weekday: createdRule.Weekday, StartTime: createdRule.StartTime, EndTime: createdRule.EndTime, EndDayOffset: createdRule.EndDayOffset, Timezone: createdRule.Timezone},
		{ProjectID: planProject, Weekday: weekday, StartTime: "09:00", EndTime: "10:00", Timezone: "Asia/Tokyo"},
	})
	if put.Code != http.StatusOK {
		t.Fatalf("PUT allocation rules for done Project status/body=%d/%s", put.Code, put.Body.String())
	}

	persisted := getAllocationRules(t, api, userID)
	pausedProjects := make(map[string]bool, 2)
	for _, rule := range persisted.Items {
		if rule.ProjectID != createProject && rule.ProjectID != planProject {
			continue
		}
		if rule.PausedReason != "project_completed" {
			t.Fatalf("rule written for done Project %s was not paused: %+v", rule.ProjectID, rule)
		}
		pausedProjects[rule.ProjectID] = true
	}
	if !pausedProjects[createProject] || !pausedProjects[planProject] {
		t.Fatalf("Rules omitted entries written for done Projects: %+v", persisted.Items)
	}
	for _, projectID := range []string{createProject, planProject} {
		response := api.request(t, userID, http.MethodGet, "/api/projects/"+projectID, "")
		if response.Code != http.StatusOK {
			t.Fatalf("GET done Project status/body=%d/%s", response.Code, response.Body.String())
		}
		var project rest.ProjectResponse
		if err := json.Unmarshal(response.Body.Bytes(), &project); err != nil {
			t.Fatal(err)
		}
		if project.Status != "done" {
			t.Fatalf("allocation write changed Project %s status to %q", projectID, project.Status)
		}
	}
	week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(start).Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET week after done Project writes status/body=%d/%s", week.Code, week.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	for _, item := range projection.Allocations {
		if item.ProjectID == createProject || item.ProjectID == planProject {
			t.Fatalf("done Project allocation appeared in future week: %+v", item)
		}
	}
	if createOccurrence.RuleID == "" {
		t.Fatalf("POST done Project allocation response=%+v", createOccurrence)
	}
}

func runAllocationMaterialization(t *testing.T, api *allocationHTTP, actorID string) {
	t.Helper()
	started := api.request(t, actorID, http.MethodPost, "/api/admin/jobs/prefill-allocations:run", "")
	if started.Code != http.StatusAccepted {
		t.Fatalf("POST allocation materialization status/body=%d/%s", started.Code, started.Body.String())
	}
	var operation rest.OperationResponse
	if err := json.Unmarshal(started.Body.Bytes(), &operation); err != nil {
		t.Fatal(err)
	}
	if operation.Name == "" {
		t.Fatalf("materialization operation response=%+v", operation)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !operation.Done && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		response := api.request(t, actorID, http.MethodGet, "/api/"+operation.Name, "")
		if response.Code != http.StatusOK {
			t.Fatalf("GET allocation operation status/body=%d/%s", response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &operation); err != nil {
			t.Fatal(err)
		}
	}
	if !operation.Done || operation.Metadata.State != "succeeded" {
		t.Fatalf("allocation materialization did not succeed: %+v", operation)
	}
}

func createAllocationProject(t *testing.T, api *allocationHTTP, userID, title string) string {
	t.Helper()
	response := api.request(t, userID, http.MethodPost, "/api/projects", fmt.Sprintf(`{"title":%q}`, title))
	if response.Code != http.StatusCreated {
		t.Fatalf("POST project status/body=%d/%s", response.Code, response.Body.String())
	}
	var project rest.ProjectResponse
	if err := json.Unmarshal(response.Body.Bytes(), &project); err != nil {
		t.Fatalf("decode Project response: %v", err)
	}
	api.trackProject(t, project.ID)
	return project.ID
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return location
}

func mondayUTC(date time.Time) time.Time {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	return date.AddDate(0, 0, -((int(date.Weekday()) + 6) % 7))
}
