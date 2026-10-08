package usecase

import (
	"fmt"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/schedule/dao"
	"github.com/Najah7/task2todaytodo/internal/application/schedule/domain"
	calendar "github.com/Najah7/task2todaytodo/internal/application/shared/calendar"
	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
)

func listReferenceTime(request CursorPageRequest, now time.Time) (time.Time, error) {
	if request.Anchor != nil && request.Anchor.AsOf != "" {
		asOf, err := time.Parse(time.RFC3339Nano, request.Anchor.AsOf)
		if err != nil {
			return time.Time{}, ErrInvalidSchedulePage
		}
		return asOf, nil
	}
	if !request.AsOf.IsZero() {
		return request.AsOf, nil
	}
	return now, nil
}

func listVisibleTarget(request CursorPageRequest) int {
	target := request.Size + 1
	if request.Anchor != nil {
		target++
	}
	return target
}

func listStartDate(request CursorPageRequest, _ time.Time, _ string) (time.Time, error) {
	if request.FromDate == "" {
		return time.Time{}, nil
	}
	return parseOccurrenceDate(request.FromDate)
}

func generateScheduleListDates(anchor, from time.Time, interval int, frequencies domain.Frequencies, root dao.Schedule, request CursorPageRequest, overrides map[string]dao.Schedule, skipped map[string]bool) ([]recurrence.RecurrenceDate, error) {
	timezone := root.Timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, recurrence.ErrRecurrenceTimezoneInvalid
	}
	startLocal, endLocal := time.Unix(root.StartAt, 0).In(location), time.Unix(root.EndAt, 0).In(location)
	limit := listVisibleTarget(request)
	for {
		dates, err := recurrence.GenerateRecurrenceDatesFromAnchorLimit(anchor, from, interval, frequencies, timezone, limit)
		if err != nil {
			return nil, err
		}
		visible := 0
		for i, occurrence := range dates {
			date := occurrence.Date
			key := date.Format("2006-01-02")
			if skipped[key] {
				continue
			}
			_, ok := calendar.ResolveWallTime(date, startLocal, 0, location)
			if !ok {
				continue
			}
			if _, ok = calendar.ResolveWallTime(date, endLocal, calendar.CalendarDayOffset(startLocal, endLocal), location); !ok {
				continue
			}
			if saved, found := overrides[root.ID+"/"+key]; found {
				if !saved.Deleted && (saved.IsException || saved.Completed || saved.ID == root.ID) && requestedDateIncludes(request, mustRequestStart(request), date) {
					visible++
				}
				if visible >= listVisibleTarget(request) {
					return dates[:i+1], nil
				}
				continue
			}
			visible++
			if visible >= listVisibleTarget(request) {
				return dates[:i+1], nil
			}
		}
		if len(dates) < limit {
			return dates, nil
		}
		limit *= 2
	}
}

func mustRequestStart(request CursorPageRequest) time.Time {
	if request.FromDate == "" {
		return time.Time{}
	}
	parsed, _ := parseOccurrenceDate(request.FromDate)
	return parsed
}

func requestedDateIncludes(request CursorPageRequest, requestedStart, date time.Time) bool {
	return request.FromDate == "" || !date.Before(requestedStart)
}

func requestedScheduleIncludes(request CursorPageRequest, requestedStart time.Time, row dao.Schedule) bool {
	return request.FromDate == "" || scheduleRowOnOrAfter(row, requestedStart)
}

func expandScheduleRows(rows []dao.Schedule, request CursorPageRequest, asOf time.Time) ([]dao.Schedule, error) {
	return expandScheduleRowsWithSkipped(rows, request, asOf, nil)
}

func expandScheduleRowsWithSkipped(rows []dao.Schedule, request CursorPageRequest, asOf time.Time, skipped map[string]map[string]bool) ([]dao.Schedule, error) {
	if skipped == nil {
		skipped = map[string]map[string]bool{}
	}
	byOccurrence := make(map[string]dao.Schedule, len(rows))
	roots := make([]dao.Schedule, 0)
	for _, row := range rows {
		key := row.SeriesID + "/" + row.OccurrenceDate
		if existing, ok := byOccurrence[key]; !ok || row.ID != row.SeriesID || existing.ID == existing.SeriesID {
			byOccurrence[key] = row
		}
		if row.ID == row.SeriesID {
			roots = append(roots, row)
		}
	}
	out := make([]dao.Schedule, 0, len(rows))
	for _, root := range roots {
		requestedStart, err := listStartDate(request, asOf, root.Timezone)
		if err != nil {
			return nil, err
		}
		location, err := time.LoadLocation(root.Timezone)
		if err != nil {
			return nil, recurrence.ErrRecurrenceTimezoneInvalid
		}
		startDate := localToday(asOf, location)
		if requestedStart.After(startDate) {
			startDate = requestedStart
		}
		if request.Anchor != nil && request.Anchor.At != "" {
			if at, parseErr := time.Parse(time.RFC3339Nano, request.Anchor.At); parseErr == nil {
				local := at.In(location)
				anchorDate := calendar.NormalizeCalendarDate(local)
				if anchorDate.After(startDate) {
					startDate = anchorDate
				}
			}
		}
		firstDate, err := parseOccurrenceDate(root.OccurrenceDate)
		if err != nil {
			return nil, fmt.Errorf("parse schedule %s occurrence date: %w", root.ID, err)
		}
		state := root.RepeatState
		if state == "" {
			state = repeatStateOneOff
			if root.IntervalWeeks > domain.OnceIntervalWeeks {
				state = repeatStateActive
			}
		}
		if state == repeatStateOneOff {
			visible := request.FromDate == "" || scheduleRowOnOrAfter(root, requestedStart)
			if saved, ok := byOccurrence[root.ID+"/"+root.OccurrenceDate]; ok && saved.ID != root.ID {
				if !saved.Deleted && !skipped[root.ID][root.OccurrenceDate] && visible {
					out = append(out, scheduleWithRootRecurrence(saved, root))
				}
			} else if !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && visible {
				root.CursorStartAt = time.Unix(root.StartAt, 0).UTC().Format(time.RFC3339Nano)
				out = append(out, root)
			}
			continue
		}
		if state == repeatStateActive && root.IntervalWeeks > 0 && !root.Deleted && !root.ProjectDone {
			phase := firstDate
			if root.FrequencyAnchorDate != 0 {
				phase = time.Unix(root.FrequencyAnchorDate, 0).UTC()
			}
			if phase.Before(firstDate) {
				phase = firstDate
			}
			if phase.Before(startDate) {
				phase = startDate
			}
			frequencies, err := frequenciesFromDAO(root.Frequencies)
			if err != nil {
				return nil, err
			}
			anchor := time.Unix(root.FrequencyAnchorDate, 0).UTC()
			if root.FrequencyAnchorDate == 0 {
				anchor = firstDate
			}
			generated, err := generateScheduleListDates(anchor, phase, root.IntervalWeeks, frequencies, root, request, byOccurrence, skipped[root.ID])
			if err != nil {
				return nil, err
			}
			location := recurrenceLocation(root.Timezone)
			startLocal, endLocal := time.Unix(root.StartAt, 0).In(location), time.Unix(root.EndAt, 0).In(location)
			for _, occurrence := range generated {
				date := occurrence.Date
				keyDate := date.Format("2006-01-02")
				if skipped[root.ID][keyDate] {
					continue
				}
				start, valid := calendar.ResolveWallTime(date, startLocal, 0, location)
				if !valid {
					continue
				}
				end, valid := calendar.ResolveWallTime(date, endLocal, calendar.CalendarDayOffset(startLocal, endLocal), location)
				if !valid {
					continue
				}
				if saved, ok := byOccurrence[root.ID+"/"+keyDate]; ok && (saved.IsException || saved.Completed || saved.Deleted) {
					if !saved.Deleted && !skipped[root.ID][keyDate] && requestedDateIncludes(request, requestedStart, date) {
						saved.CursorStartAt = time.Unix(saved.StartAt, 0).UTC().Format(time.RFC3339Nano)
						out = append(out, scheduleWithRootRecurrence(saved, root))
					}
					continue
				}
				item := root
				item.ID = VirtualOccurrenceID
				item.StartAt, item.EndAt, item.OccurrenceDate = start.Unix(), end.Unix(), keyDate
				item.Completed, item.IsException = false, false
				if date.Equal(firstDate) {
					item.ID = root.ID
				}
				item.CursorStartAt = start.UTC().Format(time.RFC3339Nano)
				out = append(out, item)
			}
		}
		if state == repeatStateActive && !firstDate.Before(localToday(asOf, location)) && requestedScheduleIncludes(request, requestedStart, root) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasSchedule(out, root.ID, root.OccurrenceDate) {
			root.CursorStartAt = time.Unix(root.StartAt, 0).UTC().Format(time.RFC3339Nano)
			out = append(out, root)
		}
		for _, saved := range rows {
			if saved.SeriesID != root.ID || saved.ID == root.ID || saved.Deleted || skipped[root.ID][saved.OccurrenceDate] || (!saved.IsException && !saved.Completed) || !requestedScheduleIncludes(request, requestedStart, saved) || hasSchedule(out, root.ID, saved.OccurrenceDate) {
				continue
			}
			saved.CursorStartAt = time.Unix(saved.StartAt, 0).UTC().Format(time.RFC3339Nano)
			out = append(out, scheduleWithRootRecurrence(saved, root))
		}
		if request.FromDate != "" && firstDate.Before(localToday(asOf, location)) && requestedScheduleIncludes(request, requestedStart, root) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasSchedule(out, root.ID, root.OccurrenceDate) {
			root.CursorStartAt = time.Unix(root.StartAt, 0).UTC().Format(time.RFC3339Nano)
			out = append(out, root)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartAt != out[j].StartAt {
			return out[i].StartAt < out[j].StartAt
		}
		if out[i].SeriesID != out[j].SeriesID {
			return out[i].SeriesID < out[j].SeriesID
		}
		return out[i].OccurrenceDate < out[j].OccurrenceDate
	})
	return out, nil
}

func scheduleRowOnOrAfter(row dao.Schedule, fromDate time.Time) bool {
	local := time.Unix(row.StartAt, 0).In(recurrenceLocation(row.Timezone))
	date := calendar.NormalizeCalendarDate(local)
	return !date.Before(fromDate)
}

func scheduleWithRootRecurrence(row, root dao.Schedule) dao.Schedule {
	row.UserID, row.ProjectID, row.ProjectDone, row.AssigneeID = root.UserID, root.ProjectID, root.ProjectDone, root.AssigneeID
	row.IntervalWeeks, row.Frequencies = root.IntervalWeeks, root.Frequencies
	row.RepeatState, row.FrequencyAnchorDate = root.RepeatState, root.FrequencyAnchorDate
	return row
}

func hasSchedule(rows []dao.Schedule, seriesID, occurrenceDate string) bool {
	for _, row := range rows {
		if row.SeriesID == seriesID && row.OccurrenceDate == occurrenceDate {
			return true
		}
	}
	return false
}

func afterScheduleAnchor(row dao.Schedule, anchor *CursorAnchor) bool {
	if anchor == nil {
		return true
	}
	if row.CursorStartAt != anchor.At {
		return row.CursorStartAt > anchor.At
	}
	seriesID := anchor.SeriesID
	if seriesID == "" {
		seriesID = anchor.ID
	}
	if row.SeriesID != seriesID {
		return row.SeriesID > seriesID
	}
	return row.OccurrenceDate > anchor.Date
}
