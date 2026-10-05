package usecase

import "errors"

var (
	ErrPasswordUpdateFailed   = errors.New("password update failed")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrUserEmailAlreadyExists = errors.New("user with this email already exists")
	ErrUserIDAlreadyExists    = errors.New("user with this ID already exists")
)
