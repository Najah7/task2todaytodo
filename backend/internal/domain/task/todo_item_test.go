package task

import "testing"

func TestNewTodoItem(t *testing.T) {
	item, err := NewTodoItem("todo-item-1", "task-1", "Buy milk")
	if err != nil {
		t.Fatalf("NewTodoItem() error = %v", err)
	}

	if item.ID != "todo-item-1" || item.TaskID != "task-1" || item.Position != 0 {
		t.Errorf("todo item = %+v, want ID, task ID, and position to be set", item)
	}
	if item.Description != "" || item.Completed || item.Position != 0 || item.IntervalWeeks != 1 {
		t.Errorf("todo item = %+v, want only required values to be set", item)
	}
}

func TestNewTodoItemWithDetails(t *testing.T) {
	frequencies := TaskFrequencies{mustTaskFrequencyForService(t, "mon")}

	item, err := NewTodoItemWithDetails("todo-item-1", "task-1", "Buy milk", "At the store", true, 2, 3, frequencies)
	if err != nil {
		t.Fatalf("NewTodoItemWithDetails() error = %v", err)
	}

	if item.Description != "At the store" || !item.Completed || item.Position != 2 || item.IntervalWeeks != 3 || !item.Frequencies.IsWeekday() {
		t.Errorf("todo item = %+v, want details to be set", item)
	}
}

func TestNewTodoItemValidation(t *testing.T) {
	tests := []struct {
		name          string
		id            TodoItemID
		taskID        TaskID
		title         string
		position      int
		intervalWeeks int
		wantErr       error
	}{
		{name: "empty ID", taskID: "task-1", title: "Todo", position: 0, intervalWeeks: 1, wantErr: ErrTodoItemIDEmpty},
		{name: "empty task ID", id: "todo-item-1", title: "Todo", position: 0, intervalWeeks: 1, wantErr: ErrTodoItemTaskIDEmpty},
		{name: "blank title", id: "todo-item-1", taskID: "task-1", title: " ", position: 0, intervalWeeks: 1, wantErr: ErrTodoItemTitleEmpty},
		{name: "negative position", id: "todo-item-1", taskID: "task-1", title: "Todo", position: -1, intervalWeeks: 1, wantErr: ErrTodoItemPositionLess},
		{name: "interval weeks less than 0", id: "todo-item-1", taskID: "task-1", title: "Todo", position: 0, intervalWeeks: -1, wantErr: ErrTodoItemIntervalWeeksLess},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewTodoItemWithDetails(tt.id, tt.taskID, tt.title, "", false, tt.position, tt.intervalWeeks, nil)
			assertTaskDomainErrorIs(t, err, tt.wantErr)
			if got.ID != tt.id || got.TaskID != tt.taskID || got.Title != tt.title {
				t.Errorf("todo item = %+v, want input values to be preserved", got)
			}
		})
	}
}

func TestTodoItemRepeatPattern(t *testing.T) {
	once, err := NewTodoItemWithDetails("todo-item-1", "task-1", "Buy milk", "", false, 0, 0, nil)
	if err != nil {
		t.Fatalf("NewTodoItemWithDetails() once error = %v", err)
	}
	if !once.IsOnce() || once.IsWeekly() {
		t.Errorf("todo item = %+v, want interval 0 to mean once", once)
	}

	weekly, err := NewTodoItemWithDetails("todo-item-1", "task-1", "Buy milk", "", false, 0, 1, TaskFrequencies{mustTaskFrequencyForService(t, "mon")})
	if err != nil {
		t.Fatalf("NewTodoItemWithDetails() weekly error = %v", err)
	}
	if weekly.IsOnce() || !weekly.IsWeekly() || !weekly.IsEveryWeekday() {
		t.Errorf("todo item = %+v, want interval 1 with weekday frequency to mean weekly weekday", weekly)
	}
}
