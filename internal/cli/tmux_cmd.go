package cli

import (
	"fmt"
	"strings"

	"github.com/lattapon-aek/Agents-Orchestfator-Management/internal/artifact"
)

func (r Runner) executeAttach(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("attach does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TmuxSessionName) == "" || strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux binding", sessionRecord.ID)
	}

	fmt.Fprintf(r.stdout, "Attaching to %s (%s)\n", sessionRecord.ID, sessionRecord.TmuxPane)
	if err := r.app.Tmux.AttachPane(sessionRecord.TmuxSessionName, sessionRecord.TmuxPane); err != nil {
		return fmt.Errorf("attach requires an interactive terminal — run this command from a real terminal session, not a script or pipe\n  (underlying error: %w)", err)
	}

	if strings.TrimSpace(sessionRecord.TaskID) == "" {
		return nil
	}

	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	return r.syncTaskArtifactsWithSession(result, sessionRecord.TaskID, artifact.Event{
		Type:        "operator.intervention",
		Actor:       "operator",
		SessionID:   sessionRecord.ID,
		Summary:     fmt.Sprintf("Operator attached directly to session %s for live intervention", sessionRecord.ID),
		StateEffect: "Re-analysis required",
	}, false, sessionRecord)
}

func (r Runner) executeCapture(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("session identifier is required")
	}
	if len(args) > 1 {
		return fmt.Errorf("capture does not accept extra positional arguments in the current milestone")
	}

	sessionRecord, err := r.loadSessionByIdentifier(strings.TrimSpace(args[0]))
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux pane binding", sessionRecord.ID)
	}

	output, err := r.app.Tmux.CapturePane(sessionRecord.TmuxPane)
	if err != nil {
		return err
	}

	fmt.Fprint(r.stdout, output)
	return nil
}

func (r Runner) executeBroadcast(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message is required")
	}

	var sessionIDs []string
	var msgParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--sessions":
			i++
			if i >= len(args) {
				return fmt.Errorf("--sessions requires a value")
			}
			for _, id := range strings.Split(args[i], ",") {
				if trimmed := strings.TrimSpace(id); trimmed != "" {
					sessionIDs = append(sessionIDs, trimmed)
				}
			}
		default:
			msgParts = append(msgParts, args[i])
		}
	}

	message := strings.TrimSpace(strings.Join(msgParts, " "))
	if message == "" {
		return fmt.Errorf("message is required")
	}
	if len(sessionIDs) == 0 {
		return fmt.Errorf("--sessions is required (e.g. --sessions SESS-001,SESS-002)")
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

	fmt.Fprintf(r.stdout, "Broadcasting to %d sessions\n\n", len(sessionIDs))

	delivered := 0
	failed := 0
	for _, id := range sessionIDs {
		record, err := sessionService.Get(id)
		if err != nil || record == nil {
			fmt.Fprintf(r.stdout, "  %-30s not found\n", id)
			failed++
			continue
		}
		if !sendableSessionStatus(record.Status) || strings.TrimSpace(record.TmuxPane) == "" {
			fmt.Fprintf(r.stdout, "  %-30s no live pane (status: %s)\n", id, record.Status)
			failed++
			continue
		}
		if err := r.app.Tmux.SendKeys(record.TmuxPane, message); err != nil {
			fmt.Fprintf(r.stdout, "  %-30s send failed: %v\n", id, err)
			failed++
			continue
		}
		fmt.Fprintf(r.stdout, "  %-30s delivered (%s)\n", id, record.AgentName)
		delivered++
	}

	fmt.Fprintln(r.stdout, "")
	fmt.Fprintf(r.stdout, "Delivered: %d  Failed: %d\n", delivered, failed)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	return nil
}
