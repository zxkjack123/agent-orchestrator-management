package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
)

func (r Runner) executeChannelAppend(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("message is required")
	}

	// Default to AOM_AGENT_NAME (injected at spawn) so agents don't need --agent.
	// Fall back to "operator" when called outside an agent session.
	agentName := senderName()
	var msgParts []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			i++
			if i >= len(args) {
				return fmt.Errorf("--agent requires a value")
			}
			agentName = strings.TrimSpace(args[i])
		case "--message":
			i++
			if i >= len(args) {
				return fmt.Errorf("--message requires a value")
			}
			msgParts = append(msgParts, args[i])
		default:
			msgParts = append(msgParts, args[i])
		}
	}

	message := strings.TrimSpace(strings.Join(msgParts, " "))
	if message == "" {
		return fmt.Errorf("message is required")
	}

	// Use lightweight root discovery — no DB open required for a channel write.
	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	// Any sandboxed provider sets AOM_RUNTIME at launch (e.g. codex sets
	// AOM_RUNTIME=codex). When set, the agent cannot write outside the worktree,
	// so messages are staged to the local outbox for the operator to flush.
	if os.Getenv("AOM_RUNTIME") != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get cwd: %w", err)
		}
		if wtRoot := worktreeContextOf(repoPath, cwd); wtRoot != "" {
			if err := appendOutboxChannel(wtRoot, agentName, message, time.Now()); err != nil {
				return err
			}
			fmt.Fprintln(r.stdout, "Message staged to outbox (outside sandbox — operator must run: aom outbox flush)")
			fmt.Fprintf(r.stdout, "Agent: %s\n", agentName)
			fmt.Fprintf(r.stdout, "Message: %s\n", message)
			return nil
		}
	}

	if err := appendChannelMessage(repoPath, agentName, message, time.Now()); err != nil {
		return err
	}

	fmt.Fprintln(r.stdout, "Message appended to channel")
	fmt.Fprintf(r.stdout, "Agent: %s\n", agentName)
	fmt.Fprintf(r.stdout, "Message: %s\n", message)
	fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(repoPath))

	// Best-effort: notify all live sessions via tmux so teammates see the message
	// immediately without having to poll channel read.
	r.notifyLiveSessions(repoPath, agentName, message)
	return nil
}

// notifyLiveSessions sends a tmux notification to every live session in the
// project except the sender. Errors are intentionally ignored — channel writes
// succeed even if tmux delivery fails (e.g. sessions stopped between the write
// and the notification).
func (r Runner) notifyLiveSessions(repoPath, fromAgent, message string) {
	result, err := r.app.Projects.Open(repoPath)
	if err != nil {
		return
	}
	svc, sqlDB, err := r.app.OpenSessionService(result.DBPath)
	if err != nil {
		return
	}
	defer sqlDB.Close()

	all, err := svc.ListByProject(result.Project.ID)
	if err != nil {
		return
	}

	notification := fmt.Sprintf("[Team] from %s: %s", fromAgent, message)
	// Track agents already notified to avoid sending duplicates when an agent
	// has multiple sessions (workspace agents accumulate sessions over time).
	// Iterate newest-first so the active session is preferred.
	notified := make(map[string]bool)
	for i := len(all) - 1; i >= 0; i-- {
		s := all[i]
		if s.AgentName == fromAgent {
			continue // don't echo back to sender
		}
		if notified[s.AgentName] {
			continue // already delivered to this agent's newest session
		}
		if !sendableSessionStatus(s.Status) || strings.TrimSpace(s.TmuxPane) == "" {
			continue
		}
		alive, _ := r.app.Tmux.PaneExists(s.TmuxPane)
		if !alive {
			continue
		}
		_ = r.app.Tmux.SendKeys(s.TmuxPane, notification)
		notified[s.AgentName] = true
	}
}

func (r Runner) executeChannelRead(args []string) error {
	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	// Warn when agent messages are staged in outbox but not yet visible in the
	// channel. Agents inside sandboxed runtimes cannot write directly to the
	// shared channel — they stage to .agent/outbox.md instead, and the
	// operator must run "aom outbox flush" to publish them.
	if n := countPendingOutboxMessages(repoPath); n > 0 {
		fmt.Fprintf(r.stdout, "⚠  %d outbox message(s) pending (not yet visible) — run: aom outbox flush\n\n", n)
	}

	content, err := readChannelFile(repoPath)
	if err != nil {
		return err
	}

	if content == "" {
		fmt.Fprintln(r.stdout, "Channel is empty")
		fmt.Fprintf(r.stdout, "Channel: %s\n", channelFilePath(repoPath))
		return nil
	}

	fmt.Fprint(r.stdout, content)
	return nil
}
