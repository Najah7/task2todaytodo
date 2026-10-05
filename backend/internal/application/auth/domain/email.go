package domain

import (
	"errors"
	"regexp"
)

var (
	ErrEmailEmpty         = errors.New("email cannot be empty")
	ErrInvalidEmailFormat = errors.New("invalid email format")
)

type Email string

func NewEmail(email string) (Email, error) {
	e := Email(email)
	if err := e.validate(); err != nil {
		return Email(""), err
	}
	return e, nil
}

func (e Email) validate() error {
	if e.String() == "" {
		return ErrEmailEmpty
	}

	const emailRegex = `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	re := regexp.MustCompile(emailRegex)
	if !re.MatchString(e.String()) {
		return ErrInvalidEmailFormat
	}
	return nil
}

func (e Email) String() string {
	return string(e)
}
