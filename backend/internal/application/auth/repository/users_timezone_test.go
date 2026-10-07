package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type timezoneQueryDB struct {
	query string
	args  []any
	row   pgx.Row
}

func (db *timezoneQueryDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("unexpected Exec")
}

func (db *timezoneQueryDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("unexpected Query")
}

func (db *timezoneQueryDB) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	db.query = query
	db.args = append([]any(nil), args...)
	return db.row
}

type timezoneQueryRow struct {
	timezone string
	err      error
}

func (row timezoneQueryRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	value, ok := dest[0].(*string)
	if !ok {
		return errors.New("timezone destination is not a string")
	}
	*value = row.timezone
	return nil
}

func TestUserRepositoryGetTimezoneUsesScalarQuery(t *testing.T) {
	db := &timezoneQueryDB{row: timezoneQueryRow{timezone: "America/Los_Angeles"}}
	repo := NewUserRepository(db)

	got, err := repo.GetTimezone(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetTimezone() error = %v", err)
	}
	if got != "America/Los_Angeles" {
		t.Fatalf("GetTimezone() = %q, want America/Los_Angeles", got)
	}
	if len(db.args) != 1 || db.args[0] != "user-1" {
		t.Fatalf("query args = %#v, want user ID", db.args)
	}
	if !strings.Contains(db.query, "SELECT timezone") || !strings.Contains(db.query, "FROM users") {
		t.Fatalf("query = %q, want scalar users timezone query", db.query)
	}
}

func TestUserRepositoryGetTimezonePropagatesQueryError(t *testing.T) {
	wantErr := errors.New("user missing")
	db := &timezoneQueryDB{row: timezoneQueryRow{err: wantErr}}
	repo := NewUserRepository(db)

	if _, err := repo.GetTimezone(context.Background(), "missing"); !errors.Is(err, wantErr) {
		t.Fatalf("GetTimezone() error = %v, want %v", err, wantErr)
	}
}
