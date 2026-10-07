//go:build integration

package tag_test

import (
	"testing"

	tagrepos "github.com/Najah7/task2todaytodo/internal/application/tag/repository"
	tagusecase "github.com/Najah7/task2todaytodo/internal/application/tag/usecase"
	"github.com/Najah7/task2todaytodo/tests/integration/internal/testdb"
	"github.com/oklog/ulid/v2"
)

func TestTagCursorListUsesCaseInsensitiveNameOrdering(t *testing.T) {
	ctx := t.Context()
	tx, err := testdb.Open(t).Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	userID := ulid.Make().String()
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,first_name,last_name,email,password) VALUES($1,'Tag','Cursor',$2,'unused')`, userID, userID+"@tag-cursor.test"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"alpha", "bEta", "GAMma"} {
		if _, err := tx.Exec(ctx, `INSERT INTO tags(id,user_id,name) VALUES($1,$2,$3)`, ulid.Make().String(), userID, name); err != nil {
			t.Fatal(err)
		}
	}

	tags := tagrepos.NewTagRepository(tx)
	first, err := tags.ListByUserIDCursor(ctx, userID, 2, nil)
	if err != nil || len(first) != 2 || first[0].Name != "alpha" || first[1].Name != "bEta" {
		t.Fatalf("tag order=%+v err=%v", first, err)
	}
	next, err := tags.ListByUserIDCursor(ctx, userID, 2, &tagusecase.CursorAnchor{Name: first[1].Name, ID: first[1].ID})
	if err != nil || len(next) != 1 || next[0].Name != "GAMma" {
		t.Fatalf("tag boundary=%+v err=%v", next, err)
	}
}
