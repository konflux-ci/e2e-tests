package gitlab

import (
	"strings"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	Success TaskStatus = ":heavy_check_mark: SUCCESS"
	Failed  TaskStatus = ":heavy_check_mark: FAILED"
	Skipped TaskStatus = ":white_check_mark: SKIPPED"
	// Add more status constants as needed
)

// TaskInfo represents information about a task
type TaskInfo struct {
	Name     string
	Duration string
	Status   TaskStatus
	// Add more fields as needed
}

// ExtractTaskStatusFromNote extracts task status from a given note body
func (gc *GitlabClient) ExtractTaskStatusFromNote(noteBody string) ([]TaskInfo, error) {
	// Split the note body by lines
	lines := strings.Split(noteBody, "\n")

	var tasks []TaskInfo
	var taskStarted bool
	for _, line := range lines {
		// Start parsing when reaching the table header
		if strings.Contains(line, "| Task | Duration | Test Suite | Status | Details |") {
			taskStarted = true
			continue
		}

		// Stop parsing when reaching the end of the table
		if strings.HasPrefix(line, "| --- | --- | --- | --- | --- |") {
			break
		}

		// Parse each row of the table
		if taskStarted && strings.HasPrefix(line, "| ") {
			// Split the row by "|"
			cols := strings.Split(line[2:], "|")
			if len(cols) != 5 {
				// Invalid row format, skip
				continue
			}

			// Extract task information
			taskName := strings.TrimSpace(cols[0])
			taskDuration := strings.TrimSpace(cols[1])
			taskStatus := TaskStatus(strings.TrimSpace(cols[3]))

			tasks = append(tasks, TaskInfo{
				Name:     taskName,
				Duration: taskDuration,
				Status:   taskStatus,
			})
		}
	}

	return tasks, nil
}
