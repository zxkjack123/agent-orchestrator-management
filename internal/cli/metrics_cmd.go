package cli

import (
	"fmt"
	"strings"
	"time"
)

func (r Runner) executeMetrics(args []string) error {
	days := 7
	filterTaskID := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--days":
			i++
			if i >= len(args) {
				return fmt.Errorf("--days requires a value")
			}
			n := 0
			if _, err := fmt.Sscanf(args[i], "%d", &n); err != nil || n <= 0 {
				return fmt.Errorf("--days must be a positive integer")
			}
			days = n
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			filterTaskID = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	allTasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	if filterTaskID != "" {
		filtered := allTasks[:0]
		for _, t := range allTasks {
			if t.ID == filterTaskID {
				filtered = append(filtered, t)
				break
			}
		}
		allTasks = filtered
	}

	logDir := logDirForTask(result.Project.RepoPath, result.StateDir)
	report := BuildVelocityReport(allTasks, logDir, days, time.Now())
	PrintVelocityReport(r.stdout, report)
	return nil
}
