package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/task"
)

// VelocityRow holds per-agent metrics.
type VelocityRow struct {
	AgentName     string
	Owned         int
	Completed     int
	AvgDuration   time.Duration
	BlockedEvents int
}

// VelocityReport is the output of a metrics run.
type VelocityReport struct {
	Days          int
	Completed     int
	AvgDuration   time.Duration
	LongBlocked   int // tasks blocked > 1h
	BottleneckHint string
	Agents        []VelocityRow
}

// BuildVelocityReport derives metrics from task records and log files.
func BuildVelocityReport(tasks []task.Record, logDir func(t task.Record) string, days int, now time.Time) VelocityReport {
	cutoff := now.AddDate(0, 0, -days)

	var report VelocityReport
	report.Days = days

	agentMap := make(map[string]*VelocityRow)

	for _, t := range tasks {
		if t.CreatedAt.Before(cutoff) {
			continue
		}

		agent := t.PreferredAgent
		if agent == "" {
			agent = t.PreferredRole
		}
		if agent == "" {
			agent = "unassigned"
		}

		row := agentMap[agent]
		if row == nil {
			agentMap[agent] = &VelocityRow{AgentName: agent}
			row = agentMap[agent]
		}
		row.Owned++

		if t.Status == "Done" || t.Status == "Archived" {
			report.Completed++
			row.Completed++

			duration := t.UpdatedAt.Sub(t.CreatedAt)
			if duration > 0 {
				row.AvgDuration = (row.AvgDuration*time.Duration(row.Completed-1) + duration) / time.Duration(row.Completed)
			}
		}

		// Count block events and long-blocked tasks from log.
		blockCount, maxBlockDuration := parseBlockEvents(logDir(t))
		row.BlockedEvents += blockCount
		if maxBlockDuration > time.Hour {
			report.LongBlocked++
		}
	}

	// Overall avg duration.
	var totalDuration time.Duration
	total := 0
	for _, row := range agentMap {
		if row.Completed > 0 {
			totalDuration += row.AvgDuration * time.Duration(row.Completed)
			total += row.Completed
		}
		report.Agents = append(report.Agents, *row)
	}
	if total > 0 {
		report.AvgDuration = totalDuration / time.Duration(total)
	}

	// Bottleneck: agent with most block events.
	maxBlocked := 0
	for _, row := range report.Agents {
		if row.BlockedEvents > maxBlocked {
			maxBlocked = row.BlockedEvents
			report.BottleneckHint = fmt.Sprintf("%s has been blocked %d time(s) in this period", row.AgentName, row.BlockedEvents)
		}
	}
	if maxBlocked == 0 {
		report.BottleneckHint = "no blocked events detected"
	}

	return report
}

// parseBlockEvents counts Blocked state entries and longest block duration from a log file.
func parseBlockEvents(logPath string) (count int, maxDuration time.Duration) {
	if logPath == "" {
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		return
	}

	var lastBlockedAt time.Time
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "### ") {
			continue
		}
		content := strings.TrimPrefix(line, "### ")
		parts := strings.SplitN(content, " | ", 3)
		if len(parts) < 2 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		eventType := strings.TrimSpace(parts[1])

		if strings.Contains(eventType, "task.blocked") || strings.Contains(eventType, "Task Blocked") {
			count++
			lastBlockedAt = ts
		} else if strings.Contains(eventType, "task.ready") || strings.Contains(eventType, "task.in_progress") {
			if !lastBlockedAt.IsZero() {
				d := ts.Sub(lastBlockedAt)
				if d > maxDuration {
					maxDuration = d
				}
				lastBlockedAt = time.Time{}
			}
		}
	}
	return
}

// PrintVelocityReport writes the formatted metrics report to w.
func PrintVelocityReport(w io.Writer, r VelocityReport) {
	fmt.Fprintf(w, "Team metrics (last %d days)\n\n", r.Days)
	fmt.Fprintf(w, "Summary\n")
	fmt.Fprintf(w, "  Tasks completed:  %d\n", r.Completed)
	if r.Completed > 0 {
		fmt.Fprintf(w, "  Avg duration:     %s\n", formatDuration(r.AvgDuration))
	}
	fmt.Fprintf(w, "  Long-blocked (>1h): %d\n", r.LongBlocked)
	fmt.Fprintf(w, "  Bottleneck:         %s\n\n", r.BottleneckHint)

	if len(r.Agents) > 0 {
		fmt.Fprintf(w, "Per-agent\n")
		fmt.Fprintf(w, "  %-30s  %6s  %9s  %12s  %7s\n",
			"Agent", "Owned", "Completed", "Avg duration", "Blocked")
		fmt.Fprintf(w, "  %-30s  %6s  %9s  %12s  %7s\n",
			strings.Repeat("-", 30), "------", "---------", "------------", "-------")
		for _, row := range r.Agents {
			avg := "-"
			if row.Completed > 0 {
				avg = formatDuration(row.AvgDuration)
			}
			fmt.Fprintf(w, "  %-30s  %6d  %9d  %12s  %7d\n",
				row.AgentName, row.Owned, row.Completed, avg, row.BlockedEvents)
		}
	}
}

// logDirForTask resolves the log.md path given a task record.
func logDirForTask(repoPath, stateDir string) func(t task.Record) string {
	return func(t task.Record) string {
		return filepath.Join(repoPath, ".aom", stateDir, t.ID, "log.md")
	}
}
