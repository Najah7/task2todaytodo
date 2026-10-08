package usecase

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
	"github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
	"github.com/Najah7/task2todaytodo/internal/logging"
)

type projectListRepository interface {
	ListByUserID(ctx context.Context, userID domain.UserID) ([]dao.Project, error)
}

type filteredProjectListRepository interface {
	ListProjectCandidates(context.Context, domain.UserID, string, bool) ([]dao.Project, error)
	ListProjectPage(context.Context, domain.UserID, ProjectListRequest, int) ([]dao.Project, error)
	ReadProjectListSummary(context.Context, domain.UserID, string, bool, time.Time) (dao.ProjectListSummary, error)
}

type ProjectOption struct {
	Value, Label, LabelJp string
	Weight                int
}

type ProjectOptions struct {
	Types      []ProjectOption
	Priorities []ProjectOption
	Statuses   []ProjectOption
}

var ErrInvalidProjectListRequest = errors.New("invalid project list request")

type ListProjectsUseCase struct {
	repo     projectListRepository
	progress ProjectProgressReader
	logger   logging.Logger
}

func NewListProjectsUseCase(repo projectListRepository, progress ProjectProgressReader, logger logging.Logger) *ListProjectsUseCase {
	return &ListProjectsUseCase{logger: logging.OrNop(logger), repo: repo, progress: progress}
}

func (uc *ListProjectsUseCase) Execute(ctx context.Context, userID domain.UserID) (output []dao.Project, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectsUseCase.Execute", err) }()

	projects, err := uc.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return applyProjectProgress(ctx, uc.progress, projects, time.Now())

}

func (uc *ListProjectsUseCase) ExecutePage(ctx context.Context, userID domain.UserID, request CursorPageRequest) (output CursorPage[dao.Project], err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectsUseCase.ExecutePage", err) }()

	if err := validateCursorPageRequest(request); err != nil {
		return CursorPage[dao.Project]{}, err
	}
	repo, ok := uc.repo.(projectCursorRepository)
	if !ok {
		return CursorPage[dao.Project]{}, ErrInvalidProjectPage
	}
	rows, err := repo.ListByUserIDCursor(ctx, userID, request.Size+1, request.Anchor)
	if err != nil {
		return CursorPage[dao.Project]{}, err
	}
	rows, err = applyProjectProgress(ctx, uc.progress, rows, time.Now())
	if err != nil {
		return CursorPage[dao.Project]{}, err
	}
	return projectPage(rows, request.Size), nil

}

func (uc *ListProjectsUseCase) ExecuteFilteredPage(ctx context.Context, userID domain.UserID, request ProjectListRequest) (output ProjectListPage, err error) {
	defer func() { logUnexpectedProjectFailure(uc.logger, ctx, "ListProjectsUseCase.ExecuteFilteredPage", err) }()
	if request.Size < 1 || request.Size > pagination.MaxPageSize || userID == "" {
		return ProjectListPage{}, ErrInvalidProjectListRequest
	}
	if request.Status != "" {
		if _, err := domain.NewProjectStatus(request.Status); err != nil || request.Trash {
			return ProjectListPage{}, ErrInvalidProjectListRequest
		}
	}
	if request.SortBy == "" {
		request.SortBy = "created_at"
	}
	if request.SortOrder == "" {
		request.SortOrder = "desc"
	}
	if !validProjectSort(request.SortBy, request.SortOrder) {
		return ProjectListPage{}, ErrInvalidProjectListRequest
	}
	if request.AsOf.IsZero() {
		request.AsOf = time.Now().UTC()
	}
	repo, ok := uc.repo.(filteredProjectListRepository)
	if !ok {
		return ProjectListPage{}, ErrInvalidProjectListRequest
	}
	summary, err := repo.ReadProjectListSummary(ctx, userID, request.Status, request.Trash, request.AsOf)
	if err != nil {
		return ProjectListPage{}, err
	}
	if request.Anchor != nil && request.Anchor.Direction != "forward" && request.Anchor.Direction != "backward" {
		return ProjectListPage{}, ErrInvalidProjectListRequest
	}
	direction := "forward"
	if request.Anchor != nil {
		direction = request.Anchor.Direction
	}
	more := false
	var rows []dao.Project
	if request.SortBy == "progress" {
		rows, err = repo.ListProjectCandidates(ctx, userID, request.Status, request.Trash)
		if err != nil {
			return ProjectListPage{}, err
		}
		rows, err = applyProjectProgress(ctx, uc.progress, rows, request.AsOf)
		if err != nil {
			return ProjectListPage{}, err
		}
		sort.SliceStable(rows, func(i, j int) bool { return projectLess(rows[i], rows[j], request.SortBy, request.SortOrder) })
		if request.Anchor != nil {
			anchorRow, err := projectFromCursor(*request.Anchor, request.SortBy)
			if err != nil {
				return ProjectListPage{}, ErrInvalidProjectListRequest
			}
			filtered := make([]dao.Project, 0, len(rows))
			for _, row := range rows {
				if direction == "forward" && projectLess(anchorRow, row, request.SortBy, request.SortOrder) ||
					direction == "backward" && projectLess(row, anchorRow, request.SortBy, request.SortOrder) {
					filtered = append(filtered, row)
				}
			}
			rows = filtered
		}
		if direction == "forward" {
			more = len(rows) > request.Size
			if more {
				rows = rows[:request.Size]
			}
		} else {
			more = len(rows) > request.Size
			if more {
				rows = rows[len(rows)-request.Size:]
			}
		}
	} else {
		rows, err = repo.ListProjectPage(ctx, userID, request, request.Size+1)
		if err != nil {
			return ProjectListPage{}, err
		}
		more = len(rows) > request.Size
		if more {
			rows = rows[:request.Size]
		}
		if direction == "backward" {
			reverseProjects(rows)
		}
	}
	pageRows := append([]dao.Project(nil), rows...)
	if request.SortBy != "progress" {
		pageRows, err = applyProjectProgress(ctx, uc.progress, pageRows, request.AsOf)
		if err != nil {
			return ProjectListPage{}, err
		}
	}
	today, err := time.Parse("2006-01-02", summary.Today)
	if err != nil {
		return ProjectListPage{}, err
	}
	for i := range pageRows {
		if pageRows[i].EndDate != nil {
			endDate, err := time.Parse("2006-01-02", *pageRows[i].EndDate)
			if err != nil {
				return ProjectListPage{}, err
			}
			days := int(endDate.Sub(today).Hours() / 24)
			pageRows[i].RemainingDays = &days
		}
	}
	page := ProjectListPage{Items: pageRows, Summary: summary}
	if len(pageRows) > 0 {
		if request.Anchor != nil && (direction == "forward" || more) {
			anchor := projectCursor(pageRows[0], request.SortBy, "backward")
			page.Previous = &anchor
		}
		if (direction == "forward" && more) || direction == "backward" {
			anchor := projectCursor(pageRows[len(pageRows)-1], request.SortBy, "forward")
			page.Next = &anchor
		}
	}
	return page, nil
}

func reverseProjects(rows []dao.Project) {
	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}
}

func projectCursor(project dao.Project, sortBy, direction string) CursorAnchor {
	anchor := CursorAnchor{ID: project.ID, Direction: direction, At: project.CursorCreatedAt, PriorityWeight: project.Priority.Weight, Progress: project.Progress}
	switch sortBy {
	case "title":
		anchor.SortValue = strings.ToLower(project.Title)
	case "end_date":
		anchor.SortValueNull = project.EndDate == nil
		if project.EndDate != nil {
			anchor.SortValue = *project.EndDate
		}
	case "progress":
		anchor.SortValueNull = project.EndDate == nil
		if project.EndDate != nil {
			anchor.SortValue = *project.EndDate
		}
	}
	return anchor
}

func projectFromCursor(anchor CursorAnchor, sortBy string) (dao.Project, error) {
	project := dao.Project{ID: anchor.ID, Priority: dao.Priority{Weight: anchor.PriorityWeight}, Progress: anchor.Progress, CursorCreatedAt: anchor.At}
	switch sortBy {
	case "title":
		project.Title = anchor.SortValue
	case "progress":
		if !anchor.SortValueNull {
			project.EndDate = &anchor.SortValue
		}
	case "end_date":
		if !anchor.SortValueNull {
			project.EndDate = &anchor.SortValue
		}
	case "created_at":
		at, err := time.Parse(time.RFC3339Nano, anchor.At)
		if err != nil {
			return dao.Project{}, err
		}
		project.CreatedAt = at.Unix()
	}
	return project, nil
}

func validProjectSort(sortBy, order string) bool {
	if order != "asc" && order != "desc" {
		return false
	}
	switch sortBy {
	case "created_at", "title", "progress", "end_date":
		return true
	default:
		return false
	}
}

func projectLess(a, b dao.Project, sortBy, order string) bool {
	compare := func(value int) bool {
		if value == 0 {
			return false
		}
		if order == "desc" {
			return value > 0
		}
		return value < 0
	}
	switch sortBy {
	case "title":
		if c := strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title)); c != 0 {
			return compare(c)
		}
	case "progress":
		if a.Progress != b.Progress {
			return compare(a.Progress - b.Progress)
		}
	case "end_date":
		if (a.EndDate == nil) != (b.EndDate == nil) {
			return b.EndDate == nil
		}
		if a.EndDate != nil && b.EndDate != nil && *a.EndDate != *b.EndDate {
			c := strings.Compare(*a.EndDate, *b.EndDate)
			return compare(c)
		}
	case "created_at":
		if a.CreatedAt != b.CreatedAt {
			return compare(int(a.CreatedAt - b.CreatedAt))
		}
	}
	if a.Priority.Weight != b.Priority.Weight {
		return a.Priority.Weight > b.Priority.Weight
	}
	if sortBy == "created_at" {
		return a.ID > b.ID
	}
	return a.ID < b.ID
}
