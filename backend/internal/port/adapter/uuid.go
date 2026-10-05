package adapter

import "github.com/google/uuid"

type UUID struct{}

func NewUUID() UUID { return UUID{} }

func (UUID) Generate() string { return uuid.NewString() }
