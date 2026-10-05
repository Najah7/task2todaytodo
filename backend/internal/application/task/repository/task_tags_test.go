package repository

import (
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTaskTagRepositoryError(t *testing.T) {
	t.Run("missing tag", func(t *testing.T) {
		if err := taskTagRepositoryError(pgx.ErrNoRows); !errors.Is(err, usecase.ErrTaskTagNotFound) {
			t.Fatalf("error = %v, want ErrTaskTagNotFound", err)
		}
	})
	t.Run("duplicate owner name", func(t *testing.T) {
		dbErr := &pgconn.PgError{Code: "23505", ConstraintName: "task_tags_user_id_name_key"}
		if err := taskTagRepositoryError(dbErr); !errors.Is(err, usecase.ErrTaskTagNameConflict) {
			t.Fatalf("error = %v, want ErrTaskTagNameConflict", err)
		}
	})
	t.Run("other constraint remains intact", func(t *testing.T) {
		dbErr := &pgconn.PgError{Code: "23505", ConstraintName: "task_tags_pkey"}
		if err := taskTagRepositoryError(dbErr); !errors.Is(err, dbErr) {
			t.Fatalf("error = %v, want original PostgreSQL error", err)
		}
	})
}
