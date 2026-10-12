package domain

import (
	"sort"
	"time"
)

func Generate(in Input) (Plan, error) {
	if err := validate(in); err != nil {
		return Plan{}, err
	}
	if hasAllDay(in.Schedules) {
		return Plan{Frames: scheduleFrames(in.Schedules, nil), PreservedCompletedFrames: normalizedCompleted(in.CompletedFrames)}, nil
	}

	completed := normalizedCompleted(in.CompletedFrames)
	occupied := initialOccupied(in, completed)
	fixed := append([]Interval(nil), occupied...)

	ordered := eligibleCandidates(in)
	var work []Frame
	started := make(map[string]bool)
	for _, c := range ordered {
		remaining := remainingMinutes(c, completed)
		if remaining <= 0 {
			continue
		}
		if isStarted(c, completed) {
			started[sourceKey(c.Kind, c.ID)] = true
			pieces, reserved := placeSplitKind(in.Day, occupied, remaining, in.GapMinutes, c.ID, c.Kind)
			work = append(work, pieces...)
			occupied = append(occupied, reserved...)
		}
	}
	// Started work gets first claim on the day. A fit check for untouched work
	// sees each selected item as one indivisible contiguous block.
	selectedNormal := make(map[string]string)
	for _, c := range ordered {
		if isStarted(c, completed) {
			continue
		}
		remaining := remainingMinutes(c, completed)
		if remaining <= 0 {
			continue
		}
		group := c.ProjectID
		windows := allocationWindows(in.Allocations, c.ProjectID, in.Day)
		if c.ProjectID == "" {
			group = projectlessGroup
			windows = projectlessWindows(in.Allocations, in.Day)
		}
		start, end, reservation, ok := findWholeFit(windows, occupied, remaining, in.GapMinutes)
		if !ok {
			continue
		}
		work = append(work, workFrame(c.Kind, c.ID, remaining, start, end))
		occupied = append(occupied, reservation)
		selectedNormal[sourceKey(c.Kind, c.ID)] = group
	}
	work = distributeSlack(work, selectedNormal, started, in.Allocations, fixed, in.Day, in.GapMinutes)
	frames := scheduleFrames(in.Schedules, completed)
	frames = append(frames, work...)
	return Plan{Frames: orderFrames(frames), PreservedCompletedFrames: completed}, nil
}

func distributeSlack(frames []Frame, selected map[string]string, started map[string]bool, allocations []ProjectAllocation, fixed []Interval, day Interval, gap int) []Frame {
	windowsByProject := make(map[string][]Interval)
	var projects []string
	for _, project := range selected {
		if !containsString(projects, project) {
			projects = append(projects, project)
		}
	}
	sort.Strings(projects)
	for _, project := range projects {
		if _, ok := windowsByProject[project]; !ok {
			if project == projectlessGroup {
				windowsByProject[project] = projectlessWindows(allocations, day)
			} else {
				windowsByProject[project] = allocationWindows(allocations, project, day)
			}
		}
	}
	assigned := make(map[int]bool)
	for _, project := range projects {
		windows := windowsByProject[project]
		for _, window := range windows {
			var eligible []int
			for index, frame := range frames {
				if assigned[index] || !frame.Kind.IsWork() || frame.Start == nil || frame.End == nil {
					continue
				}
				key := sourceKey(frame.Kind, frame.SourceID)
				if (selected[key] == project || started[key]) &&
					!frame.Start.Before(window.Start) && !frame.End.After(window.End) {
					eligible = append(eligible, index)
				}
			}
			// Keep work from other groups, and started segments crossing this
			// allocation boundary, fixed while spreading this project's frames.
			obstacles := append([]Interval(nil), fixed...)
			for index, frame := range frames {
				if frame.Start == nil || frame.End == nil || containsIndex(eligible, index) {
					continue
				}
				obstacles = append(obstacles, paddedFrame(frame, gap))
			}
			for _, slot := range freeIntervals(window, obstacles) {
				var indices []int
				for _, index := range eligible {
					frame := frames[index]
					if !frame.Start.Before(slot.Start) && !frame.End.After(slot.End) {
						indices = append(indices, index)
					}
				}
				indices = appendAndAssign(assigned, indices)
				spreadFrames(frames, indices, slot, gap)
			}
		}
	}
	dayZones := freeIntervals(day, fixed)
	var reallocated []Frame
	remove := make(map[int]bool)
	for _, slot := range dayZones {
		var indices []int
		for index, frame := range frames {
			if assigned[index] || !started[sourceKey(frame.Kind, frame.SourceID)] || !frame.Kind.IsWork() || frame.Start == nil || frame.End == nil {
				continue
			}
			if !frame.Start.Before(slot.Start) && !frame.End.After(slot.End) {
				indices = append(indices, index)
			}
		}
		if len(indices) == 0 {
			continue
		}
		obstacles := append([]Interval(nil), fixed...)
		for index, frame := range frames {
			if frame.Start == nil || frame.End == nil || containsIndex(indices, index) {
				continue
			}
			obstacles = append(obstacles, paddedFrame(frame, gap))
		}
		for _, subslot := range freeIntervals(slot, obstacles) {
			var subindices []int
			for _, index := range indices {
				frame := frames[index]
				if !frame.Start.Before(subslot.Start) && !frame.End.After(subslot.End) {
					subindices = append(subindices, index)
				}
			}
			if len(subindices) == 0 {
				continue
			}
			var queue []workAmount
			for _, index := range subindices {
				remove[index] = true
				assigned[index] = true
				queue = append(queue, workAmount{kind: frames[index].Kind, sourceID: frames[index].SourceID, minutes: frames[index].PlannedMinutes})
			}
			placed, overflow := placeWorkAmounts(queue, []Interval{subslot}, gap)
			reallocated = append(reallocated, placed...)
			reallocated = append(reallocated, overflow...)
		}
	}
	kept := make([]Frame, 0, len(frames)+len(reallocated))
	for index, frame := range frames {
		if !remove[index] {
			kept = append(kept, frame)
		}
	}
	frames = append(kept, reallocated...)
	return frames
}

const projectlessGroup = "\x00projectless"

func paddedFrame(frame Frame, gap int) Interval {
	padding := time.Duration(gap) * time.Minute
	return Interval{frame.Start.Add(-padding), frame.End.Add(padding)}
}

func containsIndex(indices []int, target int) bool {
	for _, index := range indices {
		if index == target {
			return true
		}
	}
	return false
}

type workAmount struct {
	kind     FrameKind
	sourceID string
	minutes  int
}

func placeWorkAmounts(queue []workAmount, slots []Interval, gap int) ([]Frame, []Frame) {
	var placed []Frame
	remaining := append([]workAmount(nil), queue...)
	for _, slot := range slots {
		if len(remaining) == 0 {
			break
		}
		cursor := slot.Start
		var group []Frame
		for len(remaining) > 0 {
			available := int(slot.End.Sub(cursor).Minutes())
			if available <= 0 {
				break
			}
			current := remaining[0]
			use := min(current.minutes, available)
			end := cursor.Add(time.Duration(use) * time.Minute)
			group = append(group, workFrame(current.kind, current.sourceID, use, cursor, end))
			current.minutes -= use
			if current.minutes == 0 {
				remaining = remaining[1:]
			} else {
				remaining[0] = current
			}
			if len(remaining) == 0 || current.minutes > 0 {
				break
			}
			nextStart := end.Add(time.Duration(gap) * time.Minute)
			if !slot.End.After(nextStart) {
				break
			}
			cursor = nextStart
		}
		if len(group) > 1 {
			indices := make([]int, len(group))
			for i := range group {
				indices[i] = i
			}
			spreadFrames(group, indices, slot, gap)
		}
		placed = append(placed, group...)
	}
	var overflow []Frame
	for _, item := range remaining {
		overflow = append(overflow, Frame{Kind: item.kind, SourceID: item.sourceID, PlannedMinutes: item.minutes})
	}
	return placed, overflow
}

func appendAndAssign(assigned map[int]bool, indices []int) []int {
	for _, index := range indices {
		assigned[index] = true
	}
	return indices
}

func spreadFrames(frames []Frame, indices []int, slot Interval, gap int) {
	if len(indices) < 2 {
		return
	}
	sort.Slice(indices, func(i, j int) bool { return frames[indices[i]].Start.Before(*frames[indices[j]].Start) })
	workMinutes := 0
	for _, index := range indices {
		workMinutes += frames[index].PlannedMinutes
	}
	slack := int(slot.End.Sub(slot.Start).Minutes()) - workMinutes - gap*(len(indices)-1)
	if slack <= 0 {
		return
	}
	base, remainder := slack/(len(indices)-1), slack%(len(indices)-1)
	cursor := slot.Start
	for offset, index := range indices {
		end := cursor.Add(time.Duration(frames[index].PlannedMinutes) * time.Minute)
		startCopy, endCopy := cursor, end
		frames[index].Start, frames[index].End = &startCopy, &endCopy
		if offset < len(indices)-1 {
			extra := base
			if offset < remainder {
				extra++
			}
			cursor = end.Add(time.Duration(gap+extra) * time.Minute)
		}
	}
}

func containsString(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

// Reflow places work in the caller-supplied order. Schedule positions remain
// anchors, completed frames stay fixed, and unsatisfied work remains visible.
