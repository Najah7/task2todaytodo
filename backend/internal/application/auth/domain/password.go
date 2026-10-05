package domain

import (
	"errors"
	"regexp"
)

const PasswordMinLength = 8

var (
	ErrPasswordEmpty            = errors.New("password cannot be empty")
	ErrPasswordTooShort         = errors.New("password must be at least 8 characters long")
	ErrPasswordMissingLowercase = errors.New("password must contain at least one lowercase letter")
	ErrPasswordMissingUppercase = errors.New("password must contain at least one uppercase letter")
	ErrPasswordMissingDigit     = errors.New("password must contain at least one digit")
	ErrPasswordMissingSpecial   = errors.New("password must contain at least one special character")
)

type Password struct {
	Value    string
	IsHashed bool
}

func NewPassword(value string) (Password, error) {
	if value == "" {
		return Password{}, ErrPasswordEmpty
	}

	p := Password{
		Value:    value,
		IsHashed: false,
	}
	err := p.Validate()
	if err != nil {
		return Password{}, err
	}

	passHash, err := p.Hash()
	if err != nil {
		return Password{}, err
	}

	return passHash, nil
}

// NewHashedPassword restores a password value that was previously persisted.
func NewHashedPassword(value string) (Password, error) {
	if value == "" {
		return Password{}, ErrPasswordEmpty
	}

	return Password{
		Value:    value,
		IsHashed: true,
	}, nil
}

func (p Password) Validate() error {
	if len(p.Value) < PasswordMinLength {
		return ErrPasswordTooShort
	}

	if !regexp.MustCompile(`[a-z]`).MatchString(p.String()) {
		return ErrPasswordMissingLowercase
	}

	if !regexp.MustCompile(`[A-Z]`).MatchString(p.String()) {
		return ErrPasswordMissingUppercase
	}

	if !regexp.MustCompile(`[0-9]`).MatchString(p.String()) {
		return ErrPasswordMissingDigit
	}

	if !regexp.MustCompile(`[!@#\$%\^&\*\(\)_\+\-=\[\]\{\};':"\\|,.<>\/?]`).MatchString(p.String()) {
		return ErrPasswordMissingSpecial
	}

	return nil
}

func (p Password) Hash() (Password, error) {
	if p.IsHashed {
		return Password{}, ErrPasswordMustNotBeHashed
	}

	// TODO: Implement a proper hashing mechanism here. For demonstration purposes, we'll just prepend "hashed_" to the password value.
	hashedValue := "hashed_" + p.Value

	return Password{
		Value:    hashedValue,
		IsHashed: true,
	}, nil
}

func (p Password) String() string {
	return p.Value
}
