// Package pagination handles List query parameters and cursor tokens for REST.
package pagination

import (
	"errors"
	"strconv"

	sharedpagination "github.com/Najah7/task2todaytodo/internal/application/shared/pagination"
)

var ErrInvalidPageSize = errors.New("invalid page size")

// ParsePageSize parses page_size, applying the API default and maximum.
// An omitted value and zero both select the shared default; negative and
// non-integer values are invalid.
func ParsePageSize(value string, provided bool) (int, error) {
	if !provided {
		return sharedpagination.DefaultPageSize, nil
	}
	size, err := strconv.Atoi(value)
	if err != nil || size < 0 {
		return 0, ErrInvalidPageSize
	}
	if size == 0 {
		return sharedpagination.DefaultPageSize, nil
	}
	if size > sharedpagination.MaxPageSize {
		return sharedpagination.MaxPageSize, nil
	}
	return size, nil
}
