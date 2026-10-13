package domain

func Generate(in Input) (Plan, error) {
	if err := validate(in); err != nil {
		return Plan{}, err
	}
	completed := normalizedCompleted(in.CompletedFrames)
	occupied := initialOccupied(in, completed)
	work := make([]Frame, 0, len(in.Candidates))
	for _, candidate := range eligibleCandidates(in) {
		// Completed work remains visible, but earlier partial frames do not
		// reduce estimates or give a candidate priority over other work.
		if completedSource(completed, candidate.Kind, candidate.ID) {
			continue
		}
		windows := activeWindows(in)
		if !in.IgnoreAllocations {
			if candidate.ProjectID != "" {
				windows = allocationWindows(in.Allocations, candidate.ProjectID, in.Day, windows)
			} else {
				windows = projectlessWindows(in.Allocations, in.Day, windows)
			}
		}
		start, end, reservation, ok := findWholeFit(windows, occupied, candidate.EstimatedMinutes, in.GapMinutes)
		if !ok {
			continue
		}
		work = append(work, workFrame(candidate.Kind, candidate.ID, candidate.EstimatedMinutes, start, end))
		occupied = append(occupied, reservation)
	}
	frames := scheduleFrames(in.Schedules, completed)
	frames = append(frames, work...)
	return Plan{Frames: orderFrames(frames), PreservedCompletedFrames: completed}, nil
}

func completedSource(frames []Frame, kind FrameKind, sourceID string) bool {
	for _, frame := range frames {
		if frame.Kind == kind && frame.SourceID == sourceID {
			return true
		}
	}
	return false
}
