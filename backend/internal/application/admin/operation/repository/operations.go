package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/dao"
	"github.com/Najah7/task2todaytodo/internal/application/admin/operation/usecase"
	"github.com/Najah7/task2todaytodo/internal/application/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ usecase.OperationRepository = (*OperationRepository)(nil)

type OperationRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewOperationRepository(pool *pgxpool.Pool) *OperationRepository {
	return &OperationRepository{pool: pool, queries: sqlc.New(pool)}
}

func (r *OperationRepository) HasPermission(ctx context.Context, actorID string, capability shared.Capability) (bool, error) {
	return r.queries.HasGlobalPermission(ctx, sqlc.HasGlobalPermissionParams{
		UserID: actorID, ResourceID: string(capability.Resource), Action: sqlc.Action(capability.Action),
	})
}

func operationFromRecord(record sqlc.AdminOperation) (dao.Operation, error) {
	var response map[string]int64
	if len(record.Response) > 0 {
		if err := json.Unmarshal(record.Response, &response); err != nil {
			return dao.Operation{}, err
		}
	}
	operation := dao.Operation{
		ID: record.ID, Name: "admin/operations/" + record.ID, JobName: record.JobName,
		State: record.State, RequestedBy: record.RequestedBy, Cutoff: record.Cutoff.Time.UTC(),
		CreatedAt: record.CreatedAt.Time.UTC(), StartedAt: record.StartedAt.Time.UTC(),
		ProcessedRules: record.ProcessedRules, GeneratedSchedules: record.GeneratedSchedules,
		GeneratedOccurrences: record.GeneratedOccurrences,
		Response:             response,
	}
	if record.FinishedAt.Valid {
		finishedAt := record.FinishedAt.Time.UTC()
		operation.FinishedAt = &finishedAt
	}
	if record.ErrorCode.Valid {
		operation.ErrorCode = record.ErrorCode.String
	}
	if record.ErrorMessage.Valid {
		operation.ErrorMessage = record.ErrorMessage.String
	}
	if operation.State == "running" && !record.LeaseExpiresAt.Time.After(time.Now().UTC()) {
		finishedAt := record.LeaseExpiresAt.Time.UTC()
		operation.State = "failed"
		operation.FinishedAt = &finishedAt
		operation.ErrorCode = "interrupted"
		operation.ErrorMessage = "Operation execution lease expired. Run job again to resume from saved progress."
	}
	return operation, nil
}

func (r *OperationRepository) CreateOrGetActive(ctx context.Context, candidate dao.Operation, leaseToken string, lease time.Duration) (dao.Operation, bool, string, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return dao.Operation{}, false, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)
	if err := queries.AcquireAdminOperationLock(ctx, candidate.JobName); err != nil {
		return dao.Operation{}, false, "", err
	}
	if err := queries.FailExpiredAdminOperations(ctx, candidate.JobName); err != nil {
		return dao.Operation{}, false, "", err
	}
	activeRecord, err := queries.GetActiveAdminOperation(ctx, candidate.JobName)
	if err == nil {
		active, mapErr := operationFromRecord(activeRecord)
		if mapErr != nil {
			return dao.Operation{}, false, "", mapErr
		}
		if err := tx.Commit(ctx); err != nil {
			return dao.Operation{}, false, "", err
		}
		return active, false, "", nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return dao.Operation{}, false, "", err
	}
	createdRecord, err := queries.CreateAdminOperation(ctx, sqlc.CreateAdminOperationParams{
		ID: candidate.ID, JobName: candidate.JobName, RequestedBy: candidate.RequestedBy,
		Cutoff:         pgtype.Timestamptz{Time: candidate.Cutoff.UTC(), Valid: true},
		StartedAt:      pgtype.Timestamptz{Time: candidate.StartedAt.UTC(), Valid: true},
		LeaseExpiresAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(lease), Valid: true},
		LeaseToken:     leaseToken,
	})
	if err != nil {
		return dao.Operation{}, false, "", err
	}
	created, err := operationFromRecord(createdRecord)
	if err != nil {
		return dao.Operation{}, false, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return dao.Operation{}, false, "", err
	}
	return created, true, leaseToken, nil
}

func (r *OperationRepository) Get(ctx context.Context, id string) (dao.Operation, error) {
	record, err := r.queries.GetAdminOperation(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return dao.Operation{}, usecase.ErrOperationNotFound
	}
	if err != nil {
		return dao.Operation{}, err
	}
	return operationFromRecord(record)
}

func (r *OperationRepository) List(ctx context.Context, cursor string, limit int) ([]dao.Operation, error) {
	var encodedCursor pgtype.Text
	if cursor != "" {
		encodedCursor = pgtype.Text{String: cursor, Valid: true}
	}
	records, err := r.queries.ListAdminOperations(ctx, sqlc.ListAdminOperationsParams{Cursor: encodedCursor, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	items := make([]dao.Operation, 0, len(records))
	for _, record := range records {
		item, err := operationFromRecord(record)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *OperationRepository) Heartbeat(ctx context.Context, id, token string, lease time.Duration) (bool, error) {
	rows, err := r.queries.HeartbeatAdminOperation(ctx, sqlc.HeartbeatAdminOperationParams{
		LeaseExpiresAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(lease), Valid: true},
		ID:             id, LeaseToken: token,
	})
	return rows == 1, err
}

func (r *OperationRepository) Progress(ctx context.Context, id, token string, processed, generated int64, lease time.Duration) (bool, error) {
	rows, err := r.queries.ProgressAdminOperation(ctx, sqlc.ProgressAdminOperationParams{
		LeaseExpiresAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(lease), Valid: true},
		ProcessedRules: processed, GeneratedSchedules: generated, ID: id, LeaseToken: token,
	})
	return rows == 1, err
}

func (r *OperationRepository) Succeed(ctx context.Context, id, token string, processed, generated int64) error {
	operation, err := r.queries.GetAdminOperation(ctx, id)
	if err != nil {
		return err
	}
	generatedKey := "generated_schedules"
	if operation.JobName == "prefill-allocations" {
		generatedKey = "generated_occurrences"
	}
	response, err := json.Marshal(map[string]int64{"processed_rules": processed, generatedKey: generated})
	if err != nil {
		return err
	}
	rows, err := r.queries.SucceedAdminOperation(ctx, sqlc.SucceedAdminOperationParams{
		Response: response, ProcessedRules: processed, GeneratedSchedules: generated,
		ID: id, LeaseToken: token,
	})
	if err == nil && rows == 0 {
		return usecase.ErrLeaseLost
	}
	return err
}

func (r *OperationRepository) Fail(ctx context.Context, id, token, code, message string) error {
	rows, err := r.queries.FailAdminOperation(ctx, sqlc.FailAdminOperationParams{
		ErrorCode: code, ErrorMessage: message, ID: id, LeaseToken: token,
	})
	if err == nil && rows == 0 {
		return usecase.ErrLeaseLost
	}
	return err
}
