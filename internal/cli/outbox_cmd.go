package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/agents-orchestrator-management-private/internal/config"
)

func (r Runner) executeOutbox(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("outbox subcommand is required: flush, list")
	}
	switch args[0] {
	case "flush":
		return r.executeOutboxFlush(args[1:])
	case "list":
		return r.executeOutboxList(args[1:])
	default:
		return fmt.Errorf("unknown outbox command %q", args[0])
	}
}

func (r Runner) executeOutboxFlush(args []string) error {
	_ = args
	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	worktreesDir := filepath.Join(repoPath, ".aom", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(r.stdout, "No worktrees found.")
			return nil
		}
		return fmt.Errorf("read worktrees dir: %w", err)
	}

	total := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		worktreeRoot := filepath.Join(worktreesDir, entry.Name())
		n, err := flushWorktreeOutbox(repoPath, worktreeRoot)
		if err != nil {
			return fmt.Errorf("flush %q: %w", entry.Name(), err)
		}
		if n > 0 {
			fmt.Fprintf(r.stdout, "Flushed %d message(s) from %s\n", n, entry.Name())
			total += n
		}
	}

	if total == 0 {
		fmt.Fprintln(r.stdout, "No pending outbox messages.")
	} else {
		fmt.Fprintf(r.stdout, "Total: %d message(s) flushed to channel/mailbox.\n", total)
	}
	return nil
}

func (r Runner) executeOutboxList(args []string) error {
	_ = args
	repoPath, err := config.FindProjectRoot(".")
	if err != nil {
		return err
	}

	worktreesDir := filepath.Join(repoPath, ".aom", "worktrees")
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(r.stdout, "No worktrees found.")
			return nil
		}
		return fmt.Errorf("read worktrees dir: %w", err)
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		worktreeRoot := filepath.Join(worktreesDir, entry.Name())
		msgs, err := parseOutboxFile(outboxFilePath(worktreeRoot))
		if err != nil || len(msgs) == 0 {
			continue
		}
		found = true
		fmt.Fprintf(r.stdout, "\n[%s]\n", entry.Name())
		for _, m := range msgs {
			preview := strings.ReplaceAll(m.message, "\n", " ")
			if len(preview) > 80 {
				preview = preview[:80] + "…"
			}
			if m.kind == "channel" {
				fmt.Fprintf(r.stdout, "  → channel | from: %s | %s\n", m.agent, preview)
			} else {
				fmt.Fprintf(r.stdout, "  → mailbox:%s | from: %s | %s\n", m.target, m.agent, preview)
			}
		}
	}
	if !found {
		fmt.Fprintln(r.stdout, "No pending outbox messages.")
	}
	return nil
}
