package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

var (
	ErrScheduleRangeRequired = errors.New("schedule list requires from and to")
	ErrScheduleRangeInvalid  = errors.New("schedule range must be a positive UTC interval")
)

type ListOccurrencesUseCase struct {
	uow UOW
}

func NewListOccurrencesUseCase(uow UOW) *ListOccurrencesUseCase {
	return &ListOccurrencesUseCase{uow: uow}
}

func (uc *ListOccurrencesUseCase) ExecutePage(ctx context.Context, actorID string, request CursorPageRequest) (CursorPage[dao.ScheduleOccurrence], error) {
	if request.From.IsZero() || request.To.IsZero() {
		return CursorPage[dao.ScheduleOccurrence]{}, ErrScheduleRangeRequired
	}
	if request.From.Location() != time.UTC || request.To.Location() != time.UTC || !request.To.After(request.From) {
		return CursorPage[dao.ScheduleOccurrence]{}, ErrScheduleRangeInvalid
	}
	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.ScheduleOccurrence]{}, err
	}
	var items []dao.ScheduleOccurrence
	var revisions []dao.RuleRevision
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		items, revisions, err = projectedOccurrencesWithRevisionsInTx(ctx, actorID, request.From, request.To, request.IncludeSkipped, repos)
		return err
	}); err != nil {
		return CursorPage[dao.ScheduleOccurrence]{}, err
	}
	filtered := items[:0]
	for _, item := range items {
		inScope := request.ProjectID != "" && item.ProjectID == request.ProjectID
		if request.ProjectID == "" {
			inScope = item.AssigneeID == actorID
		}
		if inScope && afterOccurrenceAnchor(item, request.Anchor) {
			filtered = append(filtered, item)
		}
	}
	pageItems, more := pagination.Window(filtered, request.Size)
	now := request.AsOf
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		capabilitiesByOccurrence, err := repos.Rules().ListOccurrenceCapabilities(ctx, actorID, pageItems)
		if err != nil {
			return err
		}
		for i := range pageItems {
			capabilities := capabilitiesByOccurrence[pageItems[i].RuleID+"/"+pageItems[i].OccurrenceDate]
			pageItems[i].CanUpdate = capabilities.CanUpdate
			pageItems[i].CanDelete = capabilities.CanDelete
			pageItems[i].CanSkip = capabilities.CanSkip
			pageItems[i].CanAssign = capabilities.CanAssign
			if followingAllowed(activeOccurrenceStart(revisions, pageItems[i]), now) {
				pageItems[i].CanUpdateFollowing = capabilities.CanUpdateFollowing
				pageItems[i].CanDeleteFollowing = capabilities.CanDeleteFollowing
			}
			if err := restrictScheduleWriteCapabilities(ctx, repos, actorID, &pageItems[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return CursorPage[dao.ScheduleOccurrence]{}, err
	}
	page := CursorPage[dao.ScheduleOccurrence]{Items: pageItems}
	if more && len(pageItems) > 0 {
		last := pageItems[len(pageItems)-1]
		page.Next = &CursorAnchor{
			At:     time.Unix(last.StartAt, 0).UTC().Format(time.RFC3339Nano),
			RuleID: last.RuleID, OccurrenceDate: last.OccurrenceDate,
		}
	}
	return page, nil
}

// RangeInTx projects every visible occurrence for an actor within a finite
// UTC range using repositories already bound to the caller's transaction. It
// is intended for trusted application workflows that need an unpaginated view
// without opening a nested UOW.
func (uc *ListOccurrencesUseCase) RangeInTx(ctx context.Context, actorID string, from, to time.Time, repos Repositories) ([]dao.ScheduleOccurrence, error) {
	if from.IsZero() || to.IsZero() {
		return nil, ErrScheduleRangeRequired
	}
	if from.Location() != time.UTC || to.Location() != time.UTC || !to.After(from) {
		return nil, ErrScheduleRangeInvalid
	}
	rows, _, err := projectedOccurrencesWithRevisionsInTx(ctx, actorID, from, to, false, repos)
	return rows, err
}

func projectedOccurrencesWithRevisionsInTx(ctx context.Context, actorID string, from, to time.Time, includeSkipped bool, repos Repositories) ([]dao.ScheduleOccurrence, []dao.RuleRevision, error) {
	revisions, err := repos.Rules().ListRuleRevisionsForRange(ctx, actorID, from, to)
	if err != nil {
		return nil, nil, err
	}
	saved, err := repos.Rules().ListPersistedOccurrences(ctx, actorID, from, to, includeSkipped)
	if err != nil {
		return nil, nil, err
	}
	ruleIDs := make([]string, 0, len(revisions))
	seenRules := make(map[string]struct{}, len(revisions))
	for _, revision := range revisions {
		if _, seen := seenRules[revision.ID]; seen {
			continue
		}
		seenRules[revision.ID] = struct{}{}
		ruleIDs = append(ruleIDs, revision.ID)
	}
	origins, err := repos.Rules().ListOccurrenceOrigins(ctx, actorID, ruleIDs, from, to)
	if err != nil {
		return nil, nil, err
	}
	for i := range revisions {
		revisions[i].PauseIntervals, err = repos.Rules().ListPauseIntervals(ctx, actorID, revisions[i].ID)
		if err != nil {
			return nil, nil, err
		}
	}
	rows, err := projectOccurrences(revisions, saved, origins, from, to, includeSkipped)
	return rows, revisions, err
}

func followingAllowed(scheduledStart, now time.Time) bool {
	return !scheduledStart.IsZero() && scheduledStart.After(now)
}

func activeOccurrenceStart(revisions []dao.RuleRevision, occurrence dao.ScheduleOccurrence) time.Time {
	date, err := time.Parse("2006-01-02", occurrence.OccurrenceDate)
	if err != nil {
		return time.Time{}
	}
	var active *dao.RuleRevision
	for i := range revisions {
		revision := &revisions[i]
		if revision.ID != occurrence.RuleID || revision.EffectiveFrom > occurrence.OccurrenceDate ||
			(revision.EffectiveUntil != nil && *revision.EffectiveUntil < occurrence.OccurrenceDate) {
			continue
		}
		if active == nil || revision.EffectiveFrom > active.EffectiveFrom ||
			(revision.EffectiveFrom == active.EffectiveFrom && revision.Revision > active.Revision) {
			active = revision
		}
	}
	if active == nil {
		return time.Time{}
	}
	start, end := parseLocalTimestamp(active.LocalStart), parseLocalTimestamp(active.LocalEnd)
	if start.IsZero() || end.IsZero() {
		return time.Time{}
	}
	phase := localCalendarDate(start)
	if effective, parseErr := time.Parse("2006-01-02", active.EffectiveFrom); parseErr == nil && effective.After(phase) {
		phase = effective
	}
	anchor := phase
	if active.RecurrenceAnchorDate != "" {
		if parsed, parseErr := time.Parse("2006-01-02", active.RecurrenceAnchorDate); parseErr == nil {
			anchor = parsed
		}
	}
	var recurrenceEnd *time.Time
	if active.EndDate != nil {
		if parsed, parseErr := time.Parse("2006-01-02", *active.EndDate); parseErr == nil {
			recurrenceEnd = &parsed
		}
	}
	occurrences, err := domain.GenerateOccurrences(domain.RecurrenceRule{
		Start: start, End: end, AnchorDate: anchor, Timezone: active.Timezone,
		Frequency: active.Frequency, Interval: active.Interval, Weekdays: mustParseWeekdays(active.Weekdays), EndDate: recurrenceEnd,
	}, date, date)
	if err != nil || len(occurrences) != 1 {
		return time.Time{}
	}
	return occurrences[0].StartAt
}

func projectOccurrences(revisions []dao.RuleRevision, saved []dao.ScheduleOccurrence, origins []dao.ScheduleOccurrenceOrigin, from, to time.Time, includeSkipped bool) ([]dao.ScheduleOccurrence, error) {
	savedByKey := make(map[string]dao.ScheduleOccurrence, len(saved))
	existingByKey := make(map[string]struct{}, len(origins)+len(saved))
	rows := make([]dao.ScheduleOccurrence, 0, len(saved))
	for _, origin := range origins {
		existingByKey[origin.RuleID+"/"+origin.OccurrenceDate] = struct{}{}
	}
	for _, row := range saved {
		savedByKey[row.RuleID+"/"+row.OccurrenceDate] = row
		existingByKey[row.RuleID+"/"+row.OccurrenceDate] = struct{}{}
		if !row.Deleted && (includeSkipped || !row.Skipped) && overlaps(row.StartAt, row.EndAt, from, to) {
			rows = append(rows, row)
		}
	}
	byRule := make(map[string][]dao.RuleRevision)
	for _, revision := range revisions {
		byRule[revision.ID] = append(byRule[revision.ID], revision)
	}
	for ruleID, rules := range byRule {
		sort.Slice(rules, func(i, j int) bool {
			if rules[i].EffectiveFrom != rules[j].EffectiveFrom {
				return rules[i].EffectiveFrom < rules[j].EffectiveFrom
			}
			return rules[i].Revision < rules[j].Revision
		})
		for i := range rules {
			rule := rules[i]
			location, err := time.LoadLocation(rule.Timezone)
			if err != nil {
				return nil, domain.ErrRecurrenceTimezoneInvalid
			}
			localStart := parseLocalTimestamp(rule.LocalStart)
			localEnd := parseLocalTimestamp(rule.LocalEnd)
			if localStart.IsZero() || localEnd.IsZero() {
				return nil, domain.ErrRuleRangeInvalid
			}
			phase := localCalendarDate(localStart)
			if rule.EffectiveFrom != "" {
				effectiveDate, parseErr := time.Parse("2006-01-02", rule.EffectiveFrom)
				if parseErr != nil {
					return nil, parseErr
				}
				if effectiveDate.After(phase) {
					phase = effectiveDate
				}
			}
			anchorDate := phase
			if rule.RecurrenceAnchorDate != "" {
				anchorDate, err = time.Parse("2006-01-02", rule.RecurrenceAnchorDate)
				if err != nil {
					return nil, err
				}
			}
			startDate := localCalendarDate(from.In(location))
			localEndDate := localCalendarDate(localEnd)
			localStartDate := localCalendarDate(localStart)
			dayOffset := int((localEndDate.Unix() - localStartDate.Unix()) / 86400)
			if dayOffset > 0 {
				startDate = startDate.AddDate(0, 0, -dayOffset)
			}
			if phase.After(startDate) {
				startDate = phase
			}
			endInstant := to.Add(-time.Nanosecond)
			endDate := localCalendarDate(endInstant.In(location))
			// EffectiveUntil is computed across all revisions before ACL
			// filtering. Visible revisions may omit the next version, so do not
			// infer a boundary from this result slice.
			if rule.EffectiveUntil != nil {
				latestEffective, err := time.Parse("2006-01-02", *rule.EffectiveUntil)
				if err != nil {
					return nil, err
				}
				if latestEffective.Before(endDate) {
					endDate = latestEffective
				}
			}
			if rule.EndDate != nil {
				ruleEnd, err := time.Parse("2006-01-02", *rule.EndDate)
				if err != nil {
					return nil, err
				}
				if ruleEnd.Before(endDate) {
					endDate = ruleEnd
				}
			}
			if endDate.Before(startDate) {
				continue
			}
			weekdays, err := parseWeekdays(rule.Weekdays)
			if err != nil {
				return nil, err
			}
			var recurrenceEnd *time.Time
			if rule.EndDate != nil {
				value, _ := time.Parse("2006-01-02", *rule.EndDate)
				recurrenceEnd = &value
			}
			occurrences, err := domain.GenerateOccurrences(domain.RecurrenceRule{
				Start: localStart, AnchorDate: anchorDate, End: localEnd, Timezone: rule.Timezone,
				Frequency: rule.Frequency, Interval: rule.Interval, Weekdays: weekdays, EndDate: recurrenceEnd,
			}, startDate, endDate)
			if err != nil {
				return nil, err
			}
			pauses := rule.PauseIntervals
			for _, occurrence := range occurrences {
				if !occurrence.StartAt.Before(to) || !occurrence.EndAt.After(from) || pausedForProject(pauses, rule.ProjectID, occurrence.StartAt) {
					continue
				}
				date := occurrence.Date.Format("2006-01-02")
				key := ruleID + "/" + date
				if _, exists := existingByKey[key]; exists {
					continue
				}
				rows = append(rows, dao.ScheduleOccurrence{
					RuleID: ruleID, OccurrenceDate: date, SourceRevisionID: rule.RevisionID,
					UserID: rule.UserID, ProjectID: rule.ProjectID, AssigneeID: rule.AssigneeID,
					Title: rule.Title, Description: rule.Description, Location: rule.Location,
					Timezone: rule.Timezone, AllDay: rule.AllDay, StartAt: occurrence.StartAt.Unix(), EndAt: occurrence.EndAt.Unix(),
				})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StartAt != rows[j].StartAt {
			return rows[i].StartAt < rows[j].StartAt
		}
		if rows[i].RuleID != rows[j].RuleID {
			return rows[i].RuleID < rows[j].RuleID
		}
		return rows[i].OccurrenceDate < rows[j].OccurrenceDate
	})
	return rows, nil
}

func afterOccurrenceAnchor(item dao.ScheduleOccurrence, anchor *CursorAnchor) bool {
	if anchor == nil {
		return true
	}
	at := time.Unix(item.StartAt, 0).UTC().Format(time.RFC3339Nano)
	if at != anchor.At {
		return at > anchor.At
	}
	ruleID := anchor.RuleID
	if ruleID == "" {
		ruleID = anchor.SeriesID
	}
	if item.RuleID != ruleID {
		return item.RuleID > ruleID
	}
	date := anchor.OccurrenceDate
	if date == "" {
		date = anchor.Date
	}
	return item.OccurrenceDate > date
}

func overlaps(startAt, endAt int64, from, to time.Time) bool {
	return calendar.Overlaps(time.Unix(startAt, 0), time.Unix(endAt, 0), from, to)
}

func localCalendarDate(value time.Time) time.Time {
	return calendar.DateOnly(value)
}

func parseLocalTimestamp(value string) time.Time {
	parsed, _ := calendar.ParseLocalTimestamp(value)
	return parsed
}

func setLocalDate(value, date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func parseWeekdays(values []string) ([]time.Weekday, error) {
	result := make([]time.Weekday, 0, len(values))
	for _, value := range values {
		switch value {
		case "sun":
			result = append(result, time.Sunday)
		case "mon":
			result = append(result, time.Monday)
		case "tue":
			result = append(result, time.Tuesday)
		case "wed":
			result = append(result, time.Wednesday)
		case "thu":
			result = append(result, time.Thursday)
		case "fri":
			result = append(result, time.Friday)
		case "sat":
			result = append(result, time.Saturday)
		default:
			return nil, domain.ErrRuleFrequencyInvalid
		}
	}
	return result, nil
}

func pausedForProject(pauses []dao.ProjectPauseInterval, projectID string, startAt time.Time) bool {
	for _, pause := range pauses {
		if pause.ProjectID != projectID || startAt.Before(pause.PausedAt) {
			continue
		}
		if pause.ResumedAt == nil || startAt.Before(*pause.ResumedAt) {
			return true
		}
	}
	return false
}
