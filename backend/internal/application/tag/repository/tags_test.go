package repository

import (
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/tag/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTagRepositoryError(t *testing.T) {
	if err := tagRepositoryError(pgx.ErrNoRows); !errors.Is(err, usecase.ErrTagNotFound) {
		t.Fatalf("missing row error = %v, want ErrTagNotFound", err)
	}
	duplicate := &pgconn.PgError{Code: "23505", ConstraintName: "tags_user_id_name_key"}
	if err := tagRepositoryError(duplicate); !errors.Is(err, usecase.ErrTagNameConflict) {
		t.Fatalf("duplicate name error = %v, want ErrTagNameConflict", err)
	}
	other := &pgconn.PgError{Code: "23505", ConstraintName: "tags_pkey"}
	if err := tagRepositoryError(other); !errors.Is(err, other) {
		t.Fatalf("other constraint error = %v, want original error", err)
	}
}
