package repository

import (
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/project/usecase"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestProjectMemberErrorMapsKnownForeignKeys(t *testing.T) {
	for _, tt := range []struct {
		constraint string
		want       error
	}{
		{"project_members_role_id_fkey", usecase.ErrProjectMemberRoleNotFound},
		{"project_members_user_id_fkey", usecase.ErrProjectMemberUserNotFound},
	} {
		t.Run(tt.constraint, func(t *testing.T) {
			got := memberError(&pgconn.PgError{Code: "23503", ConstraintName: tt.constraint, Message: "private detail"})
			if !errors.Is(got, tt.want) {
				t.Fatalf("memberError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProjectMemberErrorPreservesUnknownFailure(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "unrelated_key", Message: "private detail"}
	if got := memberError(pgErr); got != pgErr {
		t.Fatalf("memberError() = %v, want original error", got)
	}
}
