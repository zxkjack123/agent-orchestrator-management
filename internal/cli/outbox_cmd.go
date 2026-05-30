package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lattapon-aek/agent-orchestrator-management/internal/config"
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

	// Collect outbox roots from both task worktrees (.aom/worktrees/) and
	// agent workspaces (.aom/agents/<name>/workspace/) so messages from both
	// traditional-worktree and workspace (free-roam) agents are published.
	var outboxRoots []string
	for _, subdir := range []string{
		filepath.Join(repoPath, ".aom", "worktrees"),
		filepath.Join(repoPath, ".aom", "agents"),
	} {
		entries, rErr := os.ReadDir(subdir)
		if rErr != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if subdir == filepath.Join(repoPath, ".aom", "agents") {
				// Agent dirs contain a workspace/ subdirectory.
				ws := filepath.Join(subdir, entry.Name(), "workspace")
				if _, sErr := os.Stat(ws); sErr == nil {
					outboxRoots = append(outboxRoots, ws)
				}
			} else {
				outboxRoots = append(outboxRoots, filepath.Join(subdir, entry.Name()))
			}
		}
	}

	type flushed struct {
		agent   string
		message string
	}
	var published []flushed

	total := 0
	for _, root := range outboxRoots {
		msgs, fErr := parseOutboxFile(outboxFilePath(root))
		if fErr != nil || len(msgs) == 0 {
			continue
		}
		n, fErr := flushWorktreeOutbox(repoPath, root)
		if fErr != nil {
			return fmt.Errorf("flush %q: %w", root, fErr)
		}
		if n > 0 {
			name := filepath.Base(filepath.Dir(root))
			if filepath.Base(root) != "workspace" {
				name = filepath.Base(root)
			}
			fmt.Fprintf(r.stdout, "Flushed %d message(s) from %s\n", n, name)
			total += n
			for _, m := range msgs {
				published = append(published, flushed{agent: m.agent, message: m.message})
			}
		}
	}

	if total == 0 {
		fmt.Fprintln(r.stdout, "No pending outbox messages.")
	} else {
		fmt.Fprintf(r.stdout, "Total: %d message(s) flushed to channel/mailbox.\n", total)
		// After publishing, send real-time tmux notifications to all live sessions
		// so teammates see the messages immediately without polling channel read.
		for _, pub := range published {
			r.notifyLiveSessions(repoPath, pub.agent, pub.message)
		}
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
