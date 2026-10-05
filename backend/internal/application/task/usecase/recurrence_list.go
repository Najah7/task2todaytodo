package usecase

import (
	"fmt"
	"sort"
	"time"

	tasktime "github.com/Najah7/task2todaytodo/internal/application/task"
	"github.com/Najah7/task2todaytodo/internal/application/task/dao"
	"github.com/Najah7/task2todaytodo/internal/application/task/domain"
)

const VirtualOccurrenceID = "TASK2TODAYTODO000000000000"

func listReferenceTime(request CursorPageRequest, now time.Time) (time.Time, error) {
	if request.Anchor != nil && request.Anchor.AsOf != "" {
		asOf, err := time.Parse(time.RFC3339Nano, request.Anchor.AsOf)
		if err != nil {
			return time.Time{}, ErrInvalidTaskPage
		}
		return asOf, nil
	}
	if !request.AsOf.IsZero() {
		return request.AsOf, nil
	}
	return now, nil
}

func listStartDate(request CursorPageRequest, asOf time.Time, timezone string) (time.Time, error) {
	if request.FromDate != "" {
		parsed, err := parseOccurrenceDate(request.FromDate)
		if err != nil {
			return time.Time{}, ErrInvalidTaskPage
		}
		return parsed, nil
	}
	return time.Time{}, nil
}

func expandTodoItemRows(rows []dao.TodoItem, request CursorPageRequest, asOf time.Time, taskDone bool) ([]dao.TodoItem, error) {
	return expandTodoItemRowsWithSkipped(rows, request, asOf, taskDone, nil)
}

func expandTodoItemRowsWithSkipped(rows []dao.TodoItem, request CursorPageRequest, asOf time.Time, taskDone bool, skipped map[string]map[string]bool) ([]dao.TodoItem, error) {
	if skipped == nil {
		skipped = map[string]map[string]bool{}
	}
	byOccurrence := make(map[string]dao.TodoItem, len(rows))
	roots := make([]dao.TodoItem, 0)
	for _, row := range rows {
		key := row.SeriesID + "/" + row.OccurrenceDate
		if existing, ok := byOccurrence[key]; !ok || row.ID != row.SeriesID || existing.ID == existing.SeriesID {
			byOccurrence[key] = row
		}
		if row.ID == row.SeriesID {
			roots = append(roots, row)
		}
	}
	out := make([]dao.TodoItem, 0, len(rows))
	for _, root := range roots {
		requestedStart, err := listStartDate(request, asOf, root.Timezone)
		if err != nil {
			return nil, err
		}
		location, err := time.LoadLocation(root.Timezone)
		if err != nil {
			return nil, domain.ErrRecurrenceTimezoneInvalid
		}
		startDate := localToday(asOf, location)
		if requestedStart.After(startDate) {
			startDate = requestedStart
		}
		if anchorDate := todoItemAnchorDate(request.Anchor); anchorDate != "" {
			anchor, err := parseOccurrenceDate(anchorDate)
			if err != nil {
				return nil, ErrInvalidTaskPage
			}
			if anchor.After(startDate) {
				startDate = anchor
			}
		}
		firstDate, err := parseOccurrenceDate(root.OccurrenceDate)
		if err != nil {
			return nil, fmt.Errorf("parse todo item %s occurrence date: %w", root.ID, err)
		}
		state := root.RepeatState
		if state == "" {
			state = repeatStateOneOff
			if root.IntervalWeeks > domain.OnceIntervalWeeks {
				state = repeatStateActive
			}
		}
		if state == repeatStateOneOff {
			visible := request.FromDate == "" || !firstDate.Before(requestedStart)
			if saved, ok := byOccurrence[root.ID+"/"+root.OccurrenceDate]; ok && saved.ID != root.ID {
				if !saved.Deleted && !skipped[root.ID][root.OccurrenceDate] && visible {
					out = append(out, todoWithRootRecurrence(saved, root))
				}
			} else if !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && visible {
				out = append(out, root)
			}
			continue
		}
		if state == repeatStateActive && root.IntervalWeeks > 0 && !root.Deleted && !taskDone {
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
			frequencies, err := taskFrequenciesFromDAO(root.Frequencies)
			if err != nil {
				return nil, err
			}
			anchor := time.Unix(root.FrequencyAnchorDate, 0).UTC()
			if root.FrequencyAnchorDate == 0 {
				anchor = firstDate
			}
			generated, err := generateTodoListDates(anchor, phase, root.IntervalWeeks, frequencies, root, request, byOccurrence, skipped[root.ID])
			if err != nil {
				return nil, err
			}
			for _, occurrence := range generated {
				date := occurrence.Date
				keyDate := date.Format("2006-01-02")
				if skipped[root.ID][keyDate] {
					continue
				}
				if saved, ok := byOccurrence[root.ID+"/"+keyDate]; ok && saved.ID != root.ID && (saved.IsException || saved.Completed || saved.Deleted) {
					if !saved.Deleted && !skipped[root.ID][keyDate] && requestedDateIncludes(request, requestedStart, date) {
						out = append(out, todoWithRootRecurrence(saved, root))
					}
					continue
				}
				item := root
				item.ID = VirtualOccurrenceID
				positionCount, err := domain.CountRecurrenceDatesFromAnchor(anchor, date, root.IntervalWeeks, frequencies, root.Timezone)
				if err != nil {
					return nil, err
				}
				item.Position = root.Position + positionCount - 1
				item.OccurrenceDate, item.Completed, item.IsException = keyDate, false, false
				if date.Equal(firstDate) {
					item.ID = root.ID
				}
				if item.DueDate != 0 {
					item.DueDate = date.Unix()
				}
				out = append(out, item)
			}
		}
		if state == repeatStateActive && !taskDone && !firstDate.Before(localToday(asOf, location)) && requestedDateIncludes(request, requestedStart, firstDate) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasTodoItem(out, root.ID, root.OccurrenceDate) {
			out = append(out, root)
		}
		if request.FromDate != "" && firstDate.Before(localToday(asOf, location)) && requestedDateIncludes(request, requestedStart, firstDate) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasTodoItem(out, root.ID, root.OccurrenceDate) {
			out = append(out, root)
		}
		// Keep saved history addressable even when current rule no longer contains its date.
		for _, saved := range rows {
			if saved.SeriesID != root.ID || saved.ID == root.ID || saved.Deleted || skipped[root.ID][saved.OccurrenceDate] || (!saved.IsException && !saved.Completed) || !requestedDateIncludes(request, requestedStart, mustParseDate(saved.OccurrenceDate)) {
				continue
			}
			if !hasTodoItem(out, root.ID, saved.OccurrenceDate) {
				out = append(out, todoWithRootRecurrence(saved, root))
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OccurrenceDate != out[j].OccurrenceDate {
			return out[i].OccurrenceDate < out[j].OccurrenceDate
		}
		if out[i].Position != out[j].Position {
			return out[i].Position < out[j].Position
		}
		return out[i].SeriesID < out[j].SeriesID
	})
	return out, nil
}

func listVisibleTarget(request CursorPageRequest) int {
	target := request.Size + 1
	if request.Anchor != nil {
		target++
	}
	return target
}

func todoWithRootRecurrence(row, root dao.TodoItem) dao.TodoItem {
	row.IntervalWeeks, row.Frequencies = root.IntervalWeeks, root.Frequencies
	row.RepeatState, row.FrequencyAnchorDate = root.RepeatState, root.FrequencyAnchorDate
	return row
}

func generateTodoListDates(anchor, from time.Time, interval int, frequencies domain.TaskFrequencies, root dao.TodoItem, request CursorPageRequest, overrides map[string]dao.TodoItem, skipped map[string]bool) ([]domain.RecurrenceDate, error) {
	limit := listVisibleTarget(request)
	for {
		dates, err := domain.GenerateRecurrenceDatesFromAnchorLimit(anchor, from, interval, frequencies, root.Timezone, limit)
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
			if saved, ok := overrides[root.ID+"/"+key]; ok {
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

func generateScheduleListDates(anchor, from time.Time, interval int, frequencies domain.TaskFrequencies, root dao.TaskSchedule, request CursorPageRequest, overrides map[string]dao.TaskSchedule, skipped map[string]bool) ([]domain.RecurrenceDate, error) {
	timezone := root.Timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, domain.ErrRecurrenceTimezoneInvalid
	}
	startLocal, endLocal := time.Unix(root.StartAt, 0).In(location), time.Unix(root.EndAt, 0).In(location)
	limit := listVisibleTarget(request)
	for {
		dates, err := domain.GenerateRecurrenceDatesFromAnchorLimit(anchor, from, interval, frequencies, timezone, limit)
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
			_, ok := tasktime.ResolveWallTime(date, startLocal, 0, location)
			if !ok {
				continue
			}
			if _, ok = tasktime.ResolveWallTime(date, endLocal, tasktime.CalendarDayOffset(startLocal, endLocal), location); !ok {
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

func requestedScheduleIncludes(request CursorPageRequest, requestedStart time.Time, row dao.TaskSchedule) bool {
	return request.FromDate == "" || scheduleRowOnOrAfter(row, requestedStart)
}

func todoItemAnchorDate(anchor *CursorAnchor) string {
	if anchor == nil {
		return ""
	}
	if anchor.OccurrenceDate != "" {
		return anchor.OccurrenceDate
	}
	return anchor.Date
}

func hasTodoItem(rows []dao.TodoItem, seriesID, occurrenceDate string) bool {
	for _, row := range rows {
		if row.SeriesID == seriesID && row.OccurrenceDate == occurrenceDate {
			return true
		}
	}
	return false
}

func afterTodoItemAnchor(row dao.TodoItem, anchor *CursorAnchor) bool {
	if anchor == nil {
		return true
	}
	date := todoItemAnchorDate(anchor)
	if row.OccurrenceDate != date {
		return row.OccurrenceDate > date
	}
	if row.Position != anchor.Position {
		return row.Position > anchor.Position
	}
	seriesID := anchor.SeriesID
	if seriesID == "" {
		seriesID = anchor.ID
	}
	return row.SeriesID > seriesID
}

func expandTaskScheduleRows(rows []dao.TaskSchedule, request CursorPageRequest, asOf time.Time, taskDone bool) ([]dao.TaskSchedule, error) {
	return expandTaskScheduleRowsWithSkipped(rows, request, asOf, taskDone, nil)
}

func expandTaskScheduleRowsWithSkipped(rows []dao.TaskSchedule, request CursorPageRequest, asOf time.Time, taskDone bool, skipped map[string]map[string]bool) ([]dao.TaskSchedule, error) {
	if skipped == nil {
		skipped = map[string]map[string]bool{}
	}
	byOccurrence := make(map[string]dao.TaskSchedule, len(rows))
	roots := make([]dao.TaskSchedule, 0)
	for _, row := range rows {
		key := row.SeriesID + "/" + row.OccurrenceDate
		if existing, ok := byOccurrence[key]; !ok || row.ID != row.SeriesID || existing.ID == existing.SeriesID {
			byOccurrence[key] = row
		}
		if row.ID == row.SeriesID {
			roots = append(roots, row)
		}
	}
	out := make([]dao.TaskSchedule, 0, len(rows))
	for _, root := range roots {
		requestedStart, err := listStartDate(request, asOf, root.Timezone)
		if err != nil {
			return nil, err
		}
		location, err := time.LoadLocation(root.Timezone)
		if err != nil {
			return nil, domain.ErrRecurrenceTimezoneInvalid
		}
		startDate := localToday(asOf, location)
		if requestedStart.After(startDate) {
			startDate = requestedStart
		}
		if request.Anchor != nil && request.Anchor.At != "" {
			if at, parseErr := time.Parse(time.RFC3339Nano, request.Anchor.At); parseErr == nil {
				local := at.In(location)
				anchorDate := tasktime.NormalizeCalendarDate(local)
				if anchorDate.After(startDate) {
					startDate = anchorDate
				}
			}
		}
		firstDate, err := parseOccurrenceDate(root.OccurrenceDate)
		if err != nil {
			return nil, fmt.Errorf("parse task schedule %s occurrence date: %w", root.ID, err)
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
		if state == repeatStateActive && root.IntervalWeeks > 0 && !root.Deleted && !taskDone {
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
			frequencies, err := taskFrequenciesFromDAO(root.Frequencies)
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
				start, valid := tasktime.ResolveWallTime(date, startLocal, 0, location)
				if !valid {
					continue
				}
				end, valid := tasktime.ResolveWallTime(date, endLocal, tasktime.CalendarDayOffset(startLocal, endLocal), location)
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
		if state == repeatStateActive && !taskDone && !firstDate.Before(localToday(asOf, location)) && requestedScheduleIncludes(request, requestedStart, root) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasTaskSchedule(out, root.ID, root.OccurrenceDate) {
			root.CursorStartAt = time.Unix(root.StartAt, 0).UTC().Format(time.RFC3339Nano)
			out = append(out, root)
		}
		for _, saved := range rows {
			if saved.SeriesID != root.ID || saved.ID == root.ID || saved.Deleted || skipped[root.ID][saved.OccurrenceDate] || (!saved.IsException && !saved.Completed) || !requestedScheduleIncludes(request, requestedStart, saved) || hasTaskSchedule(out, root.ID, saved.OccurrenceDate) {
				continue
			}
			saved.CursorStartAt = time.Unix(saved.StartAt, 0).UTC().Format(time.RFC3339Nano)
			out = append(out, scheduleWithRootRecurrence(saved, root))
		}
		if request.FromDate != "" && firstDate.Before(localToday(asOf, location)) && requestedScheduleIncludes(request, requestedStart, root) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasTaskSchedule(out, root.ID, root.OccurrenceDate) {
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

func scheduleRowOnOrAfter(row dao.TaskSchedule, fromDate time.Time) bool {
	local := time.Unix(row.StartAt, 0).In(recurrenceLocation(row.Timezone))
	date := tasktime.NormalizeCalendarDate(local)
	return !date.Before(fromDate)
}

func scheduleWithRootRecurrence(row, root dao.TaskSchedule) dao.TaskSchedule {
	row.IntervalWeeks, row.Frequencies = root.IntervalWeeks, root.Frequencies
	row.RepeatState, row.FrequencyAnchorDate = root.RepeatState, root.FrequencyAnchorDate
	return row
}

func recurrenceLocation(timezone string) *time.Location {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return location
}

func hasTaskSchedule(rows []dao.TaskSchedule, seriesID, occurrenceDate string) bool {
	for _, row := range rows {
		if row.SeriesID == seriesID && row.OccurrenceDate == occurrenceDate {
			return true
		}
	}
	return false
}

func afterTaskScheduleAnchor(row dao.TaskSchedule, anchor *CursorAnchor) bool {
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
