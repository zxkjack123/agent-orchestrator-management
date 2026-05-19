package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/artifact"
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
	// Parse flags: session identifier (positional), --follow/-f, --diff, --interval <duration>
	var sessionID string
	var followMode bool
	var diffMode bool
	interval := 2 * time.Second

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--follow", "-f":
			followMode = true
		case "--diff":
			diffMode = true
		case "--interval":
			i++
			if i >= len(args) {
				return fmt.Errorf("--interval requires a value")
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("--interval: %w", err)
			}
			interval = d
		default:
			if strings.HasPrefix(args[i], "--") {
				return fmt.Errorf("capture: unknown flag %q", args[i])
			}
			if sessionID != "" {
				return fmt.Errorf("capture does not accept extra positional arguments in the current milestone")
			}
			sessionID = strings.TrimSpace(args[i])
		}
	}

	if sessionID == "" {
		return fmt.Errorf("session identifier is required")
	}

	sessionRecord, err := r.loadSessionByIdentifier(sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux pane binding", sessionRecord.ID)
	}

	// P4: Auto-flush outbox before capturing.
	if projResult, ferr := r.app.Projects.Open("."); ferr == nil {
		if n, ferr := flushAllOutboxes(projResult.Project.RepoPath); ferr == nil && n > 0 {
			fmt.Fprintf(r.stdout, "Auto-flushed %d outbox message(s) to channel/mailbox.\n", n)
		}
	}

	stateFile := fmt.Sprintf("/tmp/aom-capture-state-%s", sessionRecord.ID)

	if diffMode {
		prevContent, _ := readStateFile(stateFile)
		curr, err := r.app.Tmux.CapturePane(sessionRecord.TmuxPane)
		if err != nil {
			return err
		}
		newContent := newPaneLines(prevContent, curr)
		if strings.TrimSpace(newContent) != "" {
			fmt.Fprint(r.stdout, newContent)
		}
		_ = os.WriteFile(stateFile, []byte(curr), 0o600)
		return nil
	}

	if followMode {
		fmt.Fprintf(r.stdout, "Capturing %s (--follow, interval %s)\n", sessionRecord.ID, interval)
		var prev string
		for {
			curr, err := r.app.Tmux.CapturePane(sessionRecord.TmuxPane)
			if err != nil {
				return err
			}
			newContent := newPaneLines(prev, curr)
			if strings.TrimSpace(newContent) != "" {
				fmt.Fprint(r.stdout, newContent)
			}
			prev = curr
			time.Sleep(interval)
		}
	}

	output, err := r.app.Tmux.CapturePane(sessionRecord.TmuxPane)
	if err != nil {
		return err
	}

	fmt.Fprint(r.stdout, output)
	return nil
}

// readStateFile returns the content of a state file, or "" if it doesn't exist.
func readStateFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// newPaneLines returns the lines in curr that appear after the content of prev.
// Tmux pane content scrolls — new lines appear at the bottom by pushing old
// ones up. We find the last non-empty line of prev in curr and return
// everything after it.
func newPaneLines(prev, curr string) string {
	if strings.TrimSpace(prev) == "" {
		return curr
	}
	prevLines := strings.Split(strings.TrimRight(prev, "\n"), "\n")
	currLines := strings.Split(strings.TrimRight(curr, "\n"), "\n")

	// Find the last non-empty line of prev.
	lastPrevLine := ""
	for i := len(prevLines) - 1; i >= 0; i-- {
		if strings.TrimSpace(prevLines[i]) != "" {
			lastPrevLine = prevLines[i]
			break
		}
	}
	if lastPrevLine == "" {
		return curr
	}

	// Find the last occurrence of lastPrevLine in currLines.
	lastIdx := -1
	for i := len(currLines) - 1; i >= 0; i-- {
		if currLines[i] == lastPrevLine {
			lastIdx = i
			break
		}
	}
	if lastIdx < 0 || lastIdx >= len(currLines)-1 {
		return ""
	}
	return strings.Join(currLines[lastIdx+1:], "\n") + "\n"
}

func (r Runner) executeBroadcast(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message is required")
	}

	var sessionIDs []string
	var msgParts []string
	var filePath string
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
		case "--file":
			i++
			if i >= len(args) {
				return fmt.Errorf("--file requires a path")
			}
			filePath = args[i]
		default:
			msgParts = append(msgParts, args[i])
		}
	}

	var message string
	if filePath != "" {
		if len(msgParts) > 0 {
			return fmt.Errorf("--file and inline message are mutually exclusive")
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read --file %q: %w", filePath, err)
		}
		message = strings.TrimSpace(string(data))
	} else {
		message = strings.TrimSpace(strings.Join(msgParts, " "))
	}

	if message == "" {
		return fmt.Errorf("message is required (use --file <path> or pass message directly)")
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
