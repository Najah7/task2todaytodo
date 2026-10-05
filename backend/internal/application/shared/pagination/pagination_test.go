package pagination

import (
	"reflect"
	"testing"
)

func TestWindow(t *testing.T) {
	tests := []struct {
		name     string
		rows     []int
		pageSize int
		want     []int
		hasMore  bool
	}{
		{name: "empty nil", rows: nil, pageSize: 2, want: []int{}},
		{name: "empty slice", rows: []int{}, pageSize: 2, want: []int{}},
		{name: "short page", rows: []int{1, 2}, pageSize: 3, want: []int{1, 2}},
		{name: "exact page", rows: []int{1, 2}, pageSize: 2, want: []int{1, 2}},
		{name: "limit plus one", rows: []int{1, 2, 3}, pageSize: 2, want: []int{1, 2}, hasMore: true},
		{name: "zero size", rows: []int{1}, pageSize: 0, want: []int{}, hasMore: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hasMore := Window(tt.rows, tt.pageSize)
			if !reflect.DeepEqual(got, tt.want) || hasMore != tt.hasMore {
				t.Fatalf("Window() = %#v, %t; want %#v, %t", got, hasMore, tt.want, tt.hasMore)
			}
			if got == nil {
				t.Fatal("Window() returned nil items")
			}
		})
	}
}
