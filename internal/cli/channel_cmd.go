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

	agentName := "operator"
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
	return nil
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
