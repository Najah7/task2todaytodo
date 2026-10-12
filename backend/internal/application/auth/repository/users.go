package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Najah7/task2todaytodo/db/sqlc"
	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	"github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	"github.com/Najah7/task2todaytodo/internal/application/auth/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ usecase.UserRepository = UserRepository{}

type UserRepository struct {
	queries *sqlc.Queries
}

func NewUserRepository(db sqlc.DBTX) *UserRepository {
	queries := sqlc.New(db)
	return &UserRepository{
		queries: queries,
	}
}

func (r *UserRepository) WithTx(tx pgx.Tx) *UserRepository {
	return &UserRepository{
		queries: r.queries.WithTx(tx),
	}
}

func (r UserRepository) GetTimezone(ctx context.Context, userID string) (string, error) {
	return r.queries.GetUserTimezone(ctx, userID)
}

func (r UserRepository) TimezoneExists(ctx context.Context, timezone string) (bool, error) {
	return r.queries.IsTimezoneAllowed(ctx, timezone)
}

func (r UserRepository) ListTimezones(ctx context.Context) ([]string, error) {
	return r.queries.GetTimezoneChoices(ctx)
}

var _ usecase.TimezoneRepository = UserRepository{}

func recordToUserDAO(record sqlc.User) dao.User {
	return dao.User{
		ID:         record.ID,
		FirstName:  record.FirstName,
		LastName:   record.LastName,
		Email:      record.Email,
		Password:   record.Password,
		Timezone:   record.Timezone,
		WakeUpTime: pgTimeString(record.WakeUpTime),
		Bedtime:    pgTimeString(record.Bedtime),
		CreatedAt:  pgTimeUnix(record.CreatedAt),
		UpdatedAt:  pgTimeUnix(record.UpdatedAt),
	}
}

func pgTimeString(value pgtype.Time) string {
	if !value.Valid {
		return ""
	}
	micros := value.Microseconds
	hour := micros / 3_600_000_000
	minute := micros / 60_000_000 % 60
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func (r UserRepository) Get(ctx context.Context, id domain.UserID) (dao.User, error) {
	u, err := r.queries.GetUser(ctx, string(id))
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	return result, nil
}

func (r UserRepository) GetByEmail(ctx context.Context, email string) (dao.User, error) {
	u, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	return result, nil
}

func (r UserRepository) Create(ctx context.Context, user domain.User) (dao.User, error) {
	u, err := r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:        string(user.ID),
		Email:     user.Email.String(),
		Password:  user.Password.String(),
		FirstName: user.Name.FirstName,
		LastName:  user.Name.LastName,
	})
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	return result, nil
}

func (r UserRepository) Update(ctx context.Context, user domain.User) (dao.User, error) {
	u, err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:        string(user.ID),
		Email:     user.Email.String(),
		Password:  user.Password.String(),
		FirstName: user.Name.FirstName,
		LastName:  user.Name.LastName,
	})
	if err != nil {
		return dao.User{}, err
	}
	result := recordToUserDAO(u)
	return result, nil
}

func (r UserRepository) UpdateTimezone(ctx context.Context, id domain.UserID, timezone domain.UserTimezone) error {
	_, err := r.queries.UpdateUserTimezone(ctx, sqlc.UpdateUserTimezoneParams{ID: string(id), Timezone: timezone.String()})
	return err
}

func (r UserRepository) UpdateSleepSettings(ctx context.Context, id domain.UserID, wakeUpTime, bedtime string) error {
	wake, err := parsePGTime(wakeUpTime)
	if err != nil {
		return err
	}
	bed, err := parsePGTime(bedtime)
	if err != nil {
		return err
	}
	return r.queries.UpdateUserSleepSettings(ctx, sqlc.UpdateUserSleepSettingsParams{WakeUpTime: wake, Bedtime: bed, UserID: string(id)})
}

func parsePGTime(value string) (pgtype.Time, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return pgtype.Time{}, err
	}
	return pgtype.Time{Microseconds: int64(parsed.Hour())*3_600_000_000 + int64(parsed.Minute())*60_000_000, Valid: true}, nil
}
