package progress

// TaskPercent returns the exposed whole percentage for a Task. The exact
// completed/total comparison remains separately available to Task mutations.
func TaskPercent(done bool, completed, total int) int {
	if done {
		return 100
	}
	if total < 1 {
		return 0
	}
	return 100 * completed / total
}
