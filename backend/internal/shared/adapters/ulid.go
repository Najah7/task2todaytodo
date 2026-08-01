package adapters

import (
	"github.com/Najah7/task2todaytodo/internal/shared"
	"github.com/oklog/ulid/v2"
)

var _ shared.IDGenerator = ULIDGenerator{}

type ULIDGenerator struct{}

func NewULIDGenerator() ULIDGenerator {
	return ULIDGenerator{}
}

func (g ULIDGenerator) Generate() string {
	return ulid.Make().String()
}
