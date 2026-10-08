package usecase

import (
	"context"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
	"github.com/Najah7/task2todaytodo/internal/application/project/domain"
)

type filteredProjectListRepositoryFake struct {
	candidates []dao.Project
	pageRows   []dao.Project
	summary    dao.ProjectListSummary

	candidateCalls  int
	candidateUserID domain.UserID
	candidateStatus string
	candidateTrash  bool
	pageCalls       int
	pageUserID      domain.UserID
	pageRequest     ProjectListRequest
	pageLimit       int
	summaryCalls    int
	summaryUserID   domain.UserID
	summaryStatus   string
	summaryTrash    bool
	summaryAsOf     time.Time
}

func (f *filteredProjectListRepositoryFake) ListByUserID(_ context.Context, _ domain.UserID) ([]dao.Project, error) {
	return nil, nil
}

func (f *filteredProjectListRepositoryFake) ListProjectCandidates(_ context.Context, userID domain.UserID, status string, trash bool) ([]dao.Project, error) {
	f.candidateCalls++
	f.candidateUserID, f.candidateStatus, f.candidateTrash = userID, status, trash
	return append([]dao.Project(nil), f.candidates...), nil
}

func (f *filteredProjectListRepositoryFake) ListProjectPage(_ context.Context, userID domain.UserID, request ProjectListRequest, limit int) ([]dao.Project, error) {
	f.pageCalls++
	f.pageUserID, f.pageRequest, f.pageLimit = userID, request, limit
	return append([]dao.Project(nil), f.pageRows...), nil
}

func (f *filteredProjectListRepositoryFake) ReadProjectListSummary(_ context.Context, userID domain.UserID, status string, trash bool, asOf time.Time) (dao.ProjectListSummary, error) {
	f.summaryCalls++
	f.summaryUserID, f.summaryStatus, f.summaryTrash, f.summaryAsOf = userID, status, trash, asOf
	return f.summary, nil
}

type listPageProgressReaderFake struct {
	sources dao.ProjectProgressSources
	ids     [][]string
	asOf    []time.Time
}

func (f *listPageProgressReaderFake) ReadProjectProgressSources(_ context.Context, ids []string, asOf time.Time) (dao.ProjectProgressSources, error) {
	f.ids = append(f.ids, append([]string(nil), ids...))
	f.asOf = append(f.asOf, asOf)
	return f.sources, nil
}

func TestExecuteFilteredPageProgressPagingNextAndPrevious(t *testing.T) {
	for _, order := range []string{"asc", "desc"} {
		t.Run(order, func(t *testing.T) {
			const total = 41
			candidates := make([]dao.Project, 0, total)
			progressSources := dao.ProjectProgressSources{}
			for rank := total - 1; rank >= 0; rank-- {
				id := projectIDForRank(rank)
				candidates = append(candidates, dao.Project{ID: id, UserID: "actor", Status: "open", Title: id})
				progressSources.Tasks = append(progressSources.Tasks, dao.ProjectTaskProgress{
					ProjectID: id, TaskID: "task-" + id, Total: 100, Completed: rank,
				})
			}
			summary := dao.ProjectListSummary{
				TotalCount: 87, OpenCount: 41, PendingCount: 13, InProgressCount: 17,
				DoneCount: 11, OverdueCount: 5, Today: "2026-10-08", Timezone: "Asia/Tokyo",
			}
			repo := &filteredProjectListRepositoryFake{candidates: candidates, summary: summary}
			progress := &listPageProgressReaderFake{sources: progressSources}
			useCase := NewListProjectsUseCase(repo, progress, nil)
			asOf := time.Date(2026, 10, 8, 3, 4, 5, 0, time.UTC)
			request := ProjectListRequest{Size: 20, Status: "open", SortBy: "progress", SortOrder: order, AsOf: asOf}
			wantRanks := make([]int, total)
			for i := range wantRanks {
				wantRanks[i] = i
			}
			if order == "desc" {
				for left, right := 0, len(wantRanks)-1; left < right; left, right = left+1, right-1 {
					wantRanks[left], wantRanks[right] = wantRanks[right], wantRanks[left]
				}
			}

			firstPage, err := useCase.ExecuteFilteredPage(context.Background(), "actor", request)
			if err != nil {
				t.Fatalf("first page: %v", err)
			}
			assertProjectIDs(t, firstPage.Items, projectIDsForRanks(wantRanks[:20]))
			if firstPage.Next == nil || firstPage.Next.ID != projectIDForRank(wantRanks[19]) || firstPage.Next.Direction != "forward" || firstPage.Next.Progress != wantRanks[19] {
				t.Fatalf("first page next cursor = %#v, want cursor at rank %d", firstPage.Next, wantRanks[19])
			}
			if firstPage.Previous != nil {
				t.Errorf("first page previous cursor = %#v, want nil", firstPage.Previous)
			}

			request.Anchor = firstPage.Next
			secondPage, err := useCase.ExecuteFilteredPage(context.Background(), "actor", request)
			if err != nil {
				t.Fatalf("second page: %v", err)
			}
			assertProjectIDs(t, secondPage.Items, projectIDsForRanks(wantRanks[20:40]))
			if secondPage.Previous == nil || secondPage.Previous.ID != projectIDForRank(wantRanks[20]) || secondPage.Previous.Direction != "backward" {
				t.Fatalf("second page previous cursor = %#v, want backward cursor at rank %d", secondPage.Previous, wantRanks[20])
			}
			if secondPage.Next == nil || secondPage.Next.ID != projectIDForRank(wantRanks[39]) || secondPage.Next.Direction != "forward" {
				t.Fatalf("second page next cursor = %#v, want forward cursor at rank %d", secondPage.Next, wantRanks[39])
			}

			request.Anchor = secondPage.Previous
			previousPage, err := useCase.ExecuteFilteredPage(context.Background(), "actor", request)
			if err != nil {
				t.Fatalf("previous page: %v", err)
			}
			assertProjectIDs(t, previousPage.Items, projectIDsForRanks(wantRanks[:20]))
			if previousPage.Previous != nil {
				t.Errorf("first page reached backward has previous cursor = %#v, want nil", previousPage.Previous)
			}
			if previousPage.Next == nil || previousPage.Next.ID != projectIDForRank(wantRanks[19]) || previousPage.Next.Direction != "forward" {
				t.Errorf("backward page next cursor = %#v, want forward cursor at rank %d", previousPage.Next, wantRanks[19])
			}

			if repo.candidateCalls != 3 || repo.candidateUserID != "actor" || repo.candidateStatus != "open" || repo.candidateTrash {
				t.Errorf("candidate query calls=%d actor=%q status=%q trash=%t", repo.candidateCalls, repo.candidateUserID, repo.candidateStatus, repo.candidateTrash)
			}
			if repo.summaryCalls != 3 || repo.summaryUserID != "actor" || repo.summaryStatus != "open" || repo.summaryTrash || !repo.summaryAsOf.Equal(asOf) {
				t.Errorf("summary query calls=%d actor=%q status=%q trash=%t asOf=%v", repo.summaryCalls, repo.summaryUserID, repo.summaryStatus, repo.summaryTrash, repo.summaryAsOf)
			}
			if !reflect.DeepEqual(firstPage.Summary, summary) || !reflect.DeepEqual(secondPage.Summary, summary) || !reflect.DeepEqual(previousPage.Summary, summary) {
				t.Errorf("page summaries did not preserve repository counts: first=%#v second=%#v previous=%#v", firstPage.Summary, secondPage.Summary, previousPage.Summary)
			}
			if len(progress.ids) != 3 {
				t.Fatalf("progress reader calls = %d, want one per progress-sorted page", len(progress.ids))
			}
			wantCandidateIDs := make([]string, 0, total)
			for _, candidate := range candidates {
				wantCandidateIDs = append(wantCandidateIDs, candidate.ID)
			}
			for i := range progress.ids {
				if !reflect.DeepEqual(progress.ids[i], wantCandidateIDs) {
					t.Errorf("progress reader call %d IDs = %v, want all candidate IDs %v", i, progress.ids[i], wantCandidateIDs)
				}
				if !progress.asOf[i].Equal(asOf) {
					t.Errorf("progress reader call %d asOf = %v, want %v", i, progress.asOf[i], asOf)
				}
			}
		})
	}
}

func TestExecuteFilteredPageProgressTiesMissingAnchorAndBounds(t *testing.T) {
	candidates := []dao.Project{
		{ID: "z-high", Priority: dao.Priority{Weight: 10}},
		{ID: "a-low", Priority: dao.Priority{Weight: 1}},
		{ID: "zero"},
		{ID: "done", Status: "done"},
		{ID: "m-mid", Priority: dao.Priority{Weight: 5}},
		{ID: "b-high", Priority: dao.Priority{Weight: 10}},
	}
	progress := &listPageProgressReaderFake{sources: dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{
		{ProjectID: "z-high", TaskID: "z-task", Total: 2, Completed: 1},
		{ProjectID: "a-low", TaskID: "a-task", Total: 2, Completed: 1},
		{ProjectID: "m-mid", TaskID: "m-task", Total: 2, Completed: 1},
		{ProjectID: "b-high", TaskID: "b-task", Total: 2, Completed: 1},
	}}}
	repo := &filteredProjectListRepositoryFake{candidates: candidates, summary: dao.ProjectListSummary{Today: "2026-10-08"}}
	useCase := NewListProjectsUseCase(repo, progress, nil)
	request := ProjectListRequest{Size: 10, SortBy: "progress", SortOrder: "asc", AsOf: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)}

	page, err := useCase.ExecuteFilteredPage(context.Background(), "actor", request)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	assertProjectIDs(t, page.Items, []string{"zero", "b-high", "z-high", "m-mid", "a-low", "done"})
	for _, project := range page.Items {
		wantProgress := 50
		switch project.ID {
		case "zero":
			wantProgress = 0
		case "done":
			wantProgress = 100
		}
		if project.Progress != wantProgress {
			t.Errorf("project %q progress = %d, want %d", project.ID, project.Progress, wantProgress)
		}
	}

	// The anchor is intentionally absent from the candidate set. Continuation
	// must use the cursor tuple (progress, priority, ID), not look up the row.
	request.Size = 2
	request.Anchor = &CursorAnchor{ID: "a-high-removed", Direction: "forward", Progress: 50, PriorityWeight: 10}
	continued, err := useCase.ExecuteFilteredPage(context.Background(), "actor", request)
	if err != nil {
		t.Fatalf("continue from deleted anchor: %v", err)
	}
	assertProjectIDs(t, continued.Items, []string{"b-high", "z-high"})
}

func TestExecuteFilteredPageDelegatesOrdinarySortAndAppliesProgressOnlyToPage(t *testing.T) {
	for _, test := range []struct {
		name, sortBy, order, expectedCursorValue string
		wantNull                                 bool
	}{
		{name: "title asc", sortBy: "title", order: "asc", expectedCursorValue: "second title"},
		{name: "title desc", sortBy: "title", order: "desc", expectedCursorValue: "second title"},
		{name: "end date asc null last", sortBy: "end_date", order: "asc", wantNull: true},
		{name: "end date desc null last", sortBy: "end_date", order: "desc", wantNull: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			firstDate := "2026-10-10"
			repo := &filteredProjectListRepositoryFake{
				pageRows: []dao.Project{
					{ID: "first", Title: "First title", EndDate: &firstDate},
					{ID: "second", Title: "Second title"},
					{ID: "extra", Title: "Extra row"},
				},
				summary: dao.ProjectListSummary{TotalCount: 3, OpenCount: 3, Today: "2026-10-08", Timezone: "Asia/Tokyo"},
			}
			progress := &listPageProgressReaderFake{sources: dao.ProjectProgressSources{Tasks: []dao.ProjectTaskProgress{
				{ProjectID: "first", TaskID: "first-task", Done: true},
				{ProjectID: "second", TaskID: "second-task", Total: 2, Completed: 1},
				{ProjectID: "extra", TaskID: "extra-task", Done: true},
			}}}
			asOf := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
			request := ProjectListRequest{Size: 2, Status: "open", SortBy: test.sortBy, SortOrder: test.order, AsOf: asOf}
			page, err := NewListProjectsUseCase(repo, progress, nil).ExecuteFilteredPage(context.Background(), "actor", request)
			if err != nil {
				t.Fatalf("page: %v", err)
			}
			assertProjectIDs(t, page.Items, []string{"first", "second"})
			if repo.pageCalls != 1 || repo.pageUserID != "actor" || repo.pageLimit != 3 || repo.pageRequest.SortBy != test.sortBy || repo.pageRequest.SortOrder != test.order || repo.pageRequest.Status != "open" {
				t.Errorf("page query calls=%d user=%q limit=%d request=%#v", repo.pageCalls, repo.pageUserID, repo.pageLimit, repo.pageRequest)
			}
			if page.Next == nil || page.Next.ID != "second" || page.Next.Direction != "forward" {
				t.Fatalf("next cursor = %#v, want forward cursor at second row", page.Next)
			}
			if page.Next.SortValue != test.expectedCursorValue || page.Next.SortValueNull != test.wantNull {
				t.Errorf("cursor sort value/null = %q/%t, want %q/%t", page.Next.SortValue, page.Next.SortValueNull, test.expectedCursorValue, test.wantNull)
			}
			if len(progress.ids) != 1 || !reflect.DeepEqual(progress.ids[0], []string{"first", "second"}) {
				t.Errorf("progress reader IDs = %v, want only selected page rows", progress.ids)
			}
			if page.Items[0].Progress != 100 || page.Items[1].Progress != 50 {
				t.Errorf("page progress = [%d %d], want [100 50]", page.Items[0].Progress, page.Items[1].Progress)
			}
			if page.Summary.TotalCount != 3 {
				t.Errorf("summary total = %d, want repository-scoped count 3", page.Summary.TotalCount)
			}
		})
	}
}

func TestExecuteFilteredPageBackwardOrdinarySortTrimsAndRestoresDisplayOrder(t *testing.T) {
	// The repository supplies rows in reverse display order for a backward
	// cursor query; the use case trims the extra row and restores display order.
	repo := &filteredProjectListRepositoryFake{
		pageRows: []dao.Project{{ID: "near-b"}, {ID: "near-a"}, {ID: "extra"}},
		summary:  dao.ProjectListSummary{Today: "2026-10-08"},
	}
	progress := &listPageProgressReaderFake{}
	request := ProjectListRequest{
		Size: 2, SortBy: "title", SortOrder: "asc", Anchor: &CursorAnchor{ID: "after", Direction: "backward"},
		AsOf: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC),
	}
	page, err := NewListProjectsUseCase(repo, progress, nil).ExecuteFilteredPage(context.Background(), "actor", request)
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	assertProjectIDs(t, page.Items, []string{"near-a", "near-b"})
	if repo.pageLimit != 3 || repo.pageRequest.Anchor != request.Anchor {
		t.Errorf("backward query limit=%d anchor=%#v, want limit 3 and supplied anchor", repo.pageLimit, repo.pageRequest.Anchor)
	}
	if page.Previous == nil || page.Previous.ID != "near-a" || page.Previous.Direction != "backward" {
		t.Errorf("previous cursor = %#v, want backward cursor at first display row", page.Previous)
	}
	if page.Next == nil || page.Next.ID != "near-b" || page.Next.Direction != "forward" {
		t.Errorf("next cursor = %#v, want forward cursor at last display row", page.Next)
	}
	if len(progress.ids) != 1 || !reflect.DeepEqual(progress.ids[0], []string{"near-a", "near-b"}) {
		t.Errorf("progress reader IDs = %v, want only selected page rows", progress.ids)
	}
}

func projectIDForRank(rank int) string {
	if rank < 10 {
		return "p0" + strconv.Itoa(rank)
	}
	return "p" + strconv.Itoa(rank)
}

func projectIDsForRanks(ranks []int) []string {
	ids := make([]string, 0, len(ranks))
	for _, rank := range ranks {
		ids = append(ids, projectIDForRank(rank))
	}
	return ids
}

func assertProjectIDs(t *testing.T, projects []dao.Project, want []string) {
	t.Helper()
	got := make([]string, 0, len(projects))
	for _, project := range projects {
		got = append(got, project.ID)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("project IDs = %v, want %v", got, want)
	}
}
