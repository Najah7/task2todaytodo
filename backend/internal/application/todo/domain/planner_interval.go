package domain

import (
	"sort"
	"time"
)

func allocationWindows(allocations []ProjectAllocation, projectID string, day Interval) []Interval {
	var out []Interval
	for _, a := range allocations {
		if a.ProjectID != projectID {
			continue
		}
		if clipped, ok := intersect(a.Interval, day); ok {
			out = append(out, clipped)
		}
	}
	return mergeIntervals(out)
}

func projectlessWindows(allocations []ProjectAllocation, day Interval) []Interval {
	blocked := make([]Interval, 0, len(allocations))
	for _, allocation := range allocations {
		if clipped, ok := intersect(allocation.Interval, day); ok {
			blocked = append(blocked, clipped)
		}
	}
	return freeIntervals(day, blocked)
}

func findWholeFit(windows, occupied []Interval, minutes, gap int) (time.Time, time.Time, Interval, bool) {
	for _, window := range windows {
		free := freeIntervals(window, occupied)
		for _, slot := range free {
			if int(slot.End.Sub(slot.Start).Minutes()) >= minutes {
				start := slot.Start
				end := start.Add(time.Duration(minutes) * time.Minute)
				padding := time.Duration(gap) * time.Minute
				return start, end, Interval{start.Add(-padding), end.Add(padding)}, true
			}
		}
	}
	return time.Time{}, time.Time{}, Interval{}, false
}

func placeSplitKind(day Interval, occupied []Interval, minutes, gap int, sourceID string, kind FrameKind) ([]Frame, []Interval) {
	return placeSplitFromKind(day, day.Start, occupied, minutes, gap, sourceID, "", kind)
}

func placeSplitFromKind(day Interval, lowerBound time.Time, occupied []Interval, minutes, gap int, sourceID, originalFrameID string, kind FrameKind) ([]Frame, []Interval) {
	window := day
	if lowerBound.After(window.Start) {
		window.Start = lowerBound
	}
	if !window.End.After(window.Start) {
		return []Frame{{Kind: kind, SourceID: sourceID, PlannedMinutes: minutes, OriginalFrameID: originalFrameID}}, nil
	}
	free := freeIntervals(window, occupied)
	var frames []Frame
	var reservations []Interval
	left := minutes
	for _, slot := range free {
		if left == 0 {
			break
		}
		available := int(slot.End.Sub(slot.Start).Minutes())
		if available <= 0 {
			continue
		}
		use := min(left, available)
		start, end := slot.Start, slot.Start.Add(time.Duration(use)*time.Minute)
		frame := workFrame(kind, sourceID, use, start, end)
		if originalFrameID != "" && len(frames) == 0 {
			frame.OriginalFrameID = originalFrameID
		}
		frames = append(frames, frame)
		padding := time.Duration(gap) * time.Minute
		reservations = append(reservations, Interval{start.Add(-padding), end.Add(padding)})
		left -= use
	}
	if left > 0 {
		frame := Frame{Kind: kind, SourceID: sourceID, PlannedMinutes: left}
		if len(frames) == 0 {
			frame.OriginalFrameID = originalFrameID
		}
		frames = append(frames, frame)
	}
	return frames, reservations
}

func freeIntervals(window Interval, occupied []Interval) []Interval {
	blocks := make([]Interval, 0, len(occupied))
	for _, block := range occupied {
		if clipped, ok := intersect(block, window); ok {
			blocks = append(blocks, clipped)
		}
	}
	blocks = mergeIntervals(blocks)
	var result []Interval
	cursor := window.Start
	for _, block := range blocks {
		if block.Start.After(cursor) {
			result = append(result, Interval{cursor, block.Start})
		}
		if block.End.After(cursor) {
			cursor = block.End
		}
	}
	if window.End.After(cursor) {
		result = append(result, Interval{cursor, window.End})
	}
	return result
}

func initialOccupied(in Input, completed []Frame) []Interval {
	occupied := append([]Interval(nil), in.Sleep...)
	padding := time.Duration(in.GapMinutes) * time.Minute
	for _, s := range in.Schedules {
		if !s.Hidden {
			occupied = append(occupied, Interval{s.Interval.Start.Add(-padding), s.Interval.End.Add(padding)})
		}
	}
	for _, f := range completed {
		if f.Start != nil && f.End != nil {
			occupied = append(occupied, Interval{f.Start.Add(-padding), f.End.Add(padding)})
		}
	}
	return occupied
}

func intersect(a, b Interval) (Interval, bool) {
	start, end := a.Start, a.End
	if b.Start.After(start) {
		start = b.Start
	}
	if b.End.Before(end) {
		end = b.End
	}
	return Interval{start, end}, end.After(start)
}

func mergeIntervals(intervals []Interval) []Interval {
	if len(intervals) == 0 {
		return nil
	}
	items := append([]Interval(nil), intervals...)
	sort.Slice(items, func(i, j int) bool { return items[i].Start.Before(items[j].Start) })
	out := []Interval{items[0]}
	for _, current := range items[1:] {
		last := &out[len(out)-1]
		if !current.Start.After(last.End) {
			if current.End.After(last.End) {
				last.End = current.End
			}
		} else {
			out = append(out, current)
		}
	}
	return out
}

func workFrame(kind FrameKind, sourceID string, minutes int, start, end time.Time) Frame {
	startCopy, endCopy := start, end
	return Frame{Kind: kind, SourceID: sourceID, PlannedMinutes: minutes, Start: &startCopy, End: &endCopy}
}

func remainingMinutes(c Candidate, completed []Frame) int {
	used := c.CompletedBeforeDayMinutes
	for _, f := range completed {
		if f.Kind == c.Kind && f.SourceID == c.ID {
			used += f.PlannedMinutes
		}
	}
	return c.EstimatedMinutes - used
}

func orderFrames(frames []Frame) []Frame {
	sort.SliceStable(frames, func(i, j int) bool {
		if frames[i].Start == nil {
			return false
		}
		if frames[j].Start == nil {
			return true
		}
		if !frames[i].Start.Equal(*frames[j].Start) {
			return frames[i].Start.Before(*frames[j].Start)
		}
		if frames[i].Kind != frames[j].Kind {
			return frames[i].Kind == FrameSchedule
		}
		if frames[i].SourceID != frames[j].SourceID {
			return frames[i].SourceID < frames[j].SourceID
		}
		return frames[i].Position < frames[j].Position
	})
	for i := range frames {
		frames[i].Position = i
	}
	return frames
}
