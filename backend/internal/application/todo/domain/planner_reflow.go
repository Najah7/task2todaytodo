package domain

import (
	"time"
)

func Reflow(in ReflowInput) (Plan, error) {
	if err := validate(in.Input); err != nil {
		return Plan{}, err
	}
	seenOrderFrames := make(map[string]bool)
	for _, item := range in.Order {
		if item.PlannedMinutes < 0 {
			return Plan{}, ErrInvalidReflow
		}
		if item.FrameID != "" {
			if seenOrderFrames[item.FrameID] {
				return Plan{}, ErrDuplicateID
			}
			seenOrderFrames[item.FrameID] = true
		}
	}
	if hasAllDay(in.Schedules) {
		return Plan{Frames: scheduleFrames(in.Schedules, nil), PreservedCompletedFrames: normalizedCompleted(in.CompletedFrames)}, nil
	}
	completed := normalizedCompleted(in.CompletedFrames)
	byCandidate := make(map[string]Candidate)
	for _, c := range reflowCandidates(in.Input) {
		byCandidate[sourceKey(c.Kind, c.ID)] = c
	}
	bySchedule := make(map[string]Schedule)
	for _, s := range in.Schedules {
		if !s.Hidden {
			bySchedule[s.ID] = s
		}
	}
	occupied := initialOccupied(in.Input, completed)
	var frames []Frame
	completedByID := make(map[string]Frame)
	for _, f := range completed {
		completedByID[f.ID] = f
	}
	seenFrames := make(map[string]bool)
	remaining := make(map[string]int, len(byCandidate))
	for id, c := range byCandidate {
		remaining[id] = remainingMinutes(c, completed)
	}
	lowerBound := in.Day.Start
	for _, item := range in.Order {
		switch item.Kind {
		case FrameSchedule:
			s, ok := bySchedule[item.ScheduleID]
			if ok {
				anchor := s.Interval.End.Add(time.Duration(in.GapMinutes) * time.Minute)
				if anchor.After(lowerBound) {
					lowerBound = anchor
				}
			}
		case FrameActionItem, FrameTask:
			if item.FrameID != "" {
				if f, ok := completedByID[item.FrameID]; ok {
					if !seenFrames[item.FrameID] {
						frames = append(frames, f)
						seenFrames[item.FrameID] = true
						if f.End != nil {
							anchor := f.End.Add(time.Duration(in.GapMinutes) * time.Minute)
							if anchor.After(lowerBound) {
								lowerBound = anchor
							}
						}
					}
					continue
				}
			}
			if item.FrameID != "" && seenFrames[item.FrameID] {
				continue
			}
			c, ok := byCandidate[sourceKey(item.Kind, item.SourceID)]
			if !ok {
				continue
			}
			key := sourceKey(c.Kind, c.ID)
			available := remaining[key]
			requested := item.PlannedMinutes
			if item.FrameID == "" && requested <= 0 {
				requested = available
			}
			if item.FrameID != "" && requested == 0 {
				return Plan{}, ErrInvalidReflow
			}
			if requested <= 0 || available <= 0 || requested > available {
				return Plan{}, ErrInvalidReflow
			}
			amount := min(requested, available)
			pieces, reserved := placeSplitFromKind(in.Day, lowerBound, occupied, amount, in.GapMinutes, c.ID, item.FrameID, c.Kind)
			remaining[key] -= amount
			if item.FrameID != "" {
				seenFrames[item.FrameID] = true
			}
			frames = append(frames, pieces...)
			occupied = append(occupied, reserved...)
		}
	}
	for _, slot := range freeIntervals(in.Day, initialOccupied(in.Input, completed)) {
		var indices []int
		for index, frame := range frames {
			if !frame.Kind.IsWork() || frame.Start == nil || frame.End == nil {
				continue
			}
			if !frame.Start.Before(slot.Start) && !frame.End.After(slot.End) {
				indices = append(indices, index)
			}
		}
		spreadFrames(frames, indices, slot, in.GapMinutes)
	}
	for id, f := range completedByID {
		if !seenFrames[id] {
			frames = append(frames, f)
		}
	}
	for _, s := range in.Schedules {
		if !s.Hidden {
			frames = append(frames, scheduleFrame(s))
		}
	}
	return Plan{Frames: orderFrames(frames), PreservedCompletedFrames: completed}, nil
}
