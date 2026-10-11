package progress

import "testing"

func TestTaskPercent(t *testing.T) {
	tests := []struct {
		name      string
		done      bool
		completed int
		total     int
		want      int
	}{
		{name: "done overrides counts", done: true, completed: 1, total: 3, want: 100},
		{name: "completed items round down", completed: 2, total: 3, want: 66},
		{name: "no children", completed: 0, total: 0, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := TaskPercent(test.done, test.completed, test.total); got != test.want {
				t.Fatalf("TaskPercent() = %d, want %d", got, test.want)
			}
		})
	}
}
