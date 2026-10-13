package domain

import "sort"

func validate(in Input) error {
	if !in.Day.End.After(in.Day.Start) || in.Location == nil {
		return ErrInvalidInterval
	}
	if in.GapMinutes < 0 || in.GapMinutes > 1440 {
		return ErrInvalidGap
	}
	for _, interval := range in.ActiveTime {
		if !interval.End.After(interval.Start) {
			return ErrInvalidInterval
		}
	}
	for _, allocation := range in.Allocations {
		if allocation.ProjectID == "" || !allocation.Interval.End.After(allocation.Interval.Start) {
			return ErrInvalidInput
		}
	}
	scheduleIDs := make(map[string]bool, len(in.Schedules))
	for _, schedule := range in.Schedules {
		if schedule.ID == "" || !schedule.Interval.End.After(schedule.Interval.Start) {
			return ErrInvalidInput
		}
		if scheduleIDs[schedule.ID] {
			return ErrDuplicateID
		}
		scheduleIDs[schedule.ID] = true
	}
	candidateIDs := make(map[string]bool, len(in.Candidates))
	for _, candidate := range in.Candidates {
		if candidate.ID == "" || candidate.EstimatedMinutes < 1 || candidate.CompletedBeforeDayMinutes < 0 || candidate.PriorityWeight < 0 {
			return ErrInvalidCandidate
		}
		kind := candidate.Kind
		if !kind.IsWork() || candidateIDs[sourceKey(kind, candidate.ID)] {
			return ErrDuplicateID
		}
		candidateIDs[sourceKey(kind, candidate.ID)] = true
	}
	completedIDs := make(map[string]bool, len(in.CompletedFrames))
	for _, frame := range in.CompletedFrames {
		if !frame.Completed || frame.ID == "" || frame.Kind == "" || frame.PlannedMinutes < 1 {
			return ErrInvalidInput
		}
		if completedIDs[frame.ID] {
			return ErrDuplicateID
		}
		completedIDs[frame.ID] = true
		if frame.Start != nil && (frame.End == nil || !frame.End.After(*frame.Start)) {
			return ErrInvalidInterval
		}
		if frame.End != nil && frame.Start == nil {
			return ErrInvalidInterval
		}
	}
	return nil
}

func hasAllDay(schedules []Schedule) bool {
	for _, schedule := range schedules {
		if !schedule.Hidden && schedule.AllDay {
			return true
		}
	}
	return false
}

func normalizedCompleted(frames []Frame) []Frame {
	out := make([]Frame, 0, len(frames))
	for _, frame := range frames {
		if frame.Completed {
			frame.Position = len(out)
			out = append(out, frame)
		}
	}
	return out
}

func scheduleFrames(schedules []Schedule, completed []Frame) []Frame {
	out := make([]Frame, 0, len(schedules)+len(completed))
	for _, schedule := range schedules {
		if !schedule.Hidden {
			out = append(out, scheduleFrame(schedule))
		}
	}
	return append(out, completed...)
}

func scheduleFrame(schedule Schedule) Frame {
	start, end := schedule.Interval.Start, schedule.Interval.End
	return Frame{Kind: FrameSchedule, SourceID: schedule.ID, RuleID: schedule.RuleID, OccurrenceDate: schedule.OccurrenceDate, Start: &start, End: &end, Completed: schedule.Completed}
}

func eligibleCandidates(in Input) []Candidate {
	items := make([]Candidate, 0, len(in.Candidates))
	for _, candidate := range in.Candidates {
		if candidate.TaskStatus != "open" && candidate.TaskStatus != "in_progress" {
			continue
		}
		items = append(items, candidate)
	}
	return sortCandidates(items, in)
}

func sortCandidates(items []Candidate, in Input) []Candidate {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aScore, bScore := score(a, in.Today, in.Location), score(b, in.Today, in.Location)
		if aScore != bScore {
			return aScore > bScore
		}
		if a.DueAt != nil && b.DueAt != nil {
			aDue, bDue := localDateValue(*a.DueAt, in.Location), localDateValue(*b.DueAt, in.Location)
			if !aDue.Equal(bDue) {
				return aDue.Before(bDue)
			}
		}
		if (a.DueAt == nil) != (b.DueAt == nil) {
			return a.DueAt != nil
		}
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.ID < b.ID
	})
	return items
}
