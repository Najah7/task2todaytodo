package shared

import "testing"

func TestScheduleCapabilities(t *testing.T) {
	tests := []struct {
		name string
		got  Capability
		want Capability
	}{
		{"create", ScheduleCreate(), Capability{ResourceSchedule, ActionCreate}},
		{"read", ScheduleRead(), Capability{ResourceSchedule, ActionRead}},
		{"update", ScheduleUpdate(), Capability{ResourceSchedule, ActionUpdate}},
		{"delete", ScheduleDelete(), Capability{ResourceSchedule, ActionDelete}},
		{"assignment update", ScheduleAssignmentUpdate(), Capability{ResourceScheduleAssignment, ActionUpdate}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("capability = %+v, want %+v", test.got, test.want)
			}
		})
	}
}
