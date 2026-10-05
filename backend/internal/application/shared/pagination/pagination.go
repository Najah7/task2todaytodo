// Package pagination contains transport-independent List pagination policy.
package pagination

const (
	DefaultPageSize = 50
	MaxPageSize     = 100
)

// Window turns a LIMIT pageSize+1 result into the items to return and a
// hasMore flag. It always returns a non-nil items slice.
func Window[T any](rows []T, pageSize int) (items []T, hasMore bool) {
	if rows == nil {
		return []T{}, false
	}
	if pageSize < 0 {
		pageSize = 0
	}
	if len(rows) > pageSize {
		return rows[:pageSize], true
	}
	return rows, false
}
