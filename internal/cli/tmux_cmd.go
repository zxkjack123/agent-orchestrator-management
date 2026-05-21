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
	// Parse flags: session identifier (positional), --follow/-f, --diff, --summary,
	// --all (capture every active session), --interval <duration>
	var sessionID string
	var followMode bool
	var diffMode bool
	var summaryMode bool
	var allMode bool
	interval := 2 * time.Second

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--follow", "-f":
			followMode = true
		case "--diff":
			diffMode = true
		case "--summary":
			summaryMode = true
		case "--all":
			allMode = true
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

	// --all: capture every session that has a live pane.
	if allMode {
		if sessionID != "" {
			return fmt.Errorf("--all and a session identifier are mutually exclusive")
		}
		return r.executeCaptureAll(followMode, summaryMode, interval)
	}

	if sessionID == "" {
		return fmt.Errorf("session identifier is required (or use --all to capture every active session)")
	}

	sessionRecord, err := r.loadSessionByIdentifier(sessionID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(sessionRecord.TmuxPane) == "" {
		return fmt.Errorf("session %q does not have a live tmux pane binding", sessionRecord.ID)
	}

	// Auto-flush outbox before capturing.
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

	if summaryMode {
		fmt.Fprint(r.stdout, captureSummary(output))
		return nil
	}

	fmt.Fprint(r.stdout, output)
	return nil
}

// captureHeader returns a fixed-width section header for one session in the
// --all output, e.g.: "══ backend-main (SESS-001) [Working] ══════════════"
func captureHeader(agentName, sessionID, status string) string {
	label := fmt.Sprintf(" %s (%s) [%s] ", agentName, sessionID, status)
	const width = 72
	const bar = "══"
	prefix := bar
	suffix := bar
	fillLen := width - len(label) - len(prefix) - len(suffix)
	if fillLen < 2 {
		fillLen = 2
	}
	return prefix + label + strings.Repeat("═", fillLen)
}

// executeCaptureAll captures the pane content of every session that has a live
// tmux pane, printing each behind a clear header so the operator can read the
// full team status in a single command.
//
// --summary:  run captureSummary on each pane output (signal-only lines).
// --follow:   loop forever, printing only new lines per session every interval.
//             Each new line is prefixed with "[agent-name] " so output from
//             multiple sessions remains readable in a single stream.
func (r Runner) executeCaptureAll(followMode, summaryMode bool, interval time.Duration) error {
	result, err := r.app.Projects.Open(".")
	if err != nil {
		return err
	}

	// Auto-flush all outboxes once before we start reading panes.
	if n, ferr := flushAllOutboxes(result.Project.RepoPath); ferr == nil && n > 0 {
		fmt.Fprintf(r.stdout, "Auto-flushed %d outbox message(s) to channel/mailbox.\n\n", n)
	}

	sessions, err := r.loadProjectSessions(result)
	if err != nil {
		return err
	}

	// Filter to sessions that have a live pane.
	type liveSession struct {
		id        string
		agentName string
		status    string
		pane      string
	}
	var live []liveSession
	for _, s := range sessions {
		if strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		alive, _ := r.app.Tmux.PaneExists(s.TmuxPane)
		if !alive {
			continue
		}
		live = append(live, liveSession{
			id:        s.ID,
			agentName: s.AgentName,
			status:    s.Status,
			pane:      s.TmuxPane,
		})
	}

	if len(live) == 0 {
		fmt.Fprintln(r.stdout, "No active sessions with live panes found.")
		return nil
	}

	// ── single-shot mode ──────────────────────────────────────────────────
	if !followMode {
		for _, s := range live {
			fmt.Fprintln(r.stdout, captureHeader(s.agentName, s.id, s.status))
			output, err := r.app.Tmux.CapturePane(s.pane)
			if err != nil {
				fmt.Fprintf(r.stdout, "(capture error: %v)\n", err)
			} else if summaryMode {
				fmt.Fprint(r.stdout, captureSummary(output))
			} else {
				fmt.Fprint(r.stdout, output)
			}
			fmt.Fprintln(r.stdout)
		}
		fmt.Fprintf(r.stdout, "─── %d session(s) captured ───\n", len(live))
		return nil
	}

	// ── follow mode ───────────────────────────────────────────────────────
	// Track last-seen content per session using the same /tmp state files as
	// the single-session --follow path, so state survives across invocations.
	prevByID := make(map[string]string, len(live))
	for _, s := range live {
		// Seed the previous state so we don't dump the full backlog on start.
		if curr, err := r.app.Tmux.CapturePane(s.pane); err == nil {
			prevByID[s.id] = curr
		}
	}

	fmt.Fprintf(r.stdout, "Following %d session(s) — interval %s — Ctrl+C to stop\n", len(live), interval)
	for _, s := range live {
		fmt.Fprintf(r.stdout, "  • %s (%s)\n", s.agentName, s.id)
	}
	fmt.Fprintln(r.stdout)

	for {
		time.Sleep(interval)
		for _, s := range live {
			curr, err := r.app.Tmux.CapturePane(s.pane)
			if err != nil {
				continue
			}
			newContent := newPaneLines(prevByID[s.id], curr)
			if strings.TrimSpace(newContent) == "" {
				prevByID[s.id] = curr
				continue
			}
			// Prefix every new line with the agent name so different sessions
			// are distinguishable when their output interleaves.
			tag := fmt.Sprintf("[%s] ", s.agentName)
			for _, line := range strings.Split(strings.TrimRight(newContent, "\n"), "\n") {
				fmt.Fprintf(r.stdout, "%s%s\n", tag, line)
			}
			prevByID[s.id] = curr
		}
	}
}

// captureSummary filters raw pane output down to lines that carry structured
// signal: AOM log events (pipe-delimited), section headers (##), key=value
// pairs, error/warning markers, and git/tool output lines. Everything else —
// raw diff fragments, ANSI escape sequences, and blank separator lines — is
// dropped so the operator can read the summary at a glance.
func captureSummary(raw string) string {
	var kept []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// AOM log event rows: | timestamp | type | actor | ... |
		if strings.HasPrefix(trimmed, "|") && strings.Count(trimmed, "|") >= 4 {
			kept = append(kept, line)
			continue
		}
		// Markdown section headers written by agents
		if strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "# ") {
			kept = append(kept, line)
			continue
		}
		// key=value status lines (e.g. status=Done, step=s-001)
		if strings.Count(trimmed, "=") >= 1 && !strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "+") {
			if idx := strings.Index(trimmed, "="); idx > 0 && idx < 30 {
				kept = append(kept, line)
				continue
			}
		}
		// Error, warning, and completion signals
		lower := strings.ToLower(trimmed)
		for _, marker := range []string{"error:", "warning:", "fatal:", "✓", "✗", "done", "completed", "failed", "checkpoint"} {
			if strings.Contains(lower, marker) {
				kept = append(kept, line)
				break
			}
		}
	}
	if len(kept) == 0 {
		return "(no structured output detected — use aom capture <session> without --summary to see raw pane)\n"
	}
	return strings.Join(kept, "\n") + "\n"
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
