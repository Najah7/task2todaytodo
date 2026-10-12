//go:build integration

package allocation_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/port/rest"
)

func TestAllocationHTTPRulesRefreshOnlyUnstartedOrdinaryFrames(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	api.grantSystemAdmin(t, userID)
	projectID := createAllocationProject(t, api, userID, "plan refresh")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 2)
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
	exceptionDate := firstDate.AddDate(0, 0, 14)
	week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(exceptionDate).Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET saved-exception week status/body=%d/%s", week.Code, week.Body.String())
	}
	var weekProjection rest.AllocationWeekResponse
	if err := json.Unmarshal(week.Body.Bytes(), &weekProjection); err != nil {
		t.Fatal(err)
	}
	var selected rest.AllocationOccurrenceResponse
	for _, occurrence := range weekProjection.Allocations {
		if occurrence.RuleID == first.RuleID && occurrence.OccurrenceDate == exceptionDate.Format("2006-01-02") {
			selected = occurrence
			break
		}
	}
	if selected.RuleID == "" {
		t.Fatalf("exception origin not in displayed week: %+v", weekProjection.Allocations)
	}
	exceptionStart := time.Date(exceptionDate.Year(), exceptionDate.Month(), exceptionDate.Day(), 14, 0, 0, 0, location)
	exception := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, exceptionDate.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`, projectID, exceptionStart.Format(time.RFC3339), exceptionStart.Add(time.Hour).Format(time.RFC3339)),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, selected.Revision)})
	if exception.Code != http.StatusOK {
		t.Fatalf("PATCH future exception status/body=%d/%s", exception.Code, exception.Body.String())
	}

	startedAt := time.Now().UTC().Truncate(time.Minute).Add(-20 * time.Minute)
	startedEnd := startedAt.Add(time.Hour)
	if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocations SET start_at=$1,end_at=$2 WHERE rule_id=$3 AND occurrence_date=$4`, startedAt, startedEnd, first.RuleID, firstDate.Format("2006-01-02")); err != nil {
		t.Fatalf("mark first frame started: %v", err)
	}

	rules := getAllocationRules(t, api, userID)
	var rule allocationRuleItem
	for _, item := range rules.Items {
		if item.ID == first.RuleID {
			rule = item
		}
	}
	if rule.ID == "" {
		t.Fatalf("weekly Rule missing from Rules: %+v", rules)
	}
	put := putAllocationRules(t, api, userID, rules.Revision, []allocationRuleWriteItem{{
		ID: rule.ID, Revision: rule.Revision, ProjectID: projectID, Weekday: rule.Weekday,
		StartTime: "10:00", EndTime: "11:00", EndDayOffset: rule.EndDayOffset, Timezone: rule.Timezone,
	}})
	if put.Code != http.StatusOK {
		t.Fatalf("PUT changed weekly Rule status/body=%d/%s", put.Code, put.Body.String())
	}

	var gotStarted, gotFuture, gotException time.Time
	if err := api.pool.QueryRow(t.Context(), `SELECT start_at FROM project_time_allocations WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NULL`, first.RuleID, firstDate.Format("2006-01-02")).Scan(&gotStarted); err != nil {
		t.Fatalf("read started frame: %v", err)
	}
	if !gotStarted.Equal(startedAt) {
		t.Fatalf("started frame moved to %s; want preserved %s", gotStarted, startedAt)
	}
	futureDate := firstDate.AddDate(0, 0, 7)
	if err := api.pool.QueryRow(t.Context(), `SELECT start_at FROM project_time_allocations WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NULL`, first.RuleID, futureDate.Format("2006-01-02")).Scan(&gotFuture); err != nil {
		t.Fatalf("read refreshed future frame: %v", err)
	}
	wantFuture := time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 10, 0, 0, 0, location)
	if !gotFuture.Equal(wantFuture.UTC()) {
		t.Fatalf("future frame start=%s; want refreshed start %s", gotFuture, wantFuture.UTC())
	}
	if err := api.pool.QueryRow(t.Context(), `SELECT start_at FROM project_time_allocation_exceptions WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NULL`, first.RuleID, exceptionDate.Format("2006-01-02")).Scan(&gotException); err != nil {
		t.Fatalf("read saved exception: %v", err)
	}
	if !gotException.Equal(exceptionStart.UTC()) {
		t.Fatalf("saved exception moved to %s; want unchanged %s", gotException, exceptionStart.UTC())
	}
}

func TestAllocationHTTPFollowingEditInvalidatesStaleRulesRevision(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "plan revision")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 2)
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
	rules := getAllocationRules(t, api, userID)
	plans := getAllocationPlans(t, api, userID)
	selectedDate, err := time.Parse("2006-01-02", first.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	edit := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, first.OccurrenceDate),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"following","rule_revision":%d}`,
			projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339), first.RuleRevision),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, first.Revision)})
	if edit.Code != http.StatusOK {
		t.Fatalf("PATCH following Rule status/body=%d/%s", edit.Code, edit.Body.String())
	}
	stalePut := putAllocationRules(t, api, userID, rules.Revision, ruleItemsToWrite(rules.Items))
	if stalePut.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale PUT after following edit status/body=%d/%s; selected origin %s", stalePut.Code, stalePut.Body.String(), selectedDate.Format("2006-01-02"))
	}
	if response := putAllocationPlans(t, api, userID, plans.Revision, planItemsToWrite(plans.Items)); response.Code != http.StatusOK {
		t.Fatalf("Plans token became stale after following Rule edit: status/body=%d/%s", response.Code, response.Body.String())
	}
}

func TestAllocationHTTPFollowingWeekdayChangeRemovesObsoletePrefill(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	api.grantSystemAdmin(t, userID)
	projectID := createAllocationProject(t, api, userID, "Following weekday change")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 2)
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
	week := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(selectedDate).Format("2006-01-02"), "")
	if week.Code != http.StatusOK {
		t.Fatalf("GET selected week status/body=%d/%s", week.Code, week.Body.String())
	}
	var projection rest.AllocationWeekResponse
	if err := json.Unmarshal(week.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	var selected rest.AllocationOccurrenceResponse
	for _, occurrence := range projection.Allocations {
		if occurrence.RuleID == first.RuleID && occurrence.OccurrenceDate == selectedDate.Format("2006-01-02") {
			selected = occurrence
			break
		}
	}
	if selected.RuleID == "" {
		t.Fatalf("selected materialized origin missing: %+v", projection.Allocations)
	}
	newStart := time.Date(selectedDate.Year(), selectedDate.Month(), selectedDate.Day()+1, 10, 0, 0, 0, location)
	updated := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, selectedDate.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"following","rule_revision":%d}`,
			projectID, newStart.Format(time.RFC3339), newStart.Add(time.Hour).Format(time.RFC3339), selected.RuleRevision),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, selected.Revision)})
	if updated.Code != http.StatusOK {
		t.Fatalf("PATCH following weekday change status/body=%d/%s", updated.Code, updated.Body.String())
	}
	selectedWeek := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(selectedDate).Format("2006-01-02"), "")
	if selectedWeek.Code != http.StatusOK {
		t.Fatalf("GET selected week after weekday change status/body=%d/%s", selectedWeek.Code, selectedWeek.Body.String())
	}
	if err := json.Unmarshal(selectedWeek.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	selectedCount := 0
	for _, occurrence := range projection.Allocations {
		if occurrence.RuleID == first.RuleID && occurrence.OccurrenceDate == selectedDate.Format("2006-01-02") {
			selectedCount++
			if !occurrence.IsException || !occurrence.StartAt.Equal(newStart.UTC()) {
				t.Fatalf("selected edited occurrence=%+v; want one exception at %s", occurrence, newStart.UTC())
			}
		}
	}
	if selectedCount != 1 {
		t.Fatalf("selected edited occurrence count=%d; want exactly 1", selectedCount)
	}
	if count := allocationCountForOrigin(t, api, first.RuleID, firstDate); count != 1 {
		t.Fatalf("earlier future origin rows=%d; want preserved 1", count)
	}
	obsoleteDate := selectedDate.AddDate(0, 0, 7)
	if count := allocationCountForOrigin(t, api, first.RuleID, obsoleteDate); count != 0 {
		t.Fatalf("obsolete old-weekday future origin rows=%d; want 0", count)
	}
	newOriginDate := selectedDate.AddDate(0, 0, 8)
	newWeek := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(newOriginDate).Format("2006-01-02"), "")
	if newWeek.Code != http.StatusOK {
		t.Fatalf("GET new weekday week status/body=%d/%s", newWeek.Code, newWeek.Body.String())
	}
	if err := json.Unmarshal(newWeek.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	foundNewWeekday := false
	for _, occurrence := range projection.Allocations {
		if occurrence.RuleID == first.RuleID && occurrence.OccurrenceDate == newOriginDate.Format("2006-01-02") {
			foundNewWeekday = occurrence.StartAt.In(location).Weekday() == newStart.Weekday()
		}
	}
	if !foundNewWeekday {
		t.Fatalf("new weekly origin %s not projected on %s: %+v", newOriginDate.Format("2006-01-02"), newStart.Weekday(), projection.Allocations)
	}
	runAllocationMaterialization(t, api, userID)
	reloaded := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+mondayUTC(selectedDate).Format("2006-01-02"), "")
	if reloaded.Code != http.StatusOK {
		t.Fatalf("GET selected week after repeat prefill status/body=%d/%s", reloaded.Code, reloaded.Body.String())
	}
	if err := json.Unmarshal(reloaded.Body.Bytes(), &projection); err != nil {
		t.Fatal(err)
	}
	selectedCount = 0
	for _, occurrence := range projection.Allocations {
		if occurrence.RuleID == first.RuleID && occurrence.OccurrenceDate == selectedDate.Format("2006-01-02") {
			selectedCount++
		}
	}
	if selectedCount != 1 {
		t.Fatalf("selected occurrence count after repeat prefill=%d; want exactly 1", selectedCount)
	}
}

func TestAllocationHTTPConcurrentVirtualEditsUseExpectedRevisionZeroOnce(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Concurrent allocation edit")
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 2)
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
	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	for _, hour := range []int{10, 12} {
		wait.Add(1)
		go func(hour int) {
			defer wait.Done()
			candidateStart := time.Date(virtualDate.Year(), virtualDate.Month(), virtualDate.Day(), hour, 0, 0, 0, location)
			response := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", first.RuleID, virtualDate.Format("2006-01-02")),
				fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`, projectID, candidateStart.Format(time.RFC3339), candidateStart.Add(time.Hour).Format(time.RFC3339)),
				[2]string{"If-Match", `"0"`})
			statuses <- response.Code
		}(hour)
	}
	wait.Wait()
	close(statuses)
	var success, precondition int
	for status := range statuses {
		if status == http.StatusOK {
			success++
		} else if status == http.StatusPreconditionFailed {
			precondition++
		} else {
			t.Fatalf("concurrent PATCH status=%d; want 200 or 412", status)
		}
	}
	if success != 1 || precondition != 1 {
		t.Fatalf("concurrent outcomes: success=%d precondition=%d; want 1 each", success, precondition)
	}
}
