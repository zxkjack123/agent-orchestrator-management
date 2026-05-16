package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/project"
	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/task"
)

func (r Runner) executeNext(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	taskService, sqlDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	unblocked, err := taskService.ListUnblocked(result.Project.ID)
	if err != nil {
		return err
	}

	all, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	// Collect blocked tasks (have at least one active blocker).
	unblockedIDs := make(map[string]bool)
	for _, t := range unblocked {
		unblockedIDs[t.ID] = true
	}

	var blocked []task.Record
	for _, t := range all {
		if t.Status == "Done" || t.Status == "Archived" {
			continue
		}
		if !unblockedIDs[t.ID] {
			blocked = append(blocked, t)
		}
	}

	fmt.Fprintln(r.stdout, "Next tasks")
	fmt.Fprintln(r.stdout, "")

	if len(unblocked) == 0 {
		fmt.Fprintln(r.stdout, "Unblocked: none")
	} else {
		fmt.Fprintln(r.stdout, "Unblocked (work on these next):")
		for i, t := range unblocked {
			priority := task.PriorityLabel(t.Priority)
			owner := emptyFallback(t.PreferredAgent)
			if owner == "-" {
				owner = emptyFallback(t.PreferredRole)
			}
			fmt.Fprintf(r.stdout, "  %d. [%s] %s  %s  owner=%s\n", i+1, priority, t.ID, t.Title, owner)
		}
	}

	if len(blocked) > 0 {
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintln(r.stdout, "Blocked (waiting on dependencies):")
		for _, t := range blocked {
			blockers, _ := taskService.BlockedBy(t.ID)
			blockerIDs := make([]string, 0, len(blockers))
			for _, b := range blockers {
				if b.Status != "Done" && b.Status != "Archived" {
					blockerIDs = append(blockerIDs, b.ID)
				}
			}
			fmt.Fprintf(r.stdout, "  %s  %s  waiting on: %s\n", t.ID, t.Title, strings.Join(blockerIDs, ", "))
		}
	}

	return nil
}


func (r Runner) executeWatch(args []string) error {
	taskID := ""
	eventType := ""
	watchTimeout := 30 * time.Minute

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--task":
			i++
			if i >= len(args) {
				return fmt.Errorf("--task requires a value")
			}
			taskID = strings.TrimSpace(args[i])
		case "--event":
			i++
			if i >= len(args) {
				return fmt.Errorf("--event requires a value")
			}
			eventType = strings.TrimSpace(args[i])
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--timeout value %q is not a valid duration (e.g. 30m, 2h): %w", args[i], err)
			}
			watchTimeout = d
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	if taskID != "" {
		return r.executeWatchSingleTask(result, taskID, eventType, watchTimeout)
	}

	return r.executeWatchAllTasks(result, eventType, watchTimeout)
}

// executeWatchSingleTask is the original single-task watch path.
func (r Runner) executeWatchSingleTask(result *project.OpenResult, taskID, eventType string, watchTimeout time.Duration) error {
	view, err := r.loadTaskView(result, taskID)
	if err != nil {
		return err
	}
	if view == nil {
		return fmt.Errorf("task %q not found", taskID)
	}

	logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, taskID, view.Worktree)

	if eventType != "" {
		fmt.Fprintf(r.stdout, "Watching for event %q\n", eventType)
		fmt.Fprintf(r.stdout, "Task: %s\n", taskID)
		fmt.Fprintf(r.stdout, "Log: %s\n", logPath)
		fmt.Fprintf(r.stdout, "Timeout: %s\n\n", watchTimeout)

		line, err := waitForLogEvent(logPath, eventType, watchTimeout)
		if err != nil {
			if strings.Contains(err.Error(), "timed out") {
				fmt.Fprintf(r.stdout, "Watch timeout reached (%s) — event %q not detected.\n", watchTimeout, eventType)
				return nil
			}
			return err
		}

		fmt.Fprintln(r.stdout, "Event detected")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
		fmt.Fprintf(r.stdout, "Entry: %s\n", line)
		return nil
	}

	fmt.Fprintf(r.stdout, "Watching task %s (tail mode, timeout %s)\n", taskID, watchTimeout)
	fmt.Fprintf(r.stdout, "Log: %s\n\n", logPath)

	return tailLogEvents(r.stdout, logPath, watchTimeout)
}

// executeWatchAllTasks watches all active tasks simultaneously.
func (r Runner) executeWatchAllTasks(result *project.OpenResult, eventType string, watchTimeout time.Duration) error {
	taskService, taskDB, err := r.app.OpenTaskService(result.DBPath)
	if err != nil {
		return err
	}
	defer taskDB.Close()

	allTasks, err := taskService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	worktreeService, worktreeDB, err := r.app.OpenWorktreeService(result.DBPath)
	if err != nil {
		return err
	}
	defer worktreeDB.Close()

	activeStatuses := map[string]bool{
		"InProgress":     true,
		"Blocked":        true,
		"NeedsAttention": true,
		"Ready":          true,
	}

	var entries []taskLogEntry
	for _, t := range allTasks {
		if !activeStatuses[t.Status] {
			continue
		}
		mapping, _ := worktreeService.GetByTask(t.ID)
		logPath := taskArtifactLogPath(result.Project.RepoPath, result.StateDir, t.ID, mapping)
		entries = append(entries, taskLogEntry{TaskID: t.ID, LogPath: logPath})
	}

	if len(entries) == 0 {
		fmt.Fprintln(r.stdout, "No active tasks to watch.")
		return nil
	}

	if eventType != "" {
		fmt.Fprintf(r.stdout, "Watching %d active task(s) for event %q (timeout %s)\n", len(entries), eventType, watchTimeout)
		for _, e := range entries {
			fmt.Fprintf(r.stdout, "  %s → %s\n", e.TaskID, e.LogPath)
		}
		fmt.Fprintln(r.stdout, "")

		matchedTask, matchedLine, err := waitForMultiTaskLogEvent(entries, eventType, watchTimeout)
		if err != nil {
			// Timeout is expected; print a note and exit 0 rather than propagating the error.
			if strings.Contains(err.Error(), "timed out") {
				fmt.Fprintf(r.stdout, "Watch timeout reached (%s) — event %q not detected.\n", watchTimeout, eventType)
				return nil
			}
			return err
		}

		fmt.Fprintln(r.stdout, "Event detected")
		fmt.Fprintln(r.stdout, "")
		fmt.Fprintf(r.stdout, "Task: %s\n", matchedTask)
		fmt.Fprintf(r.stdout, "Event: %s\n", eventType)
		fmt.Fprintf(r.stdout, "Entry: %s\n", matchedLine)
		return nil
	}

	fmt.Fprintf(r.stdout, "Watching %d active task(s) (tail mode, timeout %s)\n", len(entries), watchTimeout)
	for _, e := range entries {
		fmt.Fprintf(r.stdout, "  %s → %s\n", e.TaskID, e.LogPath)
	}
	fmt.Fprintln(r.stdout, "")

	return tailMultiTaskLogEvents(r.stdout, entries, watchTimeout)
}


// ── M14: task request / team brief ──────────────────────────────────────────

func (r Runner) executeTeam(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("team subcommand is required (try: team brief)")
	}

	switch args[0] {
	case "brief":
		return r.executeTeamBrief(args[1:])
	default:
		return fmt.Errorf("unknown team command %q", args[0])
	}
}

func (r Runner) executeTeamBrief(args []string) error {
	_ = args

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

	sessionService, sessDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sessDB.Close()

	sessions, err := sessionService.ListByProject(result.Project.ID)
	if err != nil {
		return err
	}

	// Build session status index by task.
	taskSessionStatus := make(map[string]string)
	for _, s := range sessions {
		if s.TaskID != "" {
			taskSessionStatus[s.TaskID] = s.Status
		}
	}

	// Build agent session status index.
	agentSessionStatus := make(map[string]string)
	for _, s := range sessions {
		if s.AgentName != "" {
			agentSessionStatus[s.AgentName] = s.Status
		}
	}

	var briefTasks []artifact.TeamBriefTask
	for _, t := range allTasks {
		if t.Status == "Done" || t.Status == "Archived" {
			continue
		}
		blockers, _ := taskService.BlockedBy(t.ID)
		blockerIDs := make([]string, 0, len(blockers))
		for _, b := range blockers {
			if b.Status != "Done" && b.Status != "Archived" {
				blockerIDs = append(blockerIDs, b.ID)
			}
		}
		owner := t.PreferredAgent
		if owner == "" {
			owner = t.PreferredRole
		}
		briefTasks = append(briefTasks, artifact.TeamBriefTask{
			ID:        t.ID,
			Title:     t.Title,
			Status:    t.Status,
			Priority:  task.PriorityLabel(t.Priority),
			Agent:     owner,
			BlockedBy: blockerIDs,
		})
	}

	// Pending requests.
	pendingReqs, _ := readPendingRequests(result.Project.RepoPath)
	var reqLines []string
	for _, req := range pendingReqs {
		reqLines = append(reqLines, fmt.Sprintf("%s: \"%s\" from %s [%s]",
			req.ID, req.Title, emptyFallback(req.RequestedBy), req.Priority))
	}

	// Last 5 channel messages (raw lines from channel.md).
	channelTail := lastChannelMessages(result.Project.RepoPath, 5)

	// Agents from config.
	var briefAgents []artifact.TeamBriefAgent
	for _, a := range result.Agents {
		status := agentSessionStatus[a.Name]
		briefAgents = append(briefAgents, artifact.TeamBriefAgent{
			Name:          a.Name,
			Role:          a.Role,
			Runtime:       a.Runtime,
			SessionStatus: status,
		})
	}

	svc := artifact.NewService(result.Project.RepoPath, result.StateDir)
	briefPath, err := svc.GenerateTeamBrief(artifact.TeamBriefParams{
		ProjectName:     result.Project.Name,
		Tasks:           briefTasks,
		PendingRequests: reqLines,
		ChannelTail:     channelTail,
		Agents:          briefAgents,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "Team brief generated\n\n")
	fmt.Fprintf(r.stdout, "Path:    %s\n", briefPath)
	fmt.Fprintf(r.stdout, "Tasks:   %d active\n", len(briefTasks))
	fmt.Fprintf(r.stdout, "Pending requests: %d\n", len(pendingReqs))
	return nil
}
