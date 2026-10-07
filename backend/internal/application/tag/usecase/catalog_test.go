package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/tag/dao"
	"github.com/Najah7/task2todaytodo/internal/application/tag/domain"
)

type catalogRepositoryFake struct {
	rows             []dao.Tag
	created, renamed domain.Tag
	deleted          domain.TagID
	err              error
}

func (repo *catalogRepositoryFake) GetByUserID(_ context.Context, userID string, id domain.TagID) (dao.Tag, error) {
	if repo.err != nil {
		return dao.Tag{}, repo.err
	}
	for _, row := range repo.rows {
		if row.ID == string(id) && row.UserID == userID {
			return row, nil
		}
	}
	return dao.Tag{}, ErrTagNotFound
}
func (repo *catalogRepositoryFake) ListByUserIDCursor(_ context.Context, _ string, limit int, anchor *CursorAnchor) ([]dao.Tag, error) {
	if repo.err != nil {
		return nil, repo.err
	}
	rows := repo.rows
	if anchor != nil {
		filtered := make([]dao.Tag, 0, len(rows))
		for _, row := range rows {
			if row.Name > anchor.Name || row.Name == anchor.Name && row.ID > anchor.ID {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}
func (repo *catalogRepositoryFake) Create(_ context.Context, tag domain.Tag) (dao.Tag, error) {
	repo.created = tag
	if repo.err != nil {
		return dao.Tag{}, repo.err
	}
	row := dao.Tag{ID: string(tag.ID), UserID: tag.UserID, Name: tag.Name}
	repo.rows = append(repo.rows, row)
	return row, nil
}
func (repo *catalogRepositoryFake) RenameByUserID(_ context.Context, userID string, tag domain.Tag) (dao.Tag, error) {
	repo.renamed = tag
	if repo.err != nil {
		return dao.Tag{}, repo.err
	}
	return dao.Tag{ID: string(tag.ID), UserID: userID, Name: tag.Name}, nil
}
func (repo *catalogRepositoryFake) DeleteByUserID(_ context.Context, _ string, id domain.TagID) error {
	repo.deleted = id
	return repo.err
}

func TestCatalogCreateRenameGetAndDelete(t *testing.T) {
	repo := &catalogRepositoryFake{}
	group := NewCatalogUseCases(repo, nil)
	created, err := group.Create.Execute(context.Background(), CreateTagInput{ID: "tag-1", UserID: "user-1", Name: "  Planning  "})
	if err != nil || created.Name != "Planning" || repo.created.UserID != "user-1" {
		t.Fatalf("create result=%+v input=%+v err=%v", created, repo.created, err)
	}
	renamed, err := group.Rename.Execute(context.Background(), RenameTagInput{UserID: "user-1", ID: "tag-1", Name: " Focus "})
	if err != nil || renamed.Name != "Focus" || repo.renamed.UserID != "user-1" {
		t.Fatalf("rename result=%+v input=%+v err=%v", renamed, repo.renamed, err)
	}
	if _, err := group.Get.Execute(context.Background(), "user-1", "tag-1"); err != nil {
		t.Fatalf("get error=%v", err)
	}
	if err := group.Delete.Execute(context.Background(), "user-1", "tag-1"); err != nil || repo.deleted != "tag-1" {
		t.Fatalf("delete id/error=%q/%v", repo.deleted, err)
	}
}

func TestCatalogListUsesCursorAndValidatesPageSize(t *testing.T) {
	repo := &catalogRepositoryFake{rows: []dao.Tag{{ID: "1", Name: "A"}, {ID: "2", Name: "B"}, {ID: "3", Name: "C"}}}
	page, err := NewListTagsUseCase(repo, nil).ExecutePage(context.Background(), "user-1", CursorPageRequest{Size: 2})
	if err != nil || len(page.Items) != 2 || page.Next == nil || page.Next.ID != "2" {
		t.Fatalf("first page=%+v err=%v", page, err)
	}
	page, err = NewListTagsUseCase(repo, nil).ExecutePage(context.Background(), "user-1", CursorPageRequest{Size: 2, Anchor: page.Next})
	if err != nil || len(page.Items) != 1 || page.Next != nil {
		t.Fatalf("next page=%+v err=%v", page, err)
	}
	if _, err := NewListTagsUseCase(repo, nil).ExecutePage(context.Background(), "user-1", CursorPageRequest{Size: 0}); !errors.Is(err, ErrInvalidTagPage) {
		t.Fatalf("invalid size error=%v", err)
	}
}
