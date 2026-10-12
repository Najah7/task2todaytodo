package domain

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var utc = time.UTC

func at(hour, minute int) time.Time { return time.Date(2026, 10, 12, hour, minute, 0, 0, utc) }

func baseInput() Input {
	return Input{Day: Interval{at(9, 0), at(17, 0)}, GapMinutes: 5, Today: at(0, 0), Location: utc}
}

func candidate(id, project string, estimate, priority int) Candidate {
	return Candidate{Kind: FrameActionItem, ID: id, ProjectID: project, EstimatedMinutes: estimate, PriorityWeight: priority, TaskStatus: "open", CreatedAt: at(8, 0)}
}

func TestStartedTaskAndProjectlessWorkShareSlackInAllowedWindow(t *testing.T) {
	in := baseInput()
	in.Day = Interval{at(7, 0), at(9, 0)}
	in.Candidates = []Candidate{
		{Kind: FrameTask, ID: "started", EstimatedMinutes: 30, HasCompletedFrame: true, TaskStatus: "in_progress"},
		candidate("new-action", "", 30, 1),
	}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	started := firstFrame(plan.Frames, "started")
	action := firstFrame(plan.Frames, "new-action")
	if started.Start == nil || !started.Start.Equal(at(7, 0)) || !started.End.Equal(at(7, 30)) {
		t.Fatalf("started=%+v", started)
	}
	if action.Start == nil || !action.Start.Equal(at(8, 30)) || !action.End.Equal(at(9, 0)) {
		t.Fatalf("projectless=%+v, want equal slack spread at 08:30-09:00", action)
	}
	assertActionsSeparated(t, plan.Frames, in.GapMinutes)
}

func TestScoreUsesUrgencyBoundaries(t *testing.T) {
	in := baseInput()
	for _, tc := range []struct {
		days int
		want int
	}{{-1, 200}, {0, 200}, {1, 100}, {7, 100}, {8, 50}, {14, 50}, {15, 25}, {30, 25}, {31, 10}, {60, 10}} {
		due := at(0, 0).AddDate(0, 0, tc.days)
		got := score(Candidate{PriorityWeight: 3, DueAt: &due}, in.Today, in.Location)
		if got != 3*tc.want {
			t.Fatalf("days %d: score=%d, want %d", tc.days, got, 3*tc.want)
		}
	}
	if got := score(Candidate{PriorityWeight: 10}, in.Today, in.Location); got != 10 {
		t.Fatalf("no due score=%d, want 10", got)
	}
}

func TestGenerateSortsPriorityAndTieBreaks(t *testing.T) {
	in := baseInput()
	dueLater, dueEarlier := at(0, 0).AddDate(0, 0, 8), at(0, 0).AddDate(0, 0, 1)
	a := candidate("a", "p", 20, 50)
	a.DueAt = &dueLater
	b := candidate("b", "p", 20, 25)
	b.DueAt = &dueEarlier // both score 2500, earlier due wins
	c := candidate("c", "p", 20, 50)
	c.DueAt = &dueLater
	c.CreatedAt = at(7, 0)
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: Interval{at(9, 0), at(12, 0)}}}
	in.Candidates = []Candidate{a, b, c}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, frame := range plan.Frames {
		if frame.Kind == FrameActionItem && frame.Start != nil {
			got = append(got, frame.SourceID)
		}
	}
	if len(got) != 3 || got[0] != "b" || got[1] != "c" || got[2] != "a" {
		t.Fatalf("ordered frames=%v, want [b c a]", got)
	}
}

func TestGenerateSkipsNonFittingProjectWorkAndUsesProjectlessComplement(t *testing.T) {
	in := baseInput()
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: Interval{at(9, 0), at(10, 0)}}, {ProjectID: "p", Interval: Interval{at(11, 0), at(12, 0)}}}
	large := candidate("large", "p", 75, 100)
	small := candidate("small", "p", 30, 50)
	noProject := candidate("outside", "", 10, 100)
	in.Candidates = []Candidate{large, small, noProject}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	if countSource(plan.Frames, "large") != 0 || countSource(plan.Frames, "outside") != 1 {
		t.Fatalf("unexpected non-fitting/unallocated frame: %+v", plan.Frames)
	}
	if countSource(plan.Frames, "small") != 1 {
		t.Fatalf("smaller candidate should fit: %+v", plan.Frames)
	}
	outside := firstFrame(plan.Frames, "outside")
	if outside.Start == nil || !outside.Start.Equal(at(10, 0)) || !outside.End.Equal(at(10, 10)) {
		t.Fatalf("projectless frame=%+v, want allocation complement 10:00-10:10", outside)
	}
}

func TestTaskAndActionItemShareIDWithoutSourceCollision(t *testing.T) {
	in := baseInput()
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: Interval{at(9, 0), at(11, 0)}}}
	in.Candidates = []Candidate{
		{Kind: FrameTask, ID: "same", ProjectID: "p", EstimatedMinutes: 30, TaskStatus: "open"},
		{Kind: FrameActionItem, ID: "same", ProjectID: "p", EstimatedMinutes: 30, TaskStatus: "open"},
	}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	got := map[FrameKind]int{}
	for _, frame := range plan.Frames {
		if frame.SourceID == "same" {
			got[frame.Kind] += frame.PlannedMinutes
		}
	}
	if got[FrameTask] != 30 || got[FrameActionItem] != 30 {
		t.Fatalf("source totals=%v, want 30 minutes for Task and ActionItem independently", got)
	}
}

func TestGenerateRetainsStartedPendingWorkAndExcludesUnstartedAndDone(t *testing.T) {
	in := baseInput()
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: Interval{at(9, 0), at(12, 0)}}}
	pending := candidate("pending", "p", 60, 50)
	pending.CompletedBeforeDayMinutes = 30
	pending.HasCompletedFrame = true
	pending.TaskStatus = "pending"
	waiting := candidate("waiting", "p", 30, 25)
	waiting.HasCompletedFrame = true
	waiting.TaskStatus = "waiting_on_others"
	pendingNew := candidate("pending-new", "p", 20, 15)
	pendingNew.TaskStatus = "pending"
	waitingNew := candidate("waiting-new", "p", 20, 10)
	waitingNew.TaskStatus = "waiting_on_others"
	done := candidate("done", "p", 30, 20)
	done.CompletedBeforeDayMinutes = 10
	done.HasCompletedFrame = true
	done.TaskStatus = "done"
	in.Candidates = []Candidate{pending, waiting, pendingNew, waitingNew, done}
	automatic, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, sourceID := range []string{"pending", "waiting"} {
		if countSource(automatic.Frames, sourceID) == 0 {
			t.Fatalf("automatic plan omitted started %s source: %+v", sourceID, automatic.Frames)
		}
	}
	for _, sourceID := range []string{"pending-new", "waiting-new", "done"} {
		if countSource(automatic.Frames, sourceID) != 0 {
			t.Fatalf("automatic plan included unstarted or done %s source: %+v", sourceID, automatic.Frames)
		}
	}
	if got := totalSourceMinutes(automatic.Frames, "pending"); got != 30 {
		t.Fatalf("started pending remainder=%d, want 30 minutes", got)
	}
	if got := totalSourceMinutes(automatic.Frames, "waiting"); got != 30 {
		t.Fatalf("started waiting remainder=%d, want 30 minutes", got)
	}
	manual, err := Reflow(ReflowInput{Input: in, Order: []OrderedItem{{Kind: FrameActionItem, SourceID: "pending"}}})
	if err != nil {
		t.Fatal(err)
	}
	if countSource(manual.Frames, "pending") != 1 {
		t.Fatalf("manual reflow omitted pending task: %+v", manual.Frames)
	}
}

func totalSourceMinutes(frames []Frame, sourceID string) int {
	total := 0
	for _, frame := range frames {
		if frame.SourceID == sourceID {
			total += frame.PlannedMinutes
		}
	}
	return total
}

func TestGenerateDistributesRemainingAllocationTimeAcrossActionGaps(t *testing.T) {
	in := baseInput()
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: Interval{at(9, 0), at(10, 30)}}}
	in.Candidates = []Candidate{candidate("a", "p", 30, 50), candidate("b", "p", 30, 25)}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	var starts []time.Time
	for _, f := range plan.Frames {
		if f.Kind == FrameActionItem && f.Start != nil {
			starts = append(starts, *f.Start)
		}
	}
	if len(starts) != 2 || !starts[0].Equal(at(9, 0)) || !starts[1].Equal(at(10, 0)) {
		t.Fatalf("starts=%v, want 09:00 and 10:00", starts)
	}
}

func TestGenerateStartedSlackDistributionAvoidsNormalAllocationFrames(t *testing.T) {
	in := baseInput()
	in.Candidates = []Candidate{
		startedCandidate("a", 30), startedCandidate("b", 30), startedCandidate("c", 30),
		candidate("normal", "p", 60, 50),
	}
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: Interval{at(12, 0), at(13, 0)}}}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	assertActionsSeparated(t, plan.Frames, in.GapMinutes)
	var normal Frame
	for _, f := range plan.Frames {
		if f.SourceID == "normal" {
			normal = f
		}
	}
	if normal.Start == nil || !normal.Start.Equal(at(12, 0)) || !normal.End.Equal(at(13, 0)) {
		t.Fatalf("normal allocation moved or got overlapped: %+v", normal)
	}
}

func TestGenerateAllocationSlackTreatsCrossingStartedFrameAsObstacle(t *testing.T) {
	in := baseInput()
	in.Candidates = []Candidate{
		startedCandidate("started", 90),
		candidate("normal-a", "p", 30, 50),
		candidate("normal-b", "p", 30, 25),
		candidate("other-project", "q", 30, 10),
	}
	in.Sleep = []Interval{{at(15, 0), at(16, 0)}}
	in.Schedules = []Schedule{{ID: "schedule", Interval: Interval{at(13, 0), at(13, 30)}}}
	in.Allocations = []ProjectAllocation{
		{ProjectID: "p", Interval: Interval{at(10, 0), at(12, 0)}},
		{ProjectID: "q", Interval: Interval{at(12, 0), at(13, 0)}},
	}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	assertActionsSeparated(t, plan.Frames, in.GapMinutes)
	frames := make(map[string]Frame)
	for _, frame := range plan.Frames {
		if frame.Kind == FrameActionItem && frame.Start != nil {
			frames[frame.SourceID] = frame
		}
	}
	if got := frames["started"]; got.Start == nil || !got.Start.Equal(at(9, 0)) || !got.End.Equal(at(10, 30)) {
		t.Fatalf("started crossing allocation moved or collided: %+v", got)
	}
	for _, id := range []string{"normal-a", "normal-b"} {
		got := frames[id]
		if got.Start == nil || got.Start.Before(at(10, 35)) || got.End.After(at(12, 0)) {
			t.Fatalf("normal frame escaped allocation or crossed started frame: %+v", got)
		}
	}
}

func TestStartedWorkOverridesProjectAndSplitsAroundSleepAndSchedule(t *testing.T) {
	in := baseInput()
	in.GapMinutes = 5
	in.Sleep = []Interval{{at(10, 0), at(11, 0)}}
	in.Schedules = []Schedule{{ID: "s", Interval: Interval{at(12, 0), at(13, 0)}}}
	started := candidate("started", "", 240, 0)
	started.HasCompletedFrame = true
	in.Candidates = []Candidate{started}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	frames := framesFor(plan.Frames, "started")
	if len(frames) < 2 {
		t.Fatalf("expected split around blocked intervals, got %+v", frames)
	}
	var total int
	for _, frame := range frames {
		total += frame.PlannedMinutes
		if frame.Start == nil || frame.End == nil {
			t.Fatalf("unexpected overflow for 240 minutes: %+v", frame)
		}
		if overlaps(*frame.Start, *frame.End, at(10, 0), at(11, 0)) || overlaps(*frame.Start, *frame.End, at(11, 55), at(13, 5)) {
			t.Fatalf("frame overlaps sleep/schedule gap: %+v", frame)
		}
	}
	if total != 240 {
		t.Fatalf("planned total=%d, want 240", total)
	}
}

func TestGenerateKeepsStartedOverflowAsUnscheduledPositiveWork(t *testing.T) {
	in := baseInput()
	in.Day = Interval{at(9, 0), at(10, 0)}
	c := candidate("started", "", 90, 0)
	c.HasCompletedFrame = true
	in.Candidates = []Candidate{c}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	frames := framesFor(plan.Frames, "started")
	if len(frames) != 2 || frames[0].PlannedMinutes != 60 || frames[1].PlannedMinutes != 30 || frames[1].Start != nil || frames[1].End != nil {
		t.Fatalf("frames=%+v", frames)
	}
}

func TestGapAndCompletedFramesAreFixed(t *testing.T) {
	in := baseInput()
	doneStart, doneEnd := at(9, 0), at(9, 30)
	in.CompletedFrames = []Frame{{ID: "done", Kind: FrameActionItem, SourceID: "a", PlannedMinutes: 30, Start: &doneStart, End: &doneEnd, Completed: true}}
	c := candidate("b", "p", 30, 1)
	in.Candidates = []Candidate{c}
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: Interval{at(9, 0), at(11, 0)}}}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.PreservedCompletedFrames) != 1 || plan.PreservedCompletedFrames[0].ID != "done" {
		t.Fatalf("completed frame not preserved: %+v", plan.PreservedCompletedFrames)
	}
	if !plan.PreservedCompletedFrames[0].Start.Equal(doneStart) || !plan.PreservedCompletedFrames[0].End.Equal(doneEnd) {
		t.Fatalf("completed time moved: %+v", plan.PreservedCompletedFrames[0])
	}
	for _, f := range plan.Frames {
		if f.SourceID == "b" && f.Start != nil && f.Start.Before(at(9, 35)) {
			t.Fatalf("minimum gap violated: %+v", f)
		}
	}
}

func TestAllDayScheduleSuppressesWorkAndKeepsCompletedHistorySeparate(t *testing.T) {
	in := baseInput()
	in.Schedules = []Schedule{{ID: "all", Interval: in.Day, AllDay: true}}
	in.CompletedFrames = []Frame{{ID: "history", Kind: FrameActionItem, SourceID: "a", PlannedMinutes: 10, Completed: true}}
	in.Candidates = []Candidate{candidate("a", "p", 20, 100)}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Frames) != 1 || plan.Frames[0].Kind != FrameSchedule {
		t.Fatalf("all-day visible frames=%+v", plan.Frames)
	}
	if len(plan.PreservedCompletedFrames) != 1 || plan.PreservedCompletedFrames[0].ID != "history" {
		t.Fatalf("history was not preserved: %+v", plan.PreservedCompletedFrames)
	}
}

func TestReflowScheduleAnchorAndAllocationBypass(t *testing.T) {
	in := baseInput()
	in.Allocations = nil
	in.Schedules = []Schedule{{ID: "s", Interval: Interval{at(12, 0), at(13, 0)}}}
	in.Candidates = []Candidate{candidate("a", "", 90, 1), candidate("b", "", 30, 1)}
	plan, err := Reflow(ReflowInput{Input: in, Order: []OrderedItem{{Kind: FrameSchedule, ScheduleID: "s"}, {Kind: FrameActionItem, SourceID: "a"}, {Kind: FrameActionItem, SourceID: "b"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range plan.Frames {
		if f.Kind == FrameActionItem && f.SourceID == "a" && f.Start != nil && f.Start.Before(at(13, 5)) {
			t.Fatalf("post-schedule work moved before anchor: %+v", f)
		}
	}
	if countSource(plan.Frames, "a") == 0 || countSource(plan.Frames, "b") == 0 {
		t.Fatalf("manual items missing: %+v", plan.Frames)
	}
}

func TestReflowPreservesFrameUnitsAndOriginalIDOnlyOnFirstSplit(t *testing.T) {
	in := baseInput()
	in.Day = Interval{at(9, 0), at(10, 0)}
	in.Schedules = []Schedule{{ID: "s", Interval: Interval{at(9, 30), at(9, 40)}}}
	c := candidate("a", "", 30, 1)
	c.HasCompletedFrame = true
	in.Candidates = []Candidate{c}
	plan, err := Reflow(ReflowInput{Input: in, Order: []OrderedItem{{Kind: FrameActionItem, FrameID: "old-a", SourceID: "a", PlannedMinutes: 30}}})
	if err != nil {
		t.Fatal(err)
	}
	frames := framesFor(plan.Frames, "a")
	if len(frames) != 2 {
		t.Fatalf("expected split at schedule, got %+v", frames)
	}
	if frames[0].OriginalFrameID != "old-a" || frames[1].OriginalFrameID != "" {
		t.Fatalf("original ID must map to first segment only: %+v", frames)
	}
}

func TestReflowKeepsRepeatedSourceFramesInRequestedOrder(t *testing.T) {
	in := baseInput()
	in.Schedules = []Schedule{{ID: "s", Interval: Interval{at(10, 0), at(10, 15)}}}
	c := candidate("a", "", 30, 1)
	c.HasCompletedFrame = true
	in.Candidates = []Candidate{c, candidate("b", "", 15, 1)}
	plan, err := Reflow(ReflowInput{Input: in, Order: []OrderedItem{
		{Kind: FrameActionItem, FrameID: "a-first", SourceID: "a", PlannedMinutes: 15},
		{Kind: FrameSchedule, ScheduleID: "s"},
		{Kind: FrameActionItem, FrameID: "a-second", SourceID: "a", PlannedMinutes: 15},
		{Kind: FrameActionItem, FrameID: "b-one", SourceID: "b", PlannedMinutes: 15},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var bFrame Frame
	var aCount, bCount int
	for _, f := range plan.Frames {
		if f.SourceID == "a" && f.Kind == FrameActionItem {
			aCount++
		}
		if f.SourceID == "b" && f.Kind == FrameActionItem {
			bCount++
			bFrame = f
		}
	}
	if aCount != 2 || bCount != 1 {
		t.Fatalf("frame counts A=%d B=%d: %+v", aCount, bCount, plan.Frames)
	}
	var aAfterSchedule time.Time
	for _, f := range plan.Frames {
		if f.OriginalFrameID == "a-second" && f.Start != nil {
			aAfterSchedule = *f.Start
		}
	}
	if aAfterSchedule.Before(at(10, 20)) || bFrame.Start == nil || !bFrame.Start.After(aAfterSchedule) {
		t.Fatalf("requested frame order lost: %+v", plan.Frames)
	}
}

func TestReflowCompletedFrameActsAsAnchor(t *testing.T) {
	in := baseInput()
	start, end := at(10, 0), at(10, 30)
	in.CompletedFrames = []Frame{{ID: "done", Kind: FrameActionItem, SourceID: "a", PlannedMinutes: 30, Start: &start, End: &end, Completed: true}}
	c := candidate("a", "", 60, 1)
	c.HasCompletedFrame = true
	in.Candidates = []Candidate{c}
	plan, err := Reflow(ReflowInput{Input: in, Order: []OrderedItem{{Kind: FrameActionItem, FrameID: "done", SourceID: "a", PlannedMinutes: 30}, {Kind: FrameActionItem, FrameID: "todo", SourceID: "a", PlannedMinutes: 30}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range plan.Frames {
		if f.OriginalFrameID == "todo" && f.Start != nil && f.Start.Before(at(10, 35)) {
			t.Fatalf("work moved before completed anchor: %+v", f)
		}
	}
}

func TestGenerateHiddenScheduleReleasesTimeAndSupportsZeroGap(t *testing.T) {
	in := baseInput()
	in.GapMinutes = 0
	in.Allocations = []ProjectAllocation{{ProjectID: "p1", Interval: Interval{at(9, 0), at(10, 0)}}, {ProjectID: "p2", Interval: Interval{at(10, 0), at(11, 0)}}}
	in.Schedules = []Schedule{{ID: "hidden", Interval: Interval{at(9, 0), at(10, 0)}, Hidden: true}}
	in.Candidates = []Candidate{candidate("one", "p1", 60, 50), candidate("two", "p2", 60, 25)}
	plan, err := Generate(in)
	if err != nil {
		t.Fatal(err)
	}
	if countSource(plan.Frames, "hidden") != 0 {
		t.Fatalf("hidden schedule should not render: %+v", plan.Frames)
	}
	var one, two Frame
	for _, f := range plan.Frames {
		if f.SourceID == "one" {
			one = f
		}
		if f.SourceID == "two" {
			two = f
		}
	}
	if one.Start == nil || two.Start == nil || !one.Start.Equal(at(9, 0)) || !two.Start.Equal(at(10, 0)) {
		t.Fatalf("project windows/zero gap wrong: %+v", plan.Frames)
	}
}

func TestResolvedDayIntervalsCanSpanDSTShortAndLongDays(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		date  time.Time
		hours int
	}{{time.Date(2026, 3, 8, 0, 0, 0, 0, location), 23}, {time.Date(2026, 11, 1, 0, 0, 0, 0, location), 25}} {
		day := Interval{Start: tc.date, End: tc.date.AddDate(0, 0, 1)}
		if got := int(day.End.Sub(day.Start).Hours()); got != tc.hours {
			t.Fatalf("day length=%d, want %d", got, tc.hours)
		}
		in := Input{Day: day, Location: location, Today: tc.date, GapMinutes: 0}
		plan, err := Generate(in)
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Frames) != 0 {
			t.Fatalf("empty resolved day produced frames: %+v", plan.Frames)
		}
	}
}

func TestGeneratedPlanInvariantsAcrossFixedSeedInputs(t *testing.T) {
	rng := rand.New(rand.NewSource(20261012))
	for sample := 0; sample < 30; sample++ {
		in := Input{
			Day:   Interval{at(9, 0), at(17, 0)},
			Sleep: []Interval{{at(12, 0), at(12, 35)}, {at(15, 30), at(16, 0)}},
			Schedules: []Schedule{
				{ID: "s1", Interval: Interval{at(10, 0), at(10, 40)}},
				{ID: "s2", Interval: Interval{at(13, 0), at(14, 0)}},
				{ID: "s3", Interval: Interval{at(13, 50), at(14, 30)}},
			},
			Allocations: []ProjectAllocation{
				{ProjectID: "p1", Interval: Interval{at(9, 0), at(12, 0)}},
				{ProjectID: "p1", Interval: Interval{at(14, 30), at(17, 0)}},
				{ProjectID: "p2", Interval: Interval{at(9, 30), at(11, 30)}},
			},
			GapMinutes: 5, Today: at(0, 0), Location: utc,
		}
		for i := 0; i < 24; i++ {
			id := fmt.Sprintf("c-%02d", i)
			project := []string{"p1", "p2", ""}[rng.Intn(3)]
			c := candidate(id, project, 15+rng.Intn(90), []int{0, 10, 25, 50, 100}[rng.Intn(5)])
			c.CreatedAt = at(7, 0).Add(time.Duration(rng.Intn(300)) * time.Minute)
			if i%4 == 0 {
				c = startedCandidate(id, c.EstimatedMinutes)
				c.ProjectID = project
			}
			in.Candidates = append(in.Candidates, c)
		}
		plan, err := Generate(in)
		if err != nil {
			t.Fatalf("sample %d: %v", sample, err)
		}
		assertActionsSeparated(t, plan.Frames, in.GapMinutes)
		gotMinutes := make(map[string]int)
		gotFrames := make(map[string][]Frame)
		candidateByID := make(map[string]Candidate)
		for _, c := range in.Candidates {
			candidateByID[c.ID] = c
		}
		for _, f := range plan.Frames {
			if f.Kind != FrameActionItem {
				continue
			}
			gotMinutes[f.SourceID] += f.PlannedMinutes
			gotFrames[f.SourceID] = append(gotFrames[f.SourceID], f)
			if f.Start == nil {
				continue
			}
			if f.Start.Before(in.Day.Start) || f.End.After(in.Day.End) {
				t.Fatalf("sample %d action escaped day: %+v", sample, f)
			}
			for _, sleep := range in.Sleep {
				if f.Start.Before(sleep.End) && sleep.Start.Before(*f.End) {
					t.Fatalf("sample %d action overlaps sleep: %+v / %+v", sample, f, sleep)
				}
			}
			c := candidateByID[f.SourceID]
			if !c.HasCompletedFrame { // Untouched items must stay inside own Project allocations.
				within := false
				for _, allocation := range in.Allocations {
					if allocation.ProjectID == c.ProjectID && !f.Start.Before(allocation.Interval.Start) && !f.End.After(allocation.Interval.End) {
						within = true
						break
					}
				}
				if !within {
					t.Fatalf("sample %d candidate %s escaped allocation: %+v", sample, c.ID, f)
				}
			}
		}
		for _, c := range in.Candidates {
			want := 0
			if c.TaskStatus == "open" || c.TaskStatus == "in_progress" {
				if c.HasCompletedFrame {
					want = c.EstimatedMinutes
				}
				if !c.HasCompletedFrame && c.ProjectID != "" && gotMinutes[c.ID] != 0 {
					want = c.EstimatedMinutes
				}
			}
			if gotMinutes[c.ID] != want {
				t.Fatalf("sample %d candidate %s planned=%d want=%d", sample, c.ID, gotMinutes[c.ID], want)
			}
			if !c.HasCompletedFrame && len(gotFrames[c.ID]) > 1 {
				t.Fatalf("sample %d untouched candidate %s split: %+v", sample, c.ID, gotFrames[c.ID])
			}
		}
	}
}

func assertActionsSeparated(t *testing.T, frames []Frame, gap int) {
	t.Helper()
	var actions []Frame
	var schedules []Frame
	for _, f := range frames {
		if f.Kind == FrameSchedule {
			schedules = append(schedules, f)
		}
		if f.Kind == FrameActionItem && f.Start != nil && f.End != nil {
			actions = append(actions, f)
		}
	}
	for i, action := range actions {
		if action.Start == nil || action.End == nil || !action.End.After(*action.Start) || int(action.End.Sub(*action.Start).Minutes()) != action.PlannedMinutes {
			t.Fatalf("invalid scheduled action frame: %+v", action)
		}
		for j := 0; j < i; j++ {
			previous := actions[j]
			if intervalGap(*previous.Start, *previous.End, *action.Start, *action.End) < time.Duration(gap)*time.Minute {
				t.Fatalf("actions overlap or violate gap: %+v / %+v", previous, action)
			}
		}
		for _, schedule := range schedules {
			if intervalGap(*action.Start, *action.End, *schedule.Start, *schedule.End) < time.Duration(gap)*time.Minute {
				t.Fatalf("action overlaps schedule or violates gap: %+v / %+v", action, schedule)
			}
		}
	}
}

func intervalGap(aStart, aEnd, bStart, bEnd time.Time) time.Duration {
	if aEnd.Before(bStart) || aEnd.Equal(bStart) {
		return bStart.Sub(aEnd)
	}
	if bEnd.Before(aStart) || bEnd.Equal(aStart) {
		return aStart.Sub(bEnd)
	}
	return -1
}

func startedCandidate(id string, minutes int) Candidate {
	c := candidate(id, "", minutes, 0)
	c.HasCompletedFrame = true
	return c
}

func BenchmarkGenerateCandidates100(b *testing.B)  { benchmarkGenerate(b, 100) }
func BenchmarkGenerateCandidates1000(b *testing.B) { benchmarkGenerate(b, 1000) }

func benchmarkGenerate(b *testing.B, count int) {
	in := baseInput()
	in.Day = Interval{at(7, 0), at(23, 0)}
	in.Allocations = []ProjectAllocation{{ProjectID: "p", Interval: in.Day}}
	for i := 0; i < count; i++ {
		in.Candidates = append(in.Candidates, candidate(fmt.Sprintf("candidate-%04d", i), "p", 10+(i%60), 100-(i%100)))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Generate(in); err != nil {
			b.Fatal(err)
		}
	}
}

func countSource(frames []Frame, id string) int { return len(framesFor(frames, id)) }

func firstFrame(frames []Frame, id string) Frame {
	items := framesFor(frames, id)
	if len(items) == 0 {
		return Frame{}
	}
	return items[0]
}

func framesFor(frames []Frame, id string) []Frame {
	var result []Frame
	for _, f := range frames {
		if f.Kind.IsWork() && f.SourceID == id {
			result = append(result, f)
		}
	}
	return result
}

func overlaps(start, end, otherStart, otherEnd time.Time) bool {
	return start.Before(otherEnd) && otherStart.Before(end)
}
