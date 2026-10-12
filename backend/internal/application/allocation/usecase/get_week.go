package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/allocation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/allocation/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
)

type GetWeekUseCase struct{ repo Repository }

func NewGetWeekUseCase(repo Repository) *GetWeekUseCase { return &GetWeekUseCase{repo: repo} }
func (uc *GetWeekUseCase) Execute(ctx context.Context, userID string, weekStart time.Time) (dao.Week, error) {
	if weekStart.Weekday() != time.Monday {
		return dao.Week{}, ErrInvalidRequest
	}
	return projectWeek(ctx, uc.repo, userID, weekStart)
}

func projectWeek(ctx context.Context, repo Repository, userID string, weekStart time.Time) (dao.Week, error) {
	profile, err := repo.GetProfile(ctx, userID)
	if err != nil {
		return dao.Week{}, err
	}
	location, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return dao.Week{}, ErrInvalidRequest
	}
	date := calendar.DateOnly(weekStart)
	from, ok := calendar.ResolveForward(date, time.Time{}, 0, location)
	if !ok {
		return dao.Week{}, ErrInvalidRequest
	}
	endDate := date.AddDate(0, 0, 7)
	through, ok := calendar.ResolveForward(endDate, time.Time{}, 0, location)
	if !ok || !through.After(from) {
		return dao.Week{}, ErrInvalidRequest
	}
	visibleProjects, err := repo.ListProjects(ctx, userID)
	if err != nil {
		return dao.Week{}, err
	}
	plans, err := repo.ListPlans(ctx, userID)
	if err != nil {
		return dao.Week{}, err
	}
	weeklyRules, err := weeklyRules(ctx, repo, userID)
	if err != nil {
		return dao.Week{}, err
	}
	allocations, err := repo.ListAllocations(ctx, userID, from, through)
	if err != nil {
		return dao.Week{}, err
	}
	visibleByID := make(map[string]dao.Project, len(visibleProjects))
	for _, project := range visibleProjects {
		visibleByID[project.ID] = project
	}
	visibleAllocations := allocations[:0]
	for _, allocation := range allocations {
		if _, visible := visibleByID[allocation.ProjectID]; visible {
			visibleAllocations = append(visibleAllocations, allocation)
		}
	}
	allocations = visibleAllocations
	configured := make(map[string]bool, len(plans)+len(weeklyRules))
	for _, plan := range plans {
		configured[plan.ProjectID] = true
	}
	for _, rule := range weeklyRules {
		configured[rule.ProjectID] = true
	}
	projectIDs := make(map[string]bool, len(configured)+len(allocations))
	for projectID := range configured {
		if _, visible := visibleByID[projectID]; visible {
			projectIDs[projectID] = true
		}
	}
	for _, allocation := range allocations {
		if _, visible := visibleByID[allocation.ProjectID]; visible {
			projectIDs[allocation.ProjectID] = true
		}
	}
	projects := make([]dao.Project, 0, len(projectIDs))
	for projectID := range projectIDs {
		projects = append(projects, visibleByID[projectID])
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	rules, err := repo.ListRuleRevisionsForProjection(ctx, userID)
	if err != nil {
		return dao.Week{}, err
	}
	allocationByOrigin := make(map[string]bool, len(allocations))
	for _, allocation := range allocations {
		allocationByOrigin[occurrenceOriginKey(allocation.RuleID, allocation.OccurrenceDate)] = true
	}
	for _, rule := range rules {
		generated, err := projectedRuleOccurrences(ctx, repo, userID, rule, date.AddDate(0, 0, -1), endDate.AddDate(0, 0, 1))
		if err != nil {
			return dao.Week{}, err
		}
		for _, occurrence := range generated {
			if !occurrence.EndAt.After(from) || !occurrence.StartAt.Before(through) {
				continue
			}
			key := occurrenceOriginKey(occurrence.RuleID, occurrence.OccurrenceDate)
			if allocationByOrigin[key] {
				continue
			}
			if project, visible := visibleByID[occurrence.ProjectID]; visible {
				occurrence.ProjectTitle = project.Title
				if !projectIDs[project.ID] {
					projects = append(projects, project)
					projectIDs[project.ID] = true
				}
			}
			allocations = append(allocations, occurrence)
			allocationByOrigin[key] = true
		}
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].ID < projects[j].ID })
	exceptions, err := repo.ListSleepExceptions(ctx, userID, date.AddDate(0, 0, -1), endDate.AddDate(0, 0, 1))
	if err != nil {
		return dao.Week{}, err
	}
	week := dao.Week{WeekStart: date, Timezone: profile.Timezone, Projects: projects, Allocations: allocations,
		Days: make([]dao.Day, 0, 7), SleepIntervals: make([]dao.Interval, 0, 8), PausedRules: make([]dao.PausedRule, 0)}
	projectIndex := make(map[string]int, len(projects))
	for i := range week.Projects {
		projectIndex[week.Projects[i].ID] = i
		week.TargetMinutes += week.Projects[i].TargetMinutes
	}
	projectDurations := make(map[string]time.Duration, len(projects))
	weekDuration := time.Duration(0)
	for _, allocation := range allocations {
		duration := clippedDuration(allocation.StartAt, allocation.EndAt, from, through)
		weekDuration += duration
		projectDurations[allocation.ProjectID] += duration
	}
	week.AllocatedMinutes = int(weekDuration.Minutes())
	for i := range week.Projects {
		week.Projects[i].AllocatedMinutes = int(projectDurations[week.Projects[i].ID].Minutes())
		week.Projects[i].DifferenceMinutes = week.Projects[i].AllocatedMinutes - week.Projects[i].TargetMinutes
	}
	week.DifferenceMinutes = week.AllocatedMinutes - week.TargetMinutes
	type pausedRuleInfo struct {
		projectID     string
		reason        string
		effectiveFrom time.Time
		revision      int32
	}
	pausedByRule := make(map[string]pausedRuleInfo)
	for _, rule := range rules {
		if rule.PausedReason != "" {
			current, exists := pausedByRule[rule.ID]
			if !exists || rule.EffectiveFrom.After(current.effectiveFrom) ||
				(rule.EffectiveFrom.Equal(current.effectiveFrom) && rule.Revision > current.revision) {
				pausedByRule[rule.ID] = pausedRuleInfo{
					projectID: rule.ProjectID, reason: rule.PausedReason,
					effectiveFrom: rule.EffectiveFrom, revision: rule.Revision,
				}
			}
		}
	}
	for ruleID, paused := range pausedByRule {
		week.PausedRules = append(week.PausedRules, dao.PausedRule{RuleID: ruleID, ProjectID: paused.projectID, Reason: paused.reason})
	}
	sort.Slice(week.PausedRules, func(i, j int) bool { return week.PausedRules[i].RuleID < week.PausedRules[j].RuleID })
	exceptionByDate := make(map[string]dao.SleepException, len(exceptions))
	for _, exception := range exceptions {
		exceptionByDate[exception.LocalDate.Format("2006-01-02")] = exception
	}
	wakeDefault, _ := time.Parse("15:04", profile.WakeUpTime)
	bedDefault, _ := time.Parse("15:04", profile.Bedtime)
	awake := make(map[string]domain.Interval, 10)
	days := make(map[string]dao.Day, 7)
	for day := date.AddDate(0, 0, -1); !day.After(endDate); day = day.AddDate(0, 0, 1) {
		wake, bedtime, timezone, exception := wakeDefault, bedDefault, profile.Timezone, dao.SleepException{}
		hasException := false
		var awakeInterval domain.Interval
		if saved, ok := exceptionByDate[day.Format("2006-01-02")]; ok {
			awakeInterval, err = domain.AwakeIntervalFromLocalPair(day, saved.WakeLocal, saved.BedtimeLocal, saved.Timezone)
			exception, hasException = saved, true
		} else {
			awakeInterval, err = domain.AwakeInterval(day, wake, bedtime, timezone)
		}
		if err != nil {
			return dao.Week{}, err
		}
		awake[day.Format("2006-01-02")] = awakeInterval
		if !day.Before(date) && day.Before(endDate) {
			days[day.Format("2006-01-02")] = dao.Day{Date: day, WakeAt: awakeInterval.Start, BedtimeAt: awakeInterval.End, SleepRevision: exception.Revision, HasSleepException: hasException}
		}
	}
	weekFree := time.Duration(0)
	for i := 0; i < 7; i++ {
		dayDate := date.AddDate(0, 0, i)
		day := days[dayDate.Format("2006-01-02")]
		dayStart, ok1 := calendar.ResolveForward(dayDate, time.Time{}, 0, location)
		dayEnd, ok2 := calendar.ResolveForward(dayDate.AddDate(0, 0, 1), time.Time{}, 0, location)
		if !ok1 || !ok2 {
			return dao.Week{}, ErrInvalidRequest
		}
		dayAllocationDuration := time.Duration(0)
		for _, allocation := range allocations {
			dayAllocationDuration += clippedDuration(allocation.StartAt, allocation.EndAt, dayStart, dayEnd)
		}
		day.AllocatedMinutes = int(dayAllocationDuration.Minutes())
		awakeParts := make([]domain.Interval, 0, 3)
		for offset := -1; offset <= 1; offset++ {
			window, exists := awake[dayDate.AddDate(0, 0, offset).Format("2006-01-02")]
			if !exists {
				continue
			}
			start, end := maxTime(window.Start, maxTime(dayStart, from)), minTime(window.End, minTime(dayEnd, through))
			if end.After(start) {
				awakeParts = append(awakeParts, domain.Interval{Start: start, End: end})
			}
		}
		awakeDuration := unionDuration(awakeParts)
		occupied := make([]domain.Interval, 0)
		for _, allocation := range allocations {
			for _, awakePart := range awakeParts {
				start, end := maxTime(allocation.StartAt, awakePart.Start), minTime(allocation.EndAt, awakePart.End)
				if end.After(start) {
					occupied = append(occupied, domain.Interval{Start: start, End: end})
				}
			}
		}
		freeDuration := awakeDuration - unionDuration(occupied)
		if freeDuration < 0 {
			freeDuration = 0
		}
		day.FreeMinutes = int(freeDuration.Minutes())
		weekFree += freeDuration

		week.Days = append(week.Days, day)
	}
	week.FreeMinutes = int(weekFree.Minutes())
	for i := 0; i <= 7; i++ {
		wakeDate := date.AddDate(0, 0, i)
		previousAwake := awake[wakeDate.AddDate(0, 0, -1).Format("2006-01-02")]
		currentAwake := awake[wakeDate.Format("2006-01-02")]
		start, end := maxTime(previousAwake.End, from), minTime(currentAwake.Start, through)
		if end.After(start) {
			week.SleepIntervals = append(week.SleepIntervals, dao.Interval{StartAt: start, EndAt: end, LocalDate: wakeDate})
		}
	}
	return week, nil
}

func wall(value time.Time) time.Time {
	return time.Date(0, 1, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}
func clippedDuration(start, end, from, through time.Time) time.Duration {
	start, end = maxTime(start, from), minTime(end, through)
	if !end.After(start) {
		return 0
	}
	return end.Sub(start)
}
func maxTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}
func minTime(a, b time.Time) time.Time {
	if a.After(b) {
		return b
	}
	return a
}
func unionDuration(intervals []domain.Interval) time.Duration {
	if len(intervals) == 0 {
		return 0
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].Start.Before(intervals[j].Start) })
	start, end := intervals[0].Start, intervals[0].End
	total := time.Duration(0)
	for _, interval := range intervals[1:] {
		if interval.Start.After(end) {
			total += end.Sub(start)
			start, end = interval.Start, interval.End
		} else if interval.End.After(end) {
			end = interval.End
		}
	}
	total += end.Sub(start)
	return total
}
