package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// taskLogEntry binds a task ID to its log.md path for multi-task polling.
type taskLogEntry struct {
	TaskID  string
	LogPath string
}

// waitForLogEvent polls logPath every 3 seconds until it finds a log.md heading
// line that contains "| <eventType>". Returns the matched heading line or an error
// when the deadline is exceeded.
func waitForLogEvent(logPath, eventType string, timeout time.Duration) (string, error) {
	marker := "| " + eventType
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if line, found := scanLogForEvent(logPath, marker); found {
			return line, nil
		}
		time.Sleep(3 * time.Second)
	}

	return "", fmt.Errorf("timed out after %s waiting for event %q in %s", timeout, eventType, logPath)
}

// tailLogEvents streams new log.md lines to out as they appear, polling every 2
// seconds. Only non-empty lines are printed. Returns an error on timeout.
func tailLogEvents(out io.Writer, logPath string, timeout time.Duration) error {
	startData, _ := os.ReadFile(logPath)
	lastOffset := len(startData)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		data, err := os.ReadFile(logPath)
		if err != nil || len(data) <= lastOffset {
			continue
		}

		newPart := string(data[lastOffset:])
		lastOffset = len(data)

		for _, line := range strings.Split(newPart, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				fmt.Fprintln(out, trimmed)
			}
		}
	}

	return nil
}

func scanLogForEvent(logPath, marker string) (string, bool) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "", false
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "### ") && strings.Contains(line, marker) {
			return strings.TrimSpace(line), true
		}
	}

	return "", false
}

// tailMultiTaskLogEvents polls every task log in tasks and streams new non-empty
// lines to out, prefixed with the task ID. Returns an error when timeout expires.
func tailMultiTaskLogEvents(out io.Writer, tasks []taskLogEntry, timeout time.Duration) error {
	lastOffsets := make(map[string]int, len(tasks))
	for _, t := range tasks {
		data, _ := os.ReadFile(t.LogPath)
		lastOffsets[t.LogPath] = len(data)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		for _, t := range tasks {
			data, err := os.ReadFile(t.LogPath)
			if err != nil || len(data) <= lastOffsets[t.LogPath] {
				continue
			}

			newPart := string(data[lastOffsets[t.LogPath]:])
			lastOffsets[t.LogPath] = len(data)

			for _, line := range strings.Split(newPart, "\n") {
				if trimmed := strings.TrimSpace(line); trimmed != "" {
					fmt.Fprintf(out, "[%s] %s\n", t.TaskID, trimmed)
				}
			}
		}
	}

	return nil
}

// waitForMultiTaskLogEvent polls all task logs until any of them contains a
// heading line with the given event type marker. Returns the matching task ID
// and the matched log line, or an error when timeout expires.
func waitForMultiTaskLogEvent(tasks []taskLogEntry, eventType string, timeout time.Duration) (taskID, line string, err error) {
	marker := "| " + eventType
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		for _, t := range tasks {
			if matched, found := scanLogForEvent(t.LogPath, marker); found {
				return t.TaskID, matched, nil
			}
		}
		time.Sleep(3 * time.Second)
	}

	return "", "", fmt.Errorf("timed out after %s waiting for event %q across %d tasks", timeout, eventType, len(tasks))
}
