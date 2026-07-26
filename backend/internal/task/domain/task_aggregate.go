package domain

type TaskAggregate struct {
	Task          Task
	TodoItems     []TodoItem
	TaskSchedules []TaskSchedule
}
