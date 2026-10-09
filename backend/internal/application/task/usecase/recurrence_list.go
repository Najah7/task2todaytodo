package usecase

import (
	"fmt"
	"sort"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/shared/recurrence"
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

func expandActionItemRows(rows []dao.ActionItem, request CursorPageRequest, asOf time.Time, taskDone bool) ([]dao.ActionItem, error) {
	return expandActionItemRowsWithSkipped(rows, request, asOf, taskDone, nil)
}

func expandActionItemRowsWithSkipped(rows []dao.ActionItem, request CursorPageRequest, asOf time.Time, taskDone bool, skipped map[string]map[string]bool) ([]dao.ActionItem, error) {
	return expandActionItemRowsWithProjectState(rows, request, asOf, taskDone, false, skipped)
}

func expandActionItemRowsWithProjectState(rows []dao.ActionItem, request CursorPageRequest, asOf time.Time, taskDone, projectDone bool, skipped map[string]map[string]bool) ([]dao.ActionItem, error) {
	if skipped == nil {
		skipped = map[string]map[string]bool{}
	}
	byOccurrence := make(map[string]dao.ActionItem, len(rows))
	roots := make([]dao.ActionItem, 0)
	for _, row := range rows {
		key := row.SeriesID + "/" + row.OccurrenceDate
		if existing, ok := byOccurrence[key]; !ok || row.ID != row.SeriesID || existing.ID == existing.SeriesID {
			byOccurrence[key] = row
		}
		if row.ID == row.SeriesID {
			roots = append(roots, row)
		}
	}
	out := make([]dao.ActionItem, 0, len(rows))
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
		if anchorDate := actionItemAnchorDate(request.Anchor); anchorDate != "" {
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
			return nil, fmt.Errorf("parse action item %s occurrence date: %w", root.ID, err)
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
					out = append(out, actionItemWithRootRecurrence(saved, root))
				}
			} else if !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && visible {
				out = append(out, root)
			}
			continue
		}
		if state == repeatStateActive && root.IntervalWeeks > 0 && !root.Deleted && !taskDone && !projectDone {
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
						out = append(out, actionItemWithRootRecurrence(saved, root))
					}
					continue
				}
				item := root
				item.ID = VirtualOccurrenceID
				positionCount, err := recurrence.CountRecurrenceDatesFromAnchor(anchor, date, root.IntervalWeeks, frequencies, root.Timezone)
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
		if state == repeatStateActive && !taskDone && !firstDate.Before(localToday(asOf, location)) && requestedDateIncludes(request, requestedStart, firstDate) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasActionItem(out, root.ID, root.OccurrenceDate) {
			out = append(out, root)
		}
		if state == repeatStateActive && projectDone && !firstDate.Before(localToday(asOf, location)) && requestedDateIncludes(request, requestedStart, firstDate) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasActionItem(out, root.ID, root.OccurrenceDate) {
			out = append(out, root)
		}
		if request.FromDate != "" && firstDate.Before(localToday(asOf, location)) && requestedDateIncludes(request, requestedStart, firstDate) && !root.Deleted && !skipped[root.ID][root.OccurrenceDate] && !hasActionItem(out, root.ID, root.OccurrenceDate) {
			out = append(out, root)
		}
		// Keep saved history addressable even when current rule no longer contains its date.
		for _, saved := range rows {
			if saved.SeriesID != root.ID || saved.ID == root.ID || saved.Deleted || skipped[root.ID][saved.OccurrenceDate] || (!saved.IsException && !saved.Completed) || !requestedDateIncludes(request, requestedStart, mustParseDate(saved.OccurrenceDate)) {
				continue
			}
			if !hasActionItem(out, root.ID, saved.OccurrenceDate) {
				out = append(out, actionItemWithRootRecurrence(saved, root))
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

func actionItemWithRootRecurrence(row, root dao.ActionItem) dao.ActionItem {
	row.IntervalWeeks, row.Frequencies = root.IntervalWeeks, root.Frequencies
	row.RepeatState, row.FrequencyAnchorDate = root.RepeatState, root.FrequencyAnchorDate
	return row
}

func generateTodoListDates(anchor, from time.Time, interval int, frequencies domain.TaskFrequencies, root dao.ActionItem, request CursorPageRequest, overrides map[string]dao.ActionItem, skipped map[string]bool) ([]recurrence.RecurrenceDate, error) {
	limit := listVisibleTarget(request)
	for {
		dates, err := recurrence.GenerateRecurrenceDatesFromAnchorLimit(anchor, from, interval, frequencies, root.Timezone, limit)
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

func actionItemAnchorDate(anchor *CursorAnchor) string {
	if anchor == nil {
		return ""
	}
	if anchor.OccurrenceDate != "" {
		return anchor.OccurrenceDate
	}
	return anchor.Date
}

func hasActionItem(rows []dao.ActionItem, seriesID, occurrenceDate string) bool {
	for _, row := range rows {
		if row.SeriesID == seriesID && row.OccurrenceDate == occurrenceDate {
			return true
		}
	}
	return false
}

func afterActionItemAnchor(row dao.ActionItem, anchor *CursorAnchor) bool {
	if anchor == nil {
		return true
	}
	date := actionItemAnchorDate(anchor)
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
