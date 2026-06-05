package cli

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/artifact"
	"github.com/lattapon-aek/agent-orchestrator-management/internal/events"
)

func (r Runner) executeApprove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	record, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if record.Status != "WaitingApproval" {
		return fmt.Errorf("session %q is not waiting for approval (status: %s)", record.ID, record.Status)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record.Status = "Idle"
	updated, err := sessionService.Save(*record)
	if err != nil {
		return err
	}

	if strings.TrimSpace(record.TaskID) != "" {
		_ = r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
			Type:        "approval.approved",
			Actor:       "operator",
			SessionID:   record.ID,
			Summary:     fmt.Sprintf("Operator approved pending request for session %s", record.ID),
			StateEffect: "Session Idle",
		}, false)
	}

	fmt.Fprintln(r.stdout, "Approved")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Status: %s\n", updated.Status)
	return nil
}

func (r Runner) executeDeny(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}

	sessionIdentifier := strings.TrimSpace(args[0])
	reason := ""
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	record, err := r.loadSessionByIdentifier(sessionIdentifier)
	if err != nil {
		return err
	}
	if record.Status != "WaitingApproval" {
		return fmt.Errorf("session %q is not waiting for approval (status: %s)", record.ID, record.Status)
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	sessionService, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	record.Status = "Blocked"
	updated, err := sessionService.Save(*record)
	if err != nil {
		return err
	}

	if strings.TrimSpace(record.TaskID) != "" {
		summary := fmt.Sprintf("Operator denied pending request for session %s", record.ID)
		if reason != "" {
			summary += ": " + reason
		}
		_ = r.syncTaskArtifacts(result, record.TaskID, artifact.Event{
			Type:        "approval.denied",
			Actor:       "operator",
			SessionID:   record.ID,
			Summary:     summary,
			StateEffect: "Session Blocked",
		}, false)
	}

	fmt.Fprintln(r.stdout, "Denied")
	fmt.Fprintf(r.stdout, "Session: %s\n", updated.ID)
	fmt.Fprintf(r.stdout, "Agent: %s\n", updated.AgentName)
	fmt.Fprintf(r.stdout, "Status: %s\n", updated.Status)
	if reason != "" {
		fmt.Fprintf(r.stdout, "Reason: %s\n", reason)
	}
	return nil
}

func (r Runner) executePauseAll(args []string) error {
	reason := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--reason":
			i++
			if i >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			reason = strings.TrimSpace(args[i])
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	result, err := r.app.Projects.Open(".")
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

	pauseMsg := "PAUSE: operator has paused all agents"
	if reason != "" {
		pauseMsg += " — " + reason
	}

	var paused []string
	for i := range sessions {
		s := sessions[i]
		// Pause any session whose agent process is alive — regardless of whether
		// a task is assigned. An idle agent watching inbox should also receive the
		// pause signal so it does not accept new work during the pause window.
		// Skip dead sessions (pane at shell) — nothing to pause there.
		switch s.Status {
		case "Working", "Idle", "WaitingHandoff", "Blocked":
			// candidate — check process below
		default:
			continue
		}
		if strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		cmd := r.app.Tmux.PaneCurrentCommand(s.TmuxPane)
		if isShellProcess(cmd) {
			continue // dead — nothing to pause
		}

		s.Status = "WaitingApproval"
		if _, err := sessionService.Save(s); err != nil {
			fmt.Fprintf(r.stderr, "warning: could not pause session %s: %v\n", s.ID, err)
			continue
		}

		_ = r.app.Tmux.SendKeys(s.TmuxPane, pauseMsg)

		if s.TaskID != "" {
			_ = r.syncTaskArtifacts(result, s.TaskID, artifact.Event{
				Type:        "operator.pause",
				Actor:       "operator",
				Summary:     pauseMsg,
				StateEffect: "session WaitingApproval",
			}, false)
		}

		paused = append(paused, s.ID)
		_ = r.bus.Emit(events.Event{
			Type: events.TaskApprovalNeeded, RepoPath: result.Project.RepoPath,
			TaskID: s.TaskID, AgentName: s.AgentName,
		})
	}

	fmt.Fprintf(r.stdout, "Paused %d session(s)\n", len(paused))
	for _, id := range paused {
		fmt.Fprintf(r.stdout, "  %s  → WaitingApproval  (resume: aom approve %s)\n", id, id)
	}
	if len(paused) == 0 {
		fmt.Fprintln(r.stdout, "No Working sessions found to pause.")
		fmt.Fprintln(r.stdout, "Note: only sessions in Working state can be paused. Mock/idle sessions are unaffected.")
	}
	return nil
}

func (r Runner) executeResumeAll(args []string) error {
	_ = args

	result, err := r.app.Projects.Open(".")
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

	var resumed []string
	for i := range sessions {
		s := sessions[i]
		if s.Status != "WaitingApproval" {
			continue
		}

		s.Status = "Idle"
		if _, err := sessionService.Save(s); err != nil {
			fmt.Fprintf(r.stderr, "warning: could not resume session %s: %v\n", s.ID, err)
			continue
		}

		if s.TaskID != "" {
			_ = r.syncTaskArtifacts(result, s.TaskID, artifact.Event{
				Type:        "operator.resume",
				Actor:       "operator",
				Summary:     "Operator resumed all paused sessions",
				StateEffect: "session Idle",
			}, false)
		}

		resumed = append(resumed, s.ID)
	}

	fmt.Fprintf(r.stdout, "Resumed %d session(s)\n", len(resumed))
	for _, id := range resumed {
		fmt.Fprintf(r.stdout, "  %s  → Idle\n", id)
	}
	if len(resumed) == 0 {
		fmt.Fprintln(r.stdout, "No WaitingApproval sessions found to resume.")
	}
	return nil
}
