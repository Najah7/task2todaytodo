package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
)

type weekRepo struct {
	Repository
	profile       dao.Profile
	projects      []dao.Project
	allocations   []dao.Allocation
	exceptions    []dao.ActiveTimeException
	plans         []dao.PlanItem
	configuredSet bool
}

type projectionWeekRepo struct {
	weekRepo
	rules   []dao.Rule
	origins map[string]bool
	pauses  map[string][]dao.PauseInterval
}

func (r projectionWeekRepo) ListRuleRevisionsForProjection(context.Context, string) ([]dao.Rule, error) {
	return r.rules, nil
}

func (r projectionWeekRepo) ListRules(context.Context, string) ([]dao.Rule, error) {
	return r.rules, nil
}

func (r projectionWeekRepo) ListExceptionOrigins(context.Context, string, time.Time, time.Time) (map[string]bool, error) {
	return r.origins, nil
}

func (r projectionWeekRepo) ListPauseIntervals(_ context.Context, ruleID string) ([]dao.PauseInterval, error) {
	return r.pauses[ruleID], nil
}

func (r weekRepo) GetProfile(context.Context, string) (dao.Profile, error) {
	return r.profile, nil
}

func (r weekRepo) ListPlans(context.Context, string) ([]dao.PlanItem, error) {
	plans := r.plans
	if !r.configuredSet {
		plans = make([]dao.PlanItem, 0, len(r.projects))
		for _, project := range r.projects {
			plans = append(plans, dao.PlanItem{ProjectID: project.ID})
		}
	}
	return plans, nil
}

func (r weekRepo) ListProjects(context.Context, string) ([]dao.Project, error) {
	return r.projects, nil
}

func (r weekRepo) ListAllocations(_ context.Context, _ string, from, through time.Time) ([]dao.Allocation, error) {
	rows := make([]dao.Allocation, 0, len(r.allocations))
	for _, allocation := range r.allocations {
		if allocation.EndAt.After(from) && allocation.StartAt.Before(through) {
			rows = append(rows, allocation)
		}
	}
	return rows, nil
}

func (r weekRepo) ListRules(context.Context, string) ([]dao.Rule, error) {
	return nil, nil
}

func (r weekRepo) ListRuleRevisionsForProjection(context.Context, string) ([]dao.Rule, error) {
	return nil, nil
}

func (r weekRepo) ListActiveTimeExceptions(context.Context, string, time.Time, time.Time) ([]dao.ActiveTimeException, error) {
	return r.exceptions, nil
}

func (weekRepo) ListExceptionOrigins(context.Context, string, time.Time, time.Time) (map[string]bool, error) {
	return map[string]bool{}, nil
}

func (weekRepo) ListPauseIntervals(context.Context, string) ([]dao.PauseInterval, error) {
	return nil, nil
}

func TestGetWeekCountsSpringDSTDayAndWakingActiveTimeAcrossMonday(t *testing.T) {
	location := mustWeekLocation(t, "America/New_York")
	weekStart := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)
	week, err := newPaidGetWeekUseCase(weekRepo{profile: dao.Profile{
		Timezone: location.String(), ActiveStartTime: "07:00", ActiveEndTime: "23:00",
	}}).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}

	sunday := findWeekDay(t, week.Days, "2026-03-08")
	if got := localDayDuration(location, 2026, time.March, 8); got != 23*time.Hour {
		t.Errorf("spring-forward Sunday length = %s, want 23h", got)
	}
	if got := sunday.FreeMinutes; got != 16*60 {
		t.Errorf("spring-forward Sunday free minutes = %d, want 960", got)
	}
	if got := intervalHours(week.ActiveIntervals, "2026-03-08"); got != 16 {
		t.Errorf("spring-forward Sunday active interval = %d hours, want 16 hours", got)
	}
	if got := intervalHours(week.ActiveIntervals, "2026-03-02"); got != 16 {
		t.Errorf("first Monday active interval = %d hours, want 16 hours", got)
	}
	for _, interval := range week.ActiveIntervals {
		if interval.LocalDate.Format("2006-01-02") == "2026-03-02" && interval.StartAt.In(location).Format("2006-01-02 15:04") != "2026-03-02 07:00" {
			t.Errorf("first Monday active interval starts at %s, want Monday 07:00", interval.StartAt.In(location))
		}
	}
	if got := intervalHours(week.ActiveIntervals, "2026-03-09"); got != 0 {
		t.Errorf("final Monday active interval = %d hours, want none beyond the exclusive week boundary", got)
	}
}

func TestGetWeekCountsFallDSTDayAtTwentyFiveHours(t *testing.T) {
	location := mustWeekLocation(t, "America/New_York")
	weekStart := time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC)
	week, err := newPaidGetWeekUseCase(weekRepo{profile: dao.Profile{
		Timezone: location.String(), ActiveStartTime: "07:00", ActiveEndTime: "23:00",
	}}).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}

	sunday := findWeekDay(t, week.Days, "2026-11-01")
	if got := localDayDuration(location, 2026, time.November, 1); got != 25*time.Hour {
		t.Errorf("fall-back Sunday length = %s, want 25h", got)
	}
	if got := sunday.FreeMinutes; got != 16*60 {
		t.Errorf("fall-back Sunday free minutes = %d, want 960 activeWindow minutes", got)
	}
	if got := intervalHours(week.ActiveIntervals, "2026-11-01"); got != 16 {
		t.Errorf("fall-back Sunday active interval = %d hours, want 16 hours", got)
	}
	if got := intervalHours(week.ActiveIntervals, "2026-10-26"); got != 16 {
		t.Errorf("first Monday active interval = %d hours, want 16 hours", got)
	}
	if got := intervalHours(week.ActiveIntervals, "2026-11-02"); got != 0 {
		t.Errorf("final Monday active interval = %d hours, want none beyond the exclusive week boundary", got)
	}
}

func TestGetWeekIncludesOvernightActiveCarryInAtWeekBoundary(t *testing.T) {
	location := mustWeekLocation(t, "UTC")
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	week, err := newPaidGetWeekUseCase(weekRepo{profile: dao.Profile{
		Timezone: location.String(), ActiveStartTime: "22:00", ActiveEndTime: "02:00",
	}}).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}
	var carryIn time.Duration
	for _, interval := range week.ActiveIntervals {
		if interval.LocalDate.Format("2006-01-02") == "2026-10-11" {
			carryIn += interval.EndAt.Sub(interval.StartAt)
			if got := interval.StartAt.Format("2006-01-02 15:04"); got != "2026-10-12 00:00" {
				t.Errorf("Sunday carry-in starts at %s, want Monday 00:00", got)
			}
		}
	}
	if carryIn != 2*time.Hour {
		t.Errorf("Sunday carry-in = %s, want 2h", carryIn)
	}
}

func TestGetWeekUsesRecalculatedWakeWindowAndUnionsPastAllocationOverlap(t *testing.T) {
	location := mustWeekLocation(t, "UTC")
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	start := localWeekTime(location, 2026, time.October, 12, 8, 0)
	allocations := []dao.Allocation{
		{RuleID: "one", ProjectID: "project", StartAt: start, EndAt: localWeekTime(location, 2026, time.October, 12, 12, 0)},
		{RuleID: "two", ProjectID: "project", StartAt: localWeekTime(location, 2026, time.October, 12, 11, 0), EndAt: localWeekTime(location, 2026, time.October, 12, 13, 0)},
	}
	repo := weekRepo{
		profile:     dao.Profile{Timezone: location.String(), ActiveStartTime: "10:00", ActiveEndTime: "23:00"},
		projects:    []dao.Project{{ID: "project", Title: "Plan", TargetMinutes: 300}},
		allocations: allocations,
	}
	week, err := newPaidGetWeekUseCase(repo).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}

	monday := findWeekDay(t, week.Days, "2026-10-12")
	if got := monday.FreeMinutes; got != 10*60 {
		t.Errorf("Monday free minutes = %d, want 600 after subtracting the union 10:00–13:00 from the recalculated 10:00–23:00 activeWindow window", got)
	}
	if got := week.AllocatedMinutes; got != 6*60 {
		t.Errorf("allocated minutes = %d, want both elapsed allocation rows totaling 360", got)
	}
	if got := week.DifferenceMinutes; got != 60 {
		t.Errorf("difference = %d, want allocated minus target = +60", got)
	}
	if got := week.Projects[0].DifferenceMinutes; got != 60 {
		t.Errorf("project difference = %d, want allocated minus target = +60", got)
	}
}

func TestGetWeekProjectsEffectiveRuleRevisionsOnceAndPauseExceptions(t *testing.T) {
	location := mustWeekLocation(t, "UTC")
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, location)
	pausedAt := time.Date(2026, time.October, 14, 12, 0, 0, 0, location)
	resumedAt := pausedAt.Add(2 * time.Hour)
	movedStart := time.Date(2026, time.October, 14, 16, 0, 0, 0, location)
	rules := []dao.Rule{
		{ID: "weekly", ProjectID: "project", Revision: 1, CurrentRevision: 2, SourceRevisionID: "weekly-r1", Weekday: int(time.Monday), StartTime: "09:00", EndTime: "10:00", Timezone: "UTC", Frequency: "weekly", EffectiveFrom: weekStart.AddDate(0, 0, -7), EffectiveUntil: ptrWeekDate(weekStart.AddDate(0, 0, 1)), EffectiveAt: weekStart.AddDate(0, 0, -14), PausedReason: "allocation_conflict", PausedAt: &pausedAt},
		{ID: "weekly", ProjectID: "project", Revision: 2, CurrentRevision: 2, SourceRevisionID: "weekly-r2", Weekday: int(time.Wednesday), StartTime: "13:00", EndTime: "14:00", Timezone: "UTC", Frequency: "weekly", EffectiveFrom: weekStart.AddDate(0, 0, 2), EffectiveAt: weekStart.AddDate(0, 0, 1), PausedReason: "allocation_conflict", PausedAt: &pausedAt},
		{ID: "once", ProjectID: "project", Revision: 1, SourceRevisionID: "once-r1", Weekday: int(time.Thursday), StartTime: "11:00", EndTime: "12:00", Timezone: "UTC", Frequency: "once", EffectiveFrom: weekStart.AddDate(0, 0, 3), EffectiveAt: weekStart.AddDate(0, 0, -1)},
	}
	repo := projectionWeekRepo{
		weekRepo: weekRepo{
			profile:     dao.Profile{Timezone: "UTC", ActiveStartTime: "07:00", ActiveEndTime: "23:00"},
			projects:    []dao.Project{{ID: "project", Title: "Plan"}},
			allocations: []dao.Allocation{{RuleID: "weekly", OccurrenceDate: weekStart.AddDate(0, 0, 2), SourceRevisionID: "weekly-r2", ProjectID: "project", ProjectTitle: "Plan", StartAt: movedStart, EndAt: movedStart.Add(time.Hour), IsException: true}},
		},
		rules:   rules,
		origins: map[string]bool{"weekly|2026-10-14": true},
		pauses:  map[string][]dao.PauseInterval{"weekly": {{PausedAt: pausedAt, ResumedAt: &resumedAt}}},
	}
	week, err := newPaidGetWeekUseCase(repo).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}
	if len(week.Allocations) != 3 {
		t.Fatalf("projected allocations = %d (%+v); want Monday prior revision, moved saved exception, and one-off", len(week.Allocations), week.Allocations)
	}
	starts := make(map[string]time.Time, len(week.Allocations))
	for _, allocation := range week.Allocations {
		starts[allocation.RuleID+"|"+allocation.OccurrenceDate.Format("2006-01-02")] = allocation.StartAt
	}
	if got := starts["weekly|2026-10-12"]; !got.Equal(time.Date(2026, time.October, 12, 9, 0, 0, 0, location)) {
		t.Errorf("earlier revision occurrence start = %s, want Monday 09:00", got)
	}
	for _, allocation := range week.Allocations {
		if allocation.RuleID == "weekly" && allocation.OccurrenceDate.Format("2006-01-02") == "2026-10-12" && allocation.RuleRevision != 2 {
			t.Errorf("earlier virtual occurrence RuleRevision = %d, want latest rule revision 2", allocation.RuleRevision)
		}
	}
	if got := starts["weekly|2026-10-14"]; !got.Equal(movedStart) {
		t.Errorf("moved exception start = %s, want %s", got, movedStart)
	}
	if got := starts["once|2026-10-15"]; !got.Equal(time.Date(2026, time.October, 15, 11, 0, 0, 0, location)) {
		t.Errorf("one-off occurrence start = %s, want Thursday 11:00", got)
	}
	if len(week.PausedRules) != 1 || week.PausedRules[0].RuleID != "weekly" || week.PausedRules[0].ProjectID != "project" {
		t.Errorf("paused rule projection = %+v; want weekly rule associated with its Project", week.PausedRules)
	}
}

func TestGetWeekKeepsStartedSlotAtSameDayRevisionCutover(t *testing.T) {
	location := mustWeekLocation(t, "UTC")
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, location)
	cutover := time.Date(2026, time.October, 14, 10, 0, 0, 0, location)
	repo := projectionWeekRepo{
		weekRepo: weekRepo{
			profile:  dao.Profile{Timezone: "UTC", ActiveStartTime: "07:00", ActiveEndTime: "23:00"},
			projects: []dao.Project{{ID: "project", Title: "Plan"}},
		},
		rules: []dao.Rule{
			{ID: "rule", ProjectID: "project", Revision: 1, SourceRevisionID: "r1", Weekday: int(time.Wednesday), StartTime: "08:00", EndTime: "09:00", Timezone: "UTC", Frequency: "weekly", EffectiveFrom: weekStart, EffectiveUntil: ptrWeekDate(cutover), EffectiveUntilAt: &cutover, EffectiveAt: weekStart},
			{ID: "rule", ProjectID: "project", Revision: 2, SourceRevisionID: "r2", Weekday: int(time.Wednesday), StartTime: "18:00", EndTime: "19:00", Timezone: "UTC", Frequency: "weekly", EffectiveFrom: cutover, EffectiveAt: cutover},
		},
	}
	week, err := newPaidGetWeekUseCase(repo).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}
	var onCutover []dao.Allocation
	for _, allocation := range week.Allocations {
		if allocation.RuleID == "rule" && allocation.OccurrenceDate.Format("2006-01-02") == cutover.Format("2006-01-02") {
			onCutover = append(onCutover, allocation)
		}
	}
	if len(onCutover) != 1 || onCutover[0].StartAt.Hour() != 8 {
		t.Fatalf("same-day cutover occurrences = %+v; want only the original 08:00 slot started before cutover", onCutover)
	}
}

func TestGetWeekShowsConfiguredAndReferencedProjectsOnly(t *testing.T) {
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	start := weekStart.Add(10 * time.Hour)
	repo := weekRepo{
		profile: dao.Profile{Timezone: "UTC", ActiveStartTime: "07:00", ActiveEndTime: "23:00"},
		projects: []dao.Project{
			{ID: "configured", Title: "Configured"},
			{ID: "historical", Title: "Historical"},
			{ID: "empty", Title: "Empty readable project"},
		},
		plans: []dao.PlanItem{{ProjectID: "configured"}}, configuredSet: true,
		allocations: []dao.Allocation{{ProjectID: "historical", StartAt: start, EndAt: start.Add(time.Hour)}},
	}
	week, err := newPaidGetWeekUseCase(repo).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}
	got := make(map[string]bool, len(week.Projects))
	for _, project := range week.Projects {
		got[project.ID] = true
	}
	if len(got) != 2 || !got["configured"] || !got["historical"] || got["empty"] {
		t.Fatalf("weekly projects = %+v; want configured and allocation-referenced only", week.Projects)
	}
}

func TestGetWeekShowsProjectForPreservedVirtualFrameAfterplanRemoval(t *testing.T) {
	location := mustWeekLocation(t, "UTC")
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, location)
	deletedAt := time.Date(2026, time.October, 16, 12, 0, 0, 0, location)
	repo := projectionWeekRepo{
		weekRepo: weekRepo{
			profile:  dao.Profile{Timezone: "UTC", ActiveStartTime: "07:00", ActiveEndTime: "23:00"},
			projects: []dao.Project{{ID: "removed-target", Title: "Preserved history"}},
			plans:    []dao.PlanItem{}, configuredSet: true,
		},
		rules: []dao.Rule{{
			ID: "stopped", ProjectID: "removed-target", Revision: 1, CurrentRevision: 1,
			SourceRevisionID: "source", Weekday: int(time.Monday), StartTime: "09:00", EndTime: "10:00",
			Timezone: "UTC", Frequency: "weekly", EffectiveFrom: weekStart.AddDate(0, 0, -7),
			EffectiveAt: weekStart.AddDate(0, 0, -14), DeletedAt: &deletedAt,
		}},
	}
	week, err := newPaidGetWeekUseCase(repo).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}
	if len(week.Allocations) != 1 || week.Allocations[0].ProjectID != "removed-target" {
		t.Fatalf("preserved virtual allocations = %+v; want Monday frame for removed target", week.Allocations)
	}
	if len(week.Projects) != 1 || week.Projects[0].ID != "removed-target" || week.Allocations[0].ProjectTitle != "Preserved history" {
		t.Fatalf("preserved virtual project/title = %+v / %q; want readable project included while its frame is displayed", week.Projects, week.Allocations[0].ProjectTitle)
	}
}

func TestGetWeekPreservesStartedVirtualOriginsAfterFollowingStop(t *testing.T) {
	location := mustWeekLocation(t, "UTC")
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, location)
	stopAt := time.Date(2026, time.October, 14, 12, 0, 0, 0, location)
	lastOrigin := weekStart.AddDate(0, 0, -7)
	repo := projectionWeekRepo{
		weekRepo: weekRepo{
			profile:  dao.Profile{Timezone: "UTC", ActiveStartTime: "07:00", ActiveEndTime: "23:00"},
			projects: []dao.Project{{ID: "project", Title: "Project"}},
		},
		rules: []dao.Rule{{
			ID: "stopped", ProjectID: "project", Revision: 1, CurrentRevision: 1,
			SourceRevisionID: "source", Weekday: int(time.Tuesday), StartTime: "09:00", EndTime: "10:00",
			Timezone: "UTC", Frequency: "weekly", EffectiveFrom: weekStart.AddDate(0, 0, -14),
			EffectiveAt: weekStart.AddDate(0, 0, -14), StoppedAfter: &lastOrigin, DeletedAt: &stopAt,
		}},
	}
	week, err := newPaidGetWeekUseCase(repo).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() error = %v", err)
	}
	if len(week.Allocations) != 1 || week.Allocations[0].OccurrenceDate.Format("2006-01-02") != "2026-10-13" {
		t.Fatalf("virtual frames after following stop = %+v; want started Tuesday origin before cutoff", week.Allocations)
	}
	if week.Allocations[0].CanUpdateFollowing || week.Allocations[0].CanDeleteFollowing {
		t.Fatalf("stopped virtual frame still permits following mutation: %+v", week.Allocations[0])
	}
}

func TestGetWeekKeepsPreCutoffFramesForStoppedRule(t *testing.T) {
	weekStart := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, time.October, 14, 10, 0, 0, 0, time.UTC)
	rule := dao.Rule{
		ID: "stopped", ProjectID: "project", Revision: 1, SourceRevisionID: "source",
		Weekday: int(time.Wednesday), StartTime: "08:00", EndTime: "09:00", Timezone: "UTC",
		Frequency: "weekly", EffectiveFrom: weekStart, EffectiveAt: weekStart, DeletedAt: &cutoff,
	}
	firstRepo := projectionWeekRepo{
		weekRepo: weekRepo{profile: dao.Profile{Timezone: "UTC", ActiveStartTime: "07:00", ActiveEndTime: "23:00"}, projects: []dao.Project{{ID: "project", Title: "Plan"}}},
		rules:    []dao.Rule{rule},
	}
	first, err := newPaidGetWeekUseCase(firstRepo).Execute(context.Background(), "user", weekStart)
	if err != nil {
		t.Fatalf("GetWeek() before stop cutoff error = %v", err)
	}
	if len(first.Allocations) != 1 || first.Allocations[0].StartAt.Hour() != 8 {
		t.Fatalf("pre-cutoff frame = %+v; want Wednesday 08:00", first.Allocations)
	}
	secondWeek := weekStart.AddDate(0, 0, 7)
	second, err := newPaidGetWeekUseCase(firstRepo).Execute(context.Background(), "user", secondWeek)
	if err != nil {
		t.Fatalf("GetWeek() after stop cutoff error = %v", err)
	}
	if len(second.Allocations) != 0 {
		t.Fatalf("post-cutoff frames = %+v; want none", second.Allocations)
	}
}

func ptrWeekDate(value time.Time) *time.Time { return &value }

func findWeekDay(t *testing.T, days []dao.Day, date string) dao.Day {
	t.Helper()
	for _, day := range days {
		if day.Date.Format("2006-01-02") == date {
			return day
		}
	}
	t.Fatalf("day %s not found in %+v", date, days)
	return dao.Day{}
}

func intervalHours(intervals []dao.Interval, localDate string) int {
	for _, interval := range intervals {
		if interval.LocalDate.Format("2006-01-02") == localDate {
			return int(interval.EndAt.Sub(interval.StartAt).Hours())
		}
	}
	return 0
}

func mustWeekLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	location, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return location
}

func localWeekTime(location *time.Location, year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, location)
}

func localDayDuration(location *time.Location, year int, month time.Month, day int) time.Duration {
	start := time.Date(year, month, day, 0, 0, 0, 0, location)
	return time.Date(year, month, day+1, 0, 0, 0, 0, location).Sub(start)
}
