package application

import (
	"context"

	authdomain "github.com/Najah7/task2todaytodo/internal/application/auth/domain"
	authrepo "github.com/Najah7/task2todaytodo/internal/application/auth/repository"
	taskdomain "github.com/Najah7/task2todaytodo/internal/application/task/domain"
	taskusecase "github.com/Najah7/task2todaytodo/internal/application/task/usecase"
)

var _ taskusecase.UserTimezoneReader = userTimezoneReader{}

type userTimezoneReader struct {
	users *authrepo.UserRepository
}

func (r userTimezoneReader) GetTimezone(ctx context.Context, userID taskdomain.UserID) (string, error) {
	user, err := r.users.Get(ctx, authdomain.UserID(userID))
	if err != nil {
		return "", err
	}
	return user.Timezone, nil
}
