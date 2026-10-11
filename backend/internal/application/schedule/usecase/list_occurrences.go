package usecase

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
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
	var revisions []dao.BlueprintRevision
	var saved []dao.ScheduleOccurrence
	var origins []dao.ScheduleOccurrenceOrigin
	if err := uc.uow.Do(ctx, func(ctx context.Context, repos Repositories) error {
		var err error
		revisions, err = repos.Blueprints().ListBlueprintRevisionsForRange(ctx, actorID, request.From, request.To)
		if err != nil {
			return err
		}
		saved, err = repos.Blueprints().ListPersistedOccurrences(ctx, actorID, request.From, request.To)
		if err != nil {
			return err
		}
		blueprintIDs := make([]string, 0, len(revisions))
		seenBlueprints := make(map[string]struct{}, len(revisions))
		for _, revision := range revisions {
			if _, seen := seenBlueprints[revision.ID]; seen {
				continue
			}
			seenBlueprints[revision.ID] = struct{}{}
			blueprintIDs = append(blueprintIDs, revision.ID)
		}
		origins, err = repos.Blueprints().ListOccurrenceOrigins(ctx, actorID, blueprintIDs, request.From, request.To)
		if err != nil {
			return err
		}
		for i := range revisions {
			revisions[i].PauseIntervals, err = repos.Blueprints().ListPauseIntervals(ctx, actorID, revisions[i].ID)
			if err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return CursorPage[dao.ScheduleOccurrence]{}, err
	}
	items, err := projectOccurrences(revisions, saved, origins, request.From, request.To)
	if err != nil {
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
	page := CursorPage[dao.ScheduleOccurrence]{Items: pageItems}
	if more && len(pageItems) > 0 {
		last := pageItems[len(pageItems)-1]
		page.Next = &CursorAnchor{
			At:          time.Unix(last.StartAt, 0).UTC().Format(time.RFC3339Nano),
			BlueprintID: last.BlueprintID, OccurrenceDate: last.OccurrenceDate,
		}
	}
	return page, nil
}

func projectOccurrences(revisions []dao.BlueprintRevision, saved []dao.ScheduleOccurrence, origins []dao.ScheduleOccurrenceOrigin, from, to time.Time) ([]dao.ScheduleOccurrence, error) {
	savedByKey := make(map[string]dao.ScheduleOccurrence, len(saved))
	existingByKey := make(map[string]struct{}, len(origins)+len(saved))
	rows := make([]dao.ScheduleOccurrence, 0, len(saved))
	for _, origin := range origins {
		existingByKey[origin.BlueprintID+"/"+origin.OccurrenceDate] = struct{}{}
	}
	for _, row := range saved {
		savedByKey[row.BlueprintID+"/"+row.OccurrenceDate] = row
		existingByKey[row.BlueprintID+"/"+row.OccurrenceDate] = struct{}{}
		if !row.Deleted && !row.Skipped && overlaps(row.StartAt, row.EndAt, from, to) {
			rows = append(rows, row)
		}
	}
	byBlueprint := make(map[string][]dao.BlueprintRevision)
	for _, revision := range revisions {
		byBlueprint[revision.ID] = append(byBlueprint[revision.ID], revision)
	}
	for blueprintID, rules := range byBlueprint {
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
				return nil, recurrence.ErrRecurrenceTimezoneInvalid
			}
			localStart := parseLocalTimestamp(rule.LocalStart)
			localEnd := parseLocalTimestamp(rule.LocalEnd)
			if localStart.IsZero() || localEnd.IsZero() {
				return nil, recurrence.ErrBlueprintRangeInvalid
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
			occurrences, err := recurrence.GenerateBlueprintOccurrences(recurrence.BlueprintRule{
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
				key := blueprintID + "/" + date
				if _, exists := existingByKey[key]; exists {
					continue
				}
				rows = append(rows, dao.ScheduleOccurrence{
					BlueprintID: blueprintID, OccurrenceDate: date, SourceRevisionID: rule.RevisionID,
					UserID: rule.UserID, ProjectID: rule.ProjectID, AssigneeID: rule.AssigneeID,
					Title: rule.Title, Description: rule.Description, Location: rule.Location,
					Timezone: rule.Timezone, StartAt: occurrence.StartAt.Unix(), EndAt: occurrence.EndAt.Unix(),
				})
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StartAt != rows[j].StartAt {
			return rows[i].StartAt < rows[j].StartAt
		}
		if rows[i].BlueprintID != rows[j].BlueprintID {
			return rows[i].BlueprintID < rows[j].BlueprintID
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
	blueprintID := anchor.BlueprintID
	if blueprintID == "" {
		blueprintID = anchor.SeriesID
	}
	if item.BlueprintID != blueprintID {
		return item.BlueprintID > blueprintID
	}
	date := anchor.OccurrenceDate
	if date == "" {
		date = anchor.Date
	}
	return item.OccurrenceDate > date
}

func overlaps(startAt, endAt int64, from, to time.Time) bool {
	return time.Unix(startAt, 0).Before(to) && time.Unix(endAt, 0).After(from)
}

func localCalendarDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func parseLocalTimestamp(value string) time.Time {
	parsed, err := time.Parse("2006-01-02T15:04:05.999999999", value)
	if err != nil {
		parsed, err = time.Parse("2006-01-02 15:04:05.999999999", value)
	}
	if err != nil {
		parsed, _ = time.Parse("2006-01-02T15:04:05", value)
	}
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
			return nil, recurrence.ErrBlueprintFrequencyInvalid
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
