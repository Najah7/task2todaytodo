package domain

import (
	"errors"
	"time"
)

const defaultUserTimezone = "Asia/Tokyo"

var ErrInvalidUserTimezone = errors.New("invalid IANA timezone")

type UserTimezone string

func NewUserTimezone(value string) (UserTimezone, error) {
	if value == "Local" {
		return "", ErrInvalidUserTimezone
	}
	if _, err := time.LoadLocation(value); err != nil {
		return "", ErrInvalidUserTimezone
	}
	return UserTimezone(value), nil
}

func DefaultUserTimezone() UserTimezone {
	return UserTimezone(defaultUserTimezone)
}

func (t UserTimezone) String() string {
	return string(t)
}
