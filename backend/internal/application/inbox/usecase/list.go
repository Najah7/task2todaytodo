package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/inbox/dao"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var (
	ErrInvalidPageRequest = errors.New("invalid inbox page request")
	ErrInvalidFilter      = errors.New("invalid inbox filter")
)

type ProjectFilterMode string

const (
	ProjectFilterAll       ProjectFilterMode = "all"
	ProjectFilterNoProject ProjectFilterMode = "no_project"
	ProjectFilterProject   ProjectFilterMode = "project"
)

type ProjectFilter struct {
	Mode      ProjectFilterMode
	ProjectID string
}

type Snapshot struct {
	AsOf     time.Time
	Timezone string
	Today    string
}

type CandidateRange struct {
	ScheduleFrom time.Time
	ScheduleTo   time.Time
}

type CursorAnchor struct {
	CreatedAt       string
	Type            dao.ItemType
	ID              string
	BlueprintID     string
	OccurrenceDate  string
	ScheduleStartAt int64
	Direction       string
}

type ListRequest struct {
	Size          int
	Anchor        *CursorAnchor
	TypeFilter    string
	ProjectFilter ProjectFilter
	Title         string
	Snapshot      Snapshot
}

type Counts struct {
	All      int
	Task     int
	Schedule int
}

type Page struct {
	Items    []dao.Item
	Counts   Counts
	Next     *CursorAnchor
	Previous *CursorAnchor
	Snapshot Snapshot
}

// CandidateReader provides owner-context projections. It must keep Task and
// Schedule rules in their owning contexts and return readable, nondeleted
// candidates. List applies Inbox-specific active and date-window policy.
type CandidateReader interface {
	Timezone(ctx context.Context, actorID string) (string, error)
	ReadCandidates(ctx context.Context, actorID string, snapshot Snapshot, dateRange CandidateRange) ([]dao.Item, error)
}

type ListUseCase struct {
	reader CandidateReader
	logger telemetry.Logger
	now    func() time.Time
}

type InboxUseCases struct {
	List *ListUseCase
}

func NewListUseCase(reader CandidateReader, logger telemetry.Logger, now func() time.Time) *ListUseCase {
	if now == nil {
		now = time.Now
	}
	return &ListUseCase{reader: reader, logger: telemetry.OrNop(logger), now: now}
}

func (uc *ListUseCase) Execute(ctx context.Context, actorID string, request ListRequest) (page Page, err error) {
	defer func() { logUnexpectedInboxFailure(uc.logger, ctx, "ListUseCase.Execute", err) }()

	if uc.reader == nil || actorID == "" || request.Size < 1 || request.Size > pagination.MaxPageSize {
		return Page{}, ErrInvalidPageRequest
	}
	if err := validateFilters(request); err != nil {
		return Page{}, err
	}
	snapshot, err := uc.resolveSnapshot(ctx, actorID, request.Snapshot)
	if err != nil {
		return Page{}, err
	}
	if request.Anchor != nil {
		if err := validateAnchor(request.Anchor); err != nil {
			return Page{}, err
		}
	}
	windowStart, windowEnd, dateRange, err := inboxWindow(snapshot)
	if err != nil {
		return Page{}, err
	}
	items, err := uc.reader.ReadCandidates(ctx, actorID, snapshot, dateRange)
	if err != nil {
		return Page{}, err
	}
	items = filterActiveCandidates(items, windowStart, windowEnd)
	items = filterItems(items, request.ProjectFilter, request.Title)
	counts := countItems(items)
	if request.TypeFilter != "" && request.TypeFilter != "all" {
		wanted := dao.ItemType(request.TypeFilter)
		filtered := items[:0]
		for _, item := range items {
			if item.Type == wanted {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	sort.Slice(items, func(i, j int) bool { return itemLess(items[i], items[j]) })
	page = Page{Counts: counts, Snapshot: snapshot}
	if request.Anchor == nil || request.Anchor.Direction == "next" {
		page = forwardPage(items, request.Size, request.Anchor, counts, snapshot)
	} else {
		page = backwardPage(items, request.Size, request.Anchor, counts, snapshot)
	}
	return page, nil
}

func logUnexpectedInboxFailure(logger telemetry.Logger, ctx context.Context, operation string, err error) {
	if err == nil || telemetry.IsRoutineError(err) || errors.Is(err, ErrInvalidPageRequest) || errors.Is(err, ErrInvalidFilter) {
		return
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	logger.Error(ctx, "inbox operation failed", fields...)
}

func inboxWindow(snapshot Snapshot) (time.Time, time.Time, CandidateRange, error) {
	location, err := time.LoadLocation(snapshot.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, CandidateRange{}, ErrInvalidPageRequest
	}
	today, err := time.Parse("2006-01-02", snapshot.Today)
	if err != nil {
		return time.Time{}, time.Time{}, CandidateRange{}, ErrInvalidPageRequest
	}
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 7)
	return start, end, CandidateRange{ScheduleFrom: start.Add(-48 * time.Hour).UTC(), ScheduleTo: end.Add(48 * time.Hour).UTC()}, nil
}

func filterActiveCandidates(items []dao.Item, windowStart, windowEnd time.Time) []dao.Item {
	filtered := make([]dao.Item, 0, len(items))
	for _, item := range items {
		if item.Type == dao.ItemTypeTask {
			if item.Task != nil && item.Task.Status == "done" {
				continue
			}
			filtered = append(filtered, item)
			continue
		}
		if item.Type == dao.ItemTypeSchedule && item.Schedule != nil && !item.Schedule.Completed && !item.Schedule.Skipped && scheduleOverlapsInboxWindow(item.Schedule, windowStart, windowEnd) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func scheduleOverlapsInboxWindow(schedule *dao.SchedulePayload, windowStart, windowEnd time.Time) bool {
	if !schedule.AllDay {
		return time.Unix(schedule.StartAt, 0).Before(windowEnd) && time.Unix(schedule.EndAt, 0).After(windowStart)
	}
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return false
	}
	startDate := time.Unix(schedule.StartAt, 0).In(location).Format("2006-01-02")
	endDate := time.Unix(schedule.EndAt, 0).In(location).Format("2006-01-02")
	windowStartDate := windowStart.Format("2006-01-02")
	windowEndDate := windowEnd.Format("2006-01-02")
	return startDate < windowEndDate && endDate > windowStartDate
}

func (uc *ListUseCase) resolveSnapshot(ctx context.Context, actorID string, snapshot Snapshot) (Snapshot, error) {
	if snapshot.AsOf.IsZero() {
		if snapshot.Timezone != "" || snapshot.Today != "" {
			return Snapshot{}, ErrInvalidPageRequest
		}
		timezone, err := uc.reader.Timezone(ctx, actorID)
		if err != nil {
			return Snapshot{}, err
		}
		location, err := time.LoadLocation(timezone)
		if err != nil {
			return Snapshot{}, fmt.Errorf("invalid user timezone: %w", err)
		}
		asOf := uc.now().UTC()
		return Snapshot{AsOf: asOf, Timezone: timezone, Today: asOf.In(location).Format("2006-01-02")}, nil
	}
	if snapshot.Timezone == "" {
		return Snapshot{}, ErrInvalidPageRequest
	}
	location, err := time.LoadLocation(snapshot.Timezone)
	if err != nil {
		return Snapshot{}, ErrInvalidPageRequest
	}
	today, err := time.Parse("2006-01-02", snapshot.Today)
	if err != nil || today.Format("2006-01-02") != snapshot.Today || snapshot.AsOf.In(location).Format("2006-01-02") != snapshot.Today {
		return Snapshot{}, ErrInvalidPageRequest
	}
	return Snapshot{AsOf: snapshot.AsOf.UTC(), Timezone: snapshot.Timezone, Today: snapshot.Today}, nil
}

func validateFilters(request ListRequest) error {
	if request.TypeFilter != "" && request.TypeFilter != "all" && request.TypeFilter != string(dao.ItemTypeTask) && request.TypeFilter != string(dao.ItemTypeSchedule) {
		return ErrInvalidFilter
	}
	switch request.ProjectFilter.Mode {
	case "", ProjectFilterAll:
		if request.ProjectFilter.ProjectID != "" {
			return ErrInvalidFilter
		}
	case ProjectFilterNoProject:
		if request.ProjectFilter.ProjectID != "" {
			return ErrInvalidFilter
		}
	case ProjectFilterProject:
		if request.ProjectFilter.ProjectID == "" {
			return ErrInvalidFilter
		}
	default:
		return ErrInvalidFilter
	}
	return nil
}

func validateAnchor(anchor *CursorAnchor) error {
	if _, err := time.Parse(time.RFC3339Nano, anchor.CreatedAt); err != nil {
		return ErrInvalidPageRequest
	}
	if anchor.ID == "" || (anchor.Type != dao.ItemTypeTask && anchor.Type != dao.ItemTypeSchedule) || (anchor.Direction != "next" && anchor.Direction != "previous") {
		return ErrInvalidPageRequest
	}
	if anchor.Type == dao.ItemTypeSchedule && (anchor.BlueprintID == "" || anchor.OccurrenceDate == "") {
		return ErrInvalidPageRequest
	}
	return nil
}

func filterItems(items []dao.Item, project ProjectFilter, title string) []dao.Item {
	needle := strings.ToLower(title)
	filtered := make([]dao.Item, 0, len(items))
	for _, item := range items {
		if project.Mode == ProjectFilterNoProject && item.ProjectID != "" {
			continue
		}
		if project.Mode == ProjectFilterProject && item.ProjectID != project.ProjectID {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(item.Title), needle) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func countItems(items []dao.Item) Counts {
	counts := Counts{All: len(items)}
	for _, item := range items {
		switch item.Type {
		case dao.ItemTypeTask:
			counts.Task++
		case dao.ItemTypeSchedule:
			counts.Schedule++
		}
	}
	return counts
}

func itemLess(left, right dao.Item) bool {
	if left.CreatedAt != right.CreatedAt {
		leftCreatedAt, leftErr := time.Parse(time.RFC3339Nano, left.CreatedAt)
		rightCreatedAt, rightErr := time.Parse(time.RFC3339Nano, right.CreatedAt)
		if leftErr == nil && rightErr == nil && !leftCreatedAt.Equal(rightCreatedAt) {
			return leftCreatedAt.After(rightCreatedAt)
		}
		if leftErr != nil || rightErr != nil {
			return left.CreatedAt > right.CreatedAt
		}
	}
	if left.Type != right.Type {
		return left.Type == dao.ItemTypeTask
	}
	if left.Type == dao.ItemTypeSchedule && left.Schedule != nil && right.Schedule != nil && left.Schedule.BlueprintID == right.Schedule.BlueprintID && left.Schedule.StartAt != right.Schedule.StartAt {
		return left.Schedule.StartAt < right.Schedule.StartAt
	}
	return left.ID < right.ID
}

func forwardPage(items []dao.Item, size int, anchor *CursorAnchor, counts Counts, snapshot Snapshot) Page {
	start := 0
	if anchor != nil {
		for start < len(items) && compareItemAnchor(items[start], anchor) <= 0 {
			start++
		}
	}
	end := min(start+size, len(items))
	page := Page{Items: append([]dao.Item(nil), items[start:end]...), Counts: counts, Snapshot: snapshot}
	if start > 0 && len(page.Items) > 0 {
		page.Previous = makeAnchor(page.Items[0], "previous")
	}
	if end < len(items) && len(page.Items) > 0 {
		page.Next = makeAnchor(page.Items[len(page.Items)-1], "next")
	}
	return page
}

func backwardPage(items []dao.Item, size int, anchor *CursorAnchor, counts Counts, snapshot Snapshot) Page {
	end := 0
	for end < len(items) && compareItemAnchor(items[end], anchor) < 0 {
		end++
	}
	start := max(0, end-size)
	page := Page{Items: append([]dao.Item(nil), items[start:end]...), Counts: counts, Snapshot: snapshot}
	if start > 0 && len(page.Items) > 0 {
		page.Previous = makeAnchor(page.Items[0], "previous")
	}
	if end < len(items) && len(page.Items) > 0 {
		page.Next = makeAnchor(page.Items[len(page.Items)-1], "next")
	}
	return page
}

func makeAnchor(item dao.Item, direction string) *CursorAnchor {
	anchor := &CursorAnchor{CreatedAt: item.CreatedAt, Type: item.Type, ID: item.ID, Direction: direction}
	if item.Schedule != nil {
		anchor.BlueprintID = item.Schedule.BlueprintID
		anchor.OccurrenceDate = item.Schedule.OccurrenceDate
		anchor.ScheduleStartAt = item.Schedule.StartAt
	}
	return anchor
}

func compareItemAnchor(item dao.Item, anchor *CursorAnchor) int {
	anchorItem := dao.Item{Type: anchor.Type, ID: anchor.ID, CreatedAt: anchor.CreatedAt}
	if anchor.Type == dao.ItemTypeSchedule {
		anchorItem.Schedule = &dao.SchedulePayload{BlueprintID: anchor.BlueprintID, OccurrenceDate: anchor.OccurrenceDate, StartAt: anchor.ScheduleStartAt}
	}
	if itemLess(item, anchorItem) {
		return -1
	}
	if itemLess(anchorItem, item) {
		return 1
	}
	return 0
}
