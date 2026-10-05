package usecase

import (
	"github.com/Najah7/task2todaytodo/internal/application/auth/dao"
	domain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
)

func restoreUser(record dao.User) (domain.User, error) {
	userID := domain.UserID(record.ID)

	email, err := domain.NewEmail(record.Email)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	password, err := domain.NewHashedPassword(record.Password)
	if err != nil {
		return domain.NewZeroUser(), err
	}

	user := domain.NewUser(
		userID,
		email,
		password,
		domain.NewUserName(record.FirstName, record.LastName),
	)
	timezone := record.Timezone
	if timezone == "" {
		timezone = domain.DefaultUserTimezone().String()
	}
	userTimezone, err := domain.NewUserTimezone(timezone)
	if err != nil {
		return domain.NewZeroUser(), err
	}
	return user.UpdateTimezone(userTimezone), nil
}

func userWithoutPassword(record dao.User) dao.User {
	record.Password = ""
	return record
}
