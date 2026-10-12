//go:build integration

package allocation_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/port/rest"
)

type allocationRuleItem struct {
	ID           string `json:"id,omitempty"`
	Revision     int32  `json:"revision,omitempty"`
	ProjectID    string `json:"project_id,omitempty"`
	ProjectTitle string `json:"project_title,omitempty"`
	Weekday      int    `json:"weekday"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	EndDayOffset int    `json:"end_day_offset"`
	Timezone     string `json:"timezone"`
	PausedReason string `json:"paused_reason,omitempty"`
}

type allocationRuleCollection struct {
	Revision string               `json:"revision"`
	Timezone string               `json:"timezone"`
	Items    []allocationRuleItem `json:"items"`
}

type allocationPlanItem struct {
	ProjectID     string `json:"project_id"`
	Title         string `json:"title,omitempty"`
	TargetMinutes int    `json:"target_minutes"`
}

type allocationPlans struct {
	Revision string               `json:"revision"`
	Items    []allocationPlanItem `json:"items"`
}

type allocationRuleWriteItem struct {
	ID           string `json:"id,omitempty"`
	Revision     int32  `json:"revision,omitempty"`
	ProjectID    string `json:"project_id"`
	Weekday      int    `json:"weekday"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	EndDayOffset int    `json:"end_day_offset"`
	Timezone     string `json:"timezone"`
}

type allocationPlanWriteItem struct {
	ProjectID     string `json:"project_id"`
	TargetMinutes int    `json:"target_minutes"`
}

func TestAllocationHTTPRulesAndPlansHaveIndependentContracts(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")

	rulesResponse := api.request(t, userID, http.MethodGet, "/api/allocation-rules", "")
	if rulesResponse.Code != http.StatusOK {
		t.Fatalf("GET allocation rules status/body=%d/%s", rulesResponse.Code, rulesResponse.Body.String())
	}
	rules := getAllocationRules(t, api, userID)
	if rules.Items == nil || rules.Revision == "" {
		t.Fatalf("GET allocation rules returned incomplete collection: %+v", rules)
	}
	if containsJSONKey(t, rulesResponse.Body.Bytes(), "target_minutes") {
		t.Fatalf("GET allocation rules included target_minutes: %s", rulesResponse.Body.String())
	}
	plansResponse := api.request(t, userID, http.MethodGet, "/api/allocation-plans", "")
	if plansResponse.Code != http.StatusOK {
		t.Fatalf("GET allocation plans status/body=%d/%s", plansResponse.Code, plansResponse.Body.String())
	}
	plans := getAllocationPlans(t, api, userID)
	if plans.Items == nil || plans.Revision == "" {
		t.Fatalf("GET allocation plans returned incomplete collection: %+v", plans)
	}
	if containsJSONKey(t, plansResponse.Body.Bytes(), "rules") || containsJSONKey(t, plansResponse.Body.Bytes(), "weekday") || containsJSONKey(t, plansResponse.Body.Bytes(), "timezone") {
		t.Fatalf("GET allocation plans included rule data: %s", plansResponse.Body.String())
	}
	for _, oldPath := range []string{"/api/allocation-settings", "/api/allocation-targets", "/api/allocation-plan"} {
		if response := api.request(t, userID, http.MethodGet, oldPath, ""); response.Code != http.StatusNotFound {
			t.Fatalf("GET removed route %s status/body=%d/%s; want 404", oldPath, response.Code, response.Body.String())
		}
	}
}

func TestAllocationHTTPRulesPutPreservesPlansAndAllowsRuleOnlyProject(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	planProject := createAllocationProject(t, api, userID, "Plan only")
	ruleProject := createAllocationProject(t, api, userID, "Rule only")

	plans := getAllocationPlans(t, api, userID)
	putPlans := putAllocationPlans(t, api, userID, plans.Revision, []allocationPlanWriteItem{{ProjectID: planProject, TargetMinutes: 300}})
	if putPlans.Code != http.StatusOK {
		t.Fatalf("PUT allocation plans status/body=%d/%s", putPlans.Code, putPlans.Body.String())
	}
	savedPlans := getAllocationPlans(t, api, userID)

	rules := getAllocationRules(t, api, userID)
	putRules := putAllocationRules(t, api, userID, rules.Revision, []allocationRuleWriteItem{{
		ProjectID: ruleProject, Weekday: 2, StartTime: "09:00", EndTime: "10:00", Timezone: "Asia/Tokyo",
	}})
	if putRules.Code != http.StatusOK {
		t.Fatalf("PUT allocation rules status/body=%d/%s", putRules.Code, putRules.Body.String())
	}
	gotPlans := getAllocationPlans(t, api, userID)
	if gotPlans.Revision != savedPlans.Revision || len(gotPlans.Items) != 1 || gotPlans.Items[0].ProjectID != planProject || gotPlans.Items[0].TargetMinutes != 300 {
		t.Fatalf("Plans changed after Rules PUT: before=%+v after=%+v", savedPlans, gotPlans)
	}
	gotRules := getAllocationRules(t, api, userID)
	if len(gotRules.Items) != 1 || gotRules.Items[0].ProjectID != ruleProject {
		t.Fatalf("Rules PUT did not save rule-only project: %+v", gotRules.Items)
	}
	if gotRules.Revision == rules.Revision {
		t.Fatalf("Rules token did not change after Rules PUT: %q", gotRules.Revision)
	}
}

func TestAllocationHTTPPlansPutPreservesRulesAndAllocationSnapshots(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	api.grantSystemAdmin(t, userID)
	projectID := createAllocationProject(t, api, userID, "Plans update preserves rules")
	location := mustLocation(t, "Asia/Tokyo")

	rules := getAllocationRules(t, api, userID)
	putRules := putAllocationRules(t, api, userID, rules.Revision, []allocationRuleWriteItem{{
		ProjectID: projectID, Weekday: int(time.Now().In(location).AddDate(0, 0, 2).Weekday()), StartTime: "07:00", EndTime: "08:00", Timezone: "Asia/Tokyo",
	}})
	if putRules.Code != http.StatusOK {
		t.Fatalf("PUT initial allocation rules status/body=%d/%s", putRules.Code, putRules.Body.String())
	}
	savedRules := getAllocationRules(t, api, userID)
	if len(savedRules.Items) != 1 {
		t.Fatalf("saved Rules=%+v", savedRules)
	}

	start := time.Now().In(location).AddDate(0, 0, 2)
	start = time.Date(start.Year(), start.Month(), start.Day(), 9, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST recurring allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var concrete rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &concrete); err != nil {
		t.Fatal(err)
	}
	runAllocationMaterialization(t, api, userID)
	concreteBefore := readAllocationFrame(t, api, concrete.RuleID, start)
	firstDate, err := time.Parse("2006-01-02", concrete.OccurrenceDate)
	if err != nil {
		t.Fatal(err)
	}
	exceptionDate := firstDate.AddDate(0, 0, 7)
	week := getAllocationWeek(t, api, userID, exceptionDate)
	var exceptionOccurrence rest.AllocationOccurrenceResponse
	for _, item := range week.Allocations {
		if item.RuleID == concrete.RuleID && item.OccurrenceDate == exceptionDate.Format("2006-01-02") {
			exceptionOccurrence = item
		}
	}
	if exceptionOccurrence.RuleID == "" {
		t.Fatalf("next weekly occurrence missing before exception: %+v", week.Allocations)
	}
	exceptionStart := time.Date(exceptionDate.Year(), exceptionDate.Month(), exceptionDate.Day(), 13, 0, 0, 0, location)
	exceptionResponse := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", concrete.RuleID, exceptionDate.Format("2006-01-02")),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"current"}`, projectID, exceptionStart.Format(time.RFC3339), exceptionStart.Add(time.Hour).Format(time.RFC3339)),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, exceptionOccurrence.Revision)})
	if exceptionResponse.Code != http.StatusOK {
		t.Fatalf("PATCH future allocation exception status/body=%d/%s", exceptionResponse.Code, exceptionResponse.Body.String())
	}
	exceptionBefore := readAllocationException(t, api, concrete.RuleID, exceptionDate)
	savedRules = getAllocationRules(t, api, userID)

	plans := getAllocationPlans(t, api, userID)
	putPlans := putAllocationPlans(t, api, userID, plans.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: 360}})
	if putPlans.Code != http.StatusOK {
		t.Fatalf("PUT allocation plans status/body=%d/%s", putPlans.Code, putPlans.Body.String())
	}
	gotRules := getAllocationRules(t, api, userID)
	if gotRules.Revision != savedRules.Revision || len(gotRules.Items) != len(savedRules.Items) {
		t.Fatalf("Rules changed after Plans PUT: before=%+v after=%+v", savedRules, gotRules)
	}
	for i := range savedRules.Items {
		if gotRules.Items[i] != savedRules.Items[i] {
			t.Fatalf("Rule changed after Plans PUT: before=%+v after=%+v", savedRules.Items[i], gotRules.Items[i])
		}
	}
	if got := readAllocationFrame(t, api, concrete.RuleID, start); got != concreteBefore {
		t.Fatalf("concrete allocation changed after Plans PUT: before=%+v after=%+v", concreteBefore, got)
	}
	if got := readAllocationException(t, api, concrete.RuleID, exceptionDate); got != exceptionBefore {
		t.Fatalf("saved allocation exception changed after Plans PUT: before=%+v after=%+v", exceptionBefore, got)
	}
	updatedWeek := getAllocationWeek(t, api, userID, exceptionDate)
	if updatedWeek.TargetMinutes != 360 {
		t.Fatalf("weekly target total=%d; want 360", updatedWeek.TargetMinutes)
	}
}

func TestAllocationHTTPCollectionsOnlyInvalidateTheirOwnRevision(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Independent revisions")
	rulesBefore := getAllocationRules(t, api, userID)
	plansBefore := getAllocationPlans(t, api, userID)

	if response := putAllocationPlans(t, api, userID, plansBefore.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: 300}}); response.Code != http.StatusOK {
		t.Fatalf("PUT plans status/body=%d/%s", response.Code, response.Body.String())
	}
	if response := putAllocationRules(t, api, userID, rulesBefore.Revision, []allocationRuleWriteItem{{ProjectID: projectID, Weekday: 1, StartTime: "09:00", EndTime: "10:00", Timezone: "Asia/Tokyo"}}); response.Code != http.StatusOK {
		t.Fatalf("Rules became stale after Plans PUT: status/body=%d/%s", response.Code, response.Body.String())
	}

	rulesAfter := getAllocationRules(t, api, userID)
	plansAfter := getAllocationPlans(t, api, userID)
	if response := putAllocationRules(t, api, userID, rulesBefore.Revision, []allocationRuleWriteItem{}); response.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale PUT rules status/body=%d/%s; want 412", response.Code, response.Body.String())
	}
	if response := putAllocationPlans(t, api, userID, plansBefore.Revision, []allocationPlanWriteItem{}); response.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale PUT plans status/body=%d/%s; want 412", response.Code, response.Body.String())
	}
	if rulesAfter.Revision == rulesBefore.Revision || plansAfter.Revision == plansBefore.Revision {
		t.Fatalf("successful resource PUT did not update own token: rules %q/%q plans %q/%q", rulesBefore.Revision, rulesAfter.Revision, plansBefore.Revision, plansAfter.Revision)
	}

	// Calendar edits invalidate Rules but must leave the separate Plans token usable.
	location := mustLocation(t, "Asia/Tokyo")
	start := time.Now().In(location).AddDate(0, 0, 2)
	start = time.Date(start.Year(), start.Month(), start.Day(), 10, 0, 0, 0, location)
	created := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":true}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if created.Code != http.StatusCreated {
		t.Fatalf("POST weekly allocation status/body=%d/%s", created.Code, created.Body.String())
	}
	var occurrence rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(created.Body.Bytes(), &occurrence); err != nil {
		t.Fatal(err)
	}
	preEditRules := getAllocationRules(t, api, userID)
	preEditPlans := getAllocationPlans(t, api, userID)
	edit := api.request(t, userID, http.MethodPatch, fmt.Sprintf("/api/allocations/%s/%s", occurrence.RuleID, occurrence.OccurrenceDate),
		fmt.Sprintf(`{"project_id":%q,"start_at":%q,"end_at":%q,"scope":"following","rule_revision":%d}`,
			projectID, start.Format(time.RFC3339), start.Add(2*time.Hour).Format(time.RFC3339), occurrence.RuleRevision),
		[2]string{"If-Match", fmt.Sprintf(`"%d"`, occurrence.Revision)})
	if edit.Code != http.StatusOK {
		t.Fatalf("PATCH following Rule status/body=%d/%s", edit.Code, edit.Body.String())
	}
	if response := putAllocationRules(t, api, userID, preEditRules.Revision, ruleItemsToWrite(preEditRules.Items)); response.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale Rules PUT after following edit status/body=%d/%s; want 412", response.Code, response.Body.String())
	}
	if response := putAllocationPlans(t, api, userID, preEditPlans.Revision, planItemsToWrite(preEditPlans.Items)); response.Code != http.StatusOK {
		t.Fatalf("Plans token became stale after Rules edit: status/body=%d/%s", response.Code, response.Body.String())
	}
}

func TestAllocationHTTPEmptyReplacementsClearOnlyOwnCollectionsAndMatchRequired(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Clear collections")
	hiddenProjectID := createAllocationProject(t, api, userID, "Hidden allocation rule")
	rules := getAllocationRules(t, api, userID)
	plans := getAllocationPlans(t, api, userID)
	if response := putAllocationRules(t, api, userID, rules.Revision, []allocationRuleWriteItem{
		{ProjectID: projectID, Weekday: 1, StartTime: "09:00", EndTime: "10:00", Timezone: "Asia/Tokyo"},
		{ProjectID: hiddenProjectID, Weekday: 2, StartTime: "11:00", EndTime: "12:00", Timezone: "Asia/Tokyo"},
	}); response.Code != http.StatusOK {
		t.Fatalf("seed Rules status/body=%d/%s", response.Code, response.Body.String())
	}
	if response := putAllocationPlans(t, api, userID, plans.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: 300}}); response.Code != http.StatusOK {
		t.Fatalf("seed Plans status/body=%d/%s", response.Code, response.Body.String())
	}
	rules = getAllocationRules(t, api, userID)
	plans = getAllocationPlans(t, api, userID)
	var hiddenRuleID string
	for _, item := range rules.Items {
		if item.ProjectID == hiddenProjectID {
			hiddenRuleID = item.ID
		}
	}
	if hiddenRuleID == "" {
		t.Fatalf("hidden-row fixture rule missing: %+v", rules.Items)
	}
	if _, err := api.pool.Exec(t.Context(), `UPDATE project_time_allocation_rules SET deleted_at=now() WHERE id=$1`, hiddenRuleID); err != nil {
		t.Fatalf("hide fixture allocation rule: %v", err)
	}
	rules = getAllocationRules(t, api, userID)
	if response := api.request(t, userID, http.MethodPut, "/api/allocation-rules", `{"items":[]}`); response.Code != http.StatusPreconditionRequired {
		t.Fatalf("PUT rules without If-Match status/body=%d/%s; want 428", response.Code, response.Body.String())
	}
	if response := api.request(t, userID, http.MethodPut, "/api/allocation-plans", `{"items":[]}`, [2]string{"If-Match", `"a","b"`}); response.Code != http.StatusBadRequest {
		t.Fatalf("PUT plans with malformed If-Match status/body=%d/%s; want 400", response.Code, response.Body.String())
	}
	if response := putAllocationPlans(t, api, userID, plans.Revision, []allocationPlanWriteItem{}); response.Code != http.StatusOK {
		t.Fatalf("clear Plans status/body=%d/%s", response.Code, response.Body.String())
	}
	if got := getAllocationRules(t, api, userID); got.Revision != rules.Revision || len(got.Items) != 1 || got.Items[0].ID != rules.Items[0].ID {
		t.Fatalf("clearing Plans changed Rules: before=%+v after=%+v", rules, got)
	}
	var hiddenRows int
	if err := api.pool.QueryRow(t.Context(), `SELECT count(*) FROM project_time_allocation_rules WHERE id=$1 AND deleted_at IS NOT NULL`, hiddenRuleID).Scan(&hiddenRows); err != nil {
		t.Fatalf("read hidden allocation rule: %v", err)
	}
	if hiddenRows != 1 {
		t.Fatalf("Rules replacement removed hidden row; matching deleted rows=%d", hiddenRows)
	}
	plans = getAllocationPlans(t, api, userID)
	if response := putAllocationRules(t, api, userID, rules.Revision, []allocationRuleWriteItem{}); response.Code != http.StatusOK {
		t.Fatalf("clear Rules status/body=%d/%s", response.Code, response.Body.String())
	}
	if got := getAllocationPlans(t, api, userID); got.Revision != plans.Revision || len(got.Items) != 0 {
		t.Fatalf("clearing Rules changed Plans: before=%+v after=%+v", plans, got)
	}
}

func TestAllocationHTTPCollectionsStayPrivateForSharedProjects(t *testing.T) {
	api := newAllocationHTTP(t)
	ownerID := api.seedUser(t, "Asia/Tokyo")
	memberID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, ownerID, "Shared collection project")
	shared := api.request(t, ownerID, http.MethodPut, fmt.Sprintf("/api/projects/%s/members/%s", projectID, memberID), `{"role_id":"viewer"}`)
	if shared.Code != http.StatusNoContent {
		t.Fatalf("PUT project member status/body=%d/%s", shared.Code, shared.Body.String())
	}

	ownerRules := getAllocationRules(t, api, ownerID)
	if response := putAllocationRules(t, api, ownerID, ownerRules.Revision, []allocationRuleWriteItem{{
		ProjectID: projectID, Weekday: 1, StartTime: "09:00", EndTime: "10:00", Timezone: "Asia/Tokyo",
	}}); response.Code != http.StatusOK {
		t.Fatalf("owner PUT Rules status/body=%d/%s", response.Code, response.Body.String())
	}
	ownerPlans := getAllocationPlans(t, api, ownerID)
	if response := putAllocationPlans(t, api, ownerID, ownerPlans.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: 240}}); response.Code != http.StatusOK {
		t.Fatalf("owner PUT Plans status/body=%d/%s", response.Code, response.Body.String())
	}
	if got := getAllocationRules(t, api, memberID); len(got.Items) != 0 {
		t.Fatalf("shared member saw owner's Rules: %+v", got.Items)
	}
	if got := getAllocationPlans(t, api, memberID); len(got.Items) != 0 {
		t.Fatalf("shared member saw owner's Plans: %+v", got.Items)
	}

	memberRules := getAllocationRules(t, api, memberID)
	if response := putAllocationRules(t, api, memberID, memberRules.Revision, []allocationRuleWriteItem{{
		ProjectID: projectID, Weekday: 2, StartTime: "11:00", EndTime: "12:00", Timezone: "Asia/Tokyo",
	}}); response.Code != http.StatusOK {
		t.Fatalf("shared member PUT own Rules status/body=%d/%s", response.Code, response.Body.String())
	}
	memberPlans := getAllocationPlans(t, api, memberID)
	if response := putAllocationPlans(t, api, memberID, memberPlans.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: 120}}); response.Code != http.StatusOK {
		t.Fatalf("shared member PUT own Plans status/body=%d/%s", response.Code, response.Body.String())
	}
	if got := getAllocationRules(t, api, ownerID); len(got.Items) != 1 || got.Items[0].Weekday != 1 {
		t.Fatalf("member Rules changed owner's collection: %+v", got.Items)
	}
	if got := getAllocationPlans(t, api, ownerID); len(got.Items) != 1 || got.Items[0].TargetMinutes != 240 {
		t.Fatalf("member Plans changed owner's collection: %+v", got.Items)
	}
}

func TestAllocationHTTPRulesConflictRollsBackRulesButKeepsSavedPlans(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Independent rollback")
	plans := getAllocationPlans(t, api, userID)
	if response := putAllocationPlans(t, api, userID, plans.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: 180}}); response.Code != http.StatusOK {
		t.Fatalf("save Plans before Rules conflict status/body=%d/%s", response.Code, response.Body.String())
	}
	plansBeforeConflict := getAllocationPlans(t, api, userID)
	rulesBeforeConflict := getAllocationRules(t, api, userID)

	location := mustLocation(t, "Asia/Tokyo")
	date := time.Now().In(location).AddDate(0, 0, 2)
	start := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, location)
	oneOff := api.request(t, userID, http.MethodPost, "/api/allocations", fmt.Sprintf(
		`{"project_id":%q,"timezone":"Asia/Tokyo","start_at":%q,"end_at":%q,"repeat_weekly":false}`,
		projectID, start.Format(time.RFC3339), start.Add(time.Hour).Format(time.RFC3339)))
	if oneOff.Code != http.StatusCreated {
		t.Fatalf("POST one-off allocation status/body=%d/%s", oneOff.Code, oneOff.Body.String())
	}
	var occurrence rest.AllocationOccurrenceResponse
	if err := json.Unmarshal(oneOff.Body.Bytes(), &occurrence); err != nil {
		t.Fatal(err)
	}
	frameBefore := readAllocationFrame(t, api, occurrence.RuleID, start)
	conflict := putAllocationRules(t, api, userID, rulesBeforeConflict.Revision, []allocationRuleWriteItem{{
		ProjectID: projectID, Weekday: int(start.Weekday()), StartTime: "09:30", EndTime: "10:30", Timezone: "Asia/Tokyo",
	}})
	if conflict.Code != http.StatusConflict {
		t.Fatalf("PUT overlapping Rules status/body=%d/%s; want 409", conflict.Code, conflict.Body.String())
	}
	if got := getAllocationRules(t, api, userID); got.Revision != rulesBeforeConflict.Revision || len(got.Items) != 0 {
		t.Fatalf("Rules state/token changed after conflict: before=%+v after=%+v", rulesBeforeConflict, got)
	}
	if got := getAllocationPlans(t, api, userID); got.Revision != plansBeforeConflict.Revision || len(got.Items) != 1 || got.Items[0].TargetMinutes != 180 {
		t.Fatalf("previously saved Plans lost after Rules conflict: before=%+v after=%+v", plansBeforeConflict, got)
	}
	if got := readAllocationFrame(t, api, occurrence.RuleID, start); got != frameBefore {
		t.Fatalf("one-off occurrence changed after Rules rollback: before=%+v after=%+v", frameBefore, got)
	}
}

func TestAllocationHTTPPlansValidationFailureLeavesOnlyPlansWriteRolledBack(t *testing.T) {
	api := newAllocationHTTP(t)
	userID := api.seedUser(t, "Asia/Tokyo")
	projectID := createAllocationProject(t, api, userID, "Plans validation rollback")
	rules := getAllocationRules(t, api, userID)
	if response := putAllocationRules(t, api, userID, rules.Revision, []allocationRuleWriteItem{{
		ProjectID: projectID, Weekday: 1, StartTime: "09:00", EndTime: "10:00", Timezone: "Asia/Tokyo",
	}}); response.Code != http.StatusOK {
		t.Fatalf("seed Rules status/body=%d/%s", response.Code, response.Body.String())
	}
	plans := getAllocationPlans(t, api, userID)
	if response := putAllocationPlans(t, api, userID, plans.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: 240}}); response.Code != http.StatusOK {
		t.Fatalf("seed Plans status/body=%d/%s", response.Code, response.Body.String())
	}
	rulesBefore := getAllocationRules(t, api, userID)
	plansBefore := getAllocationPlans(t, api, userID)
	invalid := putAllocationPlans(t, api, userID, plansBefore.Revision, []allocationPlanWriteItem{{ProjectID: projectID, TargetMinutes: -1}})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("PUT negative target status/body=%d/%s; want 400", invalid.Code, invalid.Body.String())
	}
	if got := getAllocationPlans(t, api, userID); got.Revision != plansBefore.Revision || len(got.Items) != 1 || got.Items[0].TargetMinutes != 240 {
		t.Fatalf("invalid Plans replacement changed collection: before=%+v after=%+v", plansBefore, got)
	}
	if got := getAllocationRules(t, api, userID); got.Revision != rulesBefore.Revision || len(got.Items) != len(rulesBefore.Items) || got.Items[0] != rulesBefore.Items[0] {
		t.Fatalf("invalid Plans replacement changed Rules: before=%+v after=%+v", rulesBefore, got)
	}
}

type allocationFrameSnapshot struct {
	ID               string
	SourceRevisionID string
	ProjectID        string
	StartAt          time.Time
	EndAt            time.Time
	Timezone         string
	Revision         int32
}

type allocationExceptionSnapshot struct {
	SourceRevisionID string
	ProjectID        string
	StartAt          time.Time
	EndAt            time.Time
	Timezone         string
}

func readAllocationFrame(t *testing.T, api *allocationHTTP, ruleID string, date time.Time) allocationFrameSnapshot {
	t.Helper()
	var snapshot allocationFrameSnapshot
	err := api.pool.QueryRow(t.Context(), `SELECT id, source_revision_id, project_id, start_at, end_at, timezone, revision
		FROM project_time_allocations WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NULL`, ruleID, date.Format("2006-01-02")).Scan(
		&snapshot.ID, &snapshot.SourceRevisionID, &snapshot.ProjectID, &snapshot.StartAt, &snapshot.EndAt, &snapshot.Timezone, &snapshot.Revision)
	if err != nil {
		t.Fatalf("read concrete allocation snapshot: %v", err)
	}
	return snapshot
}

func readAllocationException(t *testing.T, api *allocationHTTP, ruleID string, date time.Time) allocationExceptionSnapshot {
	t.Helper()
	var snapshot allocationExceptionSnapshot
	err := api.pool.QueryRow(t.Context(), `SELECT source_revision_id, project_id, start_at, end_at, timezone
		FROM project_time_allocation_exceptions WHERE rule_id=$1 AND occurrence_date=$2 AND deleted_at IS NULL`, ruleID, date.Format("2006-01-02")).Scan(
		&snapshot.SourceRevisionID, &snapshot.ProjectID, &snapshot.StartAt, &snapshot.EndAt, &snapshot.Timezone)
	if err != nil {
		t.Fatalf("read allocation exception snapshot: %v", err)
	}
	return snapshot
}

func getAllocationRules(t *testing.T, api *allocationHTTP, userID string) allocationRuleCollection {
	t.Helper()
	response := api.request(t, userID, http.MethodGet, "/api/allocation-rules", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET allocation rules status/body=%d/%s", response.Code, response.Body.String())
	}
	var collection allocationRuleCollection
	if err := json.Unmarshal(response.Body.Bytes(), &collection); err != nil {
		t.Fatalf("decode allocation rules: %v", err)
	}
	return collection
}

func getAllocationPlans(t *testing.T, api *allocationHTTP, userID string) allocationPlans {
	t.Helper()
	response := api.request(t, userID, http.MethodGet, "/api/allocation-plans", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET allocation plans status/body=%d/%s", response.Code, response.Body.String())
	}
	var collection allocationPlans
	if err := json.Unmarshal(response.Body.Bytes(), &collection); err != nil {
		t.Fatalf("decode allocation plans: %v", err)
	}
	return collection
}

func putAllocationRules(t *testing.T, api *allocationHTTP, userID, revision string, items []allocationRuleWriteItem) *httptest.ResponseRecorder {
	t.Helper()
	return api.request(t, userID, http.MethodPut, "/api/allocation-rules", allocationJSON(t, struct {
		Items []allocationRuleWriteItem `json:"items"`
	}{Items: items}), allocationIfMatch(revision))
}

func putAllocationPlans(t *testing.T, api *allocationHTTP, userID, revision string, items []allocationPlanWriteItem) *httptest.ResponseRecorder {
	t.Helper()
	return api.request(t, userID, http.MethodPut, "/api/allocation-plans", allocationJSON(t, struct {
		Items []allocationPlanWriteItem `json:"items"`
	}{Items: items}), allocationIfMatch(revision))
}

func ruleItemsToWrite(items []allocationRuleItem) []allocationRuleWriteItem {
	result := make([]allocationRuleWriteItem, 0, len(items))
	for _, item := range items {
		result = append(result, allocationRuleWriteItem{ID: item.ID, Revision: item.Revision, ProjectID: item.ProjectID, Weekday: item.Weekday,
			StartTime: item.StartTime, EndTime: item.EndTime, EndDayOffset: item.EndDayOffset, Timezone: item.Timezone})
	}
	return result
}

func planItemsToWrite(items []allocationPlanItem) []allocationPlanWriteItem {
	result := make([]allocationPlanWriteItem, 0, len(items))
	for _, item := range items {
		result = append(result, allocationPlanWriteItem{ProjectID: item.ProjectID, TargetMinutes: item.TargetMinutes})
	}
	return result
}

func getAllocationWeek(t *testing.T, api *allocationHTTP, userID string, date time.Time) rest.AllocationWeekResponse {
	t.Helper()
	weekStart := mondayUTC(date).Format("2006-01-02")
	response := api.request(t, userID, http.MethodGet, "/api/allocation-weeks/"+weekStart, "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET allocation week status/body=%d/%s", response.Code, response.Body.String())
	}
	var week rest.AllocationWeekResponse
	if err := json.Unmarshal(response.Body.Bytes(), &week); err != nil {
		t.Fatalf("decode allocation week: %v", err)
	}
	return week
}

func allocationIfMatch(revision string) [2]string { return [2]string{"If-Match", `"` + revision + `"`} }

func allocationJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode allocation contract request: %v", err)
	}
	return string(data)
}

func containsJSONKey(t *testing.T, data []byte, target string) bool {
	t.Helper()
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode JSON for key inspection: %v", err)
	}
	return containsKey(value, target)
}

func containsKey(value any, target string) bool {
	switch current := value.(type) {
	case map[string]any:
		if _, ok := current[target]; ok {
			return true
		}
		for _, child := range current {
			if containsKey(child, target) {
				return true
			}
		}
	case []any:
		for _, child := range current {
			if containsKey(child, target) {
				return true
			}
		}
	}
	return false
}
