package repository

import (
	"errors"
	"testing"

	"github.com/Najah7/task2todaytodo/internal/application/task/usecase"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestProjectMemberRepositoryErrorMapsKnownForeignKeys(t *testing.T) {
	tests := []struct {
		name       string
		constraint string
		want       error
	}{
		{name: "role", constraint: "project_members_role_id_fkey", want: usecase.ErrProjectMemberRoleNotFound},
		{name: "user", constraint: "project_members_user_id_fkey", want: usecase.ErrProjectMemberUserNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dbErr := &pgconn.PgError{Code: "23503", ConstraintName: test.constraint, Message: "private database detail"}
			if got := projectMemberRepositoryError(dbErr); !errors.Is(got, test.want) {
				t.Fatalf("mapped error = %v; want %v", got, test.want)
			}
		})
	}
}

func TestProjectMemberRepositoryErrorPreservesUnknownFailure(t *testing.T) {
	dbErr := &pgconn.PgError{Code: "23505", ConstraintName: "unrelated_key", Message: "private database detail"}
	if got := projectMemberRepositoryError(dbErr); got != dbErr {
		t.Fatalf("unrelated error = %v; want original %v", got, dbErr)
	}
}
